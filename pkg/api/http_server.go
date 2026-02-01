package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/voilet/quic-flow/pkg/command"
	"github.com/voilet/quic-flow/pkg/hardware"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/profiling"
	"github.com/voilet/quic-flow/pkg/protocol"
	"github.com/voilet/quic-flow/pkg/session"
)

// ServerAPI 定义服务器需要提供的接口
type ServerAPI interface {
	ListClients() []string
	ListClientsWithDetails() []session.ClientInfoBrief
	ListClientsWithDetailsPaginated(offset, limit int) ([]session.ClientInfoBrief, int64)
	GetClientInfo(clientID string) (*protocol.ClientInfo, error)
	SendTo(clientID string, msg *protocol.DataMessage) error
	Broadcast(msg *protocol.DataMessage) (int, []error)
}

// HTTPServer HTTP API 服务器
type HTTPServer struct {
	server         *http.Server
	router         *gin.Engine
	serverAPI      ServerAPI
	commandManager *command.CommandManager
	hardwareStore  *hardware.Store // 硬件信息存储
	logger         *monitoring.Logger
	listenAddr     string
}

// NewHTTPServer 创建新的 HTTP API 服务器
func NewHTTPServer(addr string, serverAPI ServerAPI, commandManager *command.CommandManager, logger *monitoring.Logger) *HTTPServer {
	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	h := &HTTPServer{
		router:         gin.New(),
		serverAPI:      serverAPI,
		commandManager: commandManager,
		logger:         logger,
		listenAddr:     addr,
	}

	// 添加中间件
	h.router.Use(gin.Recovery())
	h.router.Use(h.loggerMiddleware())

	// 注册路由
	api := h.router.Group("/api")
	{
		api.GET("/clients", h.handleListClients)
		api.GET("/clients/:id", h.handleGetClient)
		api.POST("/send", h.handleSend)
		api.POST("/broadcast", h.handleBroadcast)

		// 命令相关接口
		api.POST("/command", h.handleSendCommand)
		api.POST("/command/multi", h.handleSendMultiCommand)              // 多播命令
		api.POST("/command/multi/:id/cancel", h.handleCancelMultiCommand) // 停止多播任务
		api.GET("/command/:id", h.handleGetCommand)
		api.GET("/commands", h.handleListCommands)

		// 容器管理接口
		api.POST("/containers/logs", h.handleContainerLogs) // 查看容器日志
	}
	h.router.GET("/health", h.handleHealth)

	h.server = &http.Server{
		Addr:    addr,
		Handler: h.router,
	}

	return h
}

// SetHardwareStore 设置硬件信息存储（供外部调用）
func (h *HTTPServer) SetHardwareStore(store *hardware.Store) {
	h.hardwareStore = store
}

// GetRouter 获取路由器（供外部注册路由）
func (h *HTTPServer) GetRouter() *gin.Engine {
	return h.router
}

// loggerMiddleware 日志中间件
func (h *HTTPServer) loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		h.logger.Debug("HTTP request",
			"method", method,
			"path", path,
			"status", statusCode,
			"latency", latency,
			"client_ip", c.ClientIP(),
		)
	}
}

// Start 启动 HTTP 服务器
func (h *HTTPServer) Start() error {
	h.logger.Info("Starting HTTP API server", "addr", h.listenAddr)

	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.Error("HTTP server error", "error", err)
		}
	}()

	return nil
}

// Stop 停止 HTTP 服务器
func (h *HTTPServer) Stop(ctx context.Context) error {
	h.logger.Info("Stopping HTTP API server...")
	return h.server.Shutdown(ctx)
}

// GetAPIGroup 返回 API 路由组，用于外部注册路由
func (h *HTTPServer) GetAPIGroup() *gin.RouterGroup {
	return h.router.Group("/api")
}

