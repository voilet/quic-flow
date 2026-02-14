package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/voilet/quic-flow/pkg/release/executor"
	"github.com/voilet/quic-flow/pkg/release/models"
	"github.com/voilet/quic-flow/pkg/release/variable"

	"gorm.io/gorm"
)

// Engine 发布引擎
type Engine struct {
	db           *gorm.DB
	varManager   *variable.Manager
	scriptExec   *executor.ScriptExecutor
	remoteExec   *executor.RemoteExecutor

	// 运行中的发布
	runningReleases sync.Map

	// 事件回调
	onStatusChange func(releaseID string, status models.ReleaseStatus)
	onTargetUpdate func(releaseID, targetID string, result *models.TargetResult)
}

// NewEngine 创建发布引擎
func NewEngine(db *gorm.DB) *Engine {
	varManager := variable.NewManager(db)
	scriptExec := executor.NewScriptExecutor(varManager)

	return &Engine{
		db:         db,
		varManager: varManager,
		scriptExec: scriptExec,
	}
}

// NewEngineWithRemote 创建支持远程执行的发布引擎
func NewEngineWithRemote(db *gorm.DB, cmdSender executor.CommandSender) *Engine {
	varManager := variable.NewManager(db)
	scriptExec := executor.NewScriptExecutor(varManager)
	remoteExec := executor.NewRemoteExecutor(cmdSender, varManager)

	return &Engine{
		db:         db,
		varManager: varManager,
		scriptExec: scriptExec,
		remoteExec: remoteExec,
	}
}

// SetRemoteExecutor 设置远程执行器
func (e *Engine) SetRemoteExecutor(cmdSender executor.CommandSender) {
	e.remoteExec = executor.NewRemoteExecutor(cmdSender, e.varManager)
}

// SetStatusChangeHandler 设置状态变更回调
func (e *Engine) SetStatusChangeHandler(handler func(releaseID string, status models.ReleaseStatus)) {
	e.onStatusChange = handler
}

// SetTargetUpdateHandler 设置目标更新回调
func (e *Engine) SetTargetUpdateHandler(handler func(releaseID, targetID string, result *models.TargetResult)) {
	e.onTargetUpdate = handler
}

// CreateReleaseRequest 创建发布请求
type CreateReleaseRequest struct {
	ProjectID     string
	EnvironmentID string
	Version       string
	Operation     models.OperationType
	Variables     map[string]string
	TargetIDs     []string
	Strategy      *models.ReleaseStrategy
	ScheduledAt   *time.Time
	CreatedBy     string
}

// CreateRelease 创建发布
func (e *Engine) CreateRelease(ctx context.Context, req *CreateReleaseRequest) (*models.Release, error) {
	// 验证项目
	var project models.Project
	if err := e.db.WithContext(ctx).First(&project, "id = ?", req.ProjectID).Error; err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	// 验证环境
	var env models.Environment
	if err := e.db.WithContext(ctx).First(&env, "id = ?", req.EnvironmentID).Error; err != nil {
		return nil, fmt.Errorf("environment not found: %w", err)
	}

	// 获取目标列表
	var targets []models.Target
	if len(req.TargetIDs) > 0 {
		if err := e.db.WithContext(ctx).Where("id IN ?", req.TargetIDs).Find(&targets).Error; err != nil {
			return nil, fmt.Errorf("load targets: %w", err)
		}
	} else {
		if err := e.db.WithContext(ctx).Where("environment_id = ?", req.EnvironmentID).Find(&targets).Error; err != nil {
			return nil, fmt.Errorf("load targets: %w", err)
		}
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets found")
	}

	// 设置默认策略
	strategy := req.Strategy
	if strategy == nil {
		strategy = &models.ReleaseStrategy{
			Type:      models.StrategyTypeRolling,
			BatchSize: 1,
		}
	}

	// 设置默认操作类型
	operation := req.Operation
	if operation == "" {
		operation = models.OperationTypeDeploy
	}

	// 确定初始状态
	status := models.ReleaseStatusPending
	if req.ScheduledAt != nil {
		status = models.ReleaseStatusScheduled
	}

	// 初始化目标结果
	var results models.TargetResults
	for _, t := range targets {
		results = append(results, models.TargetResult{
			TargetID:   t.ID,
			TargetName: t.Name,
			Status:     models.TargetReleaseStatusPending,
		})
	}

	// 创建发布记录
	release := &models.Release{
		ProjectID:     req.ProjectID,
		EnvironmentID: req.EnvironmentID,
		Version:       req.Version,
		Operation:     operation,
		Status:        status,
		Strategy:      *strategy,
		Variables:     req.Variables,
		TargetIDs:     req.TargetIDs,
		ScheduledAt:   req.ScheduledAt,
		Results:       results,
		CreatedBy:     req.CreatedBy,
	}

	if err := e.db.WithContext(ctx).Create(release).Error; err != nil {
		return nil, fmt.Errorf("create release: %w", err)
	}

	// 如果需要审批
	if env.RequireApproval {
		release.Status = models.ReleaseStatusApproving
		if err := e.db.WithContext(ctx).Save(release).Error; err != nil {
			return nil, err
		}

		// 创建审批记录
		approval := &models.Approval{
			ReleaseID: release.ID,
			Status:    models.ApprovalStatusPending,
			Approvers: env.Approvers,
			ExpireAt:  time.Now().Add(time.Hour),
		}
		if err := e.db.WithContext(ctx).Create(approval).Error; err != nil {
			return nil, fmt.Errorf("create approval: %w", err)
		}
	}

	return release, nil
}

