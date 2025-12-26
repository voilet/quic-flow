package dispatcher

import (
	"context"

	"github.com/voilet/quic-flow/pkg/protocol"
)

// MessageHandler 消息处理器接口 (T037)
// 业务层实现此接口来处理不同类型的消息
type MessageHandler interface {
	// OnMessage 处理收到的消息
	// ctx 包含超时控制和取消信号
	// msg 是收到的数据消息
	// 返回值：响应消息（可选），错误（如果处理失败）
	OnMessage(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error)
}

// MessageHandlerFunc 函数适配器，允许使用普通函数作为 Handler
type MessageHandlerFunc func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error)

// OnMessage 实现 MessageHandler 接口
func (f MessageHandlerFunc) OnMessage(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
	return f(ctx, msg)
}
