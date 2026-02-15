package scheduler

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/voilet/quic-flow/pkg/task/models"
	"github.com/voilet/quic-flow/pkg/task/store"
	"github.com/voilet/quic-flow/pkg/monitoring"
)

// TaskManager 任务管理器
type TaskManager struct {
	cron          *CronScheduler
	dispatcher    *TaskDispatcher
	taskStore     store.TaskStore
	executionStore store.ExecutionStore
	configVersion int64
	logger        *monitoring.Logger
}

// NewTaskManager 创建任务管理器
func NewTaskManager(
	cron *CronScheduler,
	dispatcher *TaskDispatcher,
	taskStore store.TaskStore,
	executionStore store.ExecutionStore,
	logger *monitoring.Logger,
) *TaskManager {
	return &TaskManager{
		cron:           cron,
		dispatcher:     dispatcher,
		taskStore:      taskStore,
		executionStore: executionStore,
		logger:         logger,
	}
}

// Initialize 初始化任务管理器，加载所有启用的任务
func (m *TaskManager) Initialize(ctx context.Context) error {
	// 获取所有启用的任务
	tasks, err := m.taskStore.ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("failed to list enabled tasks: %w", err)
	}

	// 添加到调度器
	for _, task := range tasks {
		if _, err := m.cron.AddTask(task); err != nil {
			m.logger.Warn("Failed to add task to scheduler",
				"task_id", task.ID,
				"task_name", task.Name,
				"error", err)
			// 继续处理其他任务
		}
	}

	m.logger.Info("Task manager initialized", "task_count", len(tasks))
	return nil
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	Name           string
	Description    string
	ExecutorType   models.ExecutorType
	ExecutorConfig string
	CronExpr       string
	
	// 增强调度字段
	Timezone      *string     // 时区
	StartTime     *time.Time  // 调度开始时间
	EndTime       *time.Time  // 调度结束时间
	MaxExecutions int64       // 最大执行次数 (0=无限制)
	
	Timeout       int
	RetryCount    int
	RetryInterval int
	Concurrency   int
	CreatedBy     string
	GroupIDs      []int64 // 关联的分组ID列表
}

// CreateTask 创建任务
func (m *TaskManager) CreateTask(ctx context.Context, req *CreateTaskRequest) (*models.Task, error) {
	// 创建任务模型
	task := &models.Task{
		Name:           req.Name,
		Description:    req.Description,
		ExecutorType:   req.ExecutorType,
		ExecutorConfig: req.ExecutorConfig,
		CronExpr:       req.CronExpr,
		Timezone:       "Local",
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
		MaxExecutions:  req.MaxExecutions,
		Timeout:        req.Timeout,
		RetryCount:     req.RetryCount,
		RetryInterval:  req.RetryInterval,
		Concurrency:    req.Concurrency,
		Status:         models.TaskStatusEnabled,
		CreatedBy:      req.CreatedBy,
	}
	
	// 设置时区
	if req.Timezone != nil && *req.Timezone != "" {
		task.Timezone = *req.Timezone
	}

	// 保存到数据库
	if err := m.taskStore.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// 绑定分组
	for _, groupID := range req.GroupIDs {
		if err := m.taskStore.BindGroup(ctx, task.ID, groupID); err != nil {
			m.logger.Warn("Failed to bind group",
				"task_id", task.ID,
				"group_id", groupID,
				"error", err)
			// 继续绑定其他分组
		}
	}

	// 如果任务启用，添加到调度器
	if task.Status == models.TaskStatusEnabled {
		if _, err := m.cron.AddTask(task); err != nil {
			m.logger.Warn("Failed to add task to scheduler",
				"task_id", task.ID,
				"error", err)
		}
	}

	// 更新配置版本
	atomic.AddInt64(&m.configVersion, 1)

	m.logger.Info("Task created",
		"task_id", task.ID,
		"task_name", task.Name)

	return task, nil
}

// UpdateTaskRequest 更新任务请求
type UpdateTaskRequest struct {
	TaskID         int64
	Name           *string
	Description    *string
	ExecutorType   *models.ExecutorType
	ExecutorConfig *string
	CronExpr       *string
	
	// 增强调度字段
	Timezone      *string     // 时区
	StartTime     *time.Time  // 调度开始时间
	EndTime       *time.Time  // 调度结束时间
	MaxExecutions *int64      // 最大执行次数
	
	Timeout       *int
	RetryCount    *int
	RetryInterval *int
	Concurrency   *int
	Status        *models.TaskStatus
	GroupIDs      []int64 // 如果提供，将替换所有分组关联
}

