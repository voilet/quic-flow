package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/voilet/QuicFlow/pkg/monitoring"
	"github.com/voilet/QuicFlow/pkg/router"
)

// SetupServerRouter 设置服务器路由器
// 用于处理来自 Client 的上报消息
func SetupServerRouter(logger *monitoring.Logger) *router.Router {
	r := router.NewRouter(logger)

	// ========================================
	// 添加全局中间件
	// ========================================
	r.Use(router.RecoveryMiddleware(logger)) // panic恢复
	r.Use(router.LoggingMiddleware(logger))  // 日志记录

	// ========================================
	// 注册消息处理器
	// ========================================

	// ping - 存活检测
	r.Register("ping", handleServerPing)

	// report.status - 客户端状态上报
	r.Register("report.status", handleStatusReport)

	// report.logs - 客户端日志上传
	r.Register("report.logs", handleLogUpload)

	// report.metrics - 客户端指标上报
	r.Register("report.metrics", handleMetricsReport)

	// report.heartbeat - 应用层心跳
	r.Register("report.heartbeat", handleAppHeartbeat)

	// command.result - 命令执行结果（异步回调）
	r.Register("command.result", handleCommandResult)

	// command.callback - 命令回调（need_callback=true时客户端主动上报）
	r.Register("command.callback", handleCommandCallback)

	// query.config - 查询配置
	r.Register("query.config", handleQueryConfig)

	logger.Info("✅ Server router initialized", "commands", r.ListCommands())

	return r
}

// ========================================
// 消息处理函数
// ========================================

// handleServerPing 处理ping
func handleServerPing(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]interface{}{
		"message": "pong from server",
		"time":    time.Now().Format(time.RFC3339),
	})
}

// handleStatusReport 处理客户端状态上报
func handleStatusReport(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var report struct {
		ClientID  string                 `json:"client_id"`
		Status    string                 `json:"status"`
		Timestamp int64                  `json:"timestamp"`
		Metrics   map[string]interface{} `json:"metrics,omitempty"`
	}

	if err := json.Unmarshal(payload, &report); err != nil {
		return nil, fmt.Errorf("invalid status report: %w", err)
	}

	// TODO: 存储状态到数据库或内存

	return json.Marshal(map[string]interface{}{
		"received": true,
		"time":     time.Now().Format(time.RFC3339),
	})
}

// handleLogUpload 处理日志上传
func handleLogUpload(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var logs struct {
		ClientID string   `json:"client_id"`
		Level    string   `json:"level"`
		Logs     []string `json:"logs"`
	}

	if err := json.Unmarshal(payload, &logs); err != nil {
		return nil, fmt.Errorf("invalid log upload: %w", err)
	}

	// TODO: 存储日志

	return json.Marshal(map[string]interface{}{
		"received": true,
		"count":    len(logs.Logs),
	})
}

// handleMetricsReport 处理指标上报
func handleMetricsReport(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var metrics struct {
		ClientID  string                 `json:"client_id"`
		Timestamp int64                  `json:"timestamp"`
		Metrics   map[string]interface{} `json:"metrics"`
	}

	if err := json.Unmarshal(payload, &metrics); err != nil {
		return nil, fmt.Errorf("invalid metrics report: %w", err)
	}

	// TODO: 存储指标到时序数据库

	return json.Marshal(map[string]interface{}{
		"received": true,
	})
}

// handleAppHeartbeat 处理应用层心跳
func handleAppHeartbeat(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var heartbeat struct {
		ClientID  string `json:"client_id"`
		Timestamp int64  `json:"timestamp"`
		Sequence  int64  `json:"sequence"`
	}

	if err := json.Unmarshal(payload, &heartbeat); err != nil {
		return nil, fmt.Errorf("invalid heartbeat: %w", err)
	}

	return json.Marshal(map[string]interface{}{
		"ack":       true,
		"server_ts": time.Now().UnixMilli(),
	})
}

// commandResults 存储命令结果的临时缓存
var commandResults = struct {
	sync.RWMutex
	data map[string]json.RawMessage
}{
	data: make(map[string]json.RawMessage),
}

