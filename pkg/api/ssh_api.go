package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/common"
)

// SSHConnectionInfoProvider 提供 SSH 连接信息的接口
type SSHConnectionInfoProvider interface {
	GetClientID() string
	GetConnectedAt() time.Time
	GetUptime() time.Duration
}

// SSHClientManagerAPI SSH 客户端管理器接口
type SSHClientManagerAPI interface {
	// Connect 连接到指定客户端的 SSH 服务
	Connect(clientID string, user, password string) error
	// Disconnect 断开到指定客户端的 SSH 连接
	Disconnect(clientID string) error
	// ExecuteCommand 在指定客户端上执行命令
	ExecuteCommand(clientID, command string) (string, error)
	// ExecuteCommandOneShot 一次性连接并执行命令（不保持连接）
	ExecuteCommandOneShot(clientID, user, password, command string) (string, error)
	// GetConnectionInfos 获取所有 SSH 连接信息
	GetConnectionInfos() []ConnectionInfoData
	// IsConnected 检查是否已建立 SSH 连接
	IsConnected(clientID string) bool
}

// ConnectionInfoData 连接信息数据结构
type ConnectionInfoData struct {
	ClientID    string
	ConnectedAt time.Time
	Uptime      time.Duration
}

// SSHConnectionInfo API 返回的 SSH 连接信息
type SSHConnectionInfo struct {
	ClientID    string `json:"client_id"`
	ConnectedAt string `json:"connected_at"`
	Uptime      string `json:"uptime"`
}

// AddSSHRoutes 添加 SSH 相关的 API 路由
func (h *HTTPServer) AddSSHRoutes(sshManager SSHClientManagerAPI) {
	if sshManager == nil {
		h.logger.Warn("SSH manager is nil, SSH routes not added")
		return
	}

	// 存储 SSH 管理器引用
	api := h.router.Group("/api/ssh")
	{
		// 建立持久 SSH 连接
		api.POST("/connect", func(c *gin.Context) {
			h.handleSSHConnect(c, sshManager)
		})

		// 断开 SSH 连接
		api.POST("/disconnect/:client_id", func(c *gin.Context) {
			h.handleSSHDisconnect(c, sshManager)
		})

		// 在已连接的客户端上执行命令
		api.POST("/exec/:client_id", func(c *gin.Context) {
			h.handleSSHExec(c, sshManager)
		})

		// 一次性连接并执行命令
		api.POST("/exec-oneshot", func(c *gin.Context) {
			h.handleSSHExecOneShot(c, sshManager)
		})

		// 列出所有 SSH 连接
		api.GET("/connections", func(c *gin.Context) {
			h.handleSSHListConnections(c, sshManager)
		})

		// 检查 SSH 连接状态
		api.GET("/status/:client_id", func(c *gin.Context) {
			h.handleSSHStatus(c, sshManager)
		})
	}

	h.logger.Info("SSH API routes added")
}

