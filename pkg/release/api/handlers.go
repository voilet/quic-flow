package api

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/voilet/quic-flow/pkg/git"
	"github.com/voilet/quic-flow/pkg/release/callback"
	"github.com/voilet/quic-flow/pkg/release/credential"
	"github.com/voilet/quic-flow/pkg/release/engine"
	"github.com/voilet/quic-flow/pkg/release/executor"
	"github.com/voilet/quic-flow/pkg/release/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"github.com/voilet/quic-flow/pkg/common"
)

// ReleaseAPI 发布系统 API
type ReleaseAPI struct {
	db            *gorm.DB
	engine        *engine.Engine
	callbackMgr   *callback.Manager
	credentialMgr *credential.Manager
}

// NewReleaseAPI 创建发布系统 API
func NewReleaseAPI(db *gorm.DB) *ReleaseAPI {
	// 从环境变量获取密钥
	secretKey := os.Getenv("QUIC_FLOW_SECRET_KEY")
	if secretKey == "" {
		// 使用默认密钥（仅用于开发环境）
		secretKey = "default-secret-key-change-in-production"
	}

	credMgr, _ := credential.NewManager(db, secretKey)

	return &ReleaseAPI{
		db:            db,
		engine:        engine.NewEngine(db),
		callbackMgr:   callback.NewManager(db),
		credentialMgr: credMgr,
	}
}

// NewReleaseAPIWithRemote 创建支持远程执行的发布系统 API
func NewReleaseAPIWithRemote(db *gorm.DB, cmdSender executor.CommandSender) *ReleaseAPI {
	// 从环境变量获取密钥
	secretKey := os.Getenv("QUIC_FLOW_SECRET_KEY")
	if secretKey == "" {
		secretKey = "default-secret-key-change-in-production"
	}

	credMgr, _ := credential.NewManager(db, secretKey)

	return &ReleaseAPI{
		db:            db,
		engine:        engine.NewEngineWithRemote(db, cmdSender),
		callbackMgr:   callback.NewManager(db),
		credentialMgr: credMgr,
	}
}

// SetRemoteExecutor 设置远程执行器
func (api *ReleaseAPI) SetRemoteExecutor(cmdSender executor.CommandSender) {
	api.engine.SetRemoteExecutor(cmdSender)
}

// SetDB 设置数据库连接（用于运行时更新）
func (api *ReleaseAPI) SetDB(db *gorm.DB) {
	api.db = db
	api.engine = engine.NewEngine(db)
}

// checkDB 检查数据库是否配置
func (api *ReleaseAPI) checkDB(c *gin.Context) bool {
	if api.db == nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Database not configured. Release system requires database setup.")
		return false
	}
	return true
}

// RegisterRoutes 注册路由
func (api *ReleaseAPI) RegisterRoutes(r *gin.RouterGroup) {
	release := r.Group("/release")
	{
		// 项目管理
		release.POST("/projects", api.CreateProject)
		release.GET("/projects", api.ListProjects)
		release.GET("/projects/:id", api.GetProject)
		release.PUT("/projects/:id", api.UpdateProject)
		release.DELETE("/projects/:id", api.DeleteProject)

		// 发布项管理
		release.POST("/projects/:id/release-items", api.CreateReleaseItem)
		release.GET("/projects/:id/release-items", api.ListReleaseItems)
		release.GET("/release-items/:id", api.GetReleaseItem)
		release.PUT("/release-items/:id", api.UpdateReleaseItem)
		release.DELETE("/release-items/:id", api.DeleteReleaseItem)

		// 环境管理
		release.POST("/projects/:id/environments", api.CreateEnvironment)
		release.GET("/projects/:id/environments", api.ListEnvironments)
		release.GET("/environments/:id", api.GetEnvironment)
		release.PUT("/environments/:id", api.UpdateEnvironment)
		release.DELETE("/environments/:id", api.DeleteEnvironment)

		// 目标管理
		release.POST("/environments/:id/targets", api.CreateTarget)
		release.GET("/environments/:id/targets", api.ListTargets)
		release.GET("/targets/:id", api.GetTarget)
		release.PUT("/targets/:id", api.UpdateTarget)
		release.DELETE("/targets/:id", api.DeleteTarget)

		// 变量管理
		release.POST("/variables", api.CreateVariable)
		release.GET("/projects/:id/variables", api.ListProjectVariables)
		release.GET("/environments/:id/variables", api.ListEnvVariables)
		release.PUT("/variables/:id", api.UpdateVariable)
		release.DELETE("/variables/:id", api.DeleteVariable)

		// 版本管理
		release.POST("/projects/:id/versions", api.CreateVersion)
		release.GET("/projects/:id/versions", api.ListVersions)
		release.GET("/versions/:id", api.GetVersion)
		release.PUT("/versions/:id", api.UpdateVersion)
		release.DELETE("/versions/:id", api.DeleteVersion)

		// 部署任务管理
		release.POST("/tasks", api.CreateDeployTask)
		release.GET("/projects/:id/tasks", api.ListDeployTasks)
		release.GET("/tasks/:id", api.GetDeployTask)
		release.POST("/tasks/:id/start", api.StartDeployTask)
		release.POST("/tasks/:id/cancel", api.CancelDeployTask)
		release.POST("/tasks/:id/pause", api.PauseDeployTask)
		release.POST("/tasks/:id/promote", api.PromoteDeployTask)
		release.POST("/tasks/:id/rollback", api.RollbackDeployTask)

		// 部署任务日志（实时日志和容器日志）
		release.GET("/tasks/:id/logs", api.GetDeployTaskLogs)
		release.GET("/tasks/:id/logs/stream", api.StreamDeployTaskLogs)
		release.GET("/tasks/:id/clients/:client_id/container-logs", api.GetClientContainerLogs)

		// 发布管理
		release.POST("/deploys", api.CreateRelease)
		release.GET("/deploys", api.ListReleases)
		release.GET("/deploys/:id", api.GetRelease)
		release.POST("/deploys/:id/start", api.StartRelease)
		release.POST("/deploys/:id/cancel", api.CancelRelease)
		release.POST("/deploys/:id/rollback", api.RollbackRelease)
		release.POST("/deploys/:id/promote", api.PromoteRelease)

		// 操作类型
		release.POST("/install", api.InstallService)
		release.POST("/update", api.UpdateService)
		release.POST("/uninstall", api.UninstallService)

		// 审批管理
		release.GET("/approvals", api.ListApprovals)
		release.POST("/approvals/:id/approve", api.ApproveRelease)
		release.POST("/approvals/:id/reject", api.RejectRelease)

		// 部署日志
		release.GET("/logs", api.ListDeployLogs)
		release.GET("/logs/:id", api.GetDeployLog)
		release.GET("/projects/:id/logs", api.ListProjectDeployLogs)
		release.GET("/projects/:id/stats", api.GetProjectDeployStats)
		release.GET("/stats", api.GetDeployStats)

		// 安装信息（版本升级增强）
		release.GET("/projects/:id/installations", api.ListProjectInstallations)

		// 进程上报
		release.POST("/process-report", api.ReceiveProcessReport)
		release.GET("/projects/:id/processes", api.ListProjectProcesses)

		// 容器上报
		release.POST("/container-report", api.ReceiveContainerReport)
		release.GET("/projects/:id/containers", api.ListProjectContainers)
		release.GET("/containers/overview", api.GetContainersOverview)

		// 脚本验证
		release.POST("/validate-script", api.ValidateScript)

		// Git 版本查询
		release.POST("/git-versions", api.GetGitVersions)

		// 配置预览（显示合并后的最终配置）
		release.POST("/config-preview", api.PreviewDeployConfig)

		// 回调配置管理
		release.POST("/projects/:id/callbacks", api.CreateCallbackConfig)
		release.GET("/projects/:id/callbacks", api.ListCallbackConfigs)
		release.GET("/callbacks/:id", api.GetCallbackConfig)
		release.PUT("/callbacks/:id", api.UpdateCallbackConfig)
		release.DELETE("/callbacks/:id", api.DeleteCallbackConfig)
		release.POST("/callbacks/:id/test", api.TestCallbackConfig)
		release.POST("/callbacks/test-direct", api.TestCallbackDirect) // 直接测试（不保存）

		// 回调历史
		release.GET("/callbacks/history", api.ListCallbackHistory)
		release.GET("/callbacks/history/:id", api.GetCallbackHistory)
		release.POST("/callbacks/history/:id/retry", api.RetryCallbackHistory)
		release.GET("/callbacks/stats", api.GetCallbackStats)
		release.GET("/tasks/:id/callbacks", api.ListTaskCallbackHistory)

		// 模板管理
		release.POST("/callbacks/template/preview", api.PreviewCallbackTemplate)
		release.POST("/callbacks/template/validate", api.ValidateCallbackTemplate)
		release.GET("/callbacks/template/variables", api.GetCallbackTemplateVariables)
		release.GET("/callbacks/template/defaults", api.GetDefaultCallbackTemplates)

		// 消息模板 CRUD
		release.GET("/templates", api.ListMessageTemplates)
		release.GET("/templates/:id", api.GetMessageTemplate)
		release.POST("/templates", api.CreateMessageTemplate)
		release.PUT("/templates/:id", api.UpdateMessageTemplate)
		release.DELETE("/templates/:id", api.DeleteMessageTemplate)
		release.POST("/templates/:id/copy", api.CopyMessageTemplate)

		// 凭证管理
		release.GET("/credentials", api.ListCredentials)
		release.GET("/credentials/:id", api.GetCredential)
		release.POST("/credentials", api.CreateCredential)
		release.PUT("/credentials/:id", api.UpdateCredential)
		release.DELETE("/credentials/:id", api.DeleteCredential)
		release.GET("/projects/:id/credentials", api.GetProjectCredentials)
		release.POST("/projects/:id/credentials", api.AddProjectCredential)
		release.DELETE("/projects/:id/credentials/:credential_id", api.RemoveProjectCredential)

		// Webhook 管理
		release.GET("/projects/:id/webhooks", api.ListWebhooks)
		release.GET("/webhooks/:id", api.GetWebhook)
		release.POST("/webhooks", api.CreateWebhook)
		release.PUT("/webhooks/:id", api.UpdateWebhook)
		release.DELETE("/webhooks/:id", api.DeleteWebhook)
		release.POST("/webhooks/:id/regenerate-secret", api.RegenerateWebhookSecret)
		release.POST("/webhooks/:id/test", api.TestWebhook)
		release.GET("/webhooks/:id/triggers", api.ListTriggerHistory)

		// 成员管理
		release.GET("/projects/:id/members", api.ListMembers)
		release.POST("/projects/:id/members", api.AddMember)
		release.PUT("/projects/:id/members/:user_id", api.UpdateMember)
		release.DELETE("/projects/:id/members/:user_id", api.RemoveMember)

		// 用户管理
		release.GET("/users/search", api.SearchUsers)
		release.GET("/users", api.ListUsers)
		release.GET("/users/:id", api.GetUser)
		release.POST("/users", api.CreateUser)
	}

	// Webhook 接收（无需认证）
	webhook := r.Group("/release/webhook")
	{
		webhook.POST("/incoming/:token", api.IncomingWebhook)
	}
}