// StartRelease 开始发布
func (e *Engine) StartRelease(ctx context.Context, releaseID string) error {
	var release models.Release
	if err := e.db.WithContext(ctx).First(&release, "id = ?", releaseID).Error; err != nil {
		return fmt.Errorf("release not found: %w", err)
	}

	// 检查状态
	if release.Status != models.ReleaseStatusPending && release.Status != models.ReleaseStatusApproving {
		return fmt.Errorf("release status is %s, cannot start", release.Status)
	}

	// 如果是待审批状态，需要检查审批
	if release.Status == models.ReleaseStatusApproving {
		var approval models.Approval
		if err := e.db.WithContext(ctx).First(&approval, "release_id = ?", releaseID).Error; err != nil {
			return fmt.Errorf("approval not found: %w", err)
		}
		if approval.Status != models.ApprovalStatusApproved {
			return fmt.Errorf("release not approved")
		}
	}

	// 更新状态为运行中
	now := time.Now()
	release.Status = models.ReleaseStatusRunning
	release.StartedAt = &now
	if err := e.db.WithContext(ctx).Save(&release).Error; err != nil {
		return err
	}

	if e.onStatusChange != nil {
		e.onStatusChange(releaseID, models.ReleaseStatusRunning)
	}

	// 异步执行发布
	go e.executeRelease(context.Background(), &release)

	return nil
}

// executeRelease 执行发布
func (e *Engine) executeRelease(ctx context.Context, release *models.Release) {
	// 标记为运行中
	e.runningReleases.Store(release.ID, release)
	defer e.runningReleases.Delete(release.ID)

	// 加载相关数据
	var project models.Project
	if err := e.db.WithContext(ctx).First(&project, "id = ?", release.ProjectID).Error; err != nil {
		e.failRelease(ctx, release, fmt.Sprintf("load project: %v", err))
		return
	}

	var env models.Environment
	if err := e.db.WithContext(ctx).First(&env, "id = ?", release.EnvironmentID).Error; err != nil {
		e.failRelease(ctx, release, fmt.Sprintf("load environment: %v", err))
		return
	}

	var targets []models.Target
	if len(release.TargetIDs) > 0 {
		e.db.WithContext(ctx).Where("id IN ?", release.TargetIDs).Find(&targets)
	} else {
		e.db.WithContext(ctx).Where("environment_id = ?", release.EnvironmentID).Find(&targets)
	}

	// 根据策略执行
	switch release.Strategy.Type {
	case models.StrategyTypeRolling:
		e.executeRolling(ctx, release, &project, &env, targets)
	case models.StrategyTypeCanary:
		e.executeCanary(ctx, release, &project, &env, targets)
	case models.StrategyTypeBlueGreen:
		e.executeBlueGreen(ctx, release, &project, &env, targets)
	default:
		e.executeRolling(ctx, release, &project, &env, targets)
	}
}

// executeRolling 滚动发布
func (e *Engine) executeRolling(
	ctx context.Context,
	release *models.Release,
	project *models.Project,
	env *models.Environment,
	targets []models.Target,
) {
	batchSize := release.Strategy.BatchSize
	if batchSize <= 0 {
		batchSize = 1
	}

	// 按批次执行
	for i := 0; i < len(targets); i += batchSize {
		end := i + batchSize
		if end > len(targets) {
			end = len(targets)
		}

		batch := targets[i:end]

		// 并行执行批次内的目标
		var wg sync.WaitGroup
		for _, target := range batch {
			wg.Add(1)
			go func(t models.Target) {
				defer wg.Done()
				e.executeTarget(ctx, release, project, env, &t)
			}(target)
		}
		wg.Wait()

		// 检查是否有失败
		if e.hasFailedTargets(release) && release.RollbackConfig != nil && release.RollbackConfig.AutoRollback {
			e.failRelease(ctx, release, "auto rollback triggered due to failed targets")
			return
		}

		// 批次间隔
		if release.Strategy.BatchInterval > 0 && end < len(targets) {
			time.Sleep(time.Duration(release.Strategy.BatchInterval) * time.Second)
		}
	}

	// 完成发布
	e.completeRelease(ctx, release)
}

