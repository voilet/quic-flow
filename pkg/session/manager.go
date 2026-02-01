package session

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	pkgerrors "github.com/voilet/quic-flow/pkg/errors"
	"github.com/voilet/quic-flow/pkg/monitoring"
)

// SessionManager 管理所有客户端会话
type SessionManager struct {
	sessions sync.Map // clientID (string) -> *ClientSession

	count atomic.Int64 // 当前会话数量

	// 心跳检查
	heartbeatTick     *time.Ticker           // 心跳检查定时器
	heartbeatInterval time.Duration          // 心跳检查间隔（默认 5 秒）
	heartbeatTimeout  time.Duration          // 心跳超时阈值（默认 45 秒）
	maxTimeoutCount   int32                  // 最大超时次数（默认 3 次）

	// 事件钩子
	hooks *monitoring.EventHooks

	// 日志
	logger *monitoring.Logger

	// 停止信号
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// SessionManagerConfig SessionManager 配置
type SessionManagerConfig struct {
	HeartbeatCheckInterval time.Duration // 心跳检查间隔（建议 5 秒）
	HeartbeatTimeout       time.Duration // 心跳超时阈值（建议 45 秒，即 3 × 15 秒）
	MaxTimeoutCount        int32         // 最大超时次数（建议 3 次）
	Hooks                  *monitoring.EventHooks
	Logger                 *monitoring.Logger
}

// NewSessionManager 创建新的 SessionManager
func NewSessionManager(config SessionManagerConfig) *SessionManager {
	// 设置默认值
	if config.HeartbeatCheckInterval == 0 {
		config.HeartbeatCheckInterval = 5 * time.Second
	}
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 45 * time.Second
	}
	if config.MaxTimeoutCount == 0 {
		config.MaxTimeoutCount = 3
	}
	if config.Logger == nil {
		config.Logger = monitoring.NewDefaultLogger()
	}

	sm := &SessionManager{
		heartbeatInterval: config.HeartbeatCheckInterval,
		heartbeatTimeout:  config.HeartbeatTimeout,
		maxTimeoutCount:   config.MaxTimeoutCount,
		hooks:             config.Hooks,
		logger:            config.Logger,
		stopCh:            make(chan struct{}),
	}

	return sm
}

// Start 启动心跳检查器
func (sm *SessionManager) Start() {
	sm.heartbeatTick = time.NewTicker(sm.heartbeatInterval)
	sm.wg.Add(1)

	go sm.heartbeatChecker()

	sm.logger.Info("SessionManager started",
		"heartbeat_interval", sm.heartbeatInterval,
		"heartbeat_timeout", sm.heartbeatTimeout)
}

// Stop 停止心跳检查器
func (sm *SessionManager) Stop() {
	close(sm.stopCh)
	if sm.heartbeatTick != nil {
		sm.heartbeatTick.Stop()
	}
	sm.wg.Wait()

	sm.logger.Info("SessionManager stopped")
}

// Add 添加新会话
func (sm *SessionManager) Add(session *ClientSession) error {
	if session == nil {
		return fmt.Errorf("%w: session is nil", pkgerrors.ErrInvalidConfig)
	}

	if session.ClientID == "" {
		return pkgerrors.ErrInvalidClientID
	}

	// 检查是否已存在
	if _, exists := sm.sessions.Load(session.ClientID); exists {
		return fmt.Errorf("%w: %s", pkgerrors.ErrSessionAlreadyExists, session.ClientID)
	}

	sm.sessions.Store(session.ClientID, session)
	sm.count.Add(1)

	sm.logger.Info("Session added",
		"client_id", session.ClientID,
		"remote_addr", session.RemoteAddr)

	return nil
}

// Remove 移除会话
func (sm *SessionManager) Remove(clientID string) error {
	if clientID == "" {
		return pkgerrors.ErrInvalidClientID
	}

	val, loaded := sm.sessions.LoadAndDelete(clientID)
	if !loaded {
		return fmt.Errorf("%w: %s", pkgerrors.ErrSessionNotFound, clientID)
	}

	sm.count.Add(-1)

	session := val.(*ClientSession)
	sm.logger.Info("Session removed",
		"client_id", clientID,
		"uptime", session.GetUptime())

	return nil
}

// Get 获取会话
func (sm *SessionManager) Get(clientID string) (*ClientSession, error) {
	if clientID == "" {
		return nil, pkgerrors.ErrInvalidClientID
	}

	val, ok := sm.sessions.Load(clientID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", pkgerrors.ErrSessionNotFound, clientID)
	}

	return val.(*ClientSession), nil
}

// Exists 检查会话是否存在
func (sm *SessionManager) Exists(clientID string) bool {
	_, ok := sm.sessions.Load(clientID)
	return ok
}

// Count 获取当前会话数量
func (sm *SessionManager) Count() int64 {
	return sm.count.Load()
}

// Range 遍历所有会话
// 回调函数返回 false 停止遍历
func (sm *SessionManager) Range(f func(clientID string, session *ClientSession) bool) {
	sm.sessions.Range(func(key, value interface{}) bool {
		clientID := key.(string)
		session := value.(*ClientSession)
		return f(clientID, session)
	})
}

// ListClientIDs 获取所有客户端 ID 列表
func (sm *SessionManager) ListClientIDs() []string {
	var ids []string
	sm.Range(func(clientID string, _ *ClientSession) bool {
		ids = append(ids, clientID)
		return true
	})
	return ids
}