// ClientDetail 客户端详情（整合设备和会话信息）
type ClientDetail struct {
	// === 基本信息 ===
	ClientID  string `json:"client_id"`
	Hostname  string `json:"hostname"`  // 主机名
	OS        string `json:"os"`        // 操作系统
	Arch      string `json:"arch"`      // 架构

	// === 硬件信息 ===
	CPUModel      string  `json:"cpu_model"`       // CPU 型号
	CPUCores      int     `json:"cpu_cores"`       // CPU 核心数
	MemoryGB      float64 `json:"memory_gb"`       // 内存 GB
	DiskTB        float64 `json:"disk_tb"`         // 磁盘 TB
	PrimaryMAC    string  `json:"primary_mac"`     // 主 MAC 地址

	// === 在线状态 ===
	Online       bool   `json:"online"`           // 是否在线
	RemoteAddr   string `json:"remote_addr"`      // 远程地址
	ConnectedAt  int64  `json:"connected_at"`     // 连接时间（毫秒）
	Uptime       string `json:"uptime"`           // 在线时长
	LastSeenAt   *int64 `json:"last_seen_at"`     // 最后在线时间

	// === 状态 ===
	Status       string `json:"status"`           // online, offline, unknown
	FirstSeenAt  int64  `json:"first_seen_at"`    // 首次发现时间
}

// ListClientsResponse 客户端列表响应
type ListClientsResponse struct {
	Total        int64          `json:"total"`
	OnlineCount  int64          `json:"online_count"`  // 心跳在线数量
	Offset       int            `json:"offset,omitempty"`
	Limit        int            `json:"limit,omitempty"`
	Clients      []ClientDetail `json:"clients"`
}

const (
	// maxClientsPerRequest API 请求返回客户端数量的最大值
	// 防止在大规模连接场景下返回过大数据量导致内存/网络问题
	maxClientsPerRequest = 10000
)

