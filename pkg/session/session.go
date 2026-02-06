package session

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"

	"github.com/voilet/quic-flow/pkg/protocol"
)

// ClientSession 代表一个已连接的客户端会话
// 内存优化版本：10W 连接约节省 9.8MB 内存
type ClientSession struct {
	// 基本信息
	ClientID string     // 客户端唯一标识
	Conn     *quic.Conn // QUIC 连接对象
	// RemoteAddr 已移除，改为按需获取以节省内存 (~4MB @ 10W 连接)

	// 时间戳（优化为 int64，节省 ~5.6MB @ 10W 连接）
	connectedAt   int64        // 连接时间（Unix 毫秒时间戳）
	lastHeartbeat atomic.Int64 // 最后心跳时间（Unix 毫秒时间戳）

	// 超时计数
	TimeoutCount atomic.Int32 // 连续超时次数（0-3）

	// 状态
	State protocol.ClientState // 连接状态（Idle/Connecting/Connected）

	// 并发控制
	mu sync.RWMutex // 保护 State 等字段的并发访问

	// 元数据（懒加载，按需创建）
	metadata map[string]interface{} // 自定义元数据

	// 断开后缓存的地址（可选）
	cachedRemoteAddr string // 仅在断开后需要时使用
}

// NewClientSession 创建新的客户端会话
func NewClientSession(clientID string, conn *quic.Conn) *ClientSession {
	nowMs := time.Now().UnixMilli()
	session := &ClientSession{
		ClientID:      clientID,
		Conn:          conn,
		connectedAt:   nowMs,
		State:         protocol.ClientState_CLIENT_STATE_CONNECTED,
		// metadata 懒加载，不预先创建
	}

	// 初始化 lastHeartbeat
	session.lastHeartbeat.Store(nowMs)

	return session
}

// GetLastHeartbeat 获取最后心跳时间
func (s *ClientSession) GetLastHeartbeat() time.Time {
	return time.UnixMilli(s.lastHeartbeat.Load())
}

// UpdateLastHeartbeat 更新最后心跳时间并重置超时计数
func (s *ClientSession) UpdateLastHeartbeat() {
	s.lastHeartbeat.Store(time.Now().UnixMilli())
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
	return time.Since(time.UnixMilli(s.connectedAt))
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
// 在关闭前缓存远程地址，以便断开后仍可获取
func (s *ClientSession) Close(reason string) error {
	s.mu.Lock()
	// 缓存远程地址（如果连接还存在）
	if s.Conn != nil {
		s.cachedRemoteAddr = s.Conn.RemoteAddr().String()
	}
	s.mu.Unlock()

	s.SetState(protocol.ClientState_CLIENT_STATE_IDLE)
	return s.Conn.CloseWithError(0, reason)
}

// GetRemoteAddr 获取客户端远程地址
// 如果连接已断开，返回缓存的地址
func (s *ClientSession) GetRemoteAddr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 如果连接还存在，直接从连接获取
	if s.Conn != nil {
		return s.Conn.RemoteAddr().String()
	}
	// 连接已断开，返回缓存的地址
	return s.cachedRemoteAddr
}

// GetMetadata 获取元数据
// 如果元数据未初始化，返回 nil, false
func (s *ClientSession) GetMetadata(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.metadata == nil {
		return nil, false
	}
	val, ok := s.metadata[key]
	return val, ok
}

// SetMetadata 设置元数据
// 使用懒加载模式，只在首次使用时创建 map
func (s *ClientSession) SetMetadata(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.metadata == nil {
		s.metadata = make(map[string]interface{}, 1)
	}
	s.metadata[key] = value
}

// ToClientInfo 转换为 ClientInfo protobuf 消息
func (s *ClientSession) ToClientInfo() *protocol.ClientInfo {
	return &protocol.ClientInfo{
		ClientId:      s.ClientID,
		RemoteAddr:    s.GetRemoteAddr(),
		ConnectedAt:   s.connectedAt,
		LastHeartbeat: s.lastHeartbeat.Load(),
		State:         s.GetState(),
	}
}
