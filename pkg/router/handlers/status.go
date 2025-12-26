package handlers

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"time"

	"github.com/voilet/QuicFlow/pkg/monitoring"
)

// StatusResult get_status命令的结果
type StatusResult struct {
	Status    string `json:"status"`
	Uptime    int64  `json:"uptime"`    // 运行时间（秒）
	Version   string `json:"version"`   // 客户端版本
	Hostname  string `json:"hostname"`  // 主机名
	OS        string `json:"os"`        // 操作系统
	Arch      string `json:"arch"`      // CPU架构
	GoVersion string `json:"go_version"`
	NumCPU    int    `json:"num_cpu"`
	NumGo     int    `json:"num_goroutine"`
}

// StatusHandler 状态查询处理器
type StatusHandler struct {
	logger    *monitoring.Logger
	startTime time.Time
	version   string
}

// NewStatusHandler 创建状态处理器
func NewStatusHandler(logger *monitoring.Logger, version string) *StatusHandler {
	return &StatusHandler{
		logger:    logger,
		startTime: time.Now(),
		version:   version,
	}
}

// Handle 处理get_status命令
func (h *StatusHandler) Handle(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	hostname, _ := os.Hostname()

	result := StatusResult{
		Status:    "running",
		Uptime:    int64(time.Since(h.startTime).Seconds()),
		Version:   h.version,
		Hostname:  hostname,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		GoVersion: runtime.Version(),
		NumCPU:    runtime.NumCPU(),
		NumGo:     runtime.NumGoroutine(),
	}

	h.logger.Debug("Status query handled", "uptime", result.Uptime)

	return json.Marshal(result)
}