// executeCanary 金丝雀发布
func (e *Engine) executeCanary(
	ctx context.Context,
	release *models.Release,
	project *models.Project,
	env *models.Environment,
	targets []models.Target,
) {
	// 分离金丝雀目标和正常目标
	var canaryTargets, normalTargets []models.Target

	if len(release.Strategy.CanaryTargets) > 0 {
		// 使用指定的金丝雀目标
		canaryMap := make(map[string]bool)
		for _, id := range release.Strategy.CanaryTargets {
			canaryMap[id] = true
		}
		for _, t := range targets {
			if canaryMap[t.ID] {
				canaryTargets = append(canaryTargets, t)
			} else {
				normalTargets = append(normalTargets, t)
			}
		}
	} else if release.Strategy.CanaryPercent > 0 {
		// 按比例选择金丝雀目标
		canaryCount := len(targets) * release.Strategy.CanaryPercent / 100
		if canaryCount < 1 {
			canaryCount = 1
		}
		canaryTargets = targets[:canaryCount]
		normalTargets = targets[canaryCount:]
	} else {
		// 默认选择第一个目标作为金丝雀
		canaryTargets = targets[:1]
		normalTargets = targets[1:]
	}

	// 执行金丝雀目标
	for _, target := range canaryTargets {
		e.executeTarget(ctx, release, project, env, &target)
	}

	// 检查金丝雀是否成功
	if e.hasFailedTargets(release) {
		e.failRelease(ctx, release, "canary deployment failed")
		return
	}

	// 验证期
	if release.Strategy.VerifyDuration > 0 {
		time.Sleep(time.Duration(release.Strategy.VerifyDuration) * time.Second)
	}

	// 如果不自动全量发布，暂停等待手动确认
	if !release.Strategy.AutoPromote {
		release.Status = models.ReleaseStatusPaused
		e.db.WithContext(ctx).Save(release)
		if e.onStatusChange != nil {
			e.onStatusChange(release.ID, models.ReleaseStatusPaused)
		}
		return
	}

	// 执行剩余目标
	for _, target := range normalTargets {
		e.executeTarget(ctx, release, project, env, &target)
	}

	e.completeRelease(ctx, release)
}

// executeBlueGreen 蓝绿发布
func (e *Engine) executeBlueGreen(
	ctx context.Context,
	release *models.Release,
	project *models.Project,
	env *models.Environment,
	targets []models.Target,
) {
	// 蓝绿发布：先部署到所有目标，然后切换流量
	var wg sync.WaitGroup
	for _, target := range targets {
		wg.Add(1)
		go func(t models.Target) {
			defer wg.Done()
			e.executeTarget(ctx, release, project, env, &t)
		}(target)
	}
	wg.Wait()

	// 检查是否全部成功
	if e.hasFailedTargets(release) {
		e.failRelease(ctx, release, "blue-green deployment failed")
		return
	}

	// TODO: 切换流量逻辑

	e.completeRelease(ctx, release)
}

// executeTarget 执行单个目标
func (e *Engine) executeTarget(
	ctx context.Context,
	release *models.Release,
	project *models.Project,
	env *models.Environment,
	target *models.Target,
) {
	now := time.Now()
	result := e.findTargetResult(release, target.ID)
	if result == nil {
		return
	}

	result.Status = models.TargetReleaseStatusRunning
	result.StartedAt = &now
	e.updateTargetResult(ctx, release, result)

	// 构建变量上下文
	varCtx := &variable.Context{
		ReleaseID:      release.ID,
		ReleaseVersion: release.Version,
		ReleaseEnv:     env.Name,
		ReleaseUser:    release.CreatedBy,
		ReleaseTime:    *release.StartedAt,
		TargetID:       target.ID,
		TargetName:     target.Name,
		TargetClientID: target.ClientID,
		AppDir:         target.Config.WorkDir,
		Custom:         release.Variables,
	}

	// 根据项目类型执行
	var err error
	switch project.Type {
	case models.DeployTypeScript:
		err = e.executeScriptDeploy(ctx, release, project, target, varCtx, result)
	case models.DeployTypeContainer:
		err = e.executeContainerDeploy(ctx, release, project, varCtx, result)
	case models.DeployTypeKubernetes:
		err = e.executeK8sDeploy(ctx, release, project, varCtx, result)
	case models.DeployTypeGitPull:
		err = e.executeGitPullDeploy(ctx, release, project, varCtx, result)
	default:
		err = fmt.Errorf("unsupported deploy type: %s", project.Type)
	}

	finishedAt := time.Now()
	result.FinishedAt = &finishedAt

	if err != nil {
		result.Status = models.TargetReleaseStatusFailed
		result.Error = err.Error()
	} else {
		result.Status = models.TargetReleaseStatusSuccess
	}

	e.updateTargetResult(ctx, release, result)
}

// executeScriptDeploy 执行脚本部署
func (e *Engine) executeScriptDeploy(
	ctx context.Context,
	release *models.Release,
	project *models.Project,
	target *models.Target,
	varCtx *variable.Context,
	result *models.TargetResult,
) error {
	if project.ScriptConfig == nil {
		return fmt.Errorf("script config is nil")
	}

	// 如果有远程执行器且目标有 ClientID，使用远程执行
	if e.remoteExec != nil && target.ClientID != "" {
		return e.executeRemoteScriptDeploy(ctx, release, project, target, varCtx, result)
	}

	// 本地执行（用于测试或本地目标）
	return e.executeLocalScriptDeploy(ctx, release, project, varCtx, result)
}

