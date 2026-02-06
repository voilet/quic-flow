package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/release/models"
	"github.com/voilet/quic-flow/pkg/release/variable"
)

// DAGEngine DAG 编排引擎
type DAGEngine struct {
	// 图结构
	dag *DAG

	// 执行器注册表
	executors map[models.TaskType]TaskExecutor

	// 任务实例跟踪
	instances sync.Map // map[uint]*models.TaskInstance

	// 变量管理器
	varManager *variable.Manager

	// 监控
	logger *monitoring.Logger

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// DAG 有向无环图
type DAG struct {
	mu     sync.RWMutex
	nodes  map[string]*Node       // taskID -> Node
	edges  map[string][]string     // taskID -> dependent taskIDs
	inDeg  map[string]int          // taskID -> in-degree
}

// Node DAG 节点
type Node struct {
	Task     *models.Task
	Depends  []*Node
	Status   models.DeployTaskStatus
	Output   interface{}
	Error    error
	StartAt  *time.Time
	EndAt    *time.Time
}

// DAGEngineConfig DAG 引擎配置
type DAGEngineConfig struct {
	Logger *monitoring.Logger
}

// NewDAGEngine 创建 DAG 引擎
func NewDAGEngine(varManager *variable.Manager, config *DAGEngineConfig) *DAGEngine {
	if config == nil {
		config = &DAGEngineConfig{}
	}
	if config.Logger == nil {
		config.Logger = monitoring.NewDefaultLogger()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &DAGEngine{
		dag: &DAG{
			nodes: make(map[string]*Node),
			edges: make(map[string][]string),
			inDeg: make(map[string]int),
		},
		executors:  make(map[models.TaskType]TaskExecutor),
		varManager: varManager,
		logger:     config.Logger,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// RegisterExecutor 注册任务执行器
func (e *DAGEngine) RegisterExecutor(taskType models.TaskType, executor TaskExecutor) {
	e.executors[taskType] = executor
	e.logger.Info("Task executor registered", "type", taskType)
}

// BuildDAG 构建执行 DAG
func (e *DAGEngine) BuildDAG(pipeline *models.Pipeline, stages *models.Stages) (*DAG, error) {
	dag := &DAG{
		nodes: make(map[string]*Node),
		edges: make(map[string][]string),
		inDeg: make(map[string]int),
	}

	if stages == nil || len(*stages) == 0 {
		return nil, fmt.Errorf("no stages in pipeline")
	}

	// 创建节点（所有任务）
	for _, stage := range *stages {
		for _, task := range stage.Tasks {
			dag.nodes[task.ID] = &Node{
				Task:   &task,
				Status: models.DeployTaskStatusPending,
			}
		}
	}

	// 构建边（依赖关系）
	for _, stage := range *stages {
		for _, task := range stage.Tasks {
			if len(task.DependsOn) > 0 {
				for _, depID := range task.DependsOn {
					// 添加边
					dag.edges[depID] = append(dag.edges[depID], task.ID)

					// 增加入度
					dag.inDeg[task.ID]++

					// 设置节点依赖关系
					if dag.nodes[depID] != nil {
						if dag.nodes[task.ID] == nil {
							dag.nodes[task.ID] = &Node{
								Task:   &task,
								Status: models.DeployTaskStatusPending,
							}
						}
						dag.nodes[task.ID].Depends = append(dag.nodes[task.ID].Depends, dag.nodes[depID])
					}
				}
			}
		}
	}

	// 检测环路
	if cycle := e.detectCycle(dag); len(cycle) > 0 {
		return nil, fmt.Errorf("detected cycle in DAG: %v", cycle)
	}

	e.dag = dag
	return dag, nil
}

// Execute 执行 DAG
func (e *DAGEngine) Execute(ctx context.Context, pipeline *models.Pipeline, stages *models.Stages, release *models.Release) error {
	e.logger.Info("Starting DAG execution",
		"pipeline", pipeline.ID,
		"release", release.ID)

	// 获取可执行任务（入度为 0）
	ready := e.getReadyTasks()

	// 创建执行上下文
	execCtx := &ExecutionContext{
		ReleaseID:  release.ID,
		PipelineID: pipeline.ID,
		Variables:  release.Variables,
		Logger:     e.logger,
	}

	// 执行任务
	var wg sync.WaitGroup
	taskQueue := make(chan string, len(ready))

	// 初始任务入队
	for _, taskID := range ready {
		taskQueue <- taskID
	}

	// 启动 worker
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go e.worker(ctx, execCtx, taskQueue, &wg)
	}

	// 等待所有 worker 完成
	close(taskQueue)
	wg.Wait()

	// 检查是否有失败的任务
	if e.hasFailedTasks() {
		return fmt.Errorf("pipeline execution failed")
	}

	e.logger.Info("DAG execution completed",
		"pipeline", pipeline.ID,
		"release", release.ID)

	return nil
}

// worker 任务执行 worker
func (e *DAGEngine) worker(ctx context.Context, execCtx *ExecutionContext, taskQueue chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	for taskID := range taskQueue {
		// 执行任务
		if err := e.executeTask(ctx, execCtx, taskID); err != nil {
			e.logger.Error("Task execution failed",
				"task_id", taskID,
				"error", err)
		}

		// 处理依赖该任务的其他任务
		e.processDependents(taskID, taskQueue)
	}
}

// executeTask 执行单个任务
func (e *DAGEngine) executeTask(ctx context.Context, execCtx *ExecutionContext, taskID string) error {
	e.dag.mu.Lock()
	node := e.dag.nodes[taskID]
	e.dag.mu.Unlock()

	if node == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// 检查依赖状态
	for _, dep := range node.Depends {
		if dep.Status != models.DeployTaskStatusCompleted {
			return fmt.Errorf("dependency not satisfied: %s", dep.Task.Name)
		}
	}

	// 获取执行器
	executor, ok := e.executors[node.Task.Type]
	if !ok {
		return fmt.Errorf("no executor for task type: %s", node.Task.Type)
	}

	// 更新状态
	node.Status = models.DeployTaskStatusRunning
	now := time.Now()
	node.StartAt = &now

	e.logger.Info("Executing task",
		"task", node.Task.Name,
		"type", node.Task.Type)

	// 执行任务
	result, err := executor.Execute(ctx, execCtx, node.Task)

	endTime := time.Now()
	node.EndAt = &endTime

	if err != nil {
		node.Status = models.DeployTaskStatusFailed
		node.Error = err
		return err
	}

	node.Status = models.DeployTaskStatusCompleted
	node.Output = result

	e.logger.Info("Task completed",
		"task", node.Task.Name,
		"type", node.Task.Type)

	return nil
}

// processDependents 处理依赖该任务的其他任务
func (e *DAGEngine) processDependents(taskID string, taskQueue chan string) {
	e.dag.mu.Lock()
	defer e.dag.mu.Unlock()

	// 获取依赖该任务的所有任务
	dependents := e.dag.edges[taskID]

	for _, depID := range dependents {
		// 减少入度
		e.dag.inDeg[depID]--

		// 如果入度为 0，可以执行
		if e.dag.inDeg[depID] == 0 {
			taskQueue <- depID
		}
	}
}

// getReadyTasks 获取可执行任务（入度为 0）
func (e *DAGEngine) getReadyTasks() []string {
	e.dag.mu.RLock()
	defer e.dag.mu.RUnlock()

	var ready []string
	for taskID, inDegree := range e.dag.inDeg {
		if inDegree == 0 {
			ready = append(ready, taskID)
		}
	}

	return ready
}

// hasFailedTasks 检查是否有失败的任务
func (e *DAGEngine) hasFailedTasks() bool {
	e.dag.mu.RLock()
	defer e.dag.mu.RUnlock()

	for _, node := range e.dag.nodes {
		if node.Status == models.DeployTaskStatusFailed {
			return true
		}
	}

	return false
}

// detectCycle 检测环路（使用 DFS）
func (e *DAGEngine) detectCycle(dag *DAG) []string {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	cycle := make([]string, 0)

	var dfs func(string) bool
	dfs = func(taskID string) bool {
		visited[taskID] = true
		recStack[taskID] = true

		for _, depID := range dag.edges[taskID] {
			if !visited[depID] {
				if dfs(depID) {
					cycle = append(cycle, depID)
					return true
				}
			} else if recStack[depID] {
				cycle = append(cycle, depID)
				return true
			}
		}

		recStack[taskID] = false
		return false
	}

	for taskID := range dag.nodes {
		if !visited[taskID] {
			if dfs(taskID) {
				cycle = append(cycle, taskID)
				return cycle
			}
		}
	}

	return nil
}

// GetTaskStatus 获取任务状态
func (e *DAGEngine) GetTaskStatus(taskID string) models.DeployTaskStatus {
	e.dag.mu.RLock()
	defer e.dag.mu.RUnlock()

	if node, ok := e.dag.nodes[taskID]; ok {
		return node.Status
	}

	return models.DeployTaskStatusPending
}

// GetTaskOutput 获取任务输出
func (e *DAGEngine) GetTaskOutput(taskID string) (interface{}, error) {
	e.dag.mu.RLock()
	defer e.dag.mu.RUnlock()

	if node, ok := e.dag.nodes[taskID]; ok {
		if node.Status == models.DeployTaskStatusCompleted {
			return node.Output, nil
		}
		return nil, fmt.Errorf("task not completed: %s", node.Status)
	}

	return nil, fmt.Errorf("task not found: %s", taskID)
}

// Cancel 取消执行
func (e *DAGEngine) Cancel() {
	e.cancel()
}
