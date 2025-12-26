package handlers

import (
	"github.com/voilet/quic-flow/pkg/command"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/router"
)

// Config 处理器配置
type Config struct {
	Version string                 // 客户端版本号
	Logger  *monitoring.Logger     // 可选，用于 Server 端处理器
}

// RegisterBuiltinHandlers 注册所有内置处理器
// 用法:
//
//	handlers.RegisterBuiltinHandlers(r, &handlers.Config{Version: "1.0.0"})
func RegisterBuiltinHandlers(r *router.Router, cfg *Config) {
	if cfg == nil {
		cfg = &Config{Version: "1.0.0"}
	}
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}

	// 初始化状态（用于 GetStatus）
	InitStatus(cfg.Version)

	// 注册内置处理器（简洁的函数式风格）
	r.Register(command.CmdExecShell, ExecShell)
	r.Register(command.CmdGetStatus, GetStatus)
}

// ============================================================================
// 命令类型常量导出（方便直接使用）
// ============================================================================

const (
	CmdExecShell  = command.CmdExecShell
	CmdGetStatus  = command.CmdGetStatus
	CmdSystemInfo = command.CmdSystemInfo
	CmdFileRead   = command.CmdFileRead
	CmdFileWrite  = command.CmdFileWrite
	CmdPing       = command.CmdPing
	CmdEcho       = command.CmdEcho
)