// SSHConnectRequest SSH 连接请求
type SSHConnectRequest struct {
	ClientID string `json:"client_id" binding:"required"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// SSHExecRequest SSH 命令执行请求
type SSHExecRequest struct {
	Command string `json:"command" binding:"required"`
}

// SSHExecOneShotRequest SSH 一次性执行请求
type SSHExecOneShotRequest struct {
	ClientID string `json:"client_id" binding:"required"`
	User     string `json:"user"`
	Password string `json:"password"`
	Command  string `json:"command" binding:"required"`
}

// handleSSHConnect 处理 SSH 连接请求
func (h *HTTPServer) handleSSHConnect(c *gin.Context, sshManager SSHClientManagerAPI) {
	var req SSHConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	// 检查是否已连接
	if sshManager.IsConnected(req.ClientID) {
		common.ErrorResp(c, common.CodeConflict, "SSH connection already exists")
		return
	}

	// 建立连接
	if err := sshManager.Connect(req.ClientID, req.User, req.Password); err != nil {
		h.logger.Error("SSH connect failed", "client_id", req.ClientID, "error", err)
		common.ErrorResp(c, common.CodeSSHConnectionFailed, err.Error())
		return
	}

	h.logger.Info("SSH connected", "client_id", req.ClientID)
	common.SuccessResp(c, gin.H{
		"client_id": req.ClientID,
		"message":   "SSH connection established",
	})
}

// handleSSHDisconnect 处理 SSH 断开请求
func (h *HTTPServer) handleSSHDisconnect(c *gin.Context, sshManager SSHClientManagerAPI) {
	clientID := c.Param("client_id")

	if err := sshManager.Disconnect(clientID); err != nil {
		h.logger.Error("SSH disconnect failed", "client_id", clientID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	h.logger.Info("SSH disconnected", "client_id", clientID)
	common.SuccessResp(c, gin.H{
		"client_id": clientID,
		"message":   "SSH connection closed",
	})
}

// handleSSHExec 处理 SSH 命令执行请求
func (h *HTTPServer) handleSSHExec(c *gin.Context, sshManager SSHClientManagerAPI) {
	clientID := c.Param("client_id")

	var req SSHExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	// 检查是否已连接
	if !sshManager.IsConnected(clientID) {
		common.ErrorResp(c, common.CodeSSHConnectionFailed, "SSH connection not found, please connect first")
		return
	}

	// 执行命令
	output, err := sshManager.ExecuteCommand(clientID, req.Command)
	if err != nil {
		h.logger.Error("SSH exec failed", "client_id", clientID, "command", req.Command, "error", err)
		common.ErrorRespWithData(c, common.CodeInternalError, gin.H{
			"error":     err.Error(),
			"output":    output,
			"client_id": clientID,
		}, "SSH command execution failed")
		return
	}

	h.logger.Info("SSH exec success", "client_id", clientID, "command", req.Command)
	common.SuccessResp(c, gin.H{
		"client_id": clientID,
		"command":   req.Command,
		"output":    output,
	})
}

// handleSSHExecOneShot 处理一次性 SSH 命令执行请求
func (h *HTTPServer) handleSSHExecOneShot(c *gin.Context, sshManager SSHClientManagerAPI) {
	var req SSHExecOneShotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	// 执行命令
	output, err := sshManager.ExecuteCommandOneShot(req.ClientID, req.User, req.Password, req.Command)
	if err != nil {
		h.logger.Error("SSH exec-oneshot failed", "client_id", req.ClientID, "command", req.Command, "error", err)
		common.ErrorRespWithData(c, common.CodeSSHConnectionFailed, gin.H{
			"error":     err.Error(),
			"output":    output,
			"client_id": req.ClientID,
		}, "SSH one-shot command execution failed")
		return
	}

	h.logger.Info("SSH exec-oneshot success", "client_id", req.ClientID, "command", req.Command)
	common.SuccessResp(c, gin.H{
		"client_id": req.ClientID,
		"command":   req.Command,
		"output":    output,
	})
}

// handleSSHListConnections 处理列出 SSH 连接请求
func (h *HTTPServer) handleSSHListConnections(c *gin.Context, sshManager SSHClientManagerAPI) {
	connections := sshManager.GetConnectionInfos()

	// 转换为 API 格式
	apiConnections := make([]SSHConnectionInfo, len(connections))
	for i, conn := range connections {
		apiConnections[i] = SSHConnectionInfo{
			ClientID:    conn.ClientID,
			ConnectedAt: conn.ConnectedAt.Format(time.RFC3339),
			Uptime:      conn.Uptime.String(),
		}
	}

	common.SuccessResp(c, gin.H{
		"connections": apiConnections,
		"count":       len(apiConnections),
	})
}

// handleSSHStatus 处理 SSH 状态查询请求
func (h *HTTPServer) handleSSHStatus(c *gin.Context, sshManager SSHClientManagerAPI) {
	clientID := c.Param("client_id")

	connected := sshManager.IsConnected(clientID)

	common.SuccessResp(c, gin.H{
		"client_id": clientID,
		"connected": connected,
	})
}
