package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/command"
	"github.com/voilet/quic-flow/pkg/common"
)

// StreamCommandRequest 流式命令请求
type StreamCommandRequest struct {
	ClientIDs   []string        `json:"client_ids" binding:"required"`
	CommandType string          `json:"command_type" binding:"required"`
	Payload     json.RawMessage `json:"payload"`
	Timeout     int             `json:"timeout"` // 秒
}

// StreamCommandEvent SSE 事件
type StreamCommandEvent struct {
	Type     string                       `json:"type"`              // "start", "progress", "result", "complete"
	TaskID   string                       `json:"task_id,omitempty"` // 任务ID（用于取消）
	ClientID string                       `json:"client_id,omitempty"`
	Result   *command.ClientCommandResult `json:"result,omitempty"`
	Summary  *StreamSummary               `json:"summary,omitempty"`
}

// StreamSummary 流式命令汇总
type StreamSummary struct {
	Total        int `json:"total"`
	SuccessCount int `json:"success_count"`
	FailedCount  int `json:"failed_count"`
	Duration     int `json:"duration_ms"`
}

// handleStreamMultiCommand 处理流式多播命令请求 (SSE)
func (h *HTTPServer) handleStreamMultiCommand(c *gin.Context) {
	if h.commandManager == nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Command manager not initialized")
		return
	}

	var req StreamCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	if len(req.ClientIDs) == 0 {
		common.ErrorResp(c, common.CodeInvalidParams, "client_ids cannot be empty")
		return
	}

	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 获取底层 ResponseWriter 并启用 Flusher
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		common.ErrorResp(c, common.CodeInternalError, "Streaming not supported")
		return
	}

	h.logger.Info("Stream multi-command request received",
		"client_count", len(req.ClientIDs),
		"command_type", req.CommandType,
		"timeout", timeout,
	)

	startTime := time.Now()
	total := len(req.ClientIDs)

	// 使用 SendCommandToMultiple 来获取 task_id 并支持取消
	// 在 goroutine 中执行，通过 channel 接收结果
	resultChan := make(chan *command.ClientCommandResult, total)
	taskIDChan := make(chan string, 1)

	// 启动后台任务执行
	go func() {
		response := h.commandManager.SendCommandToMultiple(req.ClientIDs, req.CommandType, req.Payload, timeout)

		// 发送 task_id
		taskIDChan <- response.TaskID

		// 将结果发送到通道
		for _, result := range response.Results {
			resultChan <- result
		}
		close(resultChan)
	}()

	// 等待并发送开始事件（包含 task_id）
	taskID := <-taskIDChan
	startEvent := StreamCommandEvent{
		Type:   "start",
		TaskID: taskID,
	}
	data, _ := json.Marshal(startEvent)
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	flusher.Flush()

	// 流式输出结果
	var successCount, failedCount int
	for result := range resultChan {
		// 更新统计
		if result.Status == command.CommandStatusCompleted {
			successCount++
		} else {
			failedCount++
		}

		event := StreamCommandEvent{
			Type:     "result",
			TaskID:   taskID, // 每个结果也包含 task_id
			ClientID: result.ClientID,
			Result:   result,
		}

		data, _ = json.Marshal(event)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	}

	// 发送完成事件
	duration := time.Since(startTime).Milliseconds()
	completeEvent := StreamCommandEvent{
		Type: "complete",
		Summary: &StreamSummary{
			Total:        total,
			SuccessCount: successCount,
			FailedCount:  failedCount,
			Duration:     int(duration),
		},
	}

	data, _ = json.Marshal(completeEvent)
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	flusher.Flush()

	h.logger.Info("Stream multi-command completed",
		"total", total,
		"success", successCount,
		"failed", failedCount,
		"duration_ms", duration,
	)
}

// waitForCommandResult 等待单个命令完成
func (h *HTTPServer) waitForCommandResult(commandID string, timeout time.Duration) *command.Command {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cmd, err := h.commandManager.GetCommand(commandID)
			if err != nil {
				return nil
			}

			switch cmd.Status {
			case command.CommandStatusCompleted, command.CommandStatusFailed, command.CommandStatusTimeout:
				return cmd
			}

			if time.Now().After(deadline) {
				return cmd
			}
		}
	}
}

// AddStreamRoutes 添加流式 API 路由
func (h *HTTPServer) AddStreamRoutes() {
	api := h.router.Group("/api")
	{
		api.POST("/command/stream", h.handleStreamMultiCommand)
		api.GET("/containers/logs/stream", h.handleContainerLogsStream)
	}
	h.logger.Info("Stream API routes registered")
}

