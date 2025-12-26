package handlers

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/voilet/quic-flow/pkg/command"
)

// 状态处理器的全局状态
var (
	statusStartTime time.Time
	statusVersion   string
	statusOnce      sync.Once
)

// InitStatus 初始化状态信息（在程序启动时调用一次）
func InitStatus(version string) {
	statusOnce.Do(func() {
		statusStartTime = time.Now()
		statusVersion = version
	})
}

// GetStatus 获取客户端状态
// 命令类型: get_status
// 用法: r.Register(command.CmdGetStatus, handlers.GetStatus)
func GetStatus(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	hostname, _ := os.Hostname()

	result := command.StatusResult{
		Status:       "running",
		Uptime:       int64(time.Since(statusStartTime).Seconds()),
		Version:      statusVersion,
		Hostname:     hostname,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
	}

	return json.Marshal(result)
}
