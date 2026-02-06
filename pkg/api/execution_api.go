package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/common"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/task/store"
)

// ExecutionAPI 执行监控 API
type ExecutionAPI struct {
	executionStore store.ExecutionStore
	logger         *monitoring.Logger
}

// NewExecutionAPI 创建执行监控 API
func NewExecutionAPI(executionStore store.ExecutionStore, logger *monitoring.Logger) *ExecutionAPI {
	return &ExecutionAPI{
		executionStore: executionStore,
		logger:         logger,
	}
}

// RegisterRoutes 注册路由
func (api *ExecutionAPI) RegisterRoutes(r *gin.RouterGroup) {
	executions := r.Group("/executions")
	{
		executions.GET("", api.GetExecutionList)
		executions.GET("/:id", api.GetExecutionDetail)
		executions.GET("/:id/logs", api.GetExecutionLogs)
		executions.GET("/stats", api.GetExecutionStats)
	}
}

// GetExecutionList 获取执行记录列表
func (api *ExecutionAPI) GetExecutionList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	taskIDStr := c.Query("task_id")
	clientID := c.Query("client_id")
	statusStr := c.Query("status")
	keyword := c.Query("keyword")

	params := &store.ExecutionListParams{
		Page:     page,
		PageSize: pageSize,
		ClientID: clientID,
		Keyword:  keyword,
	}

	if taskIDStr != "" {
		taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
		if err == nil {
			params.TaskID = &taskID
		}
	}

	if statusStr != "" {
		status, err := strconv.Atoi(statusStr)
		if err == nil {
			params.Status = &status
		}
	}

	executions, total, err := api.executionStore.List(c.Request.Context(), params)
	if err != nil {
		api.logger.Error("Failed to list executions", "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"executions": executions,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
	})
}

// GetExecutionDetail 获取执行记录详情
func (api *ExecutionAPI) GetExecutionDetail(c *gin.Context) {
	executionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid execution id")
		return
	}

	execution, err := api.executionStore.GetByID(c.Request.Context(), executionID)
	if err != nil {
		api.logger.Error("Failed to get execution", "execution_id", executionID, "error", err)
		common.ErrorResp(c, common.CodeNotFound, "execution not found")
		return
	}

	common.SuccessResp(c, execution)
}

// GetExecutionLogs 获取执行日志
func (api *ExecutionAPI) GetExecutionLogs(c *gin.Context) {
	executionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid execution id")
		return
	}

	execution, err := api.executionStore.GetByID(c.Request.Context(), executionID)
	if err != nil {
		api.logger.Error("Failed to get execution", "execution_id", executionID, "error", err)
		common.ErrorResp(c, common.CodeNotFound, "execution not found")
		return
	}

	common.SuccessResp(c, gin.H{
		"output":    execution.Output,
		"error_msg": execution.ErrorMsg,
	})
}

// GetExecutionStats 获取执行统计
func (api *ExecutionAPI) GetExecutionStats(c *gin.Context) {
	taskIDStr := c.Query("task_id")
	clientID := c.Query("client_id")

	var taskID *int64
	if taskIDStr != "" {
		id, err := strconv.ParseInt(taskIDStr, 10, 64)
		if err == nil {
			taskID = &id
		}
	}

	params := &store.ExecutionListParams{
		Page:     1,
		PageSize: 1000, // 获取足够多的记录用于统计
		TaskID:   taskID,
		ClientID: clientID,
	}

	executions, _, err := api.executionStore.List(c.Request.Context(), params)
	if err != nil {
		api.logger.Error("Failed to get executions for stats", "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 计算统计信息
	stats := gin.H{
		"total":        len(executions),
		"success":      0,
		"failed":       0,
		"timeout":      0,
		"cancelled":    0,
		"running":      0,
		"pending":      0,
		"avg_duration": int64(0),
		"max_duration": int64(0),
		"min_duration": int64(0),
	}

	var totalDuration int64
	var count int
	for _, exec := range executions {
		switch exec.Status {
		case 3: // Success
			stats["success"] = stats["success"].(int) + 1
		case 4: // Failed
			stats["failed"] = stats["failed"].(int) + 1
		case 5: // Timeout
			stats["timeout"] = stats["timeout"].(int) + 1
		case 6: // Cancelled
			stats["cancelled"] = stats["cancelled"].(int) + 1
		case 2: // Running
			stats["running"] = stats["running"].(int) + 1
		case 1: // Pending
			stats["pending"] = stats["pending"].(int) + 1
		}

		if exec.Duration > 0 {
			totalDuration += int64(exec.Duration)
			count++
			if stats["max_duration"].(int64) == 0 || int64(exec.Duration) > stats["max_duration"].(int64) {
				stats["max_duration"] = int64(exec.Duration)
			}
			if stats["min_duration"].(int64) == 0 || int64(exec.Duration) < stats["min_duration"].(int64) {
				stats["min_duration"] = int64(exec.Duration)
			}
		}
	}

	if count > 0 {
		stats["avg_duration"] = totalDuration / int64(count)
	}

	common.SuccessResp(c, stats)
}
