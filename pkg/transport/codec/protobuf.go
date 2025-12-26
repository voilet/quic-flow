package codec

import (
	"encoding/binary"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"

	"github.com/voilet/quic-flow/pkg/protocol"
	pkgerrors "github.com/voilet/quic-flow/pkg/errors"
)

// Codec 定义编解码器接口
type Codec interface {
	// EncodeFrame 将 Frame 编码为字节流
	EncodeFrame(frame *protocol.Frame) ([]byte, error)

	// DecodeFrame 从字节流解码 Frame
	DecodeFrame(data []byte) (*protocol.Frame, error)

	// WriteFrame 将 Frame 写入到 Writer
	WriteFrame(w io.Writer, frame *protocol.Frame) error

	// ReadFrame 从 Reader 读取并解码 Frame
	ReadFrame(r io.Reader) (*protocol.Frame, error)
}

// ProtobufCodec 是基于 Protobuf 的编解码器实现
type ProtobufCodec struct{}

// NewProtobufCodec 创建新的 Protobuf 编解码器
func NewProtobufCodec() *ProtobufCodec {
	return &ProtobufCodec{}
}

// EncodeFrame 将 Frame 编码为字节流
func (c *ProtobufCodec) EncodeFrame(frame *protocol.Frame) ([]byte, error) {
	if frame == nil {
		return nil, fmt.Errorf("%w: frame is nil", pkgerrors.ErrEncodeFailed)
	}

	data, err := proto.Marshal(frame)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", pkgerrors.ErrEncodeFailed, err)
	}

	return data, nil
}

// DecodeFrame 从字节流解码 Frame
func (c *ProtobufCodec) DecodeFrame(data []byte) (*protocol.Frame, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty data", pkgerrors.ErrDecodeFailed)
	}

	frame := &protocol.Frame{}
	if err := proto.Unmarshal(data, frame); err != nil {
		return nil, fmt.Errorf("%w: %v", pkgerrors.ErrDecodeFailed, err)
	}

	return frame, nil
}

// WriteFrame 将 Frame 写入到 Writer
// 使用 4 字节大端长度前缀 + 数据的格式
func (c *ProtobufCodec) WriteFrame(w io.Writer, frame *protocol.Frame) error {
	data, err := c.EncodeFrame(frame)
	if err != nil {
		return err
	}

	// 写入 4 字节长度前缀（大端序）
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(data)))

	if _, err := w.Write(lengthBuf); err != nil {
		return fmt.Errorf("%w: failed to write length prefix: %v", pkgerrors.ErrEncodeFailed, err)
	}

	// 写入帧数据
	n, err := w.Write(data)
	if err != nil {
		return fmt.Errorf("%w: %v", pkgerrors.ErrEncodeFailed, err)
	}

	if n != len(data) {
		return fmt.Errorf("%w: partial write", pkgerrors.ErrEncodeFailed)
	}

	return nil
}

// ReadFrame 从 Reader 读取并解码 Frame
// 使用 4 字节大端长度前缀 + 数据的格式
func (c *ProtobufCodec) ReadFrame(r io.Reader) (*protocol.Frame, error) {
	// 读取 4 字节长度前缀
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lengthBuf); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("%w: unexpected EOF reading length prefix", pkgerrors.ErrDecodeFailed)
		}
		return nil, fmt.Errorf("%w: failed to read length prefix: %v", pkgerrors.ErrDecodeFailed, err)
	}

	// 解析长度
	length := binary.BigEndian.Uint32(lengthBuf)
	if length == 0 {
		return nil, fmt.Errorf("%w: zero length frame", pkgerrors.ErrDecodeFailed)
	}
	if length > 10*1024*1024 { // 限制最大 10MB
		return nil, fmt.Errorf("%w: frame too large: %d bytes", pkgerrors.ErrDecodeFailed, length)
	}

	// 读取帧数据
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("%w: unexpected EOF reading frame data", pkgerrors.ErrDecodeFailed)
		}
		return nil, fmt.Errorf("%w: failed to read frame data: %v", pkgerrors.ErrDecodeFailed, err)
	}

	return c.DecodeFrame(data)
}

// EncodeDataMessage 辅助函数：编码 DataMessage 到 Frame
func EncodeDataMessage(msg *protocol.DataMessage) (*protocol.Frame, error) {
	if msg == nil {
		return nil, fmt.Errorf("%w: message is nil", pkgerrors.ErrInvalidMessage)
	}

	payload, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", pkgerrors.ErrEncodeFailed, err)
	}

	frame := &protocol.Frame{
		Type:      protocol.FrameType_FRAME_TYPE_DATA,
		Payload:   payload,
		Timestamp: msg.Timestamp,
	}

	return frame, nil
}

