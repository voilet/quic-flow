package callback

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/protocol"
)

const (
	// MaxPromiseCapacity Promise 最大容量（防止内存无限增长）
	MaxPromiseCapacity = 50000

	// DefaultPromiseCleanupInterval 默认清理间隔
	DefaultPromiseCleanupInterval = 10 * time.Second
)

// PromiseManager Promise 管理器 (T041)
// 负责创建、存储、查找和清理 Promise
type PromiseManager struct {
	promises sync.Map // map[string]*Promise

	// 计数器
	count atomic.Int64

	// 配置
	maxCapacity     int64
	cleanupInterval time.Duration
	defaultTimeout  time.Duration

	// 监控
	logger  *monitoring.Logger
	metrics *monitoring.Metrics
	hooks   *monitoring.EventHooks // 添加钩子支持 (T050)

	// 控制
	ctx    chan struct{}
	wg     sync.WaitGroup
}

// PromiseManagerConfig PromiseManager 配置
type PromiseManagerConfig struct {
	MaxCapacity     int64                  // 最大容量（默认 50000）
	CleanupInterval time.Duration          // 清理间隔（默认 10s）
	DefaultTimeout  time.Duration          // 默认超时时间（默认 30s）
	Logger          *monitoring.Logger
	Metrics         *monitoring.Metrics
	Hooks           *monitoring.EventHooks // 事件钩子 (T050)
}

// NewPromiseManager 创建新的 PromiseManager
func NewPromiseManager(config *PromiseManagerConfig) *PromiseManager {
	if config == nil {
		config = &PromiseManagerConfig{}
	}

	// 默认值
	if config.MaxCapacity <= 0 {
		config.MaxCapacity = MaxPromiseCapacity
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = DefaultPromiseCleanupInterval
	}
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = 30 * time.Second
	}
	if config.Logger == nil {
		config.Logger = monitoring.NewLogger(monitoring.LogLevelInfo, "text")
	}
	if config.Metrics == nil {
		config.Metrics = monitoring.NewMetrics()
	}

	pm := &PromiseManager{
		maxCapacity:     config.MaxCapacity,
		cleanupInterval: config.CleanupInterval,
		defaultTimeout:  config.DefaultTimeout,
		logger:          config.Logger,
		metrics:         config.Metrics,
		hooks:           config.Hooks, // 设置钩子 (T050)
		ctx:             make(chan struct{}),
	}

	return pm
}

// Start 启动 PromiseManager（启动清理 goroutine）
func (pm *PromiseManager) Start() {
	pm.logger.Info("Starting PromiseManager", "cleanup_interval", pm.cleanupInterval)

	pm.wg.Add(1)
	go pm.cleanupLoop()

	pm.logger.Info("PromiseManager started")
}

// Stop 停止 PromiseManager
func (pm *PromiseManager) Stop() {
	pm.logger.Info("Stopping PromiseManager...")

	close(pm.ctx)
	pm.wg.Wait()

	pm.logger.Info("PromiseManager stopped")
}

// Create 创建新的 Promise
// msgID: 消息 ID
// timeout: 超时时间（0 表示使用默认值）
func (pm *PromiseManager) Create(msgID string, timeout time.Duration) (*Promise, error) {
	// 检查容量
	currentCount := pm.count.Load()
	if currentCount >= pm.maxCapacity {
		pm.logger.Warn("Promise capacity full", "count", currentCount, "max", pm.maxCapacity)
		return nil, ErrPromiseCapacityFull
	}

	// 使用默认超时
	if timeout <= 0 {
		timeout = pm.defaultTimeout
	}

	// 创建 Promise
	promise := NewPromise(msgID, timeout, pm.onTimeout)

	// 存储 Promise
	pm.promises.Store(msgID, promise)
	pm.count.Add(1)

	// 记录指标
	pm.metrics.RecordPromiseCreated()

	pm.logger.Debug("Promise created", "msg_id", msgID, "timeout", timeout)

	return promise, nil
}

// Get 获取 Promise
func (pm *PromiseManager) Get(msgID string) (*Promise, error) {
	value, ok := pm.promises.Load(msgID)
	if !ok {
		return nil, ErrPromiseNotFound
	}

	return value.(*Promise), nil
}

// Remove 删除 Promise
func (pm *PromiseManager) Remove(msgID string) {
	if _, loaded := pm.promises.LoadAndDelete(msgID); loaded {
		pm.count.Add(-1)
		pm.logger.Debug("Promise removed", "msg_id", msgID)
	}
}

// Complete 完成 Promise（收到 Ack）
func (pm *PromiseManager) Complete(msgID string, ack *protocol.AckMessage) error {
	promise, err := pm.Get(msgID)
	if err != nil {
		return err
	}

	if promise.Complete(ack) {
		pm.logger.Debug("Promise completed", "msg_id", msgID, "status", ack.Status)
		pm.metrics.RecordPromiseCompleted()
		// 延迟删除，让接收方有时间读取
		go func() {
			time.Sleep(100 * time.Millisecond)
			pm.Remove(msgID)
		}()
		return nil
	}

	return fmt.Errorf("promise already completed")
}

// Fail 失败 Promise
func (pm *PromiseManager) Fail(msgID string, err error) error {
	promise, promiseErr := pm.Get(msgID)
	if promiseErr != nil {
		return promiseErr
	}

	if promise.Fail(err) {
		pm.logger.Debug("Promise failed", "msg_id", msgID, "error", err)
		// 延迟删除
		go func() {
			time.Sleep(100 * time.Millisecond)
			pm.Remove(msgID)
		}()
		return nil
	}

	return fmt.Errorf("promise already completed")
}

// onTimeout 超时回调
func (pm *PromiseManager) onTimeout(msgID string) {
	pm.logger.Warn("Promise timeout", "msg_id", msgID)
	pm.metrics.RecordPromiseTimeout()

	// 触发钩子 (T050)
	if pm.hooks != nil {
		pm.hooks.SafeOnPromiseTimeout(msgID)
	}

	// 延迟删除，让接收方有时间读取超时错误
	go func() {
		time.Sleep(100 * time.Millisecond)
		pm.Remove(msgID)
	}()
}

// cleanupLoop 定期清理过期的 Promise
func (pm *PromiseManager) cleanupLoop() {
	defer pm.wg.Done()

	ticker := time.NewTicker(pm.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx:
			pm.logger.Debug("Cleanup loop stopped")
			return

		case <-ticker.C:
			pm.cleanup()
		}
	}
}

// cleanup 清理已完成或超时的 Promise
func (pm *PromiseManager) cleanup() {
	var toRemove []string

	pm.promises.Range(func(key, value interface{}) bool {
		msgID := key.(string)
		promise := value.(*Promise)

		if promise.IsCompleted() {
			toRemove = append(toRemove, msgID)
		}

		return true
	})

	if len(toRemove) > 0 {
		for _, msgID := range toRemove {
			pm.Remove(msgID)
		}
		pm.logger.Debug("Cleaned up promises", "count", len(toRemove))
	}
}

// GetCount 获取当前 Promise 数量
func (pm *PromiseManager) GetCount() int64 {
	return pm.count.Load()
}