// handleListClients 处理获取客户端列表请求
// 整合数据库设备信息和当前会话状态
// 支持分页: ?offset=0&limit=100
// 支持状态筛选: ?status=online
func (h *HTTPServer) handleListClients(c *gin.Context) {
	// 解析分页参数
	offset := 0
	limit := 0 // 0 表示全部

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	// ========== 性能优化：强制最大 limit ==========
	// 防止请求无限制数据导致内存/网络问题
	// 如果没有指定 limit 或 limit 超过最大值，使用默认值
	if limit == 0 || limit > maxClientsPerRequest {
		limit = maxClientsPerRequest
	}

	// 解析状态筛选
	statusFilter := c.Query("status") // online, offline, all

	// 获取当前在线的客户端（用于填充会话信息）
	onlineClients := make(map[string]session.ClientInfoBrief)
	for _, client := range h.serverAPI.ListClientsWithDetails() {
		onlineClients[client.ClientID] = client
	}

	now := time.Now()
	const heartbeatTimeout = 5 * time.Minute // 心跳超时时间

	var details []ClientDetail
	var total int64
	var onlineCount int64 // 在线节点计数

	// 如果有 hardware store，从数据库获取设备信息
	if h.hardwareStore != nil {
		var devices []hardware.Device
		var err error

		// 根据状态筛选
		if statusFilter == "online" {
			devices, total, err = h.hardwareStore.ListDevicesByStatus("online", offset, limit)
		} else if statusFilter == "offline" {
			devices, total, err = h.hardwareStore.ListDevicesByStatus("offline", offset, limit)
		} else {
			// 获取所有设备
			devices, total, err = h.hardwareStore.ListDevices(offset, limit)
		}

		// 用于记录已在数据库中的客户端ID
		dbClientIDs := make(map[string]bool)

		if err == nil {
			details = make([]ClientDetail, len(devices))
			for i, device := range devices {
				dbClientIDs[device.ClientID] = true

				// 基于心跳判断在线状态：最后心跳时间在超时时间内
				isOnline := false
				if device.LastSeenAt != nil {
					isOnline = now.Sub(*device.LastSeenAt) < heartbeatTimeout
				}

				// 如果当前有活跃会话，强制标记为在线
				if _, hasSession := onlineClients[device.ClientID]; hasSession {
					isOnline = true
				}

				if isOnline {
					onlineCount++
				}

				// 获取当前会话信息（如果在线）
				onlineClient, hasSession := onlineClients[device.ClientID]

				detail := ClientDetail{
					ClientID:     device.ClientID,
					Hostname:     device.Hostname,
					OS:           device.OS,
					Arch:         device.KernelArch,
					CPUModel:     device.CPUModel,
					CPUCores:     0, // 从 FullHardwareInfo 获取
					MemoryGB:     device.MemoryTotalGB,
					DiskTB:       device.DiskTotalTB,
					PrimaryMAC:   device.PrimaryMAC,
					Online:       isOnline, // 基于心跳判断
					Status:       device.Status,
					FirstSeenAt:  device.FirstSeenAt.UnixMilli(),
				}

				// 从 FullHardwareInfo 获取 CPU 核心数
				if device.FullHardwareInfo.CPUCoreCount > 0 {
					detail.CPUCores = device.FullHardwareInfo.CPUCoreCount
				}

				// 如果有当前会话，填充会话信息
				if hasSession {
					detail.RemoteAddr = onlineClient.RemoteAddr
					detail.ConnectedAt = onlineClient.ConnectedAt
					uptime := now.Sub(time.UnixMilli(onlineClient.ConnectedAt))
					detail.Uptime = uptime.Round(time.Second).String()
				}

				// 最后在线时间
				if device.LastSeenAt != nil {
					ts := device.LastSeenAt.UnixMilli()
					detail.LastSeenAt = &ts
				}

				details[i] = detail
			}
		}

		// 将当前在线但不在数据库中的客户端也加入结果
		for clientID, onlineClient := range onlineClients {
			if !dbClientIDs[clientID] {
				// 这个客户端在线但不在数据库中，可能是新连接的
				uptime := now.Sub(time.UnixMilli(onlineClient.ConnectedAt))
				detail := ClientDetail{
					ClientID:    clientID,
					Online:      true,
					Status:      "online",
					RemoteAddr:  onlineClient.RemoteAddr,
					ConnectedAt: onlineClient.ConnectedAt,
					Uptime:      uptime.Round(time.Second).String(),
				}
				details = append(details, detail)
				onlineCount++
				total++ // 增加总数
			}
		}
	} else {
		// 没有 database，只返回当前在线的客户端
		// ========== 性能优化：始终使用分页 API ==========
		// 即使没有指定 limit，也使用默认的最大值，避免一次性获取全部客户端
		clients, total := h.serverAPI.ListClientsWithDetailsPaginated(offset, limit)

		onlineCount = total
		details = make([]ClientDetail, len(clients))
		for i, client := range clients {
			uptime := now.Sub(time.UnixMilli(client.ConnectedAt))
			details[i] = ClientDetail{
				ClientID:    client.ClientID,
				Online:      true,
				RemoteAddr:  client.RemoteAddr,
				ConnectedAt: client.ConnectedAt,
				Uptime:      uptime.Round(time.Second).String(),
				Status:      "online",
			}
		}
	}

	response := ListClientsResponse{
		Total:       total,
		OnlineCount: onlineCount,
		Clients:     details,
	}

	// 只在分页时返回分页信息
	if limit > 0 {
		response.Offset = offset
		response.Limit = limit
	}

	// 添加在线节点数到响应头
	c.Header("X-Online-Count", fmt.Sprintf("%d", onlineCount))

	c.JSON(http.StatusOK, response)
}

