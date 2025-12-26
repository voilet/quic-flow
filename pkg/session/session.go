package session

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"

	"github.com/voilet/quic-flow/pkg/protocol"
)

// ClientSession 代表一个已连接的客户端会话
type ClientSession struct {
	// 基本信息
	ClientID   string      // 客户端唯一标识
	Conn       *quic.Conn  // QUIC 连接对象
	RemoteAddr string      // 客户端远程地址

	// 时间戳
	ConnectedAt time.Time // 连接建立时间

	// 心跳相关（使用 atomic 保证并发安全）
	lastHeartbeat atomic.Value // time.Time - 最后心跳时间
	TimeoutCount  atomic.Int32 // 连续超时次数（0-3）

	// 状态
	State protocol.ClientState // 连接状态（Idle/Connecting/Connected）

	// 并发控制
	mu sync.RWMutex // 保护 State 等字段的并发访问

	// 元数据（可选，用于业务层扩展）
	Metadata map[string]interface{} // 自定义元数据
}

// NewClientSession 创建新的客户端会话
func NewClientSession(clientID string, conn *quic.Conn) *ClientSession {
	now := time.Now()
	session := &ClientSession{
		ClientID:    clientID,
		Conn:        conn,
		RemoteAddr:  conn.RemoteAddr().String(),
		ConnectedAt: now,
		State:       protocol.ClientState_CLIENT_STATE_CONNECTED,
		Metadata:    make(map[string]interface{}),
	}

	// 初始化 lastHeartbeat
	session.lastHeartbeat.Store(now)

	return session
}

// GetLastHeartbeat 获取最后心跳时间
func (s *ClientSession) GetLastHeartbeat() time.Time {
	return s.lastHeartbeat.Load().(time.Time)
}

// UpdateLastHeartbeat 更新最后心跳时间并重置超时计数
func (s *ClientSession) UpdateLastHeartbeat() {
	s.lastHeartbeat.Store(time.Now())
	s.TimeoutCount.Store(0) // 重置超时计数
}

// IncrementTimeoutCount 增加超时计数
// 返回新的超时计数值
func (s *ClientSession) IncrementTimeoutCount() int32 {
	return s.TimeoutCount.Add(1)
}

// GetTimeoutCount 获取当前超时计数
func (s *ClientSession) GetTimeoutCount() int32 {
	return s.TimeoutCount.Load()
}

// ResetTimeoutCount 重置超时计数为 0
func (s *ClientSession) ResetTimeoutCount() {
	s.TimeoutCount.Store(0)
}

// GetState 获取当前状态
func (s *ClientSession) GetState() protocol.ClientState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

// SetState 设置状态
func (s *ClientSession) SetState(state protocol.ClientState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = state
}

// IsConnected 检查是否处于已连接状态
func (s *ClientSession) IsConnected() bool {
	return s.GetState() == protocol.ClientState_CLIENT_STATE_CONNECTED
}

// GetUptime 获取会话持续时间
func (s *ClientSession) GetUptime() time.Duration {
	return time.Since(s.ConnectedAt)
}

// GetTimeSinceLastHeartbeat 获取距离最后心跳的时间
func (s *ClientSession) GetTimeSinceLastHeartbeat() time.Duration {
	return time.Since(s.GetLastHeartbeat())
}

// IsHeartbeatTimeout 检查心跳是否超时
// timeout: 超时阈值（例如 45 秒）
func (s *ClientSession) IsHeartbeatTimeout(timeout time.Duration) bool {
	return s.GetTimeSinceLastHeartbeat() > timeout
}

// Close 关闭会话（关闭 QUIC 连接）
func (s *ClientSession) Close(reason string) error {
	s.SetState(protocol.ClientState_CLIENT_STATE_IDLE)
	return s.Conn.CloseWithError(0, reason)
}

// GetMetadata 获取元数据
func (s *ClientSession) GetMetadata(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.Metadata[key]
	return val, ok
}

// SetMetadata 设置元数据
func (s *ClientSession) SetMetadata(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Metadata[key] = value
}

// ToClientInfo 转换为 ClientInfo protobuf 消息
func (s *ClientSession) ToClientInfo() *protocol.ClientInfo {
	return &protocol.ClientInfo{
		ClientId:      s.ClientID,
		RemoteAddr:    s.RemoteAddr,
		ConnectedAt:   s.ConnectedAt.UnixMilli(),
		LastHeartbeat: s.GetLastHeartbeat().UnixMilli(),
		State:         s.GetState(),
	}
}
