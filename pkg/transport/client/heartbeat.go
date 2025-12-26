package client

import (
	"time"

	pkgerrors "github.com/voilet/quic-flow/pkg/errors"
	"github.com/voilet/quic-flow/pkg/protocol"
	"github.com/voilet/quic-flow/pkg/transport/codec"
)

// heartbeatLoop 客户端心跳循环 (T030)
// 每 15 秒发送一次 Ping，等待 Pong 响应
func (c *Client) heartbeatLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	c.logger.Debug("Heartbeat loop started", "interval", c.config.HeartbeatInterval)

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("Heartbeat loop stopped")
			return

		case <-ticker.C:
			if !c.IsConnected() {
				c.logger.Debug("Not connected, skipping heartbeat")
				continue
			}

			if err := c.sendHeartbeat(); err != nil {
				c.logger.Error("Heartbeat failed", "error", err)
				c.metrics.RecordNetworkError()

				// 心跳失败，触发重连
				c.setState(protocol.ClientState_CLIENT_STATE_IDLE)
				c.notifyDisconnect()
			}
		}
	}
}

// sendHeartbeat 发送心跳 Ping 并等待 Pong
func (c *Client) sendHeartbeat() error {
	// 打开新的流
	stream, err := c.conn.OpenStreamSync(c.ctx)
	if err != nil {
		return err
	}

	// 构造 Ping 帧
	pingFrame, err := codec.EncodePingFrame(&protocol.PingFrame{
		ClientId: c.config.ClientID,
	}, time.Now().UnixMilli())
	if err != nil {
		c.metrics.RecordEncodingError()
		stream.Close()
		return err
	}

	// 发送 Ping
	if err := c.codec.WriteFrame(stream, pingFrame); err != nil {
		c.metrics.RecordNetworkError()
		stream.Close()
		return err
	}

	// 关闭写端，通知服务器数据发送完毕
	if err := stream.Close(); err != nil {
		return err
	}

	c.metrics.RecordHeartbeatSent()
	c.logger.Debug("Heartbeat sent", "client_id", c.config.ClientID)

	// 等待 Pong 响应（带超时）
	pongCh := make(chan error, 1)
	go func() {
		pongFrame, err := c.codec.ReadFrame(stream)
		if err != nil {
			pongCh <- err
			return
		}

		if pongFrame.Type != protocol.FrameType_FRAME_TYPE_PONG {
			pongCh <- pkgerrors.ErrInvalidFrameType
			return
		}

		// 解码 Pong
		_, err = codec.DecodePongFrame(pongFrame)
		pongCh <- err
	}()

	select {
	case err := <-pongCh:
		if err != nil {
			c.logger.Error("Failed to receive pong", "error", err)
			c.metrics.RecordDecodingError()
			return err
		}

		// 更新最后 Pong 时间
		c.lastPongTime.Store(time.Now())
		c.metrics.RecordHeartbeatReceived()
		c.logger.Debug("Pong received", "client_id", c.config.ClientID)
		return nil

	case <-time.After(c.config.HeartbeatTimeout):
		c.logger.Warn("Heartbeat timeout", "timeout", c.config.HeartbeatTimeout)
		c.metrics.RecordHeartbeatTimeout()
		return pkgerrors.ErrHeartbeatTimeout
	}
}

// GetLastPongTime 获取最后收到 Pong 的时间
func (c *Client) GetLastPongTime() time.Time {
	return c.lastPongTime.Load().(time.Time)
}

// GetTimeSinceLastPong 获取距离最后 Pong 的时间
func (c *Client) GetTimeSinceLastPong() time.Duration {
	return time.Since(c.GetLastPongTime())
}
