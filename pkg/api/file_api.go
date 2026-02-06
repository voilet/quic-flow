package api

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/common"
	"github.com/voilet/quic-flow/pkg/filetransfer"
	"gorm.io/gorm"
)

// FileTransferAPI 文件传输 API
type FileTransferAPI struct {
	manager       *filetransfer.Manager
	uploadManager *filetransfer.UploadManager
	downloadMgr   *filetransfer.DownloadManager
	db            *gorm.DB
}

// NewFileTransferAPI 创建文件传输 API
func NewFileTransferAPI(
	manager *filetransfer.Manager,
	uploadMgr *filetransfer.UploadManager,
	downloadMgr *filetransfer.DownloadManager,
	db *gorm.DB,
) *FileTransferAPI {
	return &FileTransferAPI{
		manager:       manager,
		uploadManager: uploadMgr,
		downloadMgr:   downloadMgr,
		db:            db,
	}
}

// RegisterRoutes 注册路由
func (api *FileTransferAPI) RegisterRoutes(router *gin.RouterGroup) {
	file := router.Group("/file")
	{
		// 上传相关
		file.POST("/upload/init", api.handleInitUpload)
		file.POST("/upload/chunk", api.handleUploadChunk)
		file.POST("/upload/complete", api.handleCompleteUpload)
		file.DELETE("/upload/:task_id", api.handleCancelUpload)

		// 下载相关
		file.POST("/download/request", api.handleRequestDownload)
		file.GET("/download/:task_id", api.handleDownloadFile)
		file.POST("/download/resume", api.handleResumeDownload)
		file.DELETE("/download/:task_id", api.handleCancelDownload)

		// 状态查询
		file.GET("/transfer/:task_id/progress", api.handleGetProgress)
		file.POST("/transfer/batch-status", api.handleBatchStatus)
		file.GET("/transfers", api.handleListTransfers)
		file.GET("/transfer/:task_id", api.handleGetTask)

		// 文件管理
		file.GET("/list", api.handleListFiles)
		file.GET("/info/:file_id", api.handleGetFileInfo)
		file.DELETE("/:file_id", api.handleDeleteFile)
		file.PATCH("/:file_id/metadata", api.handleUpdateMetadata)

		// 配置查询
		file.GET("/quota", api.handleGetQuota)
		file.GET("/config", api.handleGetConfig)
	}
}

// handleInitUpload 初始化上传
func (api *FileTransferAPI) handleInitUpload(c *gin.Context) {
	// 调试日志
	fmt.Printf("[DEBUG] handleInitUpload called\n")

	var req filetransfer.InitUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("[DEBUG] Validation failed: %v\n", err)
		common.ErrorResp(c, common.CodeInvalidParams, "Invalid parameters: "+err.Error())
		return
	}

	fmt.Printf("[DEBUG] Validation passed, filename=%s, file_size=%d\n", req.Filename, req.FileSize)

	// 获取用户信息（从 JWT 或 session）
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}
	clientIP := c.ClientIP()

	fmt.Printf("[DEBUG] Calling InitUpload, userID=%s, clientIP=%s\n", userID, clientIP)
	resp, err := api.uploadManager.InitUpload(c.Request.Context(), &req, userID, clientIP)
	if err != nil {
		fmt.Printf("[DEBUG] InitUpload failed: %v\n", err)
		code := common.CodeInternalError
		if te, ok := err.(*filetransfer.TransferError); ok {
			switch te.Code {
			case filetransfer.ErrCodeFileTooLarge:
				code = common.CodeFileUploadFailed
			case filetransfer.ErrCodeStorageQuotaExceeded:
				code = common.CodeFileUploadFailed
			}
		}
		common.ErrorResp(c, code, err.Error())
		return
	}

	fmt.Printf("[DEBUG] InitUpload succeeded, task_id=%s\n", resp.TaskID)
	common.SuccessResp(c, resp)
}

// handleUploadChunk 上传分块
func (api *FileTransferAPI) handleUploadChunk(c *gin.Context) {
	// 获取参数
	taskID := c.Query("task_id")
	if taskID == "" {
		taskID = c.GetHeader("X-Task-ID")
	}
	offsetStr := c.Query("offset")
	sequenceStr := c.Query("sequence")

	if taskID == "" || offsetStr == "" || sequenceStr == "" {
		common.ErrorResp(c, common.CodeInvalidParams, "Missing required parameters: task_id, offset, sequence")
		return
	}

	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "Invalid offset: "+err.Error())
		return
	}

	sequence, err := strconv.ParseInt(sequenceStr, 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "Invalid sequence: "+err.Error())
		return
	}

	// 读取数据
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "Failed to read data: "+err.Error())
		return
	}

	req := &filetransfer.UploadChunkRequest{
		TaskID:   taskID,
		Offset:   offset,
		Sequence: sequence,
		Data:     data,
	}

	resp, err := api.uploadManager.UploadChunk(c.Request.Context(), req)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, resp)
}