// ==================== 项目管理 ====================

// CreateProject 创建项目
func (api *ReleaseAPI) CreateProject(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	var req struct {
		Name            string                        `json:"name" binding:"required"`
		Description     string                        `json:"description"`
		Type            models.DeployType             `json:"type" binding:"required"`
		RepoURL         string                        `json:"repo_url"`
		RepoType        string                        `json:"repo_type"`
		ScriptConfig    *models.ScriptDeployConfig    `json:"script_config"`
		GitPullConfig   *models.GitPullDeployConfig   `json:"gitpull_config"`
		ContainerConfig *models.ContainerDeployConfig `json:"container_config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	project := &models.Project{
		Name:            req.Name,
		Description:     req.Description,
		Type:            req.Type,
		RepoURL:         req.RepoURL,
		RepoType:        req.RepoType,
		ScriptConfig:    req.ScriptConfig,
		GitPullConfig:   req.GitPullConfig,
		ContainerConfig: req.ContainerConfig,
	}

	if err := api.db.Create(project).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, project)
}

// ListProjects 列出项目
func (api *ReleaseAPI) ListProjects(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	var projects []models.Project
	if err := api.db.Find(&projects).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 构建包含版本数量的响应
	type ProjectWithCount struct {
		models.Project
		VersionCount int64 `json:"version_count"`
	}

	var result []ProjectWithCount
	for _, p := range projects {
		var count int64
		api.db.Model(&models.Version{}).Where("project_id = ?", p.ID).Count(&count)
		result = append(result, ProjectWithCount{
			Project:      p,
			VersionCount: count,
		})
	}

	common.SuccessResp(c, result)
}

// GetProject 获取项目详情
func (api *ReleaseAPI) GetProject(c *gin.Context) {
	id := c.Param("id")
	var project models.Project
	if err := api.db.Preload("Environments").First(&project, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "project not found")
		return
	}

	common.SuccessResp(c, project)
}

// UpdateProject 更新项目
func (api *ReleaseAPI) UpdateProject(c *gin.Context) {
	id := c.Param("id")
	var project models.Project
	if err := api.db.First(&project, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "project not found")
		return
	}

	var req struct {
		Name            string                        `json:"name"`
		Description     string                        `json:"description"`
		RepoURL         string                        `json:"repo_url"`
		ScriptConfig    *models.ScriptDeployConfig    `json:"script_config"`
		GitPullConfig   *models.GitPullDeployConfig   `json:"gitpull_config"`
		ContainerConfig *models.ContainerDeployConfig `json:"container_config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	if req.Name != "" {
		project.Name = req.Name
	}
	if req.Description != "" {
		project.Description = req.Description
	}
	if req.RepoURL != "" {
		project.RepoURL = req.RepoURL
	}
	if req.ScriptConfig != nil {
		project.ScriptConfig = req.ScriptConfig
	}
	if req.GitPullConfig != nil {
		project.GitPullConfig = req.GitPullConfig
	}
	if req.ContainerConfig != nil {
		project.ContainerConfig = req.ContainerConfig
	}

	if err := api.db.Save(&project).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, project)
}

// DeleteProject 删除项目
func (api *ReleaseAPI) DeleteProject(c *gin.Context) {
	id := c.Param("id")
	if err := api.db.Delete(&models.Project{}, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// ==================== 发布项管理 ====================

// CreateReleaseItem 创建发布项
func (api *ReleaseAPI) CreateReleaseItem(c *gin.Context) {
	projectID := c.Param("id")

	var req struct {
		Name              string                          `json:"name" binding:"required"`
		Description       string                          `json:"description"`
		Type              models.DeployType               `json:"type" binding:"required"`
		RepoURL           string                          `json:"repo_url"`
		RepoType          string                          `json:"repo_type"`
		SortOrder         int                             `json:"sort_order"`
		ScriptConfig      *models.ScriptDeployConfig      `json:"script_config"`
		ContainerConfig   *models.ContainerDeployConfig   `json:"container_config"`
		KubernetesConfig  *models.KubernetesDeployConfig  `json:"kubernetes_config"`
		GitPullConfig     *models.GitPullDeployConfig     `json:"gitpull_config"`
		ContainerNaming   *models.ContainerNamingConfig   `json:"container_naming"`
		CallbackConfig    *models.CallbackConfig          `json:"callback_config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	item := &models.ReleaseItem{
		ProjectID:         projectID,
		Name:              req.Name,
		Description:       req.Description,
		Type:              req.Type,
		RepoURL:           req.RepoURL,
		RepoType:          req.RepoType,
		SortOrder:         req.SortOrder,
		ScriptConfig:      req.ScriptConfig,
		ContainerConfig:   req.ContainerConfig,
		KubernetesConfig:  req.KubernetesConfig,
		GitPullConfig:     req.GitPullConfig,
		ContainerNaming:   req.ContainerNaming,
		CallbackConfig:    req.CallbackConfig,
	}

	if err := api.db.Create(item).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, item)
}

// ListReleaseItems 列出项目的发布项
func (api *ReleaseAPI) ListReleaseItems(c *gin.Context) {
	projectID := c.Param("id")
	var items []models.ReleaseItem
	if err := api.db.Where("project_id = ?", projectID).Order("sort_order, created_at").Find(&items).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, items)
}

// GetReleaseItem 获取发布项详情
func (api *ReleaseAPI) GetReleaseItem(c *gin.Context) {
	id := c.Param("id")
	var item models.ReleaseItem
	if err := api.db.Preload("Project").Preload("Versions").First(&item, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "release item not found")
		return
	}

	common.SuccessResp(c, item)
}

// UpdateReleaseItem 更新发布项
func (api *ReleaseAPI) UpdateReleaseItem(c *gin.Context) {
	id := c.Param("id")
	var item models.ReleaseItem
	if err := api.db.First(&item, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "release item not found")
		return
	}

	var req struct {
		Name              string                          `json:"name"`
		Description       string                          `json:"description"`
		Type              models.DeployType               `json:"type"`
		RepoURL           string                          `json:"repo_url"`
		RepoType          string                          `json:"repo_type"`
		SortOrder         *int                            `json:"sort_order"`
		ScriptConfig      *models.ScriptDeployConfig      `json:"script_config"`
		ContainerConfig   *models.ContainerDeployConfig   `json:"container_config"`
		KubernetesConfig  *models.KubernetesDeployConfig  `json:"kubernetes_config"`
		GitPullConfig     *models.GitPullDeployConfig     `json:"gitpull_config"`
		ContainerNaming   *models.ContainerNamingConfig   `json:"container_naming"`
		CallbackConfig    *models.CallbackConfig          `json:"callback_config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.RepoURL != "" {
		updates["repo_url"] = req.RepoURL
	}
	if req.RepoType != "" {
		updates["repo_type"] = req.RepoType
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.ScriptConfig != nil {
		updates["script_config"] = req.ScriptConfig
	}
	if req.ContainerConfig != nil {
		updates["container_config"] = req.ContainerConfig
	}
	if req.KubernetesConfig != nil {
		updates["kubernetes_config"] = req.KubernetesConfig
	}
	if req.GitPullConfig != nil {
		updates["gitpull_config"] = req.GitPullConfig
	}
	if req.ContainerNaming != nil {
		updates["container_naming"] = req.ContainerNaming
	}
	if req.CallbackConfig != nil {
		updates["callback_config"] = req.CallbackConfig
	}

	if err := api.db.Model(&item).Updates(updates).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 重新获取更新后的数据
	api.db.First(&item, "id = ?", id)
	common.SuccessResp(c, item)
}

// DeleteReleaseItem 删除发布项
func (api *ReleaseAPI) DeleteReleaseItem(c *gin.Context) {
	id := c.Param("id")
	if err := api.db.Delete(&models.ReleaseItem{}, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// ==================== 环境管理 ====================

// CreateEnvironment 创建环境
func (api *ReleaseAPI) CreateEnvironment(c *gin.Context) {
	projectID := c.Param("id")

	var req struct {
		Name            string                 `json:"name" binding:"required"`
		Description     string                 `json:"description"`
		ReleaseWindow   *models.ReleaseWindow  `json:"release_window"`
		RequireApproval bool                   `json:"require_approval"`
		Approvers       []string               `json:"approvers"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	env := &models.Environment{
		ProjectID:       projectID,
		Name:            req.Name,
		Description:     req.Description,
		ReleaseWindow:   req.ReleaseWindow,
		RequireApproval: req.RequireApproval,
		Approvers:       req.Approvers,
	}

	if err := api.db.Create(env).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, env)
}

// ListEnvironments 列出环境
func (api *ReleaseAPI) ListEnvironments(c *gin.Context) {
	projectID := c.Param("id")
	var envs []models.Environment
	if err := api.db.Where("project_id = ?", projectID).Find(&envs).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, envs)
}

// GetEnvironment 获取环境详情
func (api *ReleaseAPI) GetEnvironment(c *gin.Context) {
	id := c.Param("id")
	var env models.Environment
	if err := api.db.Preload("Targets").First(&env, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "environment not found")
		return
	}

	common.SuccessResp(c, env)
}

// UpdateEnvironment 更新环境
func (api *ReleaseAPI) UpdateEnvironment(c *gin.Context) {
	id := c.Param("id")
	var env models.Environment
	if err := api.db.First(&env, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "environment not found")
		return
	}

	if err := c.ShouldBindJSON(&env); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	if err := api.db.Save(&env).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, env)
}

// DeleteEnvironment 删除环境
func (api *ReleaseAPI) DeleteEnvironment(c *gin.Context) {
	id := c.Param("id")
	if err := api.db.Delete(&models.Environment{}, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// ==================== 目标管理 ====================

// CreateTarget 创建目标
func (api *ReleaseAPI) CreateTarget(c *gin.Context) {
	envID := c.Param("id")

	var req struct {
		ClientID string               `json:"client_id" binding:"required"`
		Name     string               `json:"name" binding:"required"`
		Type     models.TargetType    `json:"type" binding:"required"`
		Labels   map[string]string    `json:"labels"`
		Config   models.TargetConfig  `json:"config"`
		Priority int                  `json:"priority"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	target := &models.Target{
		EnvironmentID: envID,
		ClientID:      req.ClientID,
		Name:          req.Name,
		Type:          req.Type,
		Labels:        req.Labels,
		Config:        req.Config,
		Priority:      req.Priority,
		Status:        models.TargetStatusUnknown,
	}

	if err := api.db.Create(target).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"target": target})
}

// ListTargets 列出目标
func (api *ReleaseAPI) ListTargets(c *gin.Context) {
	envID := c.Param("id")
	var targets []models.Target
	if err := api.db.Where("environment_id = ?", envID).Find(&targets).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"targets": targets})
}

// GetTarget 获取目标详情
func (api *ReleaseAPI) GetTarget(c *gin.Context) {
	id := c.Param("id")
	var target models.Target
	if err := api.db.First(&target, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "target not found")
		return
	}

	common.SuccessResp(c, gin.H{"target": target})
}

// UpdateTarget 更新目标
func (api *ReleaseAPI) UpdateTarget(c *gin.Context) {
	id := c.Param("id")
	var target models.Target
	if err := api.db.First(&target, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "target not found")
		return
	}

	if err := c.ShouldBindJSON(&target); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	if err := api.db.Save(&target).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"target": target})
}