// ClientInfoBrief 客户端简要信息（用于列表展示）
type ClientInfoBrief struct {
	ClientID    string `json:"client_id"`
	RemoteAddr  string `json:"remote_addr"`
	ConnectedAt int64  `json:"connected_at"`
}

// ListClientsWithDetails 获取所有客户端详情（一次遍历）
// 比 ListClientIDs + 循环 Get 性能更好
func (sm *SessionManager) ListClientsWithDetails() []ClientInfoBrief {
	// 预分配容量
	count := sm.count.Load()
	result := make([]ClientInfoBrief, 0, count)

	sm.sessions.Range(func(key, value interface{}) bool {
		session := value.(*ClientSession)
		result = append(result, ClientInfoBrief{
			ClientID:    session.ClientID,
			RemoteAddr:  session.RemoteAddr,
			ConnectedAt: session.ConnectedAt.UnixMilli(),
		})
		return true
	})

	return result
}

// ListClientsWithDetailsPaginated 分页获取客户端详情
// offset: 起始位置
// limit: 返回数量（0 表示全部）
// 返回: 客户端列表, 总数
func (sm *SessionManager) ListClientsWithDetailsPaginated(offset, limit int) ([]ClientInfoBrief, int64) {
	total := sm.count.Load()

	// 如果 offset 超过总数，返回空列表
	if int64(offset) >= total {
		return []ClientInfoBrief{}, total
	}

	// ========== 性能优化：提前终止遍历 ==========
	// 不再收集所有客户端再分片，而是在遍历时跳过前面的元素
	// 当收集够 limit 个元素后立即停止遍历
	result := make([]ClientInfoBrief, 0, min(limit, int(total)))
	skipped := 0
	collected := 0

	sm.sessions.Range(func(key, value interface{}) bool {
		// 跳过前面的元素
		if skipped < offset {
			skipped++
			return true
		}

		// 收集元素直到达到 limit
		if limit > 0 && collected >= limit {
			return false // 停止遍历
		}

		session := value.(*ClientSession)
		result = append(result, ClientInfoBrief{
			ClientID:    session.ClientID,
			RemoteAddr:  session.RemoteAddr,
			ConnectedAt: session.ConnectedAt.UnixMilli(),
		})
		collected++
		return true
	})

	return result, total
}

// heartbeatChecker 心跳检查器（独立 goroutine）
func (sm *SessionManager) heartbeatChecker() {
	defer sm.wg.Done()

	sm.logger.Debug("Heartbeat checker started")

	for {
		select {
		case <-sm.heartbeatTick.C:
			sm.checkHeartbeats()
		case <-sm.stopCh:
			sm.logger.Debug("Heartbeat checker stopped")
			return
		}
	}
}

// checkHeartbeats 检查所有会话的心跳状态
func (sm *SessionManager) checkHeartbeats() {
	now := time.Now()

	// ========== 性能优化：汇总日志，减少 I/O 开销 ==========
	// 10 万连接场景下，每条超时日志都是开销
	// 使用汇总日志代替逐条日志，大幅减少日志 I/O
	var timeoutClients []string
	var removedClients []string

	sm.Range(func(clientID string, session *ClientSession) bool {
		lastHB := session.GetLastHeartbeat()
		timeSinceLastHB := now.Sub(lastHB)

		// 检查是否超过心跳间隔（15 秒）
		if timeSinceLastHB > 15*time.Second {
			timeoutCount := session.IncrementTimeoutCount()

			// 收集超时客户端（不逐条记录日志）
			if timeoutCount == 1 {
				timeoutClients = append(timeoutClients, clientID)
			}

			// 达到最大超时次数，清理会话
			if timeoutCount >= sm.maxTimeoutCount {
				// 触发钩子
				if sm.hooks != nil {
					sm.hooks.SafeOnHeartbeatTimeout(clientID)
				}

				// 关闭连接
				if err := session.Close("heartbeat timeout"); err != nil {
					// 只在错误时记录
					sm.logger.Error("Failed to close session",
						"client_id", clientID,
						"error", err)
				}

				// 移除会话
				if err := sm.Remove(clientID); err != nil {
					sm.logger.Error("Failed to remove session",
						"client_id", clientID,
						"error", err)
				} else {
					removedClients = append(removedClients, clientID)
				}
			}
		} else {
			// 心跳正常，重置超时计数（无日志）
			if session.GetTimeoutCount() > 0 {
				session.ResetTimeoutCount()
			}
		}

		return true // 继续遍历
	})

	// 汇总记录一条日志（替代逐条日志）
	if len(timeoutClients) > 0 || len(removedClients) > 0 {
		sm.logger.Warn("Heartbeat check summary",
			"timeout_count", len(timeoutClients),
			"removed_count", len(removedClients),
			"total_sessions", sm.Count())
	}
}

// CloseAll 关闭所有会话
func (sm *SessionManager) CloseAll(reason string) {
	sm.logger.Info("Closing all sessions", "reason", reason)

	var closedCount int64
	sm.Range(func(clientID string, session *ClientSession) bool {
		if err := session.Close(reason); err != nil {
			sm.logger.Error("Failed to close session",
				"client_id", clientID,
				"error", err)
		} else {
			closedCount++
		}
		return true
	})

	sm.logger.Info("All sessions closed", "count", closedCount)

	// 清空会话映射
	sm.sessions = sync.Map{}
	sm.count.Store(0)
}