// CallbackResult 回调结果
type CallbackResult struct {
	CallbackID  string          `json:"callback_id"`
	CommandType string          `json:"command_type"`
	Success     bool            `json:"success"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
	Duration    int64           `json:"duration_ms,omitempty"`
	ReceivedAt  time.Time       `json:"received_at"`
}

// commandCallbacks 存储回调结果的缓存
var commandCallbacks = struct {
	sync.RWMutex
	data      map[string]CallbackResult
	listeners map[string][]chan CallbackResult
}{
	data:      make(map[string]CallbackResult),
	listeners: make(map[string][]chan CallbackResult),
}

// notifyCallbackListeners 通知等待回调的监听器
func notifyCallbackListeners(callbackID string) {
	commandCallbacks.Lock()
	defer commandCallbacks.Unlock()

	result, ok := commandCallbacks.data[callbackID]
	if !ok {
		return
	}

	listeners := commandCallbacks.listeners[callbackID]
	for _, ch := range listeners {
		select {
		case ch <- result:
		default:
		}
	}
	delete(commandCallbacks.listeners, callbackID)
}

// WaitForCallback 等待回调结果
func WaitForCallback(callbackID string, timeout time.Duration) (*CallbackResult, error) {
	// 先检查是否已有结果
	commandCallbacks.RLock()
	if result, ok := commandCallbacks.data[callbackID]; ok {
		commandCallbacks.RUnlock()
		return &result, nil
	}
	commandCallbacks.RUnlock()

	// 注册监听器
	ch := make(chan CallbackResult, 1)
	commandCallbacks.Lock()
	commandCallbacks.listeners[callbackID] = append(commandCallbacks.listeners[callbackID], ch)
	commandCallbacks.Unlock()

	// 等待回调
	select {
	case result := <-ch:
		return &result, nil
	case <-time.After(timeout):
		// 清理监听器
		commandCallbacks.Lock()
		delete(commandCallbacks.listeners, callbackID)
		commandCallbacks.Unlock()
		return nil, fmt.Errorf("timeout waiting for callback: %s", callbackID)
	}
}

// GetCallbackResult 获取回调结果（非阻塞）
func GetCallbackResult(callbackID string) (*CallbackResult, bool) {
	commandCallbacks.RLock()
	defer commandCallbacks.RUnlock()
	result, ok := commandCallbacks.data[callbackID]
	if ok {
		return &result, true
	}
	return nil, false
}

// handleCommandResult 处理命令执行结果
func handleCommandResult(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var result struct {
		CommandID string          `json:"command_id"`
		Success   bool            `json:"success"`
		Result    json.RawMessage `json:"result,omitempty"`
		Error     string          `json:"error,omitempty"`
	}

	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, fmt.Errorf("invalid command result: %w", err)
	}

	// 存储结果
	commandResults.Lock()
	commandResults.data[result.CommandID] = payload
	commandResults.Unlock()

	return json.Marshal(map[string]interface{}{
		"ack": true,
	})
}

// handleCommandCallback 处理命令回调（need_callback=true时客户端主动上报）
func handleCommandCallback(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var callback struct {
		CallbackID  string          `json:"callback_id"`
		CommandType string          `json:"command_type"`
		Success     bool            `json:"success"`
		Result      json.RawMessage `json:"result,omitempty"`
		Error       string          `json:"error,omitempty"`
		Duration    int64           `json:"duration_ms,omitempty"`
	}

	if err := json.Unmarshal(payload, &callback); err != nil {
		return nil, fmt.Errorf("invalid callback payload: %w", err)
	}

	// 存储回调结果
	commandCallbacks.Lock()
	commandCallbacks.data[callback.CallbackID] = CallbackResult{
		CallbackID:  callback.CallbackID,
		CommandType: callback.CommandType,
		Success:     callback.Success,
		Result:      callback.Result,
		Error:       callback.Error,
		Duration:    callback.Duration,
		ReceivedAt:  time.Now(),
	}
	commandCallbacks.Unlock()

	// 通知等待的监听器（如果有）
	notifyCallbackListeners(callback.CallbackID)

	return json.Marshal(map[string]interface{}{
		"ack":         true,
		"callback_id": callback.CallbackID,
	})
}

// handleQueryConfig 处理配置查询
func handleQueryConfig(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var query struct {
		Keys []string `json:"keys"`
	}

	if err := json.Unmarshal(payload, &query); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	// 返回示例配置
	config := map[string]interface{}{
		"heartbeat_interval": 15,
		"log_level":          "info",
		"max_retry":          3,
	}

	// 过滤请求的keys
	if len(query.Keys) > 0 {
		filtered := make(map[string]interface{})
		for _, key := range query.Keys {
			if v, ok := config[key]; ok {
				filtered[key] = v
			}
		}
		config = filtered
	}

	return json.Marshal(map[string]interface{}{
		"config": config,
	})
}

// GetCommandResult 获取命令结果（供外部查询）
func GetCommandResult(commandID string) (json.RawMessage, bool) {
	commandResults.RLock()
	defer commandResults.RUnlock()
	result, ok := commandResults.data[commandID]
	return result, ok
}