// executeRemoteScriptDeploy 远程执行脚本部署
func (e *Engine) executeRemoteScriptDeploy(
	ctx context.Context,
	release *models.Release,
	project *models.Project,
	target *models.Target,
	varCtx *variable.Context,
	result *models.TargetResult,
) error {
	// 确定实际操作类型
	operation, err := e.remoteExec.DetermineOperation(ctx, target.ClientID, project.ScriptConfig, varCtx, release.Operation)
	if err != nil {
		return err
	}

	// 远程执行
	execResult, err := e.remoteExec.Execute(ctx, &executor.RemoteExecuteRequest{
		ReleaseID:  release.ID,
		TargetID:   target.ID,
		ClientID:   target.ClientID,
		Operation:  operation,
		Version:    release.Version,
		Config:     project.ScriptConfig,
		VarContext: varCtx,
	})

	if err != nil {
		return err
	}

	if !execResult.Success {
		return fmt.Errorf("remote script failed: %s", execResult.Error)
	}

	return nil
}

// executeLocalScriptDeploy 本地执行脚本部署
func (e *Engine) executeLocalScriptDeploy(
	ctx context.Context,
	release *models.Release,
	project *models.Project,
	varCtx *variable.Context,
	result *models.TargetResult,
) error {
	// 确定实际操作类型
	operation, err := e.scriptExec.DetermineOperation(ctx, project.ScriptConfig, varCtx, release.Operation)
	if err != nil {
		return err
	}

	// 执行脚本
	execResult, err := e.scriptExec.Execute(ctx, &executor.ExecuteRequest{
		Operation:  operation,
		Config:     project.ScriptConfig,
		VarContext: varCtx,
		OnOutput: func(line string) {
			// TODO: 记录日志
		},
	})

	if err != nil {
		return err
	}

	if !execResult.Success {
		return fmt.Errorf("script failed: %s", execResult.Error)
	}

	return nil
}

// executeContainerDeploy 执行容器部署
func (e *Engine) executeContainerDeploy(
	ctx context.Context,
	release *models.Release,
	project *models.Project,
	varCtx *variable.Context,
	result *models.TargetResult,
) error {
	if project.ContainerConfig == nil {
		return fmt.Errorf("container config is nil")
	}

	config := project.ContainerConfig

	// 如果没有远程执行器，无法执行容器部署
	if e.remoteExec == nil {
		return fmt.Errorf("remote executor not configured, cannot execute container deploy")
	}

	// 获取目标信息
	var target models.Target
	if err := e.db.WithContext(ctx).First(&target, "id = ?", result.TargetID).Error; err != nil {
		return fmt.Errorf("target not found: %w", err)
	}

	if target.ClientID == "" {
		return fmt.Errorf("target has no client_id")
	}

	// 解析变量
	image, err := e.varManager.Resolve(ctx, config.Image, varCtx)
	if err != nil {
		return fmt.Errorf("resolve image: %w", err)
	}

	containerName, err := e.varManager.Resolve(ctx, config.ContainerName, varCtx)
	if err != nil {
		return fmt.Errorf("resolve container name: %w", err)
	}

	// 解析环境变量
	env := make(map[string]string)
	for k, v := range config.Environment {
		resolved, err := e.varManager.Resolve(ctx, v, varCtx)
		if err != nil {
			return fmt.Errorf("resolve env %s: %w", k, err)
		}
		env[k] = resolved
	}

	// 执行容器部署
	execResult, err := e.remoteExec.ExecuteContainerDeploy(ctx, &executor.ContainerDeployRequest{
		ReleaseID:     release.ID,
		TargetID:      result.TargetID,
		ClientID:      target.ClientID,
		Operation:     release.Operation,
		Version:       release.Version,
		Config:        config,
		Image:         image,
		ContainerName: containerName,
		Environment:   env,
		VarContext:    varCtx,
	})

	if err != nil {
		return err
	}

	if !execResult.Success {
		return fmt.Errorf("container deploy failed: %s", execResult.Error)
	}

	return nil
}

// executeK8sDeploy 执行 K8s 部署
func (e *Engine) executeK8sDeploy(
	ctx context.Context,
	release *models.Release,
	project *models.Project,
	varCtx *variable.Context,
	result *models.TargetResult,
) error {
	if project.KubernetesConfig == nil {
		return fmt.Errorf("kubernetes config is nil")
	}

	config := project.KubernetesConfig

	// 如果没有远程执行器，无法执行 K8s 部署
	if e.remoteExec == nil {
		return fmt.Errorf("remote executor not configured, cannot execute k8s deploy")
	}

	// 获取目标信息
	var target models.Target
	if err := e.db.WithContext(ctx).First(&target, "id = ?", result.TargetID).Error; err != nil {
		return fmt.Errorf("target not found: %w", err)
	}

	if target.ClientID == "" {
		return fmt.Errorf("target has no client_id")
	}

	// 解析镜像地址
	image := config.Image
	if image != "" {
		var err error
		image, err = e.varManager.Resolve(ctx, image, varCtx)
		if err != nil {
			return fmt.Errorf("resolve image: %w", err)
		}
	}

	// 解析 YAML
	yaml := config.YAML
	if yaml == "" && config.YAMLTemplate != "" {
		yaml = config.YAMLTemplate
	}
	if yaml != "" {
		var err error
		yaml, err = e.varManager.Resolve(ctx, yaml, varCtx)
		if err != nil {
			return fmt.Errorf("resolve yaml: %w", err)
		}
	}

	// 解析环境变量
	env := make(map[string]string)
	for k, v := range config.Environment {
		resolved, err := e.varManager.Resolve(ctx, v, varCtx)
		if err != nil {
			return fmt.Errorf("resolve env %s: %w", k, err)
		}
		env[k] = resolved
	}

	// 执行 K8s 部署
	execResult, err := e.remoteExec.ExecuteK8sDeploy(ctx, &executor.K8sDeployRequest{
		ReleaseID:   release.ID,
		TargetID:    result.TargetID,
		ClientID:    target.ClientID,
		Operation:   release.Operation,
		Version:     release.Version,
		Config:      config,
		Image:       image,
		YAML:        yaml,
		Environment: env,
		VarContext:  varCtx,
	})

	if err != nil {
		return err
	}

	if !execResult.Success {
		return fmt.Errorf("k8s deploy failed: %s", execResult.Error)
	}

	return nil
}

