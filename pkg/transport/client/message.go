package client

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	pkgerrors "github.com/voilet/quic-flow/pkg/errors"
	"github.com/voilet/quic-flow/pkg/protocol"
	"github.com/voilet/quic-flow/pkg/transport/codec"
)

// SendMessage 发送消息到服务器 (T044)
// msg: 要发送的消息
// waitAck: 是否等待确认
// timeout: 超时时间（仅在 waitAck=true 时有效，0 表示 30s）
// 返回 Ack 消息（如果 waitAck=true）和错误
func (c *Client) SendMessage(ctx context.Context, msg *protocol.DataMessage, waitAck bool, timeout time.Duration) (*protocol.AckMessage, error) {
	if !c.IsConnected() {
		return nil, pkgerrors.ErrClientNotConnected
	}

	if msg == nil {
		return nil, fmt.Errorf("%w: message is nil", pkgerrors.ErrInvalidConfig)
	}

	// 生成消息 ID（如果没有）
	if msg.MsgId == "" {
		msg.MsgId = uuid.New().String()
	}

	// 设置发送方 ID
	msg.SenderId = c.config.ClientID

	// 设置时间戳
	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().UnixMilli()
	}

	// 设置 WaitAck 标志
	msg.WaitAck = waitAck

	// 编码消息
	dataFrame, err := codec.EncodeDataMessage(msg)
	if err != nil {
		c.metrics.RecordEncodingError()
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	// 打开新流
	stream, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		c.metrics.RecordNetworkError()
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	// 发送消息
	if err := c.codec.WriteFrame(stream, dataFrame); err != nil {
		c.metrics.RecordNetworkError()
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// 关闭写端
	if err := stream.Close(); err != nil {
		return nil, fmt.Errorf("failed to close stream: %w", err)
	}

	// 记录指标
	c.metrics.RecordMessageSent(int64(len(msg.Payload)))
	c.logger.Debug("Message sent", "msg_id", msg.MsgId, "wait_ack", waitAck)

	// 如果不需要 Ack，直接返回
	if !waitAck {
		return nil, nil
	}

	// 等待 Ack 响应
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ackCh := make(chan *protocol.AckMessage, 1)
	errCh := make(chan error, 1)

	// 启动读取 Ack 的 goroutine
	go func() {
		ackFrame, err := c.codec.ReadFrame(stream)
		if err != nil {
			errCh <- err
			return
		}

		if ackFrame.Type != protocol.FrameType_FRAME_TYPE_ACK {
			errCh <- pkgerrors.ErrInvalidFrameType
			return
		}

		ackMsg, err := codec.DecodeAckMessage(ackFrame)
		if err != nil {
			errCh <- err
			return
		}

		ackCh <- ackMsg
	}()

	// 等待 Ack 或超时
	select {
	case ackMsg := <-ackCh:
		c.logger.Debug("Ack received", "msg_id", msg.MsgId, "status", ackMsg.Status)
		c.metrics.RecordMessageReceived(0)
		return ackMsg, nil

	case err := <-errCh:
		c.logger.Error("Failed to receive ack", "msg_id", msg.MsgId, "error", err)
		c.metrics.RecordDecodingError()
		return nil, err

	case <-time.After(timeout):
		c.logger.Warn("Ack timeout", "msg_id", msg.MsgId, "timeout", timeout)
		c.metrics.RecordHeartbeatTimeout() // 复用此指标
		return nil, pkgerrors.ErrHeartbeatTimeout

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendMessageAsync 异步发送消息（不等待 Ack）
func (c *Client) SendMessageAsync(msg *protocol.DataMessage) error {
	_, err := c.SendMessage(c.ctx, msg, false, 0)
	return err
}