// DeleteTarget 删除目标
func (api *ReleaseAPI) DeleteTarget(c *gin.Context) {
	id := c.Param("id")
	if err := api.db.Delete(&models.Target{}, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// ==================== 变量管理 ====================

// CreateVariable 创建变量
func (api *ReleaseAPI) CreateVariable(c *gin.Context) {
	var req models.Variable
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	if err := api.db.Create(&req).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"variable": req})
}

// ListProjectVariables 列出项目变量
func (api *ReleaseAPI) ListProjectVariables(c *gin.Context) {
	projectID := c.Param("id")
	var vars []models.Variable
	if err := api.db.Where("project_id = ?", projectID).Find(&vars).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"variables": vars})
}

// ListEnvVariables 列出环境变量
func (api *ReleaseAPI) ListEnvVariables(c *gin.Context) {
	envID := c.Param("id")
	var vars []models.Variable
	if err := api.db.Where("environment_id = ?", envID).Find(&vars).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"variables": vars})
}

// UpdateVariable 更新变量
func (api *ReleaseAPI) UpdateVariable(c *gin.Context) {
	id := c.Param("id")
	var variable models.Variable
	if err := api.db.First(&variable, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "variable not found")
		return
	}

	if err := c.ShouldBindJSON(&variable); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	if err := api.db.Save(&variable).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, variable)
}

// DeleteVariable 删除变量
func (api *ReleaseAPI) DeleteVariable(c *gin.Context) {
	id := c.Param("id")
	if err := api.db.Delete(&models.Variable{}, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// ==================== 发布管理 ====================

// CreateRelease 创建发布
func (api *ReleaseAPI) CreateRelease(c *gin.Context) {
	var req engine.CreateReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// TODO: 从认证信息获取用户
	req.CreatedBy = "admin"

	release, err := api.engine.CreateRelease(c.Request.Context(), &req)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, release)
}

// ListReleases 列出发布
func (api *ReleaseAPI) ListReleases(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	projectID := c.Query("project_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	releases, total, err := api.engine.ListReleases(c.Request.Context(), projectID, limit, offset)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"releases": releases, "total": total})
}

// GetRelease 获取发布详情
func (api *ReleaseAPI) GetRelease(c *gin.Context) {
	id := c.Param("id")
	release, err := api.engine.GetRelease(c.Request.Context(), id)
	if err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "release not found")
		return
	}

	common.SuccessResp(c, release)
}

// StartRelease 开始发布
func (api *ReleaseAPI) StartRelease(c *gin.Context) {
	id := c.Param("id")
	if err := api.engine.StartRelease(c.Request.Context(), id); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// CancelRelease 取消发布
func (api *ReleaseAPI) CancelRelease(c *gin.Context) {
	id := c.Param("id")
	if err := api.engine.CancelRelease(c.Request.Context(), id); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// RollbackRelease 回滚发布
func (api *ReleaseAPI) RollbackRelease(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		TargetVersion string `json:"target_version"`
	}
	c.ShouldBindJSON(&req)

	// 获取原发布信息
	var release models.Release
	if err := api.db.First(&release, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "release not found")
		return
	}

	// 创建回滚发布
	rollbackReq := &engine.CreateReleaseRequest{
		ProjectID:     release.ProjectID,
		EnvironmentID: release.EnvironmentID,
		Version:       req.TargetVersion,
		Operation:     models.OperationTypeRollback,
		TargetIDs:     release.TargetIDs,
		CreatedBy:     "admin", // TODO: 从认证获取
	}

	rollback, err := api.engine.CreateRelease(c.Request.Context(), rollbackReq)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 自动开始回滚
	if err := api.engine.StartRelease(c.Request.Context(), rollback.ID); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, rollback)
}

// PromoteRelease 金丝雀全量发布
func (api *ReleaseAPI) PromoteRelease(c *gin.Context) {
	id := c.Param("id")
	if err := api.engine.PromoteCanary(c.Request.Context(), id); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// InstallService 安装服务
func (api *ReleaseAPI) InstallService(c *gin.Context) {
	var req engine.CreateReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	req.Operation = models.OperationTypeInstall
	req.CreatedBy = "admin"

	release, err := api.engine.CreateRelease(c.Request.Context(), &req)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 自动开始
	if err := api.engine.StartRelease(c.Request.Context(), release.ID); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, release)
}

// UpdateService 更新服务
func (api *ReleaseAPI) UpdateService(c *gin.Context) {
	var req engine.CreateReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	req.Operation = models.OperationTypeUpdate
	req.CreatedBy = "admin"

	release, err := api.engine.CreateRelease(c.Request.Context(), &req)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	if err := api.engine.StartRelease(c.Request.Context(), release.ID); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, release)
}

// UninstallService 卸载服务
func (api *ReleaseAPI) UninstallService(c *gin.Context) {
	var req engine.CreateReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	req.Operation = models.OperationTypeUninstall
	req.CreatedBy = "admin"

	release, err := api.engine.CreateRelease(c.Request.Context(), &req)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	if err := api.engine.StartRelease(c.Request.Context(), release.ID); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, release)
}

// ==================== 审批管理 ====================

// ListApprovals 列出待审批
func (api *ReleaseAPI) ListApprovals(c *gin.Context) {
	var approvals []models.Approval
	if err := api.db.Where("status = ?", models.ApprovalStatusPending).Find(&approvals).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"approvals": approvals})
}

// ApproveRelease 同意发布
func (api *ReleaseAPI) ApproveRelease(c *gin.Context) {
	id := c.Param("id")
	var approval models.Approval
	if err := api.db.First(&approval, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "approval not found")
		return
	}

	approver := "admin" // TODO: 从认证获取
	approval.Status = models.ApprovalStatusApproved
	approval.ApprovedBy = &approver

	if err := api.db.Save(&approval).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 自动开始发布
	if err := api.engine.StartRelease(c.Request.Context(), approval.ReleaseID); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// RejectRelease 拒绝发布
func (api *ReleaseAPI) RejectRelease(c *gin.Context) {
	id := c.Param("id")
	var approval models.Approval
	if err := api.db.First(&approval, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "approval not found")
		return
	}

	var req struct {
		Comment string `json:"comment"`
	}
	c.ShouldBindJSON(&req)

	approver := "admin"
	approval.Status = models.ApprovalStatusRejected
	approval.ApprovedBy = &approver
	approval.Comment = req.Comment

	if err := api.db.Save(&approval).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 更新发布状态为取消
	api.db.Model(&models.Release{}).Where("id = ?", approval.ReleaseID).Updates(map[string]interface{}{
		"status":      models.ReleaseStatusCancelled,
		"finished_at": time.Now(),
	})

	common.SuccessResp(c, gin.H{})
}

// ==================== 版本管理 ====================

// CreateVersion 创建版本
func (api *ReleaseAPI) CreateVersion(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	projectID := c.Param("id")

	var req struct {
		Version         string `json:"version" binding:"required"`
		Description     string `json:"description"`
		WorkDir         string `json:"work_dir"`
		InstallScript   string `json:"install_script"`
		UpdateScript    string `json:"update_script"`
		RollbackScript  string `json:"rollback_script"`
		UninstallScript string `json:"uninstall_script"`
		SkipValidation  bool   `json:"skip_validation"` // 跳过验证
		// Git 相关
		GitRef     string `json:"git_ref"`
		GitRefType string `json:"git_ref_type"`
		// 容器/K8s 相关（向后兼容）
		ContainerImage string `json:"container_image"`
		ContainerEnv   string `json:"container_env"`
		Replicas       int    `json:"replicas"`
		K8sYAML        string `json:"k8s_yaml"`
		// 新版部署配置（推荐使用）
		DeployConfig *models.VersionDeployConfig `json:"deploy_config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 检查项目是否存在
	var project models.Project
	if err := api.db.First(&project, "id = ?", projectID).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "project not found")
		return
	}

	// 如果是脚本部署类型，验证脚本语法
	if project.Type == models.DeployTypeScript && !req.SkipValidation {
		// 安装脚本必须有
		if strings.TrimSpace(req.InstallScript) == "" {
			common.ErrorResp(c, common.CodeServiceUnavailable, "安装脚本不能为空")
			return
		}

		valid, errors := validateAllScripts(req.InstallScript, req.UpdateScript, req.RollbackScript, req.UninstallScript)
		if !valid {
			common.ErrorRespWithData(c, common.CodeInvalidParams, gin.H{"validation_errors": errors}, "脚本语法验证失败")
			return
		}
	}

	// 确定工作目录：优先使用请求中的值，否则从项目配置继承
	workDir := req.WorkDir
	if workDir == "" {
		// 根据项目类型从配置中继承
		switch project.Type {
		case models.DeployTypeGitPull:
			if project.GitPullConfig != nil && project.GitPullConfig.WorkDir != "" {
				workDir = project.GitPullConfig.WorkDir
			}
		case models.DeployTypeScript:
			if project.ScriptConfig != nil && project.ScriptConfig.WorkDir != "" {
				workDir = project.ScriptConfig.WorkDir
			}
		case models.DeployTypeContainer:
			if project.ContainerConfig != nil && project.ContainerConfig.WorkingDir != "" {
				workDir = project.ContainerConfig.WorkingDir
			}
		}
		// 如果仍为空，使用默认值
		if workDir == "" {
			workDir = "/opt/app"
		}
	}

	replicas := req.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	version := &models.Version{
		ProjectID:       projectID,
		Version:         req.Version,
		Description:     req.Description,
		WorkDir:         workDir,
		InstallScript:   req.InstallScript,
		UpdateScript:    req.UpdateScript,
		RollbackScript:  req.RollbackScript,
		UninstallScript: req.UninstallScript,
		GitRef:          req.GitRef,
		GitRefType:      req.GitRefType,
		ContainerImage:  req.ContainerImage,
		ContainerEnv:    req.ContainerEnv,
		Replicas:        replicas,
		K8sYAML:         req.K8sYAML,
		DeployConfig:    req.DeployConfig,
		Status:          "draft",
	}

	if err := api.db.Create(version).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, version)
}

// ListVersions 列出版本
func (api *ReleaseAPI) ListVersions(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	projectID := c.Param("id")
	var versions []models.Version
	if err := api.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&versions).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, versions)
}

// GetVersion 获取版本详情
func (api *ReleaseAPI) GetVersion(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	id := c.Param("id")
	var version models.Version
	if err := api.db.First(&version, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "version not found")
		return
	}

	common.SuccessResp(c, version)
}

// UpdateVersion 更新版本
func (api *ReleaseAPI) UpdateVersion(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	id := c.Param("id")
	var version models.Version
	if err := api.db.First(&version, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "version not found")
		return
	}

	var req struct {
		Description     string                      `json:"description"`
		WorkDir         string                      `json:"work_dir"`
		InstallScript   string                      `json:"install_script"`
		UpdateScript    string                      `json:"update_script"`
		RollbackScript  string                      `json:"rollback_script"`
		UninstallScript string                      `json:"uninstall_script"`
		Status          string                      `json:"status"`
		// 容器/K8s 相关（向后兼容）
		ContainerImage  string                      `json:"container_image"`
		ContainerEnv    string                      `json:"container_env"`
		Replicas        *int                        `json:"replicas"`
		K8sYAML         string                      `json:"k8s_yaml"`
		// 新版部署配置
		DeployConfig    *models.VersionDeployConfig `json:"deploy_config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	if req.Description != "" {
		version.Description = req.Description
	}
	if req.WorkDir != "" {
		version.WorkDir = req.WorkDir
	}
	if req.InstallScript != "" {
		version.InstallScript = req.InstallScript
	}
	if req.UpdateScript != "" {
		version.UpdateScript = req.UpdateScript
	}
	if req.RollbackScript != "" {
		version.RollbackScript = req.RollbackScript
	}
	if req.UninstallScript != "" {
		version.UninstallScript = req.UninstallScript
	}
	if req.Status != "" {
		version.Status = req.Status
	}
	// 容器/K8s 相关字段更新
	if req.ContainerImage != "" {
		version.ContainerImage = req.ContainerImage
	}
	if req.ContainerEnv != "" {
		version.ContainerEnv = req.ContainerEnv
	}
	if req.Replicas != nil {
		version.Replicas = *req.Replicas
	}
	if req.K8sYAML != "" {
		version.K8sYAML = req.K8sYAML
	}
	// 新版部署配置更新
	if req.DeployConfig != nil {
		version.DeployConfig = req.DeployConfig
	}

	if err := api.db.Save(&version).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, version)
}