// executeGitPullDeploy 执行 Git 拉取部署
func (e *Engine) executeGitPullDeploy(
	ctx context.Context,
	release *models.Release,
	project *models.Project,
	varCtx *variable.Context,
	result *models.TargetResult,
) error {
	if project.GitPullConfig == nil {
		return fmt.Errorf("gitpull config is nil")
	}

	config := project.GitPullConfig

	// 如果没有远程执行器，无法执行 Git 部署
	if e.remoteExec == nil {
		return fmt.Errorf("remote executor not configured, cannot execute git pull deploy")
	}

	// 获取目标信息
	var target models.Target
	if err := e.db.WithContext(ctx).First(&target, "id = ?", result.TargetID).Error; err != nil {
		return fmt.Errorf("target not found: %w", err)
	}

	if target.ClientID == "" {
		return fmt.Errorf("target has no client_id")
	}

	// 解析变量
	repoURL, err := e.varManager.Resolve(ctx, config.RepoURL, varCtx)
	if err != nil {
		return fmt.Errorf("resolve repo url: %w", err)
	}

	workDir, err := e.varManager.Resolve(ctx, config.WorkDir, varCtx)
	if err != nil {
		return fmt.Errorf("resolve work dir: %w", err)
	}

	branch := config.Branch
	if branch != "" {
		branch, err = e.varManager.Resolve(ctx, branch, varCtx)
		if err != nil {
			return fmt.Errorf("resolve branch: %w", err)
		}
	}

	// 解析部署前脚本
	preScript := ""
	if config.PreScript != "" {
		preScript, err = e.varManager.Resolve(ctx, config.PreScript, varCtx)
		if err != nil {
			return fmt.Errorf("resolve pre script: %w", err)
		}
	}

	// 解析部署后脚本
	postScript := ""
	if config.PostScript != "" {
		postScript, err = e.varManager.Resolve(ctx, config.PostScript, varCtx)
		if err != nil {
			return fmt.Errorf("resolve post script: %w", err)
		}
	}

	// 解析环境变量
	env := make(map[string]string)
	for k, v := range config.Environment {
		resolved, err := e.varManager.Resolve(ctx, v, varCtx)
		if err != nil {
			return fmt.Errorf("resolve env %s: %w", k, err)
		}
		env[k] = resolved
	}

	// 执行 Git 部署
	execResult, err := e.remoteExec.ExecuteGitPullDeploy(ctx, &executor.GitPullDeployRequest{
		ReleaseID:   release.ID,
		TargetID:    result.TargetID,
		ClientID:    target.ClientID,
		Operation:   release.Operation,
		Version:     release.Version,
		Config:      config,
		RepoURL:     repoURL,
		Branch:      branch,
		WorkDir:     workDir,
		PreScript:   preScript,
		PostScript:  postScript,
		Environment: env,
		VarContext:  varCtx,
	})

	if err != nil {
		return err
	}

	if !execResult.Success {
		return fmt.Errorf("git pull deploy failed: %s", execResult.Error)
	}

	return nil
}

// findTargetResult 查找目标结果
func (e *Engine) findTargetResult(release *models.Release, targetID string) *models.TargetResult {
	for i := range release.Results {
		if release.Results[i].TargetID == targetID {
			return &release.Results[i]
		}
	}
	return nil
}

// updateTargetResult 更新目标结果
func (e *Engine) updateTargetResult(ctx context.Context, release *models.Release, result *models.TargetResult) {
	e.db.WithContext(ctx).Save(release)
	if e.onTargetUpdate != nil {
		e.onTargetUpdate(release.ID, result.TargetID, result)
	}
}

// hasFailedTargets 检查是否有失败的目标
func (e *Engine) hasFailedTargets(release *models.Release) bool {
	for _, r := range release.Results {
		if r.Status == models.TargetReleaseStatusFailed {
			return true
		}
	}
	return false
}

// failRelease 标记发布失败
func (e *Engine) failRelease(ctx context.Context, release *models.Release, reason string) {
	now := time.Now()
	release.Status = models.ReleaseStatusFailed
	release.FinishedAt = &now
	e.db.WithContext(ctx).Save(release)
	if e.onStatusChange != nil {
		e.onStatusChange(release.ID, models.ReleaseStatusFailed)
	}
}