// handleCompleteUpload 完成上传
func (api *FileTransferAPI) handleCompleteUpload(c *gin.Context) {
	var req filetransfer.CompleteUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "Invalid parameters: "+err.Error())
		return
	}

	resp, err := api.uploadManager.CompleteUpload(c.Request.Context(), &req)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	common.SuccessResp(c, resp)
}

// handleCancelUpload 取消上传
func (api *FileTransferAPI) handleCancelUpload(c *gin.Context) {
	taskID := c.Param("task_id")

	if err := api.uploadManager.CancelUpload(c.Request.Context(), taskID); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"task_id": taskID,
		"status":  "cancelled",
		"message": "Upload cancelled",
	})
}

// handleRequestDownload 请求下载
func (api *FileTransferAPI) handleRequestDownload(c *gin.Context) {
	var req filetransfer.RequestDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "Invalid parameters: "+err.Error())
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}
	clientIP := c.ClientIP()

	resp, err := api.downloadMgr.RequestDownload(c.Request.Context(), &req, userID, clientIP)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, resp)
}

// handleDownloadFile 下载文件（流式）
func (api *FileTransferAPI) handleDownloadFile(c *gin.Context) {
	taskID := c.Param("task_id")

	reader, fileInfo, err := api.downloadMgr.StartDownload(c.Request.Context(), taskID)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}
	defer reader.Close()

	// 设置响应头
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=\""+fileInfo.FileName+"\"")
	c.Header("Content-Length", strconv.FormatInt(fileInfo.FileSize, 10))
	if fileInfo.Checksum != "" {
		c.Header("X-Checksum", fileInfo.Checksum)
	}

	// 流式传输
	c.Status(http.StatusOK)
	io.Copy(c.Writer, reader)
}

// handleResumeDownload 恢复下载
func (api *FileTransferAPI) handleResumeDownload(c *gin.Context) {
	var req struct {
		TaskID string `json:"task_id" binding:"required"`
		Offset int64  `json:"offset"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "Invalid parameters: "+err.Error())
		return
	}

	if err := api.downloadMgr.ResumeDownload(c.Request.Context(), req.TaskID, req.Offset); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	progress, _ := api.manager.GetProgress(req.TaskID)

	common.SuccessResp(c, gin.H{
		"task_id":  req.TaskID,
		"status":   "resuming",
		"offset":   req.Offset,
		"progress": progress,
	})
}

// handleCancelDownload 取消下载
func (api *FileTransferAPI) handleCancelDownload(c *gin.Context) {
	taskID := c.Param("task_id")

	if err := api.downloadMgr.CancelDownload(c.Request.Context(), taskID); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"task_id": taskID,
		"status":  "cancelled",
		"message": "Download cancelled",
	})
}

// handleGetProgress 获取进度
func (api *FileTransferAPI) handleGetProgress(c *gin.Context) {
	taskID := c.Param("task_id")

	progress, ok := api.manager.GetProgress(taskID)
	if !ok {
		common.ErrorResp(c, common.CodeNotFound, "Task not found")
		return
	}

	common.SuccessResp(c, progress)
}

// handleBatchStatus 批量查询状态
func (api *FileTransferAPI) handleBatchStatus(c *gin.Context) {
	var req struct {
		TaskIDs []string `json:"task_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "Invalid parameters: "+err.Error())
		return
	}

	tasks := make([]gin.H, 0, len(req.TaskIDs))
	for _, taskID := range req.TaskIDs {
		progress, ok := api.manager.GetProgress(taskID)
		if ok {
			tasks = append(tasks, gin.H{
				"task_id":  taskID,
				"progress": progress,
			})
		}
	}

	common.SuccessResp(c, gin.H{
		"tasks": tasks,
	})
}

