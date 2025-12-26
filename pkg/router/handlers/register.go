package handlers

import (
	"github.com/voilet/QuicFlow/pkg/monitoring"
	"github.com/voilet/QuicFlow/pkg/router"
)

// Config 处理器配置
type Config struct {
	Logger  *monitoring.Logger
	Version string // 客户端版本号
}

// RegisterBuiltinHandlers 注册所有内置处理器
func RegisterBuiltinHandlers(r *router.Router, cfg *Config) {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.Logger == nil {
		cfg.Logger = monitoring.NewLogger(monitoring.LogLevelInfo, "text")
	}
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}

	// 注册Shell命令处理器
	shellHandler := NewShellHandler(cfg.Logger)
	r.Register("exec_shell", shellHandler.Handle)

	// 注册状态查询处理器
	statusHandler := NewStatusHandler(cfg.Logger, cfg.Version)
	r.Register("get_status", statusHandler.Handle)

	cfg.Logger.Info("Builtin handlers registered",
		"commands", r.ListCommands(),
	)
}

// RegisterCustomHandler 注册自定义处理器的辅助函数
func RegisterCustomHandler(r *router.Router, commandType string, handler router.Handler) {
	r.Register(commandType, handler)
}