// completeRelease 完成发布
func (e *Engine) completeRelease(ctx context.Context, release *models.Release) {
	now := time.Now()
	release.Status = models.ReleaseStatusSuccess
	release.FinishedAt = &now
	e.db.WithContext(ctx).Save(release)
	if e.onStatusChange != nil {
		e.onStatusChange(release.ID, models.ReleaseStatusSuccess)
	}
}

// CancelRelease 取消发布
func (e *Engine) CancelRelease(ctx context.Context, releaseID string) error {
	var release models.Release
	if err := e.db.WithContext(ctx).First(&release, "id = ?", releaseID).Error; err != nil {
		return err
	}

	if release.Status != models.ReleaseStatusRunning && release.Status != models.ReleaseStatusPaused {
		return fmt.Errorf("release status is %s, cannot cancel", release.Status)
	}

	now := time.Now()
	release.Status = models.ReleaseStatusCancelled
	release.FinishedAt = &now
	if err := e.db.WithContext(ctx).Save(&release).Error; err != nil {
		return err
	}

	if e.onStatusChange != nil {
		e.onStatusChange(releaseID, models.ReleaseStatusCancelled)
	}

	return nil
}

// PromoteCanary 金丝雀全量发布
func (e *Engine) PromoteCanary(ctx context.Context, releaseID string) error {
	var release models.Release
	if err := e.db.WithContext(ctx).First(&release, "id = ?", releaseID).Error; err != nil {
		return err
	}

	if release.Status != models.ReleaseStatusPaused {
		return fmt.Errorf("release status is %s, cannot promote", release.Status)
	}

	release.Status = models.ReleaseStatusRunning
	if err := e.db.WithContext(ctx).Save(&release).Error; err != nil {
		return err
	}

	// 继续执行剩余目标
	go e.executeRelease(context.Background(), &release)

	return nil
}

// GetRelease 获取发布信息
func (e *Engine) GetRelease(ctx context.Context, releaseID string) (*models.Release, error) {
	var release models.Release
	if err := e.db.WithContext(ctx).First(&release, "id = ?", releaseID).Error; err != nil {
		return nil, err
	}
	return &release, nil
}

// ListReleases 列出发布
func (e *Engine) ListReleases(ctx context.Context, projectID string, limit, offset int) ([]models.Release, int64, error) {
	var releases []models.Release
	var total int64

	query := e.db.WithContext(ctx).Model(&models.Release{})
	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&releases).Error; err != nil {
		return nil, 0, err
	}

	return releases, total, nil
}

// ExecuteRemote 直接执行远程命令（用于部署任务）
func (e *Engine) ExecuteRemote(clientID, script, workDir string) (string, error) {
	if e.remoteExec == nil {
		return "", fmt.Errorf("remote executor not configured")
	}

	ctx := context.Background()
	result, err := e.remoteExec.ExecuteScript(ctx, clientID, script, workDir)
	if err != nil {
		return "", err
	}

	if !result.Success {
		return result.Output, fmt.Errorf("%s", result.Error)
	}

	return result.Output, nil
}

// ExecuteContainerDeployTask 执行容器部署任务（用于 DeployTask）
func (e *Engine) ExecuteContainerDeployTask(clientID string, operation models.OperationType, version string, config *models.ContainerDeployConfig, containerImage string) (string, error) {
	if e.remoteExec == nil {
		return "", fmt.Errorf("remote executor not configured")
	}

	ctx := context.Background()

	// 确定镜像地址
	image := containerImage
	if image == "" && config != nil {
		image = config.Image
	}
	if image == "" {
		return "", fmt.Errorf("container image is required")
	}

	// 确定容器名称
	containerName := ""
	if config != nil {
		containerName = config.ContainerName
	}
	if containerName == "" {
		containerName = "app"
	}

	result, err := e.remoteExec.ExecuteContainerDeploy(ctx, &executor.ContainerDeployRequest{
		ClientID:      clientID,
		Operation:     operation,
		Version:       version,
		Config:        config,
		Image:         image,
		ContainerName: containerName,
	})

	if err != nil {
		return "", err
	}

	if !result.Success {
		return result.Output, fmt.Errorf("%s", result.Error)
	}

	return result.Output, nil
}

