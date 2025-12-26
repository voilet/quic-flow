package client

import (
	"encoding/json"
	"io"
	"time"

	"github.com/quic-go/quic-go"

	"github.com/voilet/quic-flow/pkg/dispatcher"
	"github.com/voilet/quic-flow/pkg/protocol"
	"github.com/voilet/quic-flow/pkg/transport/codec"
)

// receiveLoop 客户端消息接收循环 (T045)
// 接收来自服务器的消息并分发处理
func (c *Client) receiveLoop() {
	defer c.wg.Done()

	c.logger.Info("📥 Receive loop started - waiting for messages from server")

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Info("Receive loop stopped")
			return

		default:
			// 等待接收新流
			stream, err := c.conn.AcceptStream(c.ctx)
			if err != nil {
				select {
				case <-c.ctx.Done():
					// 客户端关闭
					return
				default:
					c.logger.Error("Failed to accept stream", "error", err)
					c.metrics.RecordNetworkError()

					// 连接可能已断开，触发重连
					c.setState(protocol.ClientState_CLIENT_STATE_IDLE)
					c.notifyDisconnect()
					return
				}
			}

			c.logger.Info("📨 Stream accepted from server, starting handler")

			// 为每个流启动处理 goroutine
			c.wg.Add(1)
			go c.handleStream(stream)
		}
	}
}

// handleStream 处理单个流
func (c *Client) handleStream(stream *quic.Stream) {
	defer c.wg.Done()
	defer stream.Close()

	c.logger.Info("开始读取流中的帧")

	// 读取帧（使用长度前缀协议）
	frame, err := c.codec.ReadFrame(stream)
	if err != nil {
		if err == io.EOF {
			c.logger.Debug("流正常结束")
		} else {
			c.logger.Error("读取帧失败", "error", err)
			c.metrics.RecordDecodingError()
		}
		return
	}

	c.logger.Info("成功读取帧", "frame_type", frame.Type, "frame_size", len(frame.Payload))

	// 根据帧类型处理
	switch frame.Type {
	case protocol.FrameType_FRAME_TYPE_DATA:
		c.logger.Info("处理DATA帧")
		c.handleData(stream, frame)

	case protocol.FrameType_FRAME_TYPE_PONG:
		// Pong 消息已经在 heartbeat.go 中处理
		c.logger.Debug("收到PONG帧（已在heartbeat中处理）")

	default:
		c.logger.Warn("未知帧类型", "frame_type", frame.Type)
	}
}

