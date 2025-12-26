package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/voilet/quic-flow/pkg/command"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/router"
	"github.com/voilet/quic-flow/pkg/router/handlers"
)

// 客户端版本号
const ClientVersion = "1.0.0"

// SetupClientRouter 设置客户端路由器
// 用于处理来自 Server 的命令
func SetupClientRouter(logger *monitoring.Logger) *router.Router {
	r := router.NewRouter(logger)

	// ========================================
	// 添加全局中间件
	// ========================================
	r.Use(router.RecoveryMiddleware(logger)) // panic恢复
	r.Use(router.LoggingMiddleware(logger))  // 日志记录

	// ========================================
	// 注册内置处理器
	// ========================================
	handlers.RegisterBuiltinHandlers(r, &handlers.Config{
		Version: ClientVersion,
	})

	// ========================================
	// 注册自定义命令处理器
	// 使用 command.CmdXxx 常量保持 Server/Client 一致
	// ========================================

	// ping - 简单的存活检测
	r.Register(command.CmdPing, handlePing)

	// echo - 回显命令
	r.Register(command.CmdEcho, handleEcho)

	// system.info - 获取系统信息
	r.Register(command.CmdSystemInfo, handleSystemInfo)

	// system.env - 获取环境变量
	r.Register("system.env", handleGetEnv)

	// file.read - 读取文件
	r.Register(command.CmdFileRead, handleFileRead)

	// config.update - 更新配置
	r.Register(command.CmdConfigUpdate, handleConfigUpdate)

	logger.Info("✅ Client router initialized", "commands", r.ListCommands())

	return r
}

// ========================================
// 命令处理函数
// ========================================

// handlePing 处理ping命令
func handlePing(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]interface{}{
		"message": "pong",
		"time":    time.Now().Format(time.RFC3339),
	})
}

// handleEcho 回显命令
func handleEcho(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	// 直接返回收到的内容
	return payload, nil
}

// handleSystemInfo 获取系统信息
func handleSystemInfo(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	hostname, _ := os.Hostname()

	info := map[string]interface{}{
		"hostname":     hostname,
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
		"num_cpu":      runtime.NumCPU(),
		"num_goroutine": runtime.NumGoroutine(),
		"go_version":   runtime.Version(),
		"time":         time.Now().Format(time.RFC3339),
	}

	return json.Marshal(info)
}

// handleGetEnv 获取环境变量
func handleGetEnv(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(payload, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	value := os.Getenv(params.Name)
	return json.Marshal(map[string]string{
		"name":  params.Name,
		"value": value,
	})
}

// handleFileRead 读取文件（示例，实际使用需要添加安全检查）
func handleFileRead(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(payload, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// 安全检查（示例）
	// TODO: 添加路径白名单检查

	content, err := os.ReadFile(params.Path)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %w", err)
	}

	// 限制返回大小
	const maxSize = 10 * 1024
	if len(content) > maxSize {
		content = content[:maxSize]
	}

	return json.Marshal(map[string]interface{}{
		"path":    params.Path,
		"content": string(content),
		"size":    len(content),
	})
}

// handleConfigUpdate 更新配置（示例）
func handleConfigUpdate(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Key   string      `json:"key"`
		Value interface{} `json:"value"`
	}
	if err := json.Unmarshal(payload, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// TODO: 实际的配置更新逻辑

	return json.Marshal(map[string]interface{}{
		"success": true,
		"key":     params.Key,
		"message": "config updated",
	})
}