// ExecuteContainerDeployWithMergedConfig 使用合并后的最终配置执行容器部署
// 用于支持三层配置合并（项目 + 版本 + 任务覆盖）
func (e *Engine) ExecuteContainerDeployWithMergedConfig(
	clientID string,
	operation models.OperationType,
	version string,
	finalConfig *models.FinalContainerConfig,
) (string, error) {
	if e.remoteExec == nil {
		return "", fmt.Errorf("remote executor not configured")
	}

	if finalConfig == nil {
		return "", fmt.Errorf("final config is required")
	}

	if finalConfig.Image == "" {
		return "", fmt.Errorf("container image is required")
	}

	containerName := finalConfig.ContainerName
	if containerName == "" {
		containerName = "app"
	}

	ctx := context.Background()

	// 从 FinalContainerConfig 创建临时的 ContainerDeployConfig 用于远程执行
	// 这保持了与现有执行器的兼容性
	tempConfig := &models.ContainerDeployConfig{
		Image:           finalConfig.Image,
		Registry:        finalConfig.Registry,
		RegistryUser:    finalConfig.RegistryUser,
		RegistryPass:    finalConfig.RegistryPass,
		ImagePullPolicy: finalConfig.ImagePullPolicy,
		ContainerName:   finalConfig.ContainerName,
		Hostname:        finalConfig.Hostname,
		User:            finalConfig.User,
		WorkingDir:      finalConfig.WorkingDir,
		Environment:     finalConfig.Environment,
		Labels:          finalConfig.Labels,
		Command:         finalConfig.Command,
		Entrypoint:      finalConfig.Entrypoint,
		Ports:           finalConfig.Ports,
		NetworkMode:     finalConfig.NetworkMode,
		Networks:        finalConfig.Networks,
		DNS:             finalConfig.DNS,
		ExtraHosts:      finalConfig.ExtraHosts,
		Volumes:         finalConfig.Volumes,
		TmpfsMounts:     finalConfig.TmpfsMounts,
		Privileged:      finalConfig.Privileged,
		CapAdd:          finalConfig.CapAdd,
		CapDrop:         finalConfig.CapDrop,
		SecurityOpt:     finalConfig.SecurityOpt,
		ReadOnlyRootfs:  finalConfig.ReadOnlyRootfs,
		Devices:         finalConfig.Devices,
		GPUs:            finalConfig.GPUs,
		MemoryLimit:     finalConfig.MemoryLimit,
		MemoryReserve:   finalConfig.MemoryReserve,
		CPULimit:        finalConfig.CPULimit,
		CPUShares:       finalConfig.CPUShares,
		Runtime:         finalConfig.Runtime,
		Init:            finalConfig.Init,
		StopSignal:      finalConfig.StopSignal,
		Sysctls:         finalConfig.Sysctls,
		LogDriver:       finalConfig.LogDriver,
		LogOpts:         finalConfig.LogOpts,
		HealthCheck:     finalConfig.HealthCheck,
		RestartPolicy:   finalConfig.RestartPolicy,
		StopTimeout:     finalConfig.StopTimeout,
		RemoveOld:       finalConfig.RemoveOld,
		PullBeforeStop:  finalConfig.PullBeforeStop,
		AutoRemove:      finalConfig.AutoRemove,
	}

	result, err := e.remoteExec.ExecuteContainerDeploy(ctx, &executor.ContainerDeployRequest{
		ClientID:      clientID,
		Operation:     operation,
		Version:       version,
		Config:        tempConfig,
		Image:         finalConfig.Image,
		ContainerName: containerName,
		Environment:   finalConfig.Environment,
	})

	if err != nil {
		return "", err
	}

	if !result.Success {
		return result.Output, fmt.Errorf("%s", result.Error)
	}

	return result.Output, nil
}

// ExecuteK8sDeployWithMergedConfig 使用合并后的最终配置执行 K8s 部署
// 用于支持三层配置合并（项目 + 版本 + 任务覆盖）
func (e *Engine) ExecuteK8sDeployWithMergedConfig(
	clientID string,
	operation models.OperationType,
	version string,
	finalConfig *models.FinalK8sConfig,
) (string, error) {
	if e.remoteExec == nil {
		return "", fmt.Errorf("remote executor not configured")
	}

	if finalConfig == nil {
		return "", fmt.Errorf("final config is required")
	}

	if finalConfig.Image == "" && finalConfig.YAML == "" && finalConfig.YAMLTemplate == "" {
		return "", fmt.Errorf("image or yaml is required for k8s deployment")
	}

	ctx := context.Background()

	// 从 FinalK8sConfig 创建临时的 KubernetesDeployConfig 用于远程执行
	tempConfig := &models.KubernetesDeployConfig{
		KubeConfig:      finalConfig.KubeConfig,
		KubeContext:     finalConfig.KubeContext,
		Namespace:       finalConfig.Namespace,
		ResourceType:    finalConfig.ResourceType,
		ResourceName:    finalConfig.ResourceName,
		ContainerName:   finalConfig.ContainerName,
		Image:           finalConfig.Image,
		Registry:        finalConfig.Registry,
		ImagePullPolicy: finalConfig.ImagePullPolicy,
		ImagePullSecret: finalConfig.ImagePullSecret,
		Replicas:        finalConfig.Replicas,
		AutoScaling:     finalConfig.AutoScaling,
		CPURequest:      finalConfig.CPURequest,
		CPULimit:        finalConfig.CPULimit,
		MemoryRequest:   finalConfig.MemoryRequest,
		MemoryLimit:     finalConfig.MemoryLimit,
		Environment:     finalConfig.Environment,
		ServiceType:     finalConfig.ServiceType,
		ServicePorts:    finalConfig.ServicePorts,
		UpdateStrategy:  finalConfig.UpdateStrategy,
		MaxUnavailable:  finalConfig.MaxUnavailable,
		MaxSurge:        finalConfig.MaxSurge,
		MinReadySeconds: finalConfig.MinReadySeconds,
		DeployTimeout:   finalConfig.DeployTimeout,
		RolloutTimeout:  finalConfig.RolloutTimeout,
		YAML:            finalConfig.YAML,
		YAMLTemplate:    finalConfig.YAMLTemplate,
	}

	result, err := e.remoteExec.ExecuteK8sDeploy(ctx, &executor.K8sDeployRequest{
		ClientID:    clientID,
		Operation:   operation,
		Version:     version,
		Config:      tempConfig,
		Image:       finalConfig.Image,
		YAML:        finalConfig.YAML,
		Environment: finalConfig.Environment,
	})

	if err != nil {
		return "", err
	}

	if !result.Success {
		return result.Output, fmt.Errorf("%s", result.Error)
	}

	return result.Output, nil
}