// handleGetClient 处理获取单个客户端信息请求
func (h *HTTPServer) handleGetClient(c *gin.Context) {
	clientID := c.Param("id")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Client ID is required",
		})
		return
	}

	info, err := h.serverAPI.GetClientInfo(clientID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("Client not found: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, info)
}

// SendRequest 发送消息请求结构
type SendRequest struct {
	ClientID string `json:"client_id" binding:"required"`
	Type     string `json:"type"`
	Payload  string `json:"payload" binding:"required"`
	WaitAck  bool   `json:"wait_ack"`
}

// SendResponse 发送消息响应
type SendResponse struct {
	Success bool   `json:"success"`
	MsgID   string `json:"msg_id"`
	Message string `json:"message"`
}

// handleSend 处理发送消息请求
func (h *HTTPServer) handleSend(c *gin.Context) {
	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// 解析消息类型
	msgType := protocol.MessageType_MESSAGE_TYPE_COMMAND
	switch req.Type {
	case "command":
		msgType = protocol.MessageType_MESSAGE_TYPE_COMMAND
	case "event":
		msgType = protocol.MessageType_MESSAGE_TYPE_EVENT
	case "query":
		msgType = protocol.MessageType_MESSAGE_TYPE_QUERY
	case "response":
		msgType = protocol.MessageType_MESSAGE_TYPE_RESPONSE
	case "":
		// 使用默认值 command
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid message type: %s", req.Type),
		})
		return
	}

	// 构造消息
	msg := &protocol.DataMessage{
		MsgId:      uuid.New().String(),
		SenderId:   "server",
		ReceiverId: req.ClientID,
		Type:       msgType,
		Payload:    []byte(req.Payload),
		WaitAck:    req.WaitAck,
		Timestamp:  time.Now().UnixMilli(),
	}

	// 发送消息
	if err := h.serverAPI.SendTo(req.ClientID, msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to send message: %v", err),
		})
		return
	}

	h.logger.Info("Message sent via API", "client_id", req.ClientID, "msg_id", msg.MsgId, "type", req.Type)

	c.JSON(http.StatusOK, SendResponse{
		Success: true,
		MsgID:   msg.MsgId,
		Message: "Message sent successfully",
	})
}

// BroadcastRequest 广播消息请求结构
type BroadcastRequest struct {
	Type    string `json:"type"`
	Payload string `json:"payload" binding:"required"`
}

// BroadcastResponse 广播消息响应
type BroadcastResponse struct {
	Success      bool     `json:"success"`
	MsgID        string   `json:"msg_id"`
	Total        int      `json:"total"`
	SuccessCount int      `json:"success_count"`
	FailedCount  int      `json:"failed_count"`
	Errors       []string `json:"errors,omitempty"`
}

// handleBroadcast 处理广播消息请求
func (h *HTTPServer) handleBroadcast(c *gin.Context) {
	var req BroadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// 解析消息类型
	msgType := protocol.MessageType_MESSAGE_TYPE_EVENT
	switch req.Type {
	case "command":
		msgType = protocol.MessageType_MESSAGE_TYPE_COMMAND
	case "event":
		msgType = protocol.MessageType_MESSAGE_TYPE_EVENT
	case "query":
		msgType = protocol.MessageType_MESSAGE_TYPE_QUERY
	case "response":
		msgType = protocol.MessageType_MESSAGE_TYPE_RESPONSE
	case "":
		// 使用默认值 event
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid message type: %s", req.Type),
		})
		return
	}

	// 构造消息
	msg := &protocol.DataMessage{
		MsgId:     uuid.New().String(),
		SenderId:  "server",
		Type:      msgType,
		Payload:   []byte(req.Payload),
		WaitAck:   false,
		Timestamp: time.Now().UnixMilli(),
	}

	// 广播消息
	successCount, errors := h.serverAPI.Broadcast(msg)

	h.logger.Info("Message broadcast via API", "msg_id", msg.MsgId, "type", req.Type, "success", successCount, "failed", len(errors))

	response := BroadcastResponse{
		Success:      true,
		MsgID:        msg.MsgId,
		Total:        successCount + len(errors),
		SuccessCount: successCount,
		FailedCount:  len(errors),
	}

	if len(errors) > 0 {
		errMsgs := make([]string, len(errors))
		for i, err := range errors {
			errMsgs[i] = err.Error()
		}
		response.Errors = errMsgs
	}

	c.JSON(http.StatusOK, response)
}

// handleHealth 健康检查
func (h *HTTPServer) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}