// handleListTransfers 列出传输历史
func (api *FileTransferAPI) handleListTransfers(c *gin.Context) {
	// 获取查询参数
	transferType := c.DefaultQuery("type", "all")
	status := c.DefaultQuery("status", "all")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	// 构建查询
	query := api.db.Model(&filetransfer.FileTransfer{})

	// 应用过滤器
	if transferType != "all" {
		query = query.Where("transfer_type = ?", transferType)
	}
	if status != "all" {
		query = query.Where("status = ?", status)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to count transfers: "+err.Error())
		return
	}

	// 排序
	orderClause := sortBy
	if sortOrder == "desc" {
		orderClause += " DESC"
	} else {
		orderClause += " ASC"
	}
	query = query.Order(orderClause)

	// 分页
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	// 查询数据
	var transfers []filetransfer.FileTransfer
	if err := query.Find(&transfers).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to query transfers: "+err.Error())
		return
	}

	// 转换为响应格式
	items := make([]gin.H, len(transfers))
	for i, t := range transfers {
		items[i] = gin.H{
			"id":                t.ID,
			"task_id":           t.TaskID,
			"file_name":         t.FileName,
			"file_path":         t.FilePath,
			"file_size":         t.FileSize,
			"file_hash":         t.FileHash,
			"type":              t.TransferType,
			"status":            t.Status,
			"progress":          t.Progress,
			"speed":             t.Speed,
			"bytes_transferred": t.BytesTransferred,
			"user_id":           t.UserID,
			"client_ip":         t.ClientIP,
			"error_message":     t.ErrorMessage,
			"started_at":        t.StartedAt,
			"completed_at":      t.CompletedAt,
			"created_at":        t.CreatedAt,
			"updated_at":        t.UpdatedAt,
		}
	}

	common.SuccessResp(c, gin.H{
		"total":     total,
		"page":      (offset / limit) + 1,
		"page_size": limit,
		"items":     items,
		"filters": gin.H{
			"type":       transferType,
			"status":     status,
			"sort_by":    sortBy,
			"sort_order": sortOrder,
		},
	})
}

// handleGetTask 获取任务详情
func (api *FileTransferAPI) handleGetTask(c *gin.Context) {
	taskID := c.Param("task_id")

	task, err := api.manager.GetTask(taskID)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, task)
}

// handleListFiles 列出文件
func (api *FileTransferAPI) handleListFiles(c *gin.Context) {
	prefix := c.DefaultQuery("path", "/")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	files, err := api.downloadMgr.ListFiles(c.Request.Context(), prefix, limit)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"path":  prefix,
		"files": files,
		"total": len(files),
	})
}

// handleGetFileInfo 获取文件信息
func (api *FileTransferAPI) handleGetFileInfo(c *gin.Context) {
	fileID := c.Param("file_id")

	// TODO: 实现获取文件信息
	_ = fileID

	common.SuccessResp(c, gin.H{})
}

// handleDeleteFile 删除文件
func (api *FileTransferAPI) handleDeleteFile(c *gin.Context) {
	fileID := c.Param("file_id")

	// TODO: 实现删除文件
	_ = fileID

	common.SuccessResp(c, gin.H{
		"file_id": fileID,
		"status":  "deleted",
	})
}

// handleUpdateMetadata 更新元数据
func (api *FileTransferAPI) handleUpdateMetadata(c *gin.Context) {
	fileID := c.Param("file_id")

	var req struct {
		Description string                 `json:"description"`
		Tags        []string               `json:"tags"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "Invalid parameters: "+err.Error())
		return
	}

	_ = fileID
	_ = req

	// TODO: 实现更新元数据

	common.SuccessResp(c, gin.H{
		"file_id":        fileID,
		"updated_fields": []string{"description", "tags"},
	})
}

// handleGetQuota 获取配额
func (api *FileTransferAPI) handleGetQuota(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	// 为匿名用户返回默认配额
	if userID == "anonymous" {
		common.SuccessResp(c, gin.H{
			"total":            107374182400, // 100GB
			"used":             0,
			"available":        107374182400,
			"usage_percentage": 0,
			"formatted": gin.H{
				"total":     "100 GB",
				"used":      "0 B",
				"available": "100 GB",
			},
		})
		return
	}

	quota, err := api.manager.GetUserQuotaInfo(c.Request.Context(), userID)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, quota)
}

// handleGetConfig 获取配置
func (api *FileTransferAPI) handleGetConfig(c *gin.Context) {
	config := api.manager.GetSystemConfig()

	common.SuccessResp(c, config)
}

// handleWebSocketProgress WebSocket 进度推送
func (api *FileTransferAPI) handleWebSocketProgress(c *gin.Context) {
	// TODO: 实现 WebSocket 进度推送
}
