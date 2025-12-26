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

	sm.Range(func(clientID string, session *ClientSession) bool {
		lastHB := session.GetLastHeartbeat()
		timeSinceLastHB := now.Sub(lastHB)

		// 检查是否超过心跳间隔（15 秒）
		if timeSinceLastHB > 15*time.Second {
			timeoutCount := session.IncrementTimeoutCount()

			sm.logger.Warn("Heartbeat timeout detected",
				"client_id", clientID,
				"time_since_last_hb", timeSinceLastHB,
				"timeout_count", timeoutCount,
				"max_timeout_count", sm.maxTimeoutCount)

			// 达到最大超时次数，清理会话
			if timeoutCount >= sm.maxTimeoutCount {
				sm.logger.Error("Heartbeat timeout threshold reached, removing session",
					"client_id", clientID,
					"timeout_count", timeoutCount)

				// 触发钩子
				if sm.hooks != nil {
					sm.hooks.SafeOnHeartbeatTimeout(clientID)
				}

				// 关闭连接
				if err := session.Close("heartbeat timeout"); err != nil {
					sm.logger.Error("Failed to close session",
						"client_id", clientID,
						"error", err)
				}

				// 移除会话
				if err := sm.Remove(clientID); err != nil {
					sm.logger.Error("Failed to remove session",
						"client_id", clientID,
						"error", err)
				}
			}
		} else {
			// 心跳正常，重置超时计数
			if session.GetTimeoutCount() > 0 {
				session.ResetTimeoutCount()
				sm.logger.Debug("Heartbeat timeout count reset",
					"client_id", clientID)
			}
		}

		return true // 继续遍历
	})
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
