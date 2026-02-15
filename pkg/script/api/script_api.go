package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/common"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/script/models"
	"github.com/voilet/quic-flow/pkg/script/store"
)

// ScriptAPI 脚本管理 API
type ScriptAPI struct {
	store  store.ScriptStore
	logger *monitoring.Logger
}

// NewScriptAPI 创建脚本管理 API
func NewScriptAPI(store store.ScriptStore, logger *monitoring.Logger) *ScriptAPI {
	return &ScriptAPI{
		store:  store,
		logger: logger,
	}
}

// RegisterRoutes 注册路由
func (api *ScriptAPI) RegisterRoutes(r *gin.RouterGroup) {
	scripts := r.Group("/scripts")
	{
		scripts.GET("", api.GetScripts)
		scripts.POST("", api.CreateScript)
		scripts.GET("/:id", api.GetScript)
		scripts.PUT("/:id", api.UpdateScript)
		scripts.DELETE("/:id", api.DeleteScript)
		scripts.GET("/:id/versions", api.GetVersions)
		scripts.POST("/:id/versions", api.CreateVersion)
		scripts.POST("/:id/execute", api.ExecuteScript)
		scripts.GET("/:id/executions", api.GetExecutions)
		scripts.GET("/executions/:execution_id", api.GetExecution)
	}
}

// GetScripts 获取脚本列表
func (api *ScriptAPI) GetScripts(c *gin.Context) {
	category := c.Query("category")
	status := c.Query("status")
	withStats := c.Query("with_stats") == "true"

	if withStats {
		scripts, err := api.store.ListWithStats(c.Request.Context(), category)
		if err != nil {
			api.logger.Error("Failed to list scripts with stats", "error", err)
			common.ErrorResp(c, common.CodeInternalError, err.Error())
			return
		}
		common.SuccessResp(c, scripts)
	} else {
		scripts, err := api.store.List(c.Request.Context(), category, status)
		if err != nil {
			api.logger.Error("Failed to list scripts", "error", err)
			common.ErrorResp(c, common.CodeInternalError, err.Error())
			return
		}
		common.SuccessResp(c, scripts)
	}
}

// CreateScript 创建脚本
func (api *ScriptAPI) CreateScript(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Interpreter string `json:"interpreter"`
		Content     string `json:"content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	// 检查名称是否已存在
	if existing, _ := api.store.GetByName(c.Request.Context(), req.Name); existing != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "脚本名称已存在")
		return
	}

	// 设置默认值
	category := models.ScriptCategory(req.Category)
	if category == "" {
		category = models.CategoryOther
	}
	interpreter := req.Interpreter
	if interpreter == "" {
		interpreter = "bash"
	}

	script := &models.Script{
		Name:        req.Name,
		Description: req.Description,
		Category:    category,
		Interpreter: interpreter,
		Content:     req.Content,
		Status:      models.ScriptStatusDraft,
	}

	if err := api.store.Create(c.Request.Context(), script); err != nil {
		api.logger.Error("Failed to create script", "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, script)
}

// GetScript 获取脚本详情
func (api *ScriptAPI) GetScript(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid script id")
		return
	}

	script, err := api.store.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		api.logger.Error("Failed to get script", "id", id, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"code":    common.CodeNotFound,
			"message": "script not found",
		})
		return
	}

	common.SuccessResp(c, script)
}

// UpdateScript 更新脚本
func (api *ScriptAPI) UpdateScript(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid script id")
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Category    *string `json:"category"`
		Interpreter *string `json:"interpreter"`
		Content     *string `json:"content"`
		Status      *string `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	script, err := api.store.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    common.CodeNotFound,
			"message": "script not found",
		})
		return
	}

	// 检查名称是否与其他脚本冲突
	if req.Name != nil && *req.Name != script.Name {
		if existing, _ := api.store.GetByName(c.Request.Context(), *req.Name); existing != nil {
			common.ErrorResp(c, common.CodeInvalidParams, "脚本名称已存在")
			return
		}
		script.Name = *req.Name
	}

	if req.Description != nil {
		script.Description = *req.Description
	}
	if req.Category != nil {
		script.Category = models.ScriptCategory(*req.Category)
	}
	if req.Interpreter != nil {
		script.Interpreter = *req.Interpreter
	}
	if req.Content != nil {
		script.Content = *req.Content
	}
	if req.Status != nil {
		script.Status = models.ScriptStatus(*req.Status)
	}

	if err := api.store.Update(c.Request.Context(), script); err != nil {
		api.logger.Error("Failed to update script", "id", id, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, script)
}