// DeleteVersion 删除版本
func (api *ReleaseAPI) DeleteVersion(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	id := c.Param("id")
	if err := api.db.Delete(&models.Version{}, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// ==================== 部署任务管理 ====================

// CreateDeployTask 创建部署任务
func (api *ReleaseAPI) CreateDeployTask(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	var req struct {
		ProjectID         string     `json:"project_id" binding:"required"`
		VersionID         string     `json:"version_id" binding:"required"`
		Operation         string     `json:"operation" binding:"required"`
		ClientIDs         []string   `json:"client_ids"`
		AutoSelectClients bool       `json:"auto_select_clients"`
		SourceVersion     string     `json:"source_version"`
		ScheduleType      string     `json:"schedule_type"`
		ScheduleFrom      *time.Time `json:"schedule_from"`
		ScheduleTo        *time.Time `json:"schedule_to"`
		CanaryEnabled     bool       `json:"canary_enabled"`
		CanaryPercent     int        `json:"canary_percent"`
		CanaryDuration    int        `json:"canary_duration"`
		CanaryAutoPromote bool       `json:"canary_auto_promote"`
		FailureStrategy   string     `json:"failure_strategy"`
		AutoRollback      bool       `json:"auto_rollback"`
		// 任务覆盖配置（用于金丝雀、A/B测试、紧急扩容等场景）
		OverrideConfig *models.TaskOverrideConfig `json:"override_config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 获取版本信息
	var version models.Version
	if err := api.db.First(&version, "id = ?", req.VersionID).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "version not found")
		return
	}

	// 处理自动选择客户端
	clientIDs := req.ClientIDs
	selectedFromVersion := ""
	if req.AutoSelectClients {
		// 查询已安装的目标
		var installations []models.TargetInstallation
		query := api.db.Where("project_id = ?", req.ProjectID)
		if req.SourceVersion != "" {
			query = query.Where("version = ?", req.SourceVersion)
			selectedFromVersion = req.SourceVersion
		}
		if err := query.Find(&installations).Error; err != nil {
			common.ErrorResp(c, common.CodeServiceUnavailable, "failed to query installations: ")
			return
		}

		// 从安装记录中提取 ClientID
		clientIDSet := make(map[string]bool)
		for _, inst := range installations {
			var target models.Target
			if err := api.db.First(&target, "id = ?", inst.TargetID).Error; err == nil {
				if target.ClientID != "" {
					clientIDSet[target.ClientID] = true
				}
			}
		}

		clientIDs = make([]string, 0, len(clientIDSet))
		for clientID := range clientIDSet {
			clientIDs = append(clientIDs, clientID)
		}

		if len(clientIDs) == 0 {
			common.ErrorResp(c, common.CodeServiceUnavailable, "no installed clients found for auto selection")
			return
		}
	} else if len(clientIDs) == 0 {
		common.ErrorResp(c, common.CodeServiceUnavailable, "client_ids is required when auto_select_clients is false")
		return
	}

	scheduleType := req.ScheduleType
	if scheduleType == "" {
		scheduleType = "immediate"
	}

	failureStrategy := req.FailureStrategy
	if failureStrategy == "" {
		failureStrategy = "continue"
	}

	canaryPercent := req.CanaryPercent
	if canaryPercent == 0 {
		canaryPercent = 10
	}

	canaryDuration := req.CanaryDuration
	if canaryDuration == 0 {
		canaryDuration = 30
	}

	task := &models.DeployTask{
		ProjectID:           req.ProjectID,
		VersionID:           req.VersionID,
		Version:             version.Version,
		Operation:           models.OperationType(req.Operation),
		ClientIDs:           clientIDs,
		AutoSelectClients:   req.AutoSelectClients,
		SourceVersion:       req.SourceVersion,
		SelectedFromVersion: selectedFromVersion,
		ScheduleType:        scheduleType,
		ScheduleFrom:        req.ScheduleFrom,
		ScheduleTo:          req.ScheduleTo,
		CanaryEnabled:       req.CanaryEnabled,
		CanaryPercent:       canaryPercent,
		CanaryDuration:      canaryDuration,
		CanaryAutoPromote:   req.CanaryAutoPromote,
		FailureStrategy:     failureStrategy,
		AutoRollback:        req.AutoRollback,
		OverrideConfig:      req.OverrideConfig,
		Status:              "pending",
		TotalCount:          len(clientIDs),
		PendingCount:        len(clientIDs),
		CreatedBy:           "admin", // TODO: 从认证获取
	}

	// 初始化结果
	results := make(models.DeployTaskResults, len(clientIDs))
	for i, clientID := range clientIDs {
		results[i] = models.DeployTaskResult{
			ClientID: clientID,
			Status:   "pending",
		}
	}
	task.Results = results

	// 如果是定时任务，设置状态
	if scheduleType == "scheduled" && req.ScheduleFrom != nil {
		task.Status = "scheduled"
	}

	if err := api.db.Create(task).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"task": task})
}

// ListDeployTasks 列出部署任务
func (api *ReleaseAPI) ListDeployTasks(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	projectID := c.Param("id")
	status := c.Query("status")

	query := api.db.Where("project_id = ?", projectID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var tasks []models.DeployTask
	if err := query.Order("created_at DESC").Find(&tasks).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"tasks": tasks})
}

// GetDeployTask 获取部署任务详情
func (api *ReleaseAPI) GetDeployTask(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	id := c.Param("id")
	var task models.DeployTask
	if err := api.db.First(&task, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "task not found")
		return
	}

	common.SuccessResp(c, gin.H{"task": task})
}

// StartDeployTask 开始部署任务
func (api *ReleaseAPI) StartDeployTask(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	id := c.Param("id")

	var task models.DeployTask
	if err := api.db.First(&task, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "task not found")
		return
	}

	if task.Status != "pending" && task.Status != "scheduled" {
		common.ErrorResp(c, common.CodeServiceUnavailable, "task cannot be started")
		return
	}

	now := time.Now()
	task.Status = "running"
	task.StartedAt = &now

	// 如果启用金丝雀，先设置为金丝雀状态
	if task.CanaryEnabled {
		task.Status = "canary"
	}

	if err := api.db.Save(&task).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 触发实际的部署执行
	go api.executeDeployTask(&task)

	common.SuccessResp(c, gin.H{"task": task})
}

// CancelDeployTask 取消部署任务
func (api *ReleaseAPI) CancelDeployTask(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	id := c.Param("id")

	var task models.DeployTask
	if err := api.db.First(&task, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "task not found")
		return
	}

	if task.Status == "completed" || task.Status == "cancelled" {
		common.ErrorResp(c, common.CodeServiceUnavailable, "task cannot be cancelled")
		return
	}

	now := time.Now()
	task.Status = "cancelled"
	task.FinishedAt = &now

	if err := api.db.Save(&task).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// PauseDeployTask 暂停部署任务
func (api *ReleaseAPI) PauseDeployTask(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	id := c.Param("id")

	var task models.DeployTask
	if err := api.db.First(&task, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "task not found")
		return
	}

	if task.Status != "running" && task.Status != "canary" {
		common.ErrorResp(c, common.CodeServiceUnavailable, "task cannot be paused")
		return
	}

	task.Status = "paused"

	if err := api.db.Save(&task).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// PromoteDeployTask 金丝雀全量发布
func (api *ReleaseAPI) PromoteDeployTask(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	id := c.Param("id")

	var task models.DeployTask
	if err := api.db.First(&task, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "task not found")
		return
	}

	if task.Status != "canary" {
		common.ErrorResp(c, common.CodeServiceUnavailable, "task is not in canary status")
		return
	}

	task.Status = "running"

	if err := api.db.Save(&task).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 触发剩余节点的部署
	go api.promoteDeployTask(&task)

	common.SuccessResp(c, gin.H{})
}

// RollbackDeployTask 回滚部署任务
func (api *ReleaseAPI) RollbackDeployTask(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	id := c.Param("id")

	var task models.DeployTask
	if err := api.db.First(&task, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "task not found")
		return
	}

	if task.Status != "canary" && task.Status != "running" && task.Status != "failed" {
		common.ErrorResp(c, common.CodeServiceUnavailable, "task cannot be rolled back")
		return
	}

	// 获取版本的回滚脚本
	var version models.Version
	if err := api.db.First(&version, "id = ?", task.VersionID).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "version not found")
		return
	}

	// 触发回滚执行
	go api.rollbackDeployTask(&task, &version)

	common.SuccessResp(c, gin.H{})
}

// executeDeployTask 执行部署任务（内部方法）
func (api *ReleaseAPI) executeDeployTask(task *models.DeployTask) {
	// 获取版本信息
	var version models.Version
	if err := api.db.First(&version, "id = ?", task.VersionID).Error; err != nil {
		return
	}

	// 获取项目信息
	var project models.Project
	if err := api.db.First(&project, "id = ?", task.ProjectID).Error; err != nil {
		return
	}

	// 写入任务开始日志
	api.writeTaskLog(task.ID, "", "info", "init", "部署任务开始执行")

	// 触发金丝雀开始回调
	if task.CanaryEnabled && task.Status == "canary" {
		// 获取环境名称
		var environmentName string
		var env models.Environment
		if err := api.db.First(&env, "id IN (SELECT environment_id FROM targets WHERE client_id = ?)", task.ClientIDs[0]).Error; err == nil {
			environmentName = env.Name
		}

		// 异步触发回调，避免阻塞部署流程
		go func() {
			ctx := context.Background()
			api.callbackMgr.TriggerCanaryStarted(ctx, task, &project, &version, environmentName)
		}()
	}

	// 确定要执行的客户端
	var clientsToExecute []string
	if task.CanaryEnabled && task.Status == "canary" {
		// 金丝雀阶段：只执行部分客户端
		canaryCount := len(task.ClientIDs) * task.CanaryPercent / 100
		if canaryCount < 1 {
			canaryCount = 1
		}
		clientsToExecute = task.ClientIDs[:canaryCount]

		// 标记金丝雀节点
		for i := range task.Results {
			for _, canaryClient := range clientsToExecute {
				if task.Results[i].ClientID == canaryClient {
					task.Results[i].IsCanary = true
				}
			}
		}
		api.writeTaskLog(task.ID, "", "info", "init", "金丝雀模式，首批执行 "+strconv.Itoa(len(clientsToExecute))+" 个节点")
	} else {
		clientsToExecute = task.ClientIDs
	}

	api.writeTaskLog(task.ID, "", "info", "init", "共 "+strconv.Itoa(len(clientsToExecute))+" 个目标待部署")

	// 使用远程执行器执行命令
	if api.engine != nil {
		for _, clientID := range clientsToExecute {
			// 查找是否为金丝雀节点
			var isCanary bool
			for _, r := range task.Results {
				if r.ClientID == clientID {
					isCanary = r.IsCanary
					break
				}
			}

			// 更新结果状态为运行中
			startTime := time.Now()
			for i := range task.Results {
				if task.Results[i].ClientID == clientID {
					task.Results[i].Status = "running"
					task.Results[i].StartedAt = &startTime
				}
			}
			api.db.Save(task)

			// 记录开始执行日志
			canaryLabel := ""
			if isCanary {
				canaryLabel = "[金丝雀] "
			}
			api.writeTaskLog(task.ID, clientID, "info", "start", canaryLabel+"开始部署到 "+clientID)

			var result string
			var err error

			// 根据项目类型选择执行方式
			switch project.Type {
			case models.DeployTypeContainer:
				// 容器部署：使用三层配置合并
				api.writeTaskLog(task.ID, clientID, "info", "container", "执行容器部署...")

				// 合并配置：项目 + 版本 + 任务覆盖
				finalConfig := models.MergeContainerConfig(
					project.ContainerConfig,
					version.DeployConfig,
					task.OverrideConfig,
				)

				if finalConfig == nil {
					api.writeTaskLog(task.ID, clientID, "error", "container", "容器配置合并失败：项目未配置容器部署")
					err = fmt.Errorf("container config merge failed: project has no container config")
				} else {
					// 向后兼容：如果版本有旧的 ContainerImage 字段，优先使用
					if version.ContainerImage != "" && finalConfig.Image == "" {
						finalConfig.Image = version.ContainerImage
					}

					result, err = api.engine.ExecuteContainerDeployWithMergedConfig(
						clientID,
						task.Operation,
						version.Version,
						finalConfig,
					)
				}
			case models.DeployTypeKubernetes:
				// K8s 部署：使用三层配置合并
				api.writeTaskLog(task.ID, clientID, "info", "kubernetes", "执行 K8s 部署...")

				// 合并配置：项目 + 版本 + 任务覆盖
				k8sConfig := models.MergeK8sConfig(
					project.KubernetesConfig,
					version.DeployConfig,
					task.OverrideConfig,
				)

				if k8sConfig == nil {
					api.writeTaskLog(task.ID, clientID, "error", "kubernetes", "K8s 配置合并失败：项目未配置 K8s 部署")
					err = fmt.Errorf("k8s config merge failed: project has no kubernetes config")
				} else {
					// 向后兼容：如果版本有旧的 K8sYAML 字段，使用它
					if version.K8sYAML != "" && k8sConfig.YAML == "" {
						k8sConfig.YAML = version.K8sYAML
					}

					result, err = api.engine.ExecuteK8sDeployWithMergedConfig(
						clientID,
						task.Operation,
						version.Version,
						k8sConfig,
					)
				}
			case models.DeployTypeGitPull:
				// Git 拉取部署：使用项目的 GitPullConfig 和版本的 GitRef
				api.writeTaskLog(task.ID, clientID, "info", "git", "执行 Git 拉取部署...")

				if project.GitPullConfig == nil {
					api.writeTaskLog(task.ID, clientID, "error", "git", "项目未配置 Git 拉取部署")
					err = fmt.Errorf("project git pull config is nil")
				} else {
					// 计算实际使用的工作目录（与 ExecuteGitPullDeployTask 逻辑一致）
					// 项目配置优先，版本配置只在非默认值且项目配置为空时使用
					workDir := project.GitPullConfig.WorkDir
					if workDir == "" {
						if version.WorkDir != "" && version.WorkDir != "/opt/app" {
							workDir = version.WorkDir
						}
					}
					if workDir == "" {
						workDir = "/opt/app"
					}

					gitRef := version.GitRef
					if gitRef == "" {
						if project.GitPullConfig.Tag != "" {
							gitRef = project.GitPullConfig.Tag
						} else if project.GitPullConfig.Branch != "" {
							gitRef = project.GitPullConfig.Branch
						} else {
							gitRef = "main"
						}
					}
					api.writeTaskLog(task.ID, clientID, "info", "git", fmt.Sprintf("仓库: %s, 分支/Tag: %s, 工作目录: %s", project.GitPullConfig.RepoURL, gitRef, workDir))

					result, err = api.engine.ExecuteGitPullDeployTask(
						clientID,
						task.Operation,
						version.Version,
						project.GitPullConfig,
						version.WorkDir,
						version.GitRef,
						version.GitRefType,
					)

					// 记录 Git 输出（无论成功或失败）
					if result != "" {
						// 限制输出长度
						outputLog := result
						if len(outputLog) > 2000 {
							outputLog = outputLog[:2000] + "... (truncated)"
						}
						api.writeTaskLog(task.ID, clientID, "debug", "git", outputLog)
					}
				}
			default:
				// 脚本部署：根据操作类型选择脚本
				api.writeTaskLog(task.ID, clientID, "info", "script", "执行脚本部署...")
				var script string
				switch task.Operation {
				case models.OperationTypeInstall:
					script = version.InstallScript
				case models.OperationTypeUpdate:
					script = version.UpdateScript
				case models.OperationTypeRollback:
					script = version.RollbackScript
				case models.OperationTypeUninstall:
					script = version.UninstallScript
				}
				result, err = api.engine.ExecuteRemote(clientID, script, version.WorkDir)
			}

			finishTime := time.Now()

			// 解析 Container ID（仅容器部署时）
			var containerID string
			if project.Type == models.DeployTypeContainer && result != "" {
				containerID = parseContainerIDFromOutput(result)
			}

			// 更新结果
			var status string
			var errMsg string
			for i := range task.Results {
				if task.Results[i].ClientID == clientID {
					task.Results[i].FinishedAt = &finishTime
					task.Results[i].Duration = int(finishTime.Sub(startTime).Seconds())

					if err != nil {
						status = "failed"
						errMsg = err.Error()
						task.Results[i].Status = status
						task.Results[i].Error = errMsg
						task.Results[i].ContainerID = containerID // 即使失败也记录 Container ID
						task.FailedCount++

						// 记录失败日志
						api.writeTaskLog(task.ID, clientID, "error", "failed", "部署失败: "+errMsg)

						// 如果是升级操作且启用了自动回滚
						if task.Operation == models.OperationTypeUpdate && task.AutoRollback {
							api.writeTaskLog(task.ID, clientID, "warn", "rollback", "触发自动回滚...")
							go api.autoRollbackClient(task, &version, clientID)
						}
					} else {
						status = "success"
						task.Results[i].Status = status
						task.Results[i].Output = result
						task.Results[i].ContainerID = containerID
						task.SuccessCount++

						// 记录成功日志
						successMsg := "部署成功"
						if containerID != "" {
							successMsg += ", Container ID: " + containerID[:12]
						}
						api.writeTaskLog(task.ID, clientID, "info", "success", successMsg)
					}
					task.PendingCount--
				}
			}
			api.db.Save(task)

			// 记录部署日志
			api.recordDeployLog(task, &version, clientID, status, 0, result, errMsg, startTime, finishTime, isCanary, containerID)

			// 检查失败策略
			if err != nil {
				switch task.FailureStrategy {
				case "abort":
					api.writeTaskLog(task.ID, "", "error", "abort", "部署失败，根据失败策略终止任务")
					now := time.Now()
					task.Status = "failed"
					task.FinishedAt = &now
					api.db.Save(task)
					return
				case "pause":
					api.writeTaskLog(task.ID, "", "warn", "pause", "部署失败，根据失败策略暂停任务")
					task.Status = "paused"
					api.db.Save(task)
					return
				}
			}
		}
	}

	// 更新任务状态
	if task.Status != "canary" {
		now := time.Now()
		if task.FailedCount > 0 {
			task.Status = "failed"
			api.writeTaskLog(task.ID, "", "error", "complete", "部署任务完成，成功: "+strconv.Itoa(task.SuccessCount)+", 失败: "+strconv.Itoa(task.FailedCount))
		} else {
			task.Status = "completed"
			api.writeTaskLog(task.ID, "", "info", "complete", "部署任务全部成功，共 "+strconv.Itoa(task.SuccessCount)+" 个目标")
		}
		task.FinishedAt = &now

		// 部署成功后更新版本状态和部署计数
		if task.Status == "completed" && task.SuccessCount > 0 {
			// 更新版本状态为 active，并增加部署计数
			api.db.Model(&models.Version{}).
				Where("id = ?", task.VersionID).
				Updates(map[string]interface{}{
					"status":       "active",
					"deploy_count": gorm.Expr("deploy_count + ?", task.SuccessCount),
				})
		}

		// 触发全部发布完成回调
		// 获取环境名称
		var environmentName string
		var env models.Environment
		if err := api.db.First(&env, "id IN (SELECT environment_id FROM targets WHERE client_id = ?)", task.ClientIDs[0]).Error; err == nil {
			environmentName = env.Name
		}

		// 异步触发回调，避免阻塞部署流程
		go func() {
			ctx := context.Background()
			// 重新获取最新任务状态
			latestTask := &models.DeployTask{}
			if err := api.db.First(latestTask, "id = ?", task.ID).Error; err == nil {
				api.callbackMgr.TriggerFullCompleted(ctx, latestTask, &project, &version, environmentName)
			}
		}()
	} else {
		// 金丝雀阶段完成
		api.writeTaskLog(task.ID, "", "info", "canary", "金丝雀阶段完成，等待推广或手动触发全量发布")

		// 触发金丝雀完成回调
		// 获取环境名称
		var environmentName string
		var env models.Environment
		if err := api.db.First(&env, "id IN (SELECT environment_id FROM targets WHERE client_id = ?)", task.ClientIDs[0]).Error; err == nil {
			environmentName = env.Name
		}

		// 异步触发回调，避免阻塞部署流程
		go func() {
			ctx := context.Background()
			// 重新获取最新任务状态
			latestTask := &models.DeployTask{}
			if err := api.db.First(latestTask, "id = ?", task.ID).Error; err == nil {
				api.callbackMgr.TriggerCanaryCompleted(ctx, latestTask, &project, &version, environmentName)
			}
		}()

		// 如果配置了自动推广，则自动触发全量发布
		if task.CanaryAutoPromote {
			api.writeTaskLog(task.ID, "", "info", "canary", "自动推广已启用，将在 "+strconv.Itoa(task.CanaryDuration)+" 分钟后自动推广")
			// TODO: 实现延迟自动推广逻辑
		}
	}
	api.db.Save(task)
}

// promoteDeployTask 金丝雀全量发布
func (api *ReleaseAPI) promoteDeployTask(task *models.DeployTask) {
	// 继续执行剩余的非金丝雀节点
	task.Status = "running"
	api.db.Save(task)
	api.executeDeployTask(task)
}

// rollbackDeployTask 回滚部署任务
func (api *ReleaseAPI) rollbackDeployTask(task *models.DeployTask, version *models.Version) {
	script := version.RollbackScript
	if script == "" {
		return
	}

	// 只回滚已成功或失败的节点
	for i := range task.Results {
		if task.Results[i].Status == "success" || task.Results[i].Status == "failed" {
			clientID := task.Results[i].ClientID
			if api.engine != nil {
				_, err := api.engine.ExecuteRemote(clientID, script, version.WorkDir)
				if err != nil {
					task.Results[i].Error = "rollback failed: " + err.Error()
				} else {
					task.Results[i].Status = "rollback"
				}
			}
		}
	}

	task.Status = "cancelled"
	now := time.Now()
	task.FinishedAt = &now
	api.db.Save(task)
}

// autoRollbackClient 自动回滚单个客户端
func (api *ReleaseAPI) autoRollbackClient(task *models.DeployTask, version *models.Version, clientID string) {
	script := version.RollbackScript
	if script == "" {
		return
	}

	if api.engine != nil {
		api.engine.ExecuteRemote(clientID, script, version.WorkDir)
	}
}

// ==================== 部署日志管理 ====================

// DeployLogWithProject 带项目名的部署日志
type DeployLogWithProject struct {
	models.DeployLog
	ProjectName string `json:"project_name"`
}

// ListDeployLogs 列出部署日志
func (api *ReleaseAPI) ListDeployLogs(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	status := c.Query("status")
	clientID := c.Query("client_id")

	// 构建查询条件
	query := api.db.Table("deploy_logs").
		Select("deploy_logs.*, projects.name as project_name").
		Joins("LEFT JOIN projects ON deploy_logs.project_id = projects.id")

	if status != "" {
		query = query.Where("deploy_logs.status = ?", status)
	}
	if clientID != "" {
		query = query.Where("deploy_logs.client_id = ?", clientID)
	}

	var total int64
	api.db.Model(&models.DeployLog{}).Count(&total)

	var logs []DeployLogWithProject
	if err := query.Order("deploy_logs.created_at DESC").
		Limit(limit).Offset(offset).
		Scan(&logs).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"logs": logs, "total": total})
}

// GetDeployLog 获取部署日志详情
func (api *ReleaseAPI) GetDeployLog(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	id := c.Param("id")
	var log models.DeployLog
	if err := api.db.First(&log, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "log not found")
		return
	}

	common.SuccessResp(c, log)
}

// ListProjectDeployLogs 列出项目的部署日志
func (api *ReleaseAPI) ListProjectDeployLogs(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	projectID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	status := c.Query("status")

	query := api.db.Model(&models.DeployLog{}).Where("project_id = ?", projectID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var logs []models.DeployLog
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"logs": logs, "total": total})
}

// GetProjectDeployStats 获取项目的部署统计
func (api *ReleaseAPI) GetProjectDeployStats(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	projectID := c.Param("id")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	since := time.Now().AddDate(0, 0, -days)

	var stats models.DeployStats
	api.db.Model(&models.DeployLog{}).
		Where("project_id = ? AND created_at >= ?", projectID, since).
		Count(&stats.TotalCount)

	api.db.Model(&models.DeployLog{}).
		Where("project_id = ? AND created_at >= ? AND status = ?", projectID, since, "success").
		Count(&stats.SuccessCount)

	api.db.Model(&models.DeployLog{}).
		Where("project_id = ? AND created_at >= ? AND status = ?", projectID, since, "failed").
		Count(&stats.FailedCount)

	if stats.TotalCount > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalCount) * 100
	}

	// 获取每日统计
	type DailyStat struct {
		Date         string `json:"date"`
		TotalCount   int64  `json:"total_count"`
		SuccessCount int64  `json:"success_count"`
		FailedCount  int64  `json:"failed_count"`
	}

	var dailyStats []DailyStat
	api.db.Model(&models.DeployLog{}).
		Select("DATE(created_at) as date, COUNT(*) as total_count, SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_count, SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_count").
		Where("project_id = ? AND created_at >= ?", projectID, since).
		Group("DATE(created_at)").
		Order("date DESC").
		Scan(&dailyStats)

	// 获取客户端统计
	type ClientStat struct {
		ClientID     string  `json:"client_id"`
		TotalCount   int64   `json:"total_count"`
		SuccessCount int64   `json:"success_count"`
		FailedCount  int64   `json:"failed_count"`
		SuccessRate  float64 `json:"success_rate"`
	}

	var clientStats []ClientStat
	api.db.Model(&models.DeployLog{}).
		Select("client_id, COUNT(*) as total_count, SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_count, SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_count").
		Where("project_id = ? AND created_at >= ?", projectID, since).
		Group("client_id").
		Order("total_count DESC").
		Scan(&clientStats)

	for i := range clientStats {
		if clientStats[i].TotalCount > 0 {
			clientStats[i].SuccessRate = float64(clientStats[i].SuccessCount) / float64(clientStats[i].TotalCount) * 100
		}
	}

	common.SuccessRespWithMsg(c, gin.H{}, "操作成功")
}

// GetDeployStats 获取整体部署统计
func (api *ReleaseAPI) GetDeployStats(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	since := time.Now().AddDate(0, 0, -days)

	var stats models.DeployStats
	api.db.Model(&models.DeployLog{}).
		Where("created_at >= ?", since).
		Count(&stats.TotalCount)

	api.db.Model(&models.DeployLog{}).
		Where("created_at >= ? AND status = ?", since, "success").
		Count(&stats.SuccessCount)

	api.db.Model(&models.DeployLog{}).
		Where("created_at >= ? AND status = ?", since, "failed").
		Count(&stats.FailedCount)

	if stats.TotalCount > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalCount) * 100
	}

	// 获取执行中的任务数量
	var runningCount int64
	api.db.Model(&models.DeployTask{}).
		Where("status IN ?", []string{"running", "canary"}).
		Count(&runningCount)

	// 获取项目统计
	type ProjectStat struct {
		ProjectID    string  `json:"project_id"`
		ProjectName  string  `json:"project_name"`
		TotalCount   int64   `json:"total_count"`
		SuccessCount int64   `json:"success_count"`
		FailedCount  int64   `json:"failed_count"`
		SuccessRate  float64 `json:"success_rate"`
	}

	var projectStats []ProjectStat
	api.db.Model(&models.DeployLog{}).
		Select("deploy_logs.project_id, projects.name as project_name, COUNT(*) as total_count, SUM(CASE WHEN deploy_logs.status = 'success' THEN 1 ELSE 0 END) as success_count, SUM(CASE WHEN deploy_logs.status = 'failed' THEN 1 ELSE 0 END) as failed_count").
		Joins("LEFT JOIN projects ON deploy_logs.project_id = projects.id").
		Where("deploy_logs.created_at >= ?", since).
		Group("deploy_logs.project_id, projects.name").
		Order("total_count DESC").
		Scan(&projectStats)

	for i := range projectStats {
		if projectStats[i].TotalCount > 0 {
			projectStats[i].SuccessRate = float64(projectStats[i].SuccessCount) / float64(projectStats[i].TotalCount) * 100
		}
	}

	common.SuccessRespWithMsg(c, gin.H{}, "操作成功")
}

// parseContainerIDFromOutput 从部署输出中解析 Container ID
// 查找格式: DEPLOY_CONTAINER_ID=<id>
func parseContainerIDFromOutput(output string) string {
	re := regexp.MustCompile(`DEPLOY_CONTAINER_ID=([a-f0-9]{64}|[a-f0-9]{12})`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// writeTaskLog 写入部署任务实时日志
func (api *ReleaseAPI) writeTaskLog(taskID, clientID, level, stage, message string) {
	if api.db == nil {
		return
	}

	log := &models.DeployTaskLog{
		TaskID:    taskID,
		ClientID:  clientID,
		Level:     level,
		Stage:     stage,
		Message:   message,
		Timestamp: time.Now(),
	}

	api.db.Create(log)
}

// recordDeployLog 记录部署日志
func (api *ReleaseAPI) recordDeployLog(task *models.DeployTask, version *models.Version, clientID string, status string, exitCode int, output, errMsg string, startedAt, finishedAt time.Time, isCanary bool, containerID string) {
	log := &models.DeployLog{
		TaskID:      task.ID,
		ProjectID:   task.ProjectID,
		VersionID:   task.VersionID,
		Version:     task.Version,
		ClientID:    clientID,
		Operation:   task.Operation,
		IsCanary:    isCanary,
		Status:      status,
		ExitCode:    exitCode,
		Output:      output,
		Error:       errMsg,
		ContainerID: containerID,
		StartedAt:   startedAt,
		FinishedAt:  finishedAt,
		Duration:    int(finishedAt.Sub(startedAt).Seconds()),
		CreatedBy:   task.CreatedBy,
	}

	api.db.Create(log)
}

// ==================== 部署任务日志 API ====================

// GetDeployTaskLogs 获取部署任务的执行日志
func (api *ReleaseAPI) GetDeployTaskLogs(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	taskID := c.Param("id")
	clientID := c.Query("client_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	query := api.db.Model(&models.DeployTaskLog{}).Where("task_id = ?", taskID)
	if clientID != "" {
		query = query.Where("client_id = ?", clientID)
	}

	var total int64
	query.Count(&total)

	var logs []models.DeployTaskLog
	if err := query.Order("timestamp ASC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"logs": logs, "total": total})
}

// StreamDeployTaskLogs 实时推送部署任务日志 (SSE)
func (api *ReleaseAPI) StreamDeployTaskLogs(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	taskID := c.Param("id")
	clientID := c.Query("client_id")

	// 验证任务存在
	var task models.DeployTask
	if err := api.db.First(&task, "id = ?", taskID).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "task not found")
		return
	}

	// 设置 SSE 响应头
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	// 记录上次读取的日志 ID
	var lastID string

	// 推送已有日志
	query := api.db.Model(&models.DeployTaskLog{}).Where("task_id = ?", taskID)
	if clientID != "" {
		query = query.Where("client_id = ?", clientID)
	}

	var existingLogs []models.DeployTaskLog
	query.Order("timestamp ASC").Find(&existingLogs)

	for _, log := range existingLogs {
		data := map[string]interface{}{
			"id":        log.ID,
			"client_id": log.ClientID,
			"level":     log.Level,
			"stage":     log.Stage,
			"message":   log.Message,
			"timestamp": log.Timestamp,
		}
		c.SSEvent("log", data)
		lastID = log.ID
	}
	c.Writer.Flush()

	// 轮询新日志
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// 监听客户端关闭
	clientGone := c.Writer.CloseNotify()

	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			// 刷新任务状态
			api.db.First(&task, "id = ?", taskID)

			// 查询新日志
			newQuery := api.db.Model(&models.DeployTaskLog{}).Where("task_id = ?", taskID)
			if clientID != "" {
				newQuery = newQuery.Where("client_id = ?", clientID)
			}
			if lastID != "" {
				newQuery = newQuery.Where("id > ?", lastID)
			}

			var newLogs []models.DeployTaskLog
			newQuery.Order("timestamp ASC").Find(&newLogs)

			for _, log := range newLogs {
				data := map[string]interface{}{
					"id":        log.ID,
					"client_id": log.ClientID,
					"level":     log.Level,
					"stage":     log.Stage,
					"message":   log.Message,
					"timestamp": log.Timestamp,
				}
				c.SSEvent("log", data)
				lastID = log.ID
			}

			// 发送任务状态
			c.SSEvent("status", map[string]interface{}{
				"task_status":   task.Status,
				"success_count": task.SuccessCount,
				"failed_count":  task.FailedCount,
				"pending_count": task.PendingCount,
			})
			c.Writer.Flush()

			// 任务完成时发送结束事件
			if task.Status == "completed" || task.Status == "failed" || task.Status == "cancelled" {
				c.SSEvent("done", map[string]interface{}{
					"task_status": task.Status,
				})
				c.Writer.Flush()
				return
			}
		}
	}
}

// GetClientContainerLogs 获取指定客户端的容器日志
func (api *ReleaseAPI) GetClientContainerLogs(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	taskID := c.Param("id")
	clientID := c.Param("client_id")
	lines, _ := strconv.Atoi(c.DefaultQuery("lines", "100"))
	since := c.Query("since")

	// 获取任务信息
	var task models.DeployTask
	if err := api.db.First(&task, "id = ?", taskID).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "task not found")
		return
	}

	// 查找该客户端的执行结果
	var containerID string
	for _, result := range task.Results {
		if result.ClientID == clientID {
			containerID = result.ContainerID
			break
		}
	}

	if containerID == "" {
		// 尝试从部署日志中获取
		var deployLog models.DeployLog
		if err := api.db.First(&deployLog, "task_id = ? AND client_id = ?", taskID, clientID).Error; err == nil {
			containerID = deployLog.ContainerID
		}
	}

	if containerID == "" {
		common.ErrorResp(c, common.CodeServiceUnavailable, "container ID not found for this client")
		return
	}

	// 构建 docker logs 命令
	logsCmd := "docker logs"
	if lines > 0 {
		logsCmd += " --tail " + strconv.Itoa(lines)
	}
	if since != "" {
		logsCmd += " --since " + since
	}
	logsCmd += " " + containerID

	// 使用远程执行器获取日志
	if api.engine == nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "remote executor not available")
		return
	}

	output, err := api.engine.ExecuteRemote(clientID, logsCmd, "")
	if err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "failed to get container logs: ")
		return
	}
	_ = output // 日志输出暂不返回

	common.SuccessRespWithMsg(c, gin.H{}, "操作成功")
}

// ==================== 脚本验证 ====================

// ScriptValidationResult 脚本验证结果
type ScriptValidationResult struct {
	Valid   bool     `json:"valid"`
	Errors  []string `json:"errors,omitempty"`
	Warning string   `json:"warning,omitempty"`
}

// ValidateScript 验证 shell 脚本语法
func (api *ReleaseAPI) ValidateScript(c *gin.Context) {
	var req struct {
		Script string `json:"script" binding:"required"`
		Name   string `json:"name"` // 脚本名称，用于错误提示
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	result := validateShellScript(req.Script, req.Name)
	common.SuccessResp(c, gin.H{"result": result})
}

// validateShellScript 验证 shell 脚本
func validateShellScript(script, name string) *ScriptValidationResult {
	result := &ScriptValidationResult{Valid: true}

	if strings.TrimSpace(script) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "脚本内容不能为空")
		return result
	}

	// 检查是否有 shebang
	lines := strings.Split(script, "\n")
	hasShebang := false
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "#!") {
		hasShebang = true
	}

	if !hasShebang {
		result.Warning = "建议在脚本开头添加 shebang (如 #!/bin/bash)"
	}

	// 使用 bash -n 检查语法
	cmd := exec.Command("bash", "-n")
	cmd.Stdin = strings.NewReader(script)
	output, err := cmd.CombinedOutput()

	if err != nil {
		result.Valid = false
		// 解析错误信息
		errorOutput := strings.TrimSpace(string(output))
		if errorOutput != "" {
			// 格式化错误信息
			errorLines := strings.Split(errorOutput, "\n")
			for _, line := range errorLines {
				if strings.TrimSpace(line) != "" {
					// 移除 "bash: line X:" 前缀，保留有意义的错误信息
					if idx := strings.Index(line, ":"); idx != -1 {
						line = strings.TrimSpace(line[idx+1:])
						if idx2 := strings.Index(line, ":"); idx2 != -1 {
							line = strings.TrimSpace(line[idx2+1:])
						}
					}
					if name != "" {
						result.Errors = append(result.Errors, name+": "+line)
					} else {
						result.Errors = append(result.Errors, line)
					}
				}
			}
		}
		if len(result.Errors) == 0 {
			result.Errors = append(result.Errors, "脚本语法错误")
		}
	}

	return result
}

// validateAllScripts 验证所有脚本
func validateAllScripts(installScript, updateScript, rollbackScript, uninstallScript string) (bool, []string) {
	var allErrors []string

	// 安装脚本必须有
	if strings.TrimSpace(installScript) == "" {
		allErrors = append(allErrors, "安装脚本不能为空")
	} else {
		result := validateShellScript(installScript, "安装脚本")
		if !result.Valid {
			allErrors = append(allErrors, result.Errors...)
		}
	}

	// 其他脚本可选，但如果有内容则验证
	if strings.TrimSpace(updateScript) != "" {
		result := validateShellScript(updateScript, "升级脚本")
		if !result.Valid {
			allErrors = append(allErrors, result.Errors...)
		}
	}

	if strings.TrimSpace(rollbackScript) != "" {
		result := validateShellScript(rollbackScript, "回滚脚本")
		if !result.Valid {
			allErrors = append(allErrors, result.Errors...)
		}
	}

	if strings.TrimSpace(uninstallScript) != "" {
		result := validateShellScript(uninstallScript, "卸载脚本")
		if !result.Valid {
			allErrors = append(allErrors, result.Errors...)
		}
	}

	return len(allErrors) == 0, allErrors
}

// GetGitVersions 获取 Git 仓库版本信息
// 直接在 Server 端执行 git 命令，不需要选择 Client
func (api *ReleaseAPI) GetGitVersions(c *gin.Context) {
	var req struct {
		ProjectID       string `json:"project_id"`
		RepoURL         string `json:"repo_url"`
		AuthType        string `json:"auth_type"`
		SSHKey          string `json:"ssh_key"`
		Token           string `json:"token"`
		Username        string `json:"username"`
		Password        string `json:"password"`
		MaxTags         int    `json:"max_tags"`
		MaxCommits      int    `json:"max_commits"`
		IncludeBranches bool   `json:"include_branches"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 构建配置
	var repoURL, authType, sshKey, token, username, password string

	// 如果提供了 project_id，从项目获取配置
	if req.ProjectID != "" && api.db != nil {
		var project models.Project
		if err := api.db.First(&project, "id = ?", req.ProjectID).Error; err == nil {
			if project.GitPullConfig != nil {
				repoURL = project.GitPullConfig.RepoURL
				authType = project.GitPullConfig.AuthType
				sshKey = project.GitPullConfig.SSHKey
				token = project.GitPullConfig.Token
				username = project.GitPullConfig.Username
				password = project.GitPullConfig.Password
			}
		}
	}

	// 请求参数覆盖项目配置
	if req.RepoURL != "" {
		repoURL = req.RepoURL
	}
	if req.AuthType != "" {
		authType = req.AuthType
	}
	if req.SSHKey != "" {
		sshKey = req.SSHKey
	}
	if req.Token != "" {
		token = req.Token
	}
	if req.Username != "" {
		username = req.Username
	}
	if req.Password != "" {
		password = req.Password
	}

	// 验证必须有仓库地址
	if repoURL == "" {
		common.ErrorResp(c, common.CodeServiceUnavailable, "repo_url is required")
		return
	}

	// 设置默认值
	maxTags := req.MaxTags
	if maxTags <= 0 {
		maxTags = 20
	}
	maxCommits := req.MaxCommits
	if maxCommits <= 0 {
		maxCommits = 10
	}

	// 创建 Git 客户端并直接在 Server 端执行
	gitClient := git.NewClientWithAuth(authType, sshKey, token, username, password)

	result, err := gitClient.FetchVersions(c.Request.Context(), &git.FetchVersionsRequest{
		RepoURL:         repoURL,
		MaxTags:         maxTags,
		MaxCommits:      maxCommits,
		IncludeBranches: req.IncludeBranches,
	})

	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}
	_ = result // 版本信息暂不返回

	common.SuccessRespWithMsg(c, gin.H{}, "操作成功")
}

// ==================== 版本升级增强 API ====================

// ListProjectInstallations 查询项目下所有已安装目标
// GET /release/projects/:id/installations
func (api *ReleaseAPI) ListProjectInstallations(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	projectID := c.Param("id")
	sourceVersion := c.Query("source_version") // 可选：按源版本过滤

	var installations []models.TargetInstallation
	query := api.db.Where("project_id = ?", projectID)
	if sourceVersion != "" {
		query = query.Where("version = ?", sourceVersion)
	}

	if err := query.Find(&installations).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 关联查询目标和环境信息
	var result []models.InstallationInfo
	for _, inst := range installations {
		var target models.Target
		if err := api.db.First(&target, "id = ?", inst.TargetID).Error; err != nil {
			continue
		}

		var env models.Environment
		api.db.First(&env, "id = ?", target.EnvironmentID)

		info := models.InstallationInfo{
			ClientID:      target.ClientID,
			TargetID:      inst.TargetID,
			TargetName:    target.Name,
			Environment:   env.Name,
			EnvironmentID: env.ID,
			Version:       inst.Version,
			Status:        inst.Status,
			InstalledAt:   inst.InstalledAt,
		}
		if inst.LastUpdatedAt != nil {
			info.LastUpdatedAt = *inst.LastUpdatedAt
		} else {
			info.LastUpdatedAt = inst.InstalledAt
		}

		result = append(result, info)
	}

	common.SuccessRespWithMsg(c, gin.H{}, "操作成功")
}

// ==================== 进程上报 API ====================

// ReceiveProcessReport 接收进程上报
// POST /release/process-report
func (api *ReleaseAPI) ReceiveProcessReport(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	var req struct {
		ClientID   string              `json:"client_id" binding:"required"`
		ProjectID  string              `json:"project_id" binding:"required"`
		ReleaseID  string              `json:"release_id"`
		VersionID  string              `json:"version_id"`
		Version    string              `json:"version"`
		Processes  []models.ProcessInfo `json:"processes"`
		ReportedAt time.Time           `json:"reported_at"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	if req.ReportedAt.IsZero() {
		req.ReportedAt = time.Now()
	}

	report := &models.ProcessReport{
		ClientID:   req.ClientID,
		ProjectID:  req.ProjectID,
		VersionID:  req.VersionID,
		Version:    req.Version,
		ReleaseID:  req.ReleaseID,
		Processes:  req.Processes,
		ReportedAt: req.ReportedAt,
	}

	if err := api.db.Create(report).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"id": report})
}

// ListProjectProcesses 查询项目下所有客户端的进程状态
// GET /release/projects/:id/processes
func (api *ReleaseAPI) ListProjectProcesses(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	projectID := c.Param("id")

	// 获取每个客户端的最新进程上报
	var reports []models.ProcessReport
	subQuery := api.db.Model(&models.ProcessReport{}).
		Select("MAX(reported_at) as max_time, client_id").
		Where("project_id = ?", projectID).
		Group("client_id")

	if err := api.db.Model(&models.ProcessReport{}).
		Joins("JOIN (?) AS latest ON process_reports.client_id = latest.client_id AND process_reports.reported_at = latest.max_time", subQuery).
		Where("project_id = ?", projectID).
		Find(&reports).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	type ClientProcessInfo struct {
		ClientID   string              `json:"client_id"`
		Version    string              `json:"version"`
		Processes  []models.ProcessInfo `json:"processes"`
		LastReport time.Time           `json:"last_report"`
		Status     string              `json:"status"`
	}

	var result []ClientProcessInfo
	for _, report := range reports {
		status := "healthy"
		if len(report.Processes) == 0 {
			status = "no_processes"
		}

		result = append(result, ClientProcessInfo{
			ClientID:   report.ClientID,
			Version:    report.Version,
			Processes:  report.Processes,
			LastReport: report.ReportedAt,
			Status:     status,
		})
	}

	common.SuccessResp(c, gin.H{"clients": result, "total": len(result)})
}

// ==================== 容器上报 API ====================

// ReceiveContainerReport 接收容器上报
// POST /release/container-report
func (api *ReleaseAPI) ReceiveContainerReport(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	var req struct {
		ClientID      string                `json:"client_id" binding:"required"`
		ProjectID     string                `json:"project_id"`
		Containers    []models.ContainerInfo `json:"containers"`
		DockerVersion string                `json:"docker_version"`
		TotalCount    int                   `json:"total_count"`
		RunningCount  int                   `json:"running_count"`
		ReportedAt    time.Time             `json:"reported_at"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	if req.ReportedAt.IsZero() {
		req.ReportedAt = time.Now()
	}

	report := &models.ContainerReport{
		ClientID:      req.ClientID,
		ProjectID:     req.ProjectID,
		Containers:    req.Containers,
		DockerVersion: req.DockerVersion,
		TotalCount:    req.TotalCount,
		RunningCount:  req.RunningCount,
		ReportedAt:    req.ReportedAt,
	}

	if err := api.db.Create(report).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"id": report})
}

// ListProjectContainers 查询项目下所有客户端的容器状态
// GET /release/projects/:id/containers
func (api *ReleaseAPI) ListProjectContainers(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	projectID := c.Param("id")

	// 获取项目的容器前缀
	var project models.Project
	if err := api.db.First(&project, "id = ?", projectID).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Project not found")
		return
	}

	prefix := ""
	if project.ContainerNaming != nil {
		prefix = project.ContainerNaming.Prefix
	}

	// 获取每个客户端的最新容器上报
	var reports []models.ContainerReport
	subQuery := api.db.Model(&models.ContainerReport{}).
		Select("MAX(reported_at) as max_time, client_id").
		Group("client_id")

	if err := api.db.Model(&models.ContainerReport{}).
		Joins("JOIN (?) AS latest ON container_reports.client_id = latest.client_id AND container_reports.reported_at = latest.max_time", subQuery).
		Find(&reports).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	type ClientContainerInfo struct {
		ClientID   string                 `json:"client_id"`
		Containers []models.ContainerInfo `json:"containers"`
		LastReport time.Time              `json:"last_report"`
	}

	var result []ClientContainerInfo
	totalContainers := 0
	runningCount := 0

	for _, report := range reports {
		// 过滤匹配前缀的容器
		var matchedContainers []models.ContainerInfo
		for _, container := range report.Containers {
			if prefix == "" || strings.HasPrefix(container.ContainerName, prefix) {
				matchedContainers = append(matchedContainers, container)
				totalContainers++
				if container.State == "running" {
					runningCount++
				}
			}
		}

		if len(matchedContainers) > 0 {
			result = append(result, ClientContainerInfo{
				ClientID:   report.ClientID,
				Containers: matchedContainers,
				LastReport: report.ReportedAt,
			})
		}
	}

	common.SuccessResp(c, gin.H{"project_id": projectID, "prefix": prefix, "summary": gin.H{"total_clients": len(result), "total_containers": totalContainers, "running_count": runningCount, "stopped_count": totalContainers - runningCount}, "clients": result})
}

