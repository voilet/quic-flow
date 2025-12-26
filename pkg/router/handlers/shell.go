package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/voilet/quic-flow/pkg/command"
)

// Shell 配置
const (
	shellMaxOutputSize  = 10 * 1024 // 10KB
	shellDefaultTimeout = 30        // 秒
	shellMaxTimeout     = 300       // 5分钟
)

// ExecShell 执行 Shell 命令
// 命令类型: exec_shell
// 用法: r.Register(command.CmdExecShell, handlers.ExecShell)
func ExecShell(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var params command.ShellParams
	if err := json.Unmarshal(payload, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if strings.TrimSpace(params.Command) == "" {
		return nil, fmt.Errorf("command is empty")
	}

	// 设置超时
	timeout := shellDefaultTimeout
	if params.Timeout > 0 {
		timeout = params.Timeout
	}
	if timeout > shellMaxTimeout {
		timeout = shellMaxTimeout
	}

	// 执行命令
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "sh", "-c", params.Command)
	if params.WorkDir != "" {
		cmd.Dir = params.WorkDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// 构建结果
	result := command.ShellResult{
		Success:  err == nil,
		ExitCode: 0,
		Stdout:   truncateOutput(stdout.String(), shellMaxOutputSize),
		Stderr:   truncateOutput(stderr.String(), shellMaxOutputSize),
		Message:  "success",
	}

	if err != nil {
		result.Message = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if execCtx.Err() == context.DeadlineExceeded {
			result.Message = fmt.Sprintf("timeout after %ds", timeout)
			result.ExitCode = -1
		} else {
			result.ExitCode = -1
		}
	}

	return json.Marshal(result)
}

// truncateOutput 截断输出
func truncateOutput(s string, maxSize int) string {
	if len(s) > maxSize {
		return s[:maxSize] + "... (truncated)"
	}
	return s
}