// UpdateTask 更新任务
func (m *TaskManager) UpdateTask(ctx context.Context, req *UpdateTaskRequest) error {
	// 获取现有任务
	task, err := m.taskStore.GetByID(ctx, req.TaskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// 更新字段
	if req.Name != nil {
		task.Name = *req.Name
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.ExecutorType != nil {
		task.ExecutorType = *req.ExecutorType
	}
	if req.ExecutorConfig != nil {
		task.ExecutorConfig = *req.ExecutorConfig
	}
	if req.CronExpr != nil {
		task.CronExpr = *req.CronExpr
	}
	// 更新增强调度字段
	if req.Timezone != nil {
		task.Timezone = *req.Timezone
	}
	if req.StartTime != nil {
		task.StartTime = req.StartTime
	}
	if req.EndTime != nil {
		task.EndTime = req.EndTime
	}
	if req.MaxExecutions != nil {
		task.MaxExecutions = *req.MaxExecutions
	}
	if req.Timeout != nil {
		task.Timeout = *req.Timeout
	}
	if req.RetryCount != nil {
		task.RetryCount = *req.RetryCount
	}
	if req.RetryInterval != nil {
		task.RetryInterval = *req.RetryInterval
	}
	if req.Concurrency != nil {
		task.Concurrency = *req.Concurrency
	}
	if req.Status != nil {
		task.Status = *req.Status
	}

	// 更新分组关联
	if req.GroupIDs != nil {
		// 获取现有分组
		existingGroupIDs, err := m.taskStore.GetGroupIDs(ctx, task.ID)
		if err != nil {
			return fmt.Errorf("failed to get existing groups: %w", err)
		}

		// 解绑不在新列表中的分组
		for _, groupID := range existingGroupIDs {
			found := false
			for _, newGroupID := range req.GroupIDs {
				if groupID == newGroupID {
					found = true
					break
				}
			}
			if !found {
				if err := m.taskStore.UnbindGroup(ctx, task.ID, groupID); err != nil {
					m.logger.Warn("Failed to unbind group",
						"task_id", task.ID,
						"group_id", groupID,
						"error", err)
				}
			}
		}

		// 绑定新分组
		for _, groupID := range req.GroupIDs {
			found := false
			for _, existingGroupID := range existingGroupIDs {
				if groupID == existingGroupID {
					found = true
					break
				}
			}
			if !found {
				if err := m.taskStore.BindGroup(ctx, task.ID, groupID); err != nil {
					m.logger.Warn("Failed to bind group",
						"task_id", task.ID,
						"group_id", groupID,
						"error", err)
				}
			}
		}
	}

	// 保存到数据库
	if err := m.taskStore.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	// 更新调度器中的任务
	if err := m.cron.UpdateTask(task); err != nil {
		m.logger.Warn("Failed to update task in scheduler",
			"task_id", task.ID,
			"error", err)
	}

	// 更新配置版本
	atomic.AddInt64(&m.configVersion, 1)

	m.logger.Info("Task updated", "task_id", task.ID)
	return nil
}

// EnableTask 启用任务
func (m *TaskManager) EnableTask(ctx context.Context, taskID int64) error {
	status := models.TaskStatusEnabled
	return m.UpdateTask(ctx, &UpdateTaskRequest{
		TaskID: taskID,
		Status: &status,
	})
}

// DisableTask 禁用任务
func (m *TaskManager) DisableTask(ctx context.Context, taskID int64) error {
	status := models.TaskStatusDisabled
	return m.UpdateTask(ctx, &UpdateTaskRequest{
		TaskID: taskID,
		Status: &status,
	})
}

// DeleteTask 删除任务
func (m *TaskManager) DeleteTask(ctx context.Context, taskID int64) error {
	// 先从调度器移除
	if err := m.cron.RemoveTask(taskID); err != nil {
		m.logger.Warn("Failed to remove task from scheduler",
			"task_id", taskID,
			"error", err)
	}

	// 从数据库删除（软删除）
	if err := m.taskStore.Delete(ctx, taskID); err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	// 更新配置版本
	atomic.AddInt64(&m.configVersion, 1)

	m.logger.Info("Task deleted", "task_id", taskID)
	return nil
}

// TriggerTask 手动触发任务执行
func (m *TaskManager) TriggerTask(ctx context.Context, taskID int64) error {
	// 获取任务
	task, err := m.taskStore.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// 创建执行记录
	execution := &models.Execution{
		TaskID:        task.ID,
		TaskName:      task.Name,
		ExecutionType: models.ExecutionTypeManual,
		Status:        models.ExecutionStatusPending,
		CreatedAt:     time.Now(),
	}

	// 保存执行记录（这里不指定 ClientID，因为会分发到多个客户端）
	// 实际执行时，每个客户端会创建自己的执行记录
	if err := m.executionStore.Create(ctx, execution); err != nil {
		m.logger.Warn("Failed to create execution record",
			"task_id", taskID,
			"error", err)
	}

	// 分发任务
	if err := m.dispatcher.Dispatch(ctx, task); err != nil {
		return fmt.Errorf("failed to dispatch task: %w", err)
	}

	m.logger.Info("Task triggered manually", "task_id", taskID)
	return nil
}

// GetNextRunTime 获取任务下次执行时间
func (m *TaskManager) GetNextRunTime(taskID int64) (time.Time, error) {
	return m.cron.GetNextRunTime(taskID)
}

// GetConfigVersion 获取配置版本号
func (m *TaskManager) GetConfigVersion() int64 {
	return atomic.LoadInt64(&m.configVersion)
}

// GetTaskStats 获取任务统计信息
func (m *TaskManager) GetTaskStats(ctx context.Context, taskID int64) (*store.TaskStats, error) {
	return m.taskStore.GetTaskStats(ctx, taskID)
}

// ResetTaskStats 重置任务统计信息
func (m *TaskManager) ResetTaskStats(ctx context.Context, taskID int64) error {
	task, err := m.taskStore.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// 重置统计
	task.ExecutionCount = 0
	task.SuccessCount = 0
	task.FailedCount = 0
	task.LastRunTime = nil

	if err := m.taskStore.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to reset task stats: %w", err)
	}

	m.logger.Info("Task stats reset", "task_id", taskID)
	return nil
}

// UpdateNextRunTime 更新下次执行时间
func (m *TaskManager) UpdateNextRunTime(ctx context.Context, taskID int64, nextRunTime *time.Time) error {
	return m.taskStore.UpdateNextRunTime(ctx, taskID, nextRunTime)
}