// handleSendCommand 处理下发命令请求
func (h *HTTPServer) handleSendCommand(c *gin.Context) {
	if h.commandManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Command manager not initialized",
		})
		return
	}

	var req command.CommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// 设置默认超时
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// 下发命令
	cmd, err := h.commandManager.SendCommand(req.ClientID, req.CommandType, req.Payload, timeout)
	if err != nil {
		h.logger.Error("Failed to send command",
			"client_id", req.ClientID,
			"command_type", req.CommandType,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to send command: %v", err),
		})
		return
	}

	h.logger.Info("Command sent via API",
		"command_id", cmd.CommandID,
		"client_id", req.ClientID,
		"command_type", req.CommandType,
		"timeout", timeout,
	)

	c.JSON(http.StatusOK, command.CommandResponse{
		Success:   true,
		CommandID: cmd.CommandID,
		Message:   "Command sent successfully",
	})
}

// handleGetCommand 处理查询命令状态请求
func (h *HTTPServer) handleGetCommand(c *gin.Context) {
	if h.commandManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Command manager not initialized",
		})
		return
	}

	commandID := c.Param("id")
	if commandID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Command ID is required",
		})
		return
	}

	cmd, err := h.commandManager.GetCommand(commandID)
	if err != nil {
		c.JSON(http.StatusNotFound, command.CommandStatusResponse{
			Success: false,
			Error:   fmt.Sprintf("Command not found: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, command.CommandStatusResponse{
		Success: true,
		Command: cmd,
	})
}

// ListCommandsRequest 查询命令列表请求
type ListCommandsRequest struct {
	ClientID string                `form:"client_id"` // 可选：按客户端ID过滤
	Status   command.CommandStatus `form:"status"`    // 可选：按状态过滤
}

// ListCommandsResponse 查询命令列表响应
type ListCommandsResponse struct {
	Success  bool               `json:"success"`
	Total    int                `json:"total"`
	Commands []*command.Command `json:"commands"`
}

// handleListCommands 处理查询命令列表请求
func (h *HTTPServer) handleListCommands(c *gin.Context) {
	if h.commandManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Command manager not initialized",
		})
		return
	}

	var req ListCommandsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid query parameters: %v", err),
		})
		return
	}

	commands := h.commandManager.ListCommands(req.ClientID, req.Status)

	c.JSON(http.StatusOK, ListCommandsResponse{
		Success:  true,
		Total:    len(commands),
		Commands: commands,
	})
}

// handleSendMultiCommand 处理多播命令请求（同时下发到多个客户端）
func (h *HTTPServer) handleSendMultiCommand(c *gin.Context) {
	if h.commandManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Command manager not initialized",
		})
		return
	}

	var req command.MultiCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// 验证客户端列表
	if len(req.ClientIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "client_ids cannot be empty",
		})
		return
	}

	// 设置默认超时
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	h.logger.Info("Multi-command request received",
		"client_count", len(req.ClientIDs),
		"command_type", req.CommandType,
		"timeout", timeout,
	)

	// 下发多播命令
	response := h.commandManager.SendCommandToMultiple(req.ClientIDs, req.CommandType, req.Payload, timeout)

	h.logger.Info("Multi-command completed",
		"total", response.Total,
		"success", response.SuccessCount,
		"failed", response.FailedCount,
	)

	c.JSON(http.StatusOK, response)
}

// handleCancelMultiCommand 处理停止多播任务请求
func (h *HTTPServer) handleCancelMultiCommand(c *gin.Context) {
	if h.commandManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Command manager not initialized",
		})
		return
	}

	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Task ID is required",
		})
		return
	}

	// 取消任务
	err := h.commandManager.CancelMultiCommand(taskID)
	if err != nil {
		h.logger.Warn("Failed to cancel multi-command task",
			"task_id", taskID,
			"error", err,
		)
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("Failed to cancel task: %v", err),
		})
		return
	}

	h.logger.Info("Multi-command task cancelled via API",
		"task_id", taskID,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"task_id": taskID,
		"message": "Task cancelled successfully",
	})
}

// AddTerminalRoutes 添加 WebSocket 终端路由
func (h *HTTPServer) AddTerminalRoutes(tm *TerminalManager) {
	api := h.router.Group("/api/terminal")
	{
		// WebSocket 终端连接
		api.GET("/ws/:client_id", tm.HandleWebSocket)

		// 列出所有终端会话
		api.GET("/sessions", tm.HandleTerminalSessionsList)

		// 关闭指定终端会话
		api.DELETE("/sessions/:session_id", tm.HandleTerminalSessionClose)
	}
	h.logger.Info("Terminal API routes added")
}