// handleData 处理数据消息
func (c *Client) handleData(stream *quic.Stream, frame *protocol.Frame) {
	// 解码数据消息
	dataMsg, err := codec.DecodeDataMessage(frame)
	if err != nil {
		c.logger.Error("Failed to decode data message", "error", err)
		c.metrics.RecordDecodingError()
		return
	}

	c.logger.Info("✅ Data message received", "msg_id", dataMsg.MsgId, "type", dataMsg.Type, "wait_ack", dataMsg.WaitAck)
	c.metrics.RecordMessageReceived(int64(len(dataMsg.Payload)))

	// 分发消息到 Dispatcher（如果已设置）
	disp := c.GetDispatcher()
	c.logger.Info("Checking dispatcher", "has_dispatcher", disp != nil)
	if disp != nil {
		// 创建响应通道
		responseCh := make(chan *dispatcher.DispatchResponse, 1)

		// 异步分发消息
		if err := disp.Dispatch(c.ctx, dataMsg, responseCh); err != nil {
			c.logger.Error("Failed to dispatch message", "msg_id", dataMsg.MsgId, "error", err)

			// 发送失败的 Ack
			if dataMsg.WaitAck {
				if err := c.sendAck(stream, dataMsg.MsgId, protocol.AckStatus_ACK_STATUS_FAILURE, nil, err.Error()); err != nil {
					c.logger.Error("Failed to send error ack", "msg_id", dataMsg.MsgId, "error", err)
				}
			}
			return
		}

		// 等待处理结果（如果需要响应）
		if dataMsg.WaitAck {
			c.logger.Info("Waiting for handler response", "msg_id", dataMsg.MsgId)
			select {
			case resp := <-responseCh:
				if resp.Error != nil {
					c.logger.Error("Handler failed to process message", "msg_id", dataMsg.MsgId, "error", resp.Error)
					if err := c.sendAck(stream, dataMsg.MsgId, protocol.AckStatus_ACK_STATUS_FAILURE, nil, resp.Error.Error()); err != nil {
						c.logger.Error("Failed to send error ack", "msg_id", dataMsg.MsgId, "error", err)
					}
				} else {
					c.logger.Info("✅ Handler processed message successfully", "msg_id", dataMsg.MsgId, "has_response", resp.Response != nil)

					// 从响应中提取结果
					var result []byte
					if resp.Response != nil && resp.Response.Type == protocol.MessageType_MESSAGE_TYPE_RESPONSE {
						// CommandHandler 返回的响应 Payload 就是 AckMessage 的 JSON
						// 我们需要从中提取 Result
						var ackMsg protocol.AckMessage
						if err := json.Unmarshal(resp.Response.Payload, &ackMsg); err != nil {
							c.logger.Error("Failed to unmarshal ack message from response", "msg_id", dataMsg.MsgId, "error", err)
						} else {
							result = ackMsg.Result
							c.logger.Info("Extracted result from response", "msg_id", dataMsg.MsgId, "result_size", len(result))
						}
					}

					// 发送成功的 Ack（包含执行结果）
					c.logger.Info("Sending success ACK with result", "msg_id", dataMsg.MsgId, "result_size", len(result))
					if err := c.sendAck(stream, dataMsg.MsgId, protocol.AckStatus_ACK_STATUS_SUCCESS, result, ""); err != nil {
						c.logger.Error("Failed to send success ack", "msg_id", dataMsg.MsgId, "error", err)
					} else {
						c.logger.Info("✅ ACK sent successfully with result", "msg_id", dataMsg.MsgId)
					}
				}
			case <-time.After(30 * time.Second):
				c.logger.Error("Message processing timeout", "msg_id", dataMsg.MsgId)
				if err := c.sendAck(stream, dataMsg.MsgId, protocol.AckStatus_ACK_STATUS_FAILURE, nil, "processing timeout"); err != nil {
					c.logger.Error("Failed to send timeout ack", "msg_id", dataMsg.MsgId, "error", err)
				}
			case <-c.ctx.Done():
				c.logger.Debug("Client context cancelled while processing message", "msg_id", dataMsg.MsgId)
				return
			}
		}
	} else {
		// 如果没有设置 Dispatcher，只发送 Ack
		c.logger.Warn("⚠️  No dispatcher set, message cannot be processed", "msg_id", dataMsg.MsgId, "type", dataMsg.Type)
		if dataMsg.WaitAck {
			if err := c.sendAck(stream, dataMsg.MsgId, protocol.AckStatus_ACK_STATUS_FAILURE, nil, "no dispatcher configured"); err != nil {
				c.logger.Error("Failed to send ack", "msg_id", dataMsg.MsgId, "error", err)
			}
		}
	}
}

// sendAck 发送 Ack 消息
// 使用长度前缀协议，不需要关闭写端来标识消息边界
func (c *Client) sendAck(stream *quic.Stream, msgID string, status protocol.AckStatus, result []byte, errorMsg string) error {
	ackMsg := &protocol.AckMessage{
		MsgId:  msgID,
		Status: status,
		Result: result,
		Error:  errorMsg,
	}

	ackFrame, err := codec.EncodeAckMessage(ackMsg, time.Now().UnixMilli())
	if err != nil {
		c.metrics.RecordEncodingError()
		return err
	}

	c.logger.Info("准备发送ACK", "msg_id", msgID, "frame_size", len(ackFrame.Payload))

	// 使用长度前缀协议写入ACK帧
	if err := c.codec.WriteFrame(stream, ackFrame); err != nil {
		c.metrics.RecordNetworkError()
		c.logger.Error("写入ACK帧失败", "msg_id", msgID, "error", err)
		return err
	}

	c.logger.Info("✅ Ack sent", "msg_id", msgID, "status", status, "has_result", len(result) > 0)
	return nil
}