// ContainerLogsStreamRequest 容器日志流式请求参数
type ContainerLogsStreamRequest struct {
	ClientID      string `form:"client_id" binding:"required"`
	ContainerID   string `form:"container_id"`
	ContainerName string `form:"container_name"`
	Tail          int    `form:"tail"`
	Timestamps    bool   `form:"timestamps"`
}

// ContainerLogsStreamEvent SSE 事件
type ContainerLogsStreamEvent struct {
	Type      string `json:"type"` // "start", "logs", "error", "complete"
	Logs      string `json:"logs,omitempty"`
	LineCount int    `json:"line_count,omitempty"`
	Error     string `json:"error,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// handleContainerLogsStream 处理容器日志流式请求 (SSE)
// GET /api/containers/logs/stream?client_id=xxx&container_id=xxx
func (h *HTTPServer) handleContainerLogsStream(c *gin.Context) {
	if h.commandManager == nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Command manager not initialized")
		return
	}

	var req ContainerLogsStreamRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	// 验证必须提供 container_id 或 container_name
	if req.ContainerID == "" && req.ContainerName == "" {
		common.ErrorResp(c, common.CodeInvalidParams, "container_id or container_name is required")
		return
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 获取底层 ResponseWriter 并启用 Flusher
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		common.ErrorResp(c, common.CodeInternalError, "Streaming not supported")
		return
	}

	h.logger.Info("Container logs stream started",
		"client_id", req.ClientID,
		"container_id", req.ContainerID,
		"container_name", req.ContainerName,
	)

	// 发送开始事件
	startEvent := ContainerLogsStreamEvent{
		Type:      "start",
		Timestamp: time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(startEvent)
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	flusher.Flush()

	// 构造命令参数
	tail := req.Tail
	if tail <= 0 {
		tail = 100
	}

	params := command.ContainerLogsParams{
		ContainerID:   req.ContainerID,
		ContainerName: req.ContainerName,
		Tail:          tail,
		Timestamps:    req.Timestamps,
	}

	// 记录已发送的日志行数，用于去重
	lastLineCount := 0
	lastLogs := ""

	// 轮询间隔
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// 监听客户端断开连接
	clientGone := c.Request.Context().Done()

	// 首次立即获取日志
	h.fetchAndSendLogs(c, flusher, req.ClientID, params, &lastLogs, &lastLineCount)

	// 持续轮询获取新日志
	for {
		select {
		case <-clientGone:
			h.logger.Info("Container logs stream client disconnected",
				"client_id", req.ClientID,
			)
			return
		case <-ticker.C:
			// 获取最新日志
			h.fetchAndSendLogs(c, flusher, req.ClientID, params, &lastLogs, &lastLineCount)
		}
	}
}

// fetchAndSendLogs 获取并发送容器日志
func (h *HTTPServer) fetchAndSendLogs(c *gin.Context, flusher http.Flusher, clientID string, params command.ContainerLogsParams, lastLogs *string, lastLineCount *int) {
	payloadBytes, err := json.Marshal(params)
	if err != nil {
		return
	}

	// 发送命令获取日志
	timeout := 10 * time.Second
	cmd, err := h.commandManager.SendCommand(clientID, command.CmdContainerLogs, payloadBytes, timeout)
	if err != nil {
		event := ContainerLogsStreamEvent{
			Type:      "error",
			Error:     fmt.Sprintf("Failed to send command: %v", err),
			Timestamp: time.Now().UnixMilli(),
		}
		data, _ := json.Marshal(event)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	// 等待命令完成
	result := h.waitForCommandResult(cmd.CommandID, timeout)
	if result == nil {
		return
	}

	if result.Status != command.CommandStatusCompleted {
		event := ContainerLogsStreamEvent{
			Type:      "error",
			Error:     fmt.Sprintf("Command failed: %s", result.Error),
			Timestamp: time.Now().UnixMilli(),
		}
		data, _ := json.Marshal(event)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	// 解析结果
	var logsResult command.ContainerLogsResult
	if err := json.Unmarshal(result.Result, &logsResult); err != nil {
		return
	}

	if !logsResult.Success {
		event := ContainerLogsStreamEvent{
			Type:      "error",
			Error:     logsResult.Error,
			Timestamp: time.Now().UnixMilli(),
		}
		data, _ := json.Marshal(event)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	// 检查是否有新日志（简单比较）
	if logsResult.Logs != *lastLogs {
		*lastLogs = logsResult.Logs
		*lastLineCount = logsResult.LineCount

		event := ContainerLogsStreamEvent{
			Type:      "logs",
			Logs:      logsResult.Logs,
			LineCount: logsResult.LineCount,
			Timestamp: time.Now().UnixMilli(),
		}
		data, _ := json.Marshal(event)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	}
}