// AddAuditRoutes adds audit API routes
func (h *HTTPServer) AddAuditRoutes(auditAPI *AuditAPI) {
	api := h.router.Group("/api")
	auditAPI.RegisterRoutes(api)
	h.logger.Info("Audit API routes added")
}

// AddRecordingRoutes adds recording API routes
func (h *HTTPServer) AddRecordingRoutes(recordingAPI *RecordingAPI) {
	api := h.router.Group("/api")
	recordingAPI.RegisterRoutes(api)
	h.logger.Info("Recording API routes added")
}

// ReleaseAPIRegistrar 发布API注册接口
type ReleaseAPIRegistrar interface {
	RegisterRoutes(r *gin.RouterGroup)
}

// AddReleaseRoutes adds release API routes
func (h *HTTPServer) AddReleaseRoutes(releaseAPI ReleaseAPIRegistrar) {
	api := h.router.Group("/api")
	releaseAPI.RegisterRoutes(api)
	h.logger.Info("Release API routes added")
}

// AddSetupRoutes 添加数据库初始化引导路由
func (h *HTTPServer) AddSetupRoutes(setupAPI *SetupAPI) {
	api := h.router.Group("/api")
	setupAPI.RegisterRoutes(api)
	h.logger.Info("Setup API routes added")
}

// AddProfilingRoutes 添加性能分析路由
func (h *HTTPServer) AddProfilingRoutes(profilingHandler *profiling.Handler) {
	api := h.router.Group("/api")
	profilingHandler.RegisterRoutes(api)
	h.logger.Info("Profiling API routes added")
}

// AddStandardProfilingRoutes 添加标准 pprof 路由（兼容 go tool pprof）
func (h *HTTPServer) AddStandardProfilingRoutes(stdHandler *profiling.StandardHandler) {
	stdHandler.RegisterRoutes(h.router)
	h.logger.Info("Standard pprof routes added at /debug/pprof/")
}

// ContainerLogsRequest 查看容器日志请求
type ContainerLogsRequest struct {
	ClientID      string `json:"client_id" binding:"required"`
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	Tail          int    `json:"tail"`
	Since         string `json:"since"`
	Until         string `json:"until"`
	Timestamps    bool   `json:"timestamps"`
}

// handleContainerLogs 处理查看容器日志请求
func (h *HTTPServer) handleContainerLogs(c *gin.Context) {
	if h.commandManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Command manager not initialized",
		})
		return
	}

	var req ContainerLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// 验证必须提供 container_id 或 container_name
	if req.ContainerID == "" && req.ContainerName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "container_id or container_name is required",
		})
		return
	}

	// 构造命令参数
	params := command.ContainerLogsParams{
		ContainerID:   req.ContainerID,
		ContainerName: req.ContainerName,
		Tail:          req.Tail,
		Since:         req.Since,
		Until:         req.Until,
		Timestamps:    req.Timestamps,
	}

	payloadBytes, err := json.Marshal(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to marshal params: %v", err),
		})
		return
	}

	// 设置超时（容器日志获取可能需要一点时间）
	timeout := 30 * time.Second

	// 下发命令
	cmd, err := h.commandManager.SendCommand(req.ClientID, command.CmdContainerLogs, payloadBytes, timeout)
	if err != nil {
		h.logger.Error("Failed to send container logs command",
			"client_id", req.ClientID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to send command: %v", err),
		})
		return
	}

	h.logger.Info("Container logs command sent",
		"command_id", cmd.CommandID,
		"client_id", req.ClientID,
		"container_id", req.ContainerID,
		"container_name", req.ContainerName,
	)

	c.JSON(http.StatusOK, command.CommandResponse{
		Success:   true,
		CommandID: cmd.CommandID,
		Message:   "Container logs command sent successfully",
	})
}
