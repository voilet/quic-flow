package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/router"
)

// ========================================
// Server端处理器
// 用于处理来自Client的消息
// ========================================

// ClientReportHandler 客户端上报处理器
type ClientReportHandler struct {
	logger *monitoring.Logger
}

// NewClientReportHandler 创建客户端上报处理器
func NewClientReportHandler(logger *monitoring.Logger) *ClientReportHandler {
	return &ClientReportHandler{
		logger: logger,
	}
}

// HandleStatusReport 处理客户端状态上报
func (h *ClientReportHandler) HandleStatusReport(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var report struct {
		ClientID  string                 `json:"client_id"`
		Status    string                 `json:"status"`
		Timestamp int64                  `json:"timestamp"`
		Metrics   map[string]interface{} `json:"metrics,omitempty"`
	}

	if err := json.Unmarshal(payload, &report); err != nil {
		return nil, fmt.Errorf("invalid status report: %w", err)
	}

	h.logger.Info("Client status report received",
		"client_id", report.ClientID,
		"status", report.Status,
	)

	// 这里可以添加存储逻辑、告警逻辑等

	return json.Marshal(map[string]interface{}{
		"received": true,
		"time":     time.Now().Format(time.RFC3339),
	})
}

// HandleLogUpload 处理客户端日志上传
func (h *ClientReportHandler) HandleLogUpload(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var logs struct {
		ClientID string   `json:"client_id"`
		Level    string   `json:"level"`
		Logs     []string `json:"logs"`
	}

	if err := json.Unmarshal(payload, &logs); err != nil {
		return nil, fmt.Errorf("invalid log upload: %w", err)
	}

	h.logger.Info("Client logs received",
		"client_id", logs.ClientID,
		"level", logs.Level,
		"count", len(logs.Logs),
	)

	// 这里可以添加日志存储逻辑

	return json.Marshal(map[string]interface{}{
		"received": true,
		"count":    len(logs.Logs),
	})
}

// HandleHeartbeat 处理应用层心跳（不同于QUIC层心跳）
func (h *ClientReportHandler) HandleHeartbeat(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var heartbeat struct {
		ClientID  string `json:"client_id"`
		Timestamp int64  `json:"timestamp"`
		Sequence  int64  `json:"sequence"`
	}

	if err := json.Unmarshal(payload, &heartbeat); err != nil {
		return nil, fmt.Errorf("invalid heartbeat: %w", err)
	}

	h.logger.Debug("Application heartbeat received",
		"client_id", heartbeat.ClientID,
		"sequence", heartbeat.Sequence,
	)

	return json.Marshal(map[string]interface{}{
		"ack":       true,
		"server_ts": time.Now().UnixMilli(),
	})
}

// CommandResultHandler 命令结果处理器（Client主动上报执行结果）
type CommandResultHandler struct {
	logger *monitoring.Logger
	// 可以添加结果存储回调
	onResult func(commandID string, result json.RawMessage, err error)
}

// NewCommandResultHandler 创建命令结果处理器
func NewCommandResultHandler(logger *monitoring.Logger) *CommandResultHandler {
	return &CommandResultHandler{
		logger: logger,
	}
}

// SetResultCallback 设置结果回调
func (h *CommandResultHandler) SetResultCallback(callback func(commandID string, result json.RawMessage, err error)) {
	h.onResult = callback
}

// Handle 处理命令执行结果
func (h *CommandResultHandler) Handle(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var result struct {
		CommandID string          `json:"command_id"`
		Success   bool            `json:"success"`
		Result    json.RawMessage `json:"result,omitempty"`
		Error     string          `json:"error,omitempty"`
	}

	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, fmt.Errorf("invalid command result: %w", err)
	}

	h.logger.Info("Command result received",
		"command_id", result.CommandID,
		"success", result.Success,
	)

	// 调用回调
	if h.onResult != nil {
		var err error
		if result.Error != "" {
			err = fmt.Errorf(result.Error)
		}
		h.onResult(result.CommandID, result.Result, err)
	}

	return json.Marshal(map[string]interface{}{
		"ack": true,
	})
}

// RegisterServerHandlers 注册服务器端处理器
func RegisterServerHandlers(r *router.Router, cfg *Config) {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.Logger == nil {
		cfg.Logger = monitoring.NewLogger(monitoring.LogLevelInfo, "text")
	}

	// 客户端上报处理器
	reportHandler := NewClientReportHandler(cfg.Logger)
	r.Register("report.status", reportHandler.HandleStatusReport)
	r.Register("report.logs", reportHandler.HandleLogUpload)
	r.Register("report.heartbeat", reportHandler.HandleHeartbeat)

	// 命令结果处理器
	resultHandler := NewCommandResultHandler(cfg.Logger)
	r.Register("command.result", resultHandler.Handle)

	cfg.Logger.Info("Server handlers registered",
		"commands", r.ListCommands(),
	)
}

// SetupServerRouter 创建并配置服务器端路由器
func SetupServerRouter(logger *monitoring.Logger) *router.Router {
	r := router.NewRouter(logger)

	// 添加中间件
	r.Use(router.RecoveryMiddleware(logger))
	r.Use(router.LoggingMiddleware(logger))

	// 注册服务器端处理器
	RegisterServerHandlers(r, &Config{
		Logger: logger,
	})

	return r
}