// ExecuteGitPullDeployTask 执行 Git 拉取部署任务（用于 DeployTask）
// 合并项目配置和版本配置
func (e *Engine) ExecuteGitPullDeployTask(
	clientID string,
	operation models.OperationType,
	version string,
	projectConfig *models.GitPullDeployConfig,
	versionWorkDir string,
	gitRef string,
	gitRefType string,
) (string, error) {
	if e.remoteExec == nil {
		return "", fmt.Errorf("remote executor not configured")
	}

	if projectConfig == nil {
		return "", fmt.Errorf("git pull config is required")
	}

	ctx := context.Background()

	// 合并工作目录：
	// 1. 项目配置优先（GitPullConfig 的 WorkDir 是权威配置）
	// 2. 版本配置只在非默认值且项目配置为空时使用
	// 3. 最后使用默认值
	workDir := projectConfig.WorkDir
	if workDir == "" {
		// 项目没配置，使用版本配置（但忽略默认值 /opt/app）
		if versionWorkDir != "" && versionWorkDir != "/opt/app" {
			workDir = versionWorkDir
		}
	}
	if workDir == "" {
		workDir = "/opt/app"
	}

	// 确定 Git 引用
	branch := projectConfig.Branch
	tag := projectConfig.Tag
	commit := projectConfig.Commit

	// 版本的 GitRef 覆盖项目配置
	if gitRef != "" {
		switch gitRefType {
		case "tag":
			tag = gitRef
			branch = ""
			commit = ""
		case "branch":
			branch = gitRef
			tag = ""
			commit = ""
		case "commit":
			commit = gitRef
			tag = ""
		}
	}

	// 构建配置副本（防止修改原始配置）
	config := &models.GitPullDeployConfig{
		RepoURL:       projectConfig.RepoURL,
		Branch:        branch,
		Tag:           tag,
		Commit:        commit,
		Depth:         projectConfig.Depth,
		Submodules:    projectConfig.Submodules,
		AuthType:      projectConfig.AuthType,
		SSHKey:        projectConfig.SSHKey,
		Token:         projectConfig.Token,
		Username:      projectConfig.Username,
		Password:      projectConfig.Password,
		WorkDir:       workDir,
		CleanBefore:   projectConfig.CleanBefore,
		BackupBefore:  projectConfig.BackupBefore,
		BackupDir:     projectConfig.BackupDir,
		BackupKeep:    projectConfig.BackupKeep,
		PreScript:     projectConfig.PreScript,
		PostScript:    projectConfig.PostScript,
		Environment:   projectConfig.Environment,
		Interpreter:   projectConfig.Interpreter,
		CloneTimeout:  projectConfig.CloneTimeout,
		ScriptTimeout: projectConfig.ScriptTimeout,
	}

	result, err := e.remoteExec.ExecuteGitPullDeploy(ctx, &executor.GitPullDeployRequest{
		ClientID:    clientID,
		Operation:   operation,
		Version:     version,
		Config:      config,
		RepoURL:     config.RepoURL,
		Branch:      config.Branch,
		WorkDir:     workDir,
		PreScript:   config.PreScript,
		PostScript:  config.PostScript,
		Environment: config.Environment,
	})

	if err != nil {
		return "", err
	}

	if !result.Success {
		return result.GitOutput + "\n" + result.ScriptOutput, fmt.Errorf("%s", result.Error)
	}

	// 返回完整输出
	output := result.GitOutput
	if result.ScriptOutput != "" {
		output += "\n" + result.ScriptOutput
	}
	if result.Commit != "" {
		output += "\nCommit: " + result.Commit
	}

	return output, nil
}

// FetchGitVersions 获取 Git 仓库版本信息
func (e *Engine) FetchGitVersions(ctx context.Context, clientID string, config *models.GitPullDeployConfig, maxTags, maxCommits int, includeBranches bool) (*executor.GitVersionsResult, error) {
	if e.remoteExec == nil {
		return nil, fmt.Errorf("remote executor not configured")
	}

	// 解析仓库地址
	repoURL := ""
	workDir := ""
	if config != nil {
		repoURL = config.RepoURL
		workDir = config.WorkDir
	}

	result, err := e.remoteExec.FetchGitVersions(ctx, &executor.GitVersionsRequest{
		ClientID:        clientID,
		Config:          config,
		RepoURL:         repoURL,
		WorkDir:         workDir,
		MaxTags:         maxTags,
		MaxCommits:      maxCommits,
		IncludeBranches: includeBranches,
	})

	if err != nil {
		return nil, err
	}

	if !result.Success {
		return result, fmt.Errorf("%s", result.Error)
	}

	return result, nil
}