// GetContainersOverview 全局容器概览
// GET /release/containers/overview
func (api *ReleaseAPI) GetContainersOverview(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	// 获取所有项目及其容器前缀
	var projects []models.Project
	if err := api.db.Find(&projects).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 获取每个客户端的最新容器上报
	var reports []models.ContainerReport
	subQuery := api.db.Model(&models.ContainerReport{}).
		Select("MAX(reported_at) as max_time, client_id").
		Group("client_id")

	api.db.Model(&models.ContainerReport{}).
		Joins("JOIN (?) AS latest ON container_reports.client_id = latest.client_id AND container_reports.reported_at = latest.max_time", subQuery).
		Find(&reports)

	// 按项目统计
	type ProjectStats struct {
		ProjectID   string `json:"project_id"`
		ProjectName string `json:"project_name"`
		Prefix      string `json:"prefix"`
		Count       int    `json:"count"`
		Running     int    `json:"running"`
	}

	projectStats := make(map[string]*ProjectStats)
	totalContainers := 0

	for _, project := range projects {
		prefix := ""
		if project.ContainerNaming != nil {
			prefix = project.ContainerNaming.Prefix
		}
		projectStats[project.ID] = &ProjectStats{
			ProjectID:   project.ID,
			ProjectName: project.Name,
			Prefix:      prefix,
		}
	}

	for _, report := range reports {
		for _, container := range report.Containers {
			totalContainers++
			// 尝试匹配项目
			for _, stats := range projectStats {
				if stats.Prefix != "" && strings.HasPrefix(container.ContainerName, stats.Prefix) {
					stats.Count++
					if container.State == "running" {
						stats.Running++
					}
					break
				}
			}
		}
	}

	// 转换为列表
	var byProject []ProjectStats
	for _, stats := range projectStats {
		if stats.Count > 0 {
			byProject = append(byProject, *stats)
		}
	}

	common.SuccessRespWithMsg(c, gin.H{}, "操作成功")
}