// DeleteScript 删除脚本
func (api *ScriptAPI) DeleteScript(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid script id")
		return
	}

	if err := api.store.Delete(c.Request.Context(), uint(id)); err != nil {
		api.logger.Error("Failed to delete script", "id", id, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"message": "script deleted",
	})
}

// GetVersions 获取脚本版本列表
func (api *ScriptAPI) GetVersions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid script id")
		return
	}

	versions, err := api.store.GetVersions(c.Request.Context(), uint(id))
	if err != nil {
		api.logger.Error("Failed to get script versions", "id", id, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, versions)
}

// CreateVersion 创建脚本版本
func (api *ScriptAPI) CreateVersion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid script id")
		return
	}

	var req struct {
		Version   string `json:"version" binding:"required"`
		Content   string `json:"content" binding:"required"`
		ChangeLog string `json:"change_log"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	// 检查脚本是否存在
	script, err := api.store.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    common.CodeNotFound,
			"message": "script not found",
		})
		return
	}

	version := &models.ScriptVersion{
		ScriptID:  script.ID,
		Version:   req.Version,
		Content:   req.Content,
		ChangeLog: req.ChangeLog,
	}

	if err := api.store.CreateVersion(c.Request.Context(), version); err != nil {
		api.logger.Error("Failed to create script version", "script_id", id, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 同时更新脚本内容
	script.Content = req.Content
	script.Status = models.ScriptStatusPublished
	_ = api.store.Update(c.Request.Context(), script)

	common.SuccessResp(c, version)
}

// ExecuteScript 执行脚本
func (api *ScriptAPI) ExecuteScript(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid script id")
		return
	}

	var req struct {
		VersionID uint     `json:"version_id"`
		ClientIDs []string `json:"client_ids" binding:"required"`
		Timeout   int      `json:"timeout"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	// 获取脚本
	script, err := api.store.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    common.CodeNotFound,
			"message": "script not found",
		})
		return
	}

	// 确定版本
	var versionID uint = req.VersionID
	if versionID == 0 {
		// 使用最新版本
		latestVersion, err := api.store.GetLatestVersion(c.Request.Context(), uint(id))
		if err != nil || latestVersion == nil {
			// 没有版本，使用当前脚本内容
			versionID = 0
		} else {
			versionID = latestVersion.ID
		}
	}

	// 序列化客户端ID列表
	clientIDsJSON, _ := json.Marshal(req.ClientIDs)

	timeout := req.Timeout
	if timeout == 0 {
		timeout = 300
	}

	execution := &models.ScriptExecution{
		ScriptID:    script.ID,
		VersionID:   versionID,
		TriggerType: "manual",
		ClientIDs:   string(clientIDsJSON),
		Status:      models.ExecutionStatusPending,
		Timeout:     timeout,
	}

	if err := api.store.CreateExecution(c.Request.Context(), execution); err != nil {
		api.logger.Error("Failed to create execution", "script_id", id, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// TODO: 实际执行脚本的逻辑（通过命令管理器发送到客户端）
	// 这里只是创建执行记录，实际执行需要与命令系统集成

	now := time.Now()
	execution.Status = models.ExecutionStatusRunning
	execution.StartedAt = &now
	_ = api.store.UpdateExecution(c.Request.Context(), execution)

	common.SuccessResp(c, gin.H{
		"execution_id": execution.ID,
		"script_id":    script.ID,
		"client_count": len(req.ClientIDs),
		"status":       execution.Status,
		"message":      "Script execution started",
	})
}

// GetExecutions 获取脚本执行记录列表
func (api *ScriptAPI) GetExecutions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid script id")
		return
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	executions, err := api.store.ListExecutions(c.Request.Context(), uint(id), limit)
	if err != nil {
		api.logger.Error("Failed to list executions", "script_id", id, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, executions)
}

// GetExecution 获取执行记录详情
func (api *ScriptAPI) GetExecution(c *gin.Context) {
	executionID, err := strconv.ParseUint(c.Param("execution_id"), 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid execution id")
		return
	}

	execution, err := api.store.GetExecution(c.Request.Context(), uint(executionID))
	if err != nil {
		api.logger.Error("Failed to get execution", "execution_id", executionID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"code":    common.CodeNotFound,
			"message": "execution not found",
		})
		return
	}

	common.SuccessResp(c, execution)
}
