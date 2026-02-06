package api

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/batch"
	"github.com/voilet/quic-flow/pkg/common"
)

// BatchAPI 批量执行 API 扩展
type BatchAPI struct {
	executor *batch.BatchExecutor
}

// NewBatchAPI 创建批量执行 API
func NewBatchAPI(executor *batch.BatchExecutor) *BatchAPI {
	return &BatchAPI{
		executor: executor,
	}
}

// RegisterRoutes 注册批量执行相关路由
func (b *BatchAPI) RegisterRoutes(api *gin.RouterGroup) {
	batchGroup := api.Group("/batch")
	{
		batchGroup.POST("/execute", b.handleExecute)
		batchGroup.GET("/jobs", b.handleListJobs)
		batchGroup.GET("/jobs/:id", b.handleGetJob)
		batchGroup.POST("/jobs/:id/cancel", b.handleCancelJob)
		batchGroup.GET("/stats", b.handleStats)
	}
}

// BatchExecuteRequest 批量执行请求
type BatchExecuteRequest struct {
	Command       string          `json:"command" binding:"required"` // 命令类型
	Payload       json.RawMessage `json:"payload"`                    // 命令参数
	TargetClients []string        `json:"target_clients"`             // 目标客户端（空表示全部）
	WaitForResult bool            `json:"wait_for_result"`            // 是否等待执行结果
	Timeout       int             `json:"timeout"`                    // 超时时间（秒）
}

// BatchExecuteResponse 批量执行响应
type BatchExecuteResponse struct {
	JobID string        `json:"job_id,omitempty"`
	Job   *BatchJobInfo `json:"job,omitempty"`
}

// BatchJobInfo 任务信息（用于 API 响应）
type BatchJobInfo struct {
	ID           string               `json:"id"`
	Status       batch.BatchJobStatus `json:"status"`
	Command      string               `json:"command"`
	CreatedAt    time.Time            `json:"created_at"`
	TotalCount   int64                `json:"total_count"`
	SuccessCount int64                `json:"success_count"`
	FailedCount  int64                `json:"failed_count"`
	PendingCount int64                `json:"pending_count"`
	Progress     float64              `json:"progress"`
	Duration     string               `json:"duration,omitempty"`
}

// toJobInfo 转换为 API 响应格式
func toJobInfo(job *batch.BatchJob) *BatchJobInfo {
	if job == nil {
		return nil
	}

	progress := float64(0)
	if job.TotalCount > 0 {
		progress = float64(job.SuccessCount+job.FailedCount) / float64(job.TotalCount) * 100
	}

	return &BatchJobInfo{
		ID:           job.ID,
		Status:       job.Status,
		Command:      job.Command,
		CreatedAt:    job.CreatedAt,
		TotalCount:   job.TotalCount,
		SuccessCount: job.SuccessCount,
		FailedCount:  job.FailedCount,
		PendingCount: job.PendingCount,
		Progress:     progress,
	}
}

// handleExecute 处理批量执行请求
func (b *BatchAPI) handleExecute(c *gin.Context) {
	if b.executor == nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Batch executor not initialized")
		return
	}

	var req BatchExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	// 创建任务
	job, err := b.executor.Execute(req.Command, req.Payload, req.TargetClients, req.WaitForResult)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, BatchExecuteResponse{
		JobID: job.ID,
		Job:   toJobInfo(job),
	})
}

// handleListJobs 处理获取任务列表请求
func (b *BatchAPI) handleListJobs(c *gin.Context) {
	if b.executor == nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Batch executor not initialized")
		return
	}

	jobs := b.executor.ListJobs()
	jobInfos := make([]*BatchJobInfo, len(jobs))
	for i, job := range jobs {
		jobInfos[i] = toJobInfo(job)
	}

	common.SuccessResp(c, gin.H{
		"total": len(jobs),
		"jobs":  jobInfos,
	})
}

// handleGetJob 处理获取单个任务请求
func (b *BatchAPI) handleGetJob(c *gin.Context) {
	if b.executor == nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Batch executor not initialized")
		return
	}

	jobID := c.Param("id")
	job, found := b.executor.GetJob(jobID)
	if !found {
		common.ErrorResp(c, common.CodeTaskNotFound, "任务不存在")
		return
	}

	// 获取详细结果
	var results []*batch.TaskResult
	job.Results.Range(func(key, value interface{}) bool {
		results = append(results, value.(*batch.TaskResult))
		return true
	})

	var errors []string
	job.Errors.Range(func(key, value interface{}) bool {
		errors = append(errors, value.(error).Error())
		return true
	})

	common.SuccessResp(c, gin.H{
		"job":     toJobInfo(job),
		"results": results,
		"errors":  errors,
	})
}

// handleCancelJob 处理取消任务请求
func (b *BatchAPI) handleCancelJob(c *gin.Context) {
	if b.executor == nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Batch executor not initialized")
		return
	}

	jobID := c.Param("id")
	if b.executor.CancelJob(jobID) {
		common.SuccessResp(c, struct{}{})
	} else {
		common.ErrorResp(c, common.CodeTaskNotFound, "Failed to cancel job (not found or already completed)")
	}
}

// handleStats 处理获取统计信息请求
func (b *BatchAPI) handleStats(c *gin.Context) {
	if b.executor == nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Batch executor not initialized")
		return
	}

	stats := b.executor.GetStats()
	common.SuccessResp(c, stats)
}

// AddBatchRoutes 向 HTTPServer 添加批量执行路由
func (h *HTTPServer) AddBatchRoutes(executor *batch.BatchExecutor) {
	batchAPI := NewBatchAPI(executor)
	api := h.router.Group("/api")
	batchAPI.RegisterRoutes(api)
	h.logger.Info("Batch API routes registered")
}