// DecodeDataMessage 辅助函数：从 Frame 解码 DataMessage
func DecodeDataMessage(frame *protocol.Frame) (*protocol.DataMessage, error) {
	if frame == nil {
		return nil, fmt.Errorf("%w: frame is nil", pkgerrors.ErrInvalidMessage)
	}

	if frame.Type != protocol.FrameType_FRAME_TYPE_DATA {
		return nil, fmt.Errorf("%w: expected DATA frame, got %v", pkgerrors.ErrInvalidFrameType, frame.Type)
	}

	msg := &protocol.DataMessage{}
	if err := proto.Unmarshal(frame.Payload, msg); err != nil {
		return nil, fmt.Errorf("%w: %v", pkgerrors.ErrDecodeFailed, err)
	}

	return msg, nil
}

// EncodePingFrame 辅助函数：编码 PingFrame 到 Frame
func EncodePingFrame(ping *protocol.PingFrame, timestamp int64) (*protocol.Frame, error) {
	if ping == nil {
		return nil, fmt.Errorf("%w: ping is nil", pkgerrors.ErrInvalidMessage)
	}

	payload, err := proto.Marshal(ping)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", pkgerrors.ErrEncodeFailed, err)
	}

	frame := &protocol.Frame{
		Type:      protocol.FrameType_FRAME_TYPE_PING,
		Payload:   payload,
		Timestamp: timestamp,
	}

	return frame, nil
}

// DecodePingFrame 辅助函数：从 Frame 解码 PingFrame
func DecodePingFrame(frame *protocol.Frame) (*protocol.PingFrame, error) {
	if frame == nil {
		return nil, fmt.Errorf("%w: frame is nil", pkgerrors.ErrInvalidMessage)
	}

	if frame.Type != protocol.FrameType_FRAME_TYPE_PING {
		return nil, fmt.Errorf("%w: expected PING frame, got %v", pkgerrors.ErrInvalidFrameType, frame.Type)
	}

	ping := &protocol.PingFrame{}
	if err := proto.Unmarshal(frame.Payload, ping); err != nil {
		return nil, fmt.Errorf("%w: %v", pkgerrors.ErrDecodeFailed, err)
	}

	return ping, nil
}

// EncodePongFrame 辅助函数：编码 PongFrame 到 Frame
func EncodePongFrame(pong *protocol.PongFrame, timestamp int64) (*protocol.Frame, error) {
	if pong == nil {
		return nil, fmt.Errorf("%w: pong is nil", pkgerrors.ErrInvalidMessage)
	}

	payload, err := proto.Marshal(pong)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", pkgerrors.ErrEncodeFailed, err)
	}

	frame := &protocol.Frame{
		Type:      protocol.FrameType_FRAME_TYPE_PONG,
		Payload:   payload,
		Timestamp: timestamp,
	}

	return frame, nil
}

// DecodePongFrame 辅助函数：从 Frame 解码 PongFrame
func DecodePongFrame(frame *protocol.Frame) (*protocol.PongFrame, error) {
	if frame == nil {
		return nil, fmt.Errorf("%w: frame is nil", pkgerrors.ErrInvalidMessage)
	}

	if frame.Type != protocol.FrameType_FRAME_TYPE_PONG {
		return nil, fmt.Errorf("%w: expected PONG frame, got %v", pkgerrors.ErrInvalidFrameType, frame.Type)
	}

	pong := &protocol.PongFrame{}
	if err := proto.Unmarshal(frame.Payload, pong); err != nil {
		return nil, fmt.Errorf("%w: %v", pkgerrors.ErrDecodeFailed, err)
	}

	return pong, nil
}

// EncodeAckMessage 辅助函数：编码 AckMessage 到 Frame
func EncodeAckMessage(ack *protocol.AckMessage, timestamp int64) (*protocol.Frame, error) {
	if ack == nil {
		return nil, fmt.Errorf("%w: ack is nil", pkgerrors.ErrInvalidMessage)
	}

	payload, err := proto.Marshal(ack)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", pkgerrors.ErrEncodeFailed, err)
	}

	frame := &protocol.Frame{
		Type:      protocol.FrameType_FRAME_TYPE_ACK,
		Payload:   payload,
		Timestamp: timestamp,
	}

	return frame, nil
}

// DecodeAckMessage 辅助函数：从 Frame 解码 AckMessage
func DecodeAckMessage(frame *protocol.Frame) (*protocol.AckMessage, error) {
	if frame == nil {
		return nil, fmt.Errorf("%w: frame is nil", pkgerrors.ErrInvalidMessage)
	}

	if frame.Type != protocol.FrameType_FRAME_TYPE_ACK {
		return nil, fmt.Errorf("%w: expected ACK frame, got %v", pkgerrors.ErrInvalidFrameType, frame.Type)
	}

	ack := &protocol.AckMessage{}
	if err := proto.Unmarshal(frame.Payload, ack); err != nil {
		return nil, fmt.Errorf("%w: %v", pkgerrors.ErrDecodeFailed, err)
	}

	return ack, nil
}