// ==================== 配置预览 API ====================

// PreviewDeployConfig 预览部署配置（显示合并后的最终配置）
// POST /release/config-preview
func (api *ReleaseAPI) PreviewDeployConfig(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	var req struct {
		ProjectID      string                     `json:"project_id" binding:"required"`
		VersionID      string                     `json:"version_id"`
		OverrideConfig *models.TaskOverrideConfig `json:"override_config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 获取项目
	var project models.Project
	if err := api.db.First(&project, "id = ?", req.ProjectID).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "project not found")
		return
	}

	// 获取版本配置（如果提供了版本 ID）
	var versionConfig *models.VersionDeployConfig
	if req.VersionID != "" {
		var version models.Version
		if err := api.db.First(&version, "id = ?", req.VersionID).Error; err != nil {
			common.ErrorResp(c, common.CodeServiceUnavailable, "version not found")
			return
		}
		versionConfig = version.DeployConfig
	}

	// 根据项目类型返回不同的合并配置
	switch project.Type {
	case models.DeployTypeContainer:
		if project.ContainerConfig == nil {
			common.ErrorResp(c, common.CodeServiceUnavailable, "project container config not found")
			return
		}
		finalConfig := models.MergeContainerConfig(
			project.ContainerConfig,
			versionConfig,
			req.OverrideConfig,
		)
		common.SuccessResp(c, gin.H{
			"success":      true,
			"deploy_type":  "container",
			"final_config": finalConfig,
			"config_sources": gin.H{
				"project":  project.ContainerConfig,
				"version":  versionConfig,
				"override": req.OverrideConfig,
			},
		})

	case models.DeployTypeKubernetes:
		if project.KubernetesConfig == nil {
			common.ErrorResp(c, common.CodeServiceUnavailable, "project kubernetes config not found")
			return
		}
		finalConfig := models.MergeK8sConfig(
			project.KubernetesConfig,
			versionConfig,
			req.OverrideConfig,
		)
		common.SuccessResp(c, gin.H{
			"success":      true,
			"deploy_type":  "kubernetes",
			"final_config": finalConfig,
			"config_sources": gin.H{
				"project":  project.KubernetesConfig,
				"version":  versionConfig,
				"override": req.OverrideConfig,
			},
		})

	case models.DeployTypeScript:
		// 脚本部署暂不支持配置合并
		common.SuccessResp(c, gin.H{
			"success":     true,
			"deploy_type": "script",
			"message":     "script deployment does not support config merge preview",
			"config_sources": gin.H{
				"project": project.ScriptConfig,
			},
		})

	case models.DeployTypeGitPull:
		// Git 拉取部署暂不支持配置合并
		common.SuccessResp(c, gin.H{
			"success":     true,
			"deploy_type": "gitpull",
			"message":     "gitpull deployment does not support config merge preview",
			"config_sources": gin.H{
				"project": project.GitPullConfig,
			},
		})

	default:
		common.ErrorResp(c, common.CodeServiceUnavailable, "unsupported deploy type: ")
	}
}
