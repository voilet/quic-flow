package command

import (
	"encoding/json"
	"time"
)

// CommandStatus 命令执行状态
type CommandStatus string

const (
	CommandStatusPending   CommandStatus = "pending"   // 已下发，等待客户端执行
	CommandStatusExecuting CommandStatus = "executing" // 客户端正在执行
	CommandStatusCompleted CommandStatus = "completed" // 执行完成（成功）
	CommandStatusFailed    CommandStatus = "failed"    // 执行失败
	CommandStatusTimeout   CommandStatus = "timeout"   // 执行超时
)

// Command 命令信息
type Command struct {
	// 基本信息
	CommandID  string        `json:"command_id"`  // 命令唯一ID（等同于msg_id）
	ClientID   string        `json:"client_id"`   // 目标客户端ID
	CommandType string       `json:"command_type"` // 命令类型（业务自定义，如 "restart", "update_config" 等）
	Payload    json.RawMessage `json:"payload"`      // 命令参数（JSON格式）

	// 状态信息
	Status     CommandStatus `json:"status"`      // 当前状态
	Result     json.RawMessage `json:"result,omitempty"`     // 执行结果（JSON格式）
	Error      string        `json:"error,omitempty"`      // 错误信息

	// 时间信息
	CreatedAt  time.Time     `json:"created_at"`  // 创建时间
	SentAt     *time.Time    `json:"sent_at,omitempty"`     // 发送时间
	CompletedAt *time.Time   `json:"completed_at,omitempty"` // 完成时间
	Timeout    time.Duration `json:"timeout"`     // 超时时长
}

// CommandRequest HTTP请求结构 - 下发命令
type CommandRequest struct {
	ClientID    string          `json:"client_id" binding:"required"`    // 目标客户端
	CommandType string          `json:"command_type" binding:"required"` // 命令类型
	Payload     json.RawMessage `json:"payload"`                         // 命令参数
	Timeout     int             `json:"timeout,omitempty"`               // 超时时间（秒），默认30s
}

// CommandResponse HTTP响应结构 - 下发命令结果
type CommandResponse struct {
	Success   bool   `json:"success"`
	CommandID string `json:"command_id"`
	Message   string `json:"message"`
}

// CommandStatusResponse HTTP响应结构 - 查询命令状态
type CommandStatusResponse struct {
	Success bool     `json:"success"`
	Command *Command `json:"command,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// CommandExecutor 客户端命令执行器接口
// 业务层需要实现此接口来处理具体的命令
type CommandExecutor interface {
	// Execute 执行命令
	// commandType: 命令类型
	// payload: 命令参数（JSON格式）
	// 返回: 执行结果（JSON格式）和错误
	Execute(commandType string, payload []byte) (result []byte, err error)
}

// CommandPayload 命令载荷（放在DataMessage.Payload中）
type CommandPayload struct {
	CommandType  string          `json:"command_type"`            // 命令类型
	Payload      json.RawMessage `json:"payload"`                 // 命令参数
	NeedCallback bool            `json:"need_callback,omitempty"` // 是否需要异步回调（执行完毕后主动上报结果）
	CallbackID   string          `json:"callback_id,omitempty"`   // 回调ID（用于关联请求和回调）
}

// CallbackPayload 回调载荷（客户端执行完命令后发送给服务器）
type CallbackPayload struct {
	CallbackID  string          `json:"callback_id"`           // 回调ID（对应CommandPayload.CallbackID）
	CommandType string          `json:"command_type"`          // 原命令类型
	Success     bool            `json:"success"`               // 执行是否成功
	Result      json.RawMessage `json:"result,omitempty"`      // 执行结果
	Error       string          `json:"error,omitempty"`       // 错误信息
	Duration    int64           `json:"duration_ms,omitempty"` // 执行耗时（毫秒）
}

// MultiCommandRequest HTTP请求结构 - 多播命令（同时下发到多个客户端）
type MultiCommandRequest struct {
	ClientIDs   []string        `json:"client_ids" binding:"required,min=1"` // 目标客户端列表
	CommandType string          `json:"command_type" binding:"required"`     // 命令类型
	Payload     json.RawMessage `json:"payload"`                             // 命令参数
	Timeout     int             `json:"timeout,omitempty"`                   // 超时时间（秒），默认30s
}

// ClientCommandResult 单个客户端的命令执行结果
type ClientCommandResult struct {
	ClientID  string          `json:"client_id"`            // 客户端ID
	CommandID string          `json:"command_id"`           // 命令ID
	Status    CommandStatus   `json:"status"`               // 执行状态
	Result    json.RawMessage `json:"result,omitempty"`     // 执行结果
	Error     string          `json:"error,omitempty"`      // 错误信息
}

// MultiCommandResponse HTTP响应结构 - 多播命令结果
type MultiCommandResponse struct {
	Success      bool                   `json:"success"`       // 整体是否成功（所有命令都发送成功）
	Total        int                    `json:"total"`         // 总客户端数
	SuccessCount int                    `json:"success_count"` // 成功发送的数量
	FailedCount  int                    `json:"failed_count"`  // 发送失败的数量
	Results      []*ClientCommandResult `json:"results"`       // 各客户端的结果
	Message      string                 `json:"message"`       // 摘要信息
}
