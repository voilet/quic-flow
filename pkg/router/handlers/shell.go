package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/voilet/quic-flow/pkg/monitoring"
)

// ShellParams exec_shell命令的参数
type ShellParams struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // 超时时间（秒），默认30秒
}

// ShellResult exec_shell命令的结果
type ShellResult struct {
	Success  bool   `json:"success"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Message  string `json:"message"`
}

// ShellHandler Shell命令处理器
type ShellHandler struct {
	logger         *monitoring.Logger
	maxOutputSize  int // 最大输出大小
	defaultTimeout int // 默认超时（秒）
	maxTimeout     int // 最大超时（秒）
}

// NewShellHandler 创建Shell处理器
func NewShellHandler(logger *monitoring.Logger) *ShellHandler {
	return &ShellHandler{
		logger:         logger,
		maxOutputSize:  10 * 1024, // 10KB
		defaultTimeout: 30,
		maxTimeout:     300, // 5分钟
	}
}

// Handle 处理exec_shell命令
func (h *ShellHandler) Handle(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var params ShellParams
	if err := json.Unmarshal(payload, &params); err != nil {
		return nil, fmt.Errorf("invalid exec_shell params: %w", err)
	}

	// 验证命令
	if strings.TrimSpace(params.Command) == "" {
		return nil, fmt.Errorf("command is empty")
	}

	h.logger.Info("执行Shell命令", "command", params.Command)

	// 设置超时
	timeout := h.defaultTimeout
	if params.Timeout > 0 {
		timeout = params.Timeout
	}
	if timeout > h.maxTimeout {
		timeout = h.maxTimeout
	}

	// 创建带超时的上下文
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// 执行Shell命令
	cmd := exec.CommandContext(execCtx, "sh", "-c", params.Command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// 截断输出
	stdoutStr := h.truncateOutput(stdout.String())
	stderrStr := h.truncateOutput(stderr.String())

	// 构建结果
	result := ShellResult{
		Success:  err == nil,
		ExitCode: 0,
		Stdout:   stdoutStr,
		Stderr:   stderrStr,
		Message:  "命令执行成功",
	}

	if err != nil {
		result.Message = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if execCtx.Err() == context.DeadlineExceeded {
			result.Message = fmt.Sprintf("命令执行超时（%d秒）", timeout)
			result.ExitCode = -1
		} else {
			result.ExitCode = -1
		}
	}

	h.logger.Info("Shell命令执行完成",
		"command", params.Command,
		"success", result.Success,
		"exit_code", result.ExitCode,
	)

	return json.Marshal(result)
}

// truncateOutput 截断输出
func (h *ShellHandler) truncateOutput(s string) string {
	if len(s) > h.maxOutputSize {
		return s[:h.maxOutputSize] + "... (truncated)"
	}
	return s
}
