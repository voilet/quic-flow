package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/voilet/QuicFlow/pkg/monitoring"
	"github.com/voilet/QuicFlow/pkg/protocol"
	"github.com/voilet/QuicFlow/pkg/router"
)

// ClientAPI 客户端接口（用于发送消息）
type ClientAPI interface {
	SendMessage(ctx context.Context, msg *protocol.DataMessage, waitAck bool, timeout time.Duration) (*protocol.AckMessage, error)
}

// CommandHandler 命令处理器（客户端）
type CommandHandler struct {
	client   ClientAPI
	logger   *monitoring.Logger
	executor CommandExecutor // 业务层实现的命令执行器
}

// NewCommandHandler 创建命令处理器
func NewCommandHandler(client ClientAPI, executor CommandExecutor, logger *monitoring.Logger) *CommandHandler {
	return &CommandHandler{
		client:   client,
		logger:   logger,
		executor: executor,
	}
}

// HandleCommand 处理收到的命令消息
// 这个方法应该被注册为 MESSAGE_TYPE_COMMAND 的处理器
func (h *CommandHandler) HandleCommand(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
	startTime := time.Now()

	h.logger.Info("Received command from server",
		"command_id", msg.MsgId,
		"sender", msg.SenderId,
	)

	// 解析命令载荷
	var cmdPayload CommandPayload
	if err := json.Unmarshal(msg.Payload, &cmdPayload); err != nil {
		h.logger.Error("Failed to unmarshal command payload",
			"command_id", msg.MsgId,
			"error", err,
		)
		return h.buildErrorResponse(msg.MsgId, fmt.Sprintf("invalid command payload: %v", err)), nil
	}

	h.logger.Info("Executing command",
		"command_id", msg.MsgId,
		"command_type", cmdPayload.CommandType,
		"need_callback", cmdPayload.NeedCallback,
		"callback_id", cmdPayload.CallbackID,
	)

	// 如果需要回调，设置回调上下文
	if cmdPayload.NeedCallback {
		callbackID := cmdPayload.CallbackID
		if callbackID == "" {
			callbackID = msg.MsgId // 使用消息ID作为回调ID
		}

		// 创建回调函数
		callbackFunc := func(cbID, cmdType string, success bool, result []byte, errMsg string) {
			h.sendCallback(ctx, cbID, cmdType, success, result, errMsg, time.Since(startTime))
		}

		// 设置回调上下文
		ctx = router.WithCallbackContext(ctx, true, callbackID, callbackFunc)
		ctx = router.WithCommandContext(ctx, cmdPayload.CommandType, msg.MsgId, msg.SenderId)
	}

	// 执行命令
	result, err := h.executor.Execute(cmdPayload.CommandType, cmdPayload.Payload)

	duration := time.Since(startTime)

	if err != nil {
		h.logger.Error("Command execution failed",
			"command_id", msg.MsgId,
			"command_type", cmdPayload.CommandType,
			"error", err,
			"duration", duration,
		)

		// 如果需要回调，发送失败回调
		if cmdPayload.NeedCallback {
			h.sendCallback(ctx, cmdPayload.CallbackID, cmdPayload.CommandType, false, nil, err.Error(), duration)
		}

		return h.buildErrorResponse(msg.MsgId, fmt.Sprintf("command execution failed: %v", err)), nil
	}

	h.logger.Info("Command execution succeeded",
		"command_id", msg.MsgId,
		"command_type", cmdPayload.CommandType,
		"duration", duration,
	)

	// 如果需要回调，发送成功回调
	if cmdPayload.NeedCallback {
		h.sendCallback(ctx, cmdPayload.CallbackID, cmdPayload.CommandType, true, result, "", duration)
	}

	// 构造成功响应
	return h.buildSuccessResponse(msg.MsgId, result), nil
}

// sendCallback 发送回调到服务器
func (h *CommandHandler) sendCallback(ctx context.Context, callbackID, commandType string, success bool, result []byte, errMsg string, duration time.Duration) {
	if callbackID == "" {
		callbackID = uuid.New().String()
	}

	// 构造回调载荷
	callback := CallbackPayload{
		CallbackID:  callbackID,
		CommandType: commandType,
		Success:     success,
		Result:      result,
		Error:       errMsg,
		Duration:    duration.Milliseconds(),
	}

	callbackBytes, err := json.Marshal(callback)
	if err != nil {
		h.logger.Error("Failed to marshal callback payload",
			"callback_id", callbackID,
			"error", err,
		)
		return
	}

	// 构造消息载荷（包含command_type用于路由）
	msgPayload := struct {
		CommandType string          `json:"command_type"`
		Payload     json.RawMessage `json:"payload"`
	}{
		CommandType: "command.callback",
		Payload:     callbackBytes,
	}

	msgPayloadBytes, _ := json.Marshal(msgPayload)

	// 构造回调消息
	msg := &protocol.DataMessage{
		MsgId:     uuid.New().String(),
		SenderId:  "", // 由客户端填充
		Type:      protocol.MessageType_MESSAGE_TYPE_EVENT,
		Payload:   msgPayloadBytes,
		WaitAck:   false, // 回调不需要等待确认
		Timestamp: time.Now().UnixMilli(),
	}

	// 异步发送回调（不阻塞命令处理）
	go func() {
		_, sendErr := h.client.SendMessage(context.Background(), msg, false, 10*time.Second)
		if sendErr != nil {
			h.logger.Error("Failed to send callback",
				"callback_id", callbackID,
				"error", sendErr,
			)
		} else {
			h.logger.Info("Callback sent successfully",
				"callback_id", callbackID,
				"command_type", commandType,
				"success", success,
			)
		}
	}()
}

// buildSuccessResponse 构造成功响应
func (h *CommandHandler) buildSuccessResponse(commandID string, result []byte) *protocol.DataMessage {
	ack := &protocol.AckMessage{
		MsgId:  commandID,
		Status: protocol.AckStatus_ACK_STATUS_SUCCESS,
		Result: result,
	}

	ackBytes, _ := json.Marshal(ack)

	return &protocol.DataMessage{
		MsgId:     commandID,
		Type:      protocol.MessageType_MESSAGE_TYPE_RESPONSE,
		Payload:   ackBytes,
		Timestamp: time.Now().UnixMilli(),
	}
}

// buildErrorResponse 构造错误响应
func (h *CommandHandler) buildErrorResponse(commandID string, errMsg string) *protocol.DataMessage {
	ack := &protocol.AckMessage{
		MsgId:  commandID,
		Status: protocol.AckStatus_ACK_STATUS_FAILURE,
		Error:  errMsg,
	}

	ackBytes, _ := json.Marshal(ack)

	return &protocol.DataMessage{
		MsgId:     commandID,
		Type:      protocol.MessageType_MESSAGE_TYPE_RESPONSE,
		Payload:   ackBytes,
		Timestamp: time.Now().UnixMilli(),
	}
}

// SetExecutor 设置命令执行器（允许动态更换）
func (h *CommandHandler) SetExecutor(executor CommandExecutor) {
	h.executor = executor
}
