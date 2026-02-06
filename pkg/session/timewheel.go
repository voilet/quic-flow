package session

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/voilet/quic-flow/pkg/monitoring"
)

// TimeWheelHeartbeatChecker 时间轮心跳检查器
// 使用时间轮算法将心跳检查的复杂度从 O(n) 降到 O(1)
// 适用于 10 万+ 长连接场景
type TimeWheelHeartbeatChecker struct {
	manager Manager // 反向引用会话管理器

	// 时间轮配置
	slots       []map[string]struct{} // 环形槽位，每个槽位存储该时间到期的会话 ID
	slotSize    int                   // 槽位数（默认 60，对应 60 秒）
	current     int                   // 当前指针位置
	tick        atomic.Int64          // 当前 tick 计数

	// 会话映射（用于快速查找和删除）
	sessionToSlot sync.Map // clientID -> slotIndex
	sessionExpiry sync.Map // clientID -> expiryTick

	// 时间精度
	tickInterval time.Duration // 每个 tick 的时间间隔（1 秒）

	// 配置
	heartbeatTimeout time.Duration // 心跳超时阈值
	maxTimeoutCount  int32         // 最大超时次数

	// 事件钩子和日志
	hooks  *monitoring.EventHooks
	logger *monitoring.Logger

	// 控制
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// Manager 会话管理器接口（解耦具体实现）
type Manager interface {
	Get(clientID string) (*ClientSession, error)
	Remove(clientID string) error
	Count() int64
}

// NewTimeWheelHeartbeatChecker 创建新的时间轮心跳检查器
func NewTimeWheelHeartbeatChecker(manager Manager, config TimeWheelConfig) *TimeWheelHeartbeatChecker {
	// 设置默认值
	if config.SlotSize == 0 {
		config.SlotSize = 60 // 60 秒
	}
	if config.TickInterval == 0 {
		config.TickInterval = 1 * time.Second
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

	// 创建时间轮槽位
	slots := make([]map[string]struct{}, config.SlotSize)
	for i := 0; i < config.SlotSize; i++ {
		slots[i] = make(map[string]struct{})
	}

	tw := &TimeWheelHeartbeatChecker{
		manager:          manager,
		slots:            slots,
		slotSize:         config.SlotSize,
		current:          0,
		tickInterval:     config.TickInterval,
		heartbeatTimeout: config.HeartbeatTimeout,
		maxTimeoutCount:  config.MaxTimeoutCount,
		hooks:            config.Hooks,
		logger:           config.Logger,
		stopCh:           make(chan struct{}),
	}

	return tw
}

// TimeWheelConfig 时间轮配置
type TimeWheelConfig struct {
	SlotSize         int                      // 槽位数（必须是 2 的幂或合适的数值）
	TickInterval     time.Duration            // 每个 tick 的时间间隔
	HeartbeatTimeout time.Duration            // 心跳超时阈值
	MaxTimeoutCount  int32                    // 最大超时次数
	Hooks            *monitoring.EventHooks   // 事件钩子
	Logger           *monitoring.Logger       // 日志记录器
}

// Start 启动时间轮
func (tw *TimeWheelHeartbeatChecker) Start() {
	tw.wg.Add(1)
	go tw.run()

	tw.logger.Info("TimeWheel heartbeat checker started",
		"slot_size", tw.slotSize,
		"tick_interval", tw.tickInterval,
		"heartbeat_timeout", tw.heartbeatTimeout)
}

// Stop 停止时间轮
func (tw *TimeWheelHeartbeatChecker) Stop() {
	close(tw.stopCh)
	tw.wg.Wait()

	tw.logger.Info("TimeWheel heartbeat checker stopped")
}

// run 时间轮主循环
func (tw *TimeWheelHeartbeatChecker) run() {
	defer tw.wg.Done()

	ticker := time.NewTicker(tw.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tw.onTick()
		case <-tw.stopCh:
			return
		}
	}
}

// onTick 每个 tick 触发一次
func (tw *TimeWheelHeartbeatChecker) onTick() {
	// 移动指针到下一槽位
	tw.current = (tw.current + 1) % tw.slotSize
	tw.tick.Add(1)

	// 获取当前槽位的所有会话
	currentTick := tw.tick.Load()
	slot := tw.slots[tw.current]

	// 检查槽位中的会话是否真正到期
	var expiredClients []string
	for clientID := range slot {
		if expiryTick, ok := tw.sessionExpiry.Load(clientID); ok {
			if expiryTick.(int64) <= currentTick {
				expiredClients = append(expiredClients, clientID)
			}
		}
	}

	// 处理到期的会话
	tw.processExpiredSessions(expiredClients)
}

// processExpiredSessions 处理到期的会话
func (tw *TimeWheelHeartbeatChecker) processExpiredSessions(clients []string) {
	if len(clients) == 0 {
		return
	}

	var timeoutClients []string
	var removedClients []string

	now := time.Now()

	for _, clientID := range clients {
		session, err := tw.manager.Get(clientID)
		if err != nil {
			// 会话不存在，从时间轮移除
			tw.Unregister(clientID)
			continue
		}

		// 检查心跳是否真正超时
		lastHB := session.GetLastHeartbeat()
		timeSinceLastHB := now.Sub(lastHB)

		if timeSinceLastHB > 15*time.Second {
			timeoutCount := session.IncrementTimeoutCount()

			if timeoutCount == 1 {
				timeoutClients = append(timeoutClients, clientID)
			}

			// 达到最大超时次数，清理会话
			if timeoutCount >= tw.maxTimeoutCount {
				// 触发钩子
				if tw.hooks != nil {
					tw.hooks.SafeOnHeartbeatTimeout(clientID)
				}

				// 关闭连接
				if err := session.Close("heartbeat timeout"); err != nil {
					tw.logger.Error("Failed to close session",
						"client_id", clientID,
						"error", err)
				}

				// 移除会话
				if err := tw.manager.Remove(clientID); err != nil {
					tw.logger.Error("Failed to remove session",
						"client_id", clientID,
						"error", err)
				} else {
					removedClients = append(removedClients, clientID)
				}

				// 从时间轮移除
				tw.Unregister(clientID)
			}
		} else {
			// 心跳正常，重置超时计数
			if session.GetTimeoutCount() > 0 {
				session.ResetTimeoutCount()
			}
			// 重新计算超时时间
			tw.UpdateHeartbeat(clientID)
		}
	}

	// 汇总日志
	if len(timeoutClients) > 0 || len(removedClients) > 0 {
		tw.logger.Warn("TimeWheel heartbeat check summary",
			"timeout_count", len(timeoutClients),
			"removed_count", len(removedClients),
			"slot", tw.current,
			"total_sessions", tw.manager.Count())
	}
}

// Register 注册会话到时间轮
// 在新会话建立时调用
func (tw *TimeWheelHeartbeatChecker) Register(clientID string) {
	expiryTick := tw.tick.Load() + int64(tw.heartbeatTimeout/tw.tickInterval)
	slotIndex := int(expiryTick % int64(tw.slotSize))

	tw.sessionToSlot.Store(clientID, slotIndex)
	tw.sessionExpiry.Store(clientID, expiryTick)

	tw.slots[slotIndex][clientID] = struct{}{}

	tw.logger.Debug("Session registered to time wheel",
		"client_id", clientID,
		"slot", slotIndex,
		"expiry_tick", expiryTick)
}

// UpdateHeartbeat 更新会话心跳时间
// 在收到心跳时调用，将会话移动到新的槽位
func (tw *TimeWheelHeartbeatChecker) UpdateHeartbeat(clientID string) {
	// 从旧槽位移除
	if oldSlotIdx, ok := tw.sessionToSlot.Load(clientID); ok {
		delete(tw.slots[oldSlotIdx.(int)], clientID)
	}

	// 添加到新槽位
	expiryTick := tw.tick.Load() + int64(tw.heartbeatTimeout/tw.tickInterval)
	slotIndex := int(expiryTick % int64(tw.slotSize))

	tw.sessionToSlot.Store(clientID, slotIndex)
	tw.sessionExpiry.Store(clientID, expiryTick)

	tw.slots[slotIndex][clientID] = struct{}{}
}

// Unregister 从时间轮注销会话
// 在会话断开时调用
func (tw *TimeWheelHeartbeatChecker) Unregister(clientID string) {
	// 从槽位移除
	if slotIdx, ok := tw.sessionToSlot.Load(clientID); ok {
		delete(tw.slots[slotIdx.(int)], clientID)
	}

	// 从映射中移除
	tw.sessionToSlot.Delete(clientID)
	tw.sessionExpiry.Delete(clientID)

	tw.logger.Debug("Session unregistered from time wheel",
		"client_id", clientID)
}

// GetStats 获取时间轮统计信息
func (tw *TimeWheelHeartbeatChecker) GetStats() *TimeWheelStats {
	slotSizes := make([]int, tw.slotSize)
	totalSessions := 0
	minSize := int(^uint(0) >> 1)
	maxSize := 0

	for i, slot := range tw.slots {
		size := len(slot)
		slotSizes[i] = size
		totalSessions += size

		if size < minSize {
			minSize = size
		}
		if size > maxSize {
			maxSize = size
		}
	}

	avgSize := 0.0
	if tw.slotSize > 0 {
		avgSize = float64(totalSessions) / float64(tw.slotSize)
	}

	return &TimeWheelStats{
		SlotSize:       tw.slotSize,
		CurrentSlot:    tw.current,
		CurrentTick:    tw.tick.Load(),
		SlotSizes:      slotSizes,
		TotalSessions:  totalSessions,
		MinSlotSize:    minSize,
		MaxSlotSize:    maxSize,
		AvgSlotSize:    avgSize,
		TickInterval:   tw.tickInterval,
		HeartbeatTimeout: tw.heartbeatTimeout,
	}
}

// TimeWheelStats 时间轮统计信息
type TimeWheelStats struct {
	SlotSize        int           // 槽位总数
	CurrentSlot     int           // 当前槽位
	CurrentTick     int64         // 当前 tick
	SlotSizes       []int         // 每个槽位的会话数
	TotalSessions   int           // 总会话数
	MinSlotSize     int           // 最小槽位大小
	MaxSlotSize     int           // 最大槽位大小
	AvgSlotSize     float64       // 平均槽位大小
	TickInterval    time.Duration // tick 间隔
	HeartbeatTimeout time.Duration // 心跳超时阈值
}
