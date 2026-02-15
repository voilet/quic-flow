package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/common"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/task/scheduler"
	"github.com/voilet/quic-flow/pkg/task/store"
)

// TaskAPI 任务管理 API
type TaskAPI struct {
	taskManager *scheduler.TaskManager
	taskStore   store.TaskStore
	logger      *monitoring.Logger
}

// NewTaskAPI 创建任务管理 API
func NewTaskAPI(taskManager *scheduler.TaskManager, taskStore store.TaskStore, logger *monitoring.Logger) *TaskAPI {
	return &TaskAPI{
		taskManager: taskManager,
		taskStore:   taskStore,
		logger:      logger,
	}
}

// RegisterRoutes 注册路由
func (api *TaskAPI) RegisterRoutes(r *gin.RouterGroup) {
	tasks := r.Group("/tasks")
	{
		tasks.GET("", api.ListTasks)
		tasks.POST("", api.CreateTask)
		tasks.GET("/:id", api.GetTask)
		tasks.PUT("/:id", api.UpdateTask)
		tasks.DELETE("/:id", api.DeleteTask)
		tasks.POST("/:id/enable", api.EnableTask)
		tasks.POST("/:id/disable", api.DisableTask)
		tasks.POST("/:id/trigger", api.TriggerTask)
		tasks.GET("/:id/next-run", api.GetNextRunTime)
		// 新增 API - 执行统计
		tasks.GET("/:id/stats", api.GetTaskStats)
		tasks.POST("/:id/reset-stats", api.ResetTaskStats)
	}
	// 添加测试路由以验证注册是否成功
	r.GET("/tasks-test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Task API routes are registered"})
	})
}

// ListTasks 获取任务列表
func (api *TaskAPI) ListTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	statusStr := c.Query("status")
	keyword := c.Query("keyword")

	var status *int
	if statusStr != "" {
		s, err := strconv.Atoi(statusStr)
		if err == nil {
			status = &s
		}
	}

	params := &store.ListParams{
		Page:     page,
		PageSize: pageSize,
		Status:   status,
		Keyword:  keyword,
	}

	tasks, total, err := api.taskStore.List(c.Request.Context(), params)
	if err != nil {
		api.logger.Error("Failed to list tasks", "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"tasks":     tasks,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// CreateTask 创建任务
func (api *TaskAPI) CreateTask(c *gin.Context) {
	var req scheduler.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	task, err := api.taskManager.CreateTask(c.Request.Context(), &req)
	if err != nil {
		api.logger.Error("Failed to create task", "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, task)
}

// GetTask 获取任务详情
func (api *TaskAPI) GetTask(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid task id")
		return
	}

	task, err := api.taskStore.GetByID(c.Request.Context(), taskID)
	if err != nil {
		api.logger.Error("Failed to get task", "task_id", taskID, "error", err)
		common.ErrorResp(c, common.CodeTaskNotFound, "task not found")
		return
	}

	common.SuccessResp(c, task)
}

// UpdateTask 更新任务
func (api *TaskAPI) UpdateTask(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid task id")
		return
	}

	var req scheduler.UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	req.TaskID = taskID
	if err := api.taskManager.UpdateTask(c.Request.Context(), &req); err != nil {
		api.logger.Error("Failed to update task", "task_id", taskID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, struct{}{})
}

// DeleteTask 删除任务
func (api *TaskAPI) DeleteTask(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid task id")
		return
	}

	if err := api.taskManager.DeleteTask(c.Request.Context(), taskID); err != nil {
		api.logger.Error("Failed to delete task", "task_id", taskID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, struct{}{})
}

// EnableTask 启用任务
func (api *TaskAPI) EnableTask(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid task id")
		return
	}

	if err := api.taskManager.EnableTask(c.Request.Context(), taskID); err != nil {
		api.logger.Error("Failed to enable task", "task_id", taskID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, struct{}{})
}

// DisableTask 禁用任务
func (api *TaskAPI) DisableTask(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid task id")
		return
	}

	if err := api.taskManager.DisableTask(c.Request.Context(), taskID); err != nil {
		api.logger.Error("Failed to disable task", "task_id", taskID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, struct{}{})
}

// TriggerTask 手动触发任务
func (api *TaskAPI) TriggerTask(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid task id")
		return
	}

	if err := api.taskManager.TriggerTask(c.Request.Context(), taskID); err != nil {
		api.logger.Error("Failed to trigger task", "task_id", taskID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, struct{}{})
}

// GetNextRunTime 获取下次执行时间
func (api *TaskAPI) GetNextRunTime(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid task id")
		return
	}

	nextRun, err := api.taskManager.GetNextRunTime(taskID)
	if err != nil {
		api.logger.Error("Failed to get next run time", "task_id", taskID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"next_run_time": nextRun,
	})
}

// GetTaskStats 获取任务统计信息
func (api *TaskAPI) GetTaskStats(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid task id")
		return
	}

	stats, err := api.taskManager.GetTaskStats(c.Request.Context(), taskID)
	if err != nil {
		api.logger.Error("Failed to get task stats", "task_id", taskID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, stats)
}

// ResetTaskStats 重置任务统计信息
func (api *TaskAPI) ResetTaskStats(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid task id")
		return
	}

	if err := api.taskManager.ResetTaskStats(c.Request.Context(), taskID); err != nil {
		api.logger.Error("Failed to reset task stats", "task_id", taskID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, struct{}{})
}
