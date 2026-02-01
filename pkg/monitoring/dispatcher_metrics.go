package monitoring

import (
	"sync"
	"sync/atomic"
	"time"
)

// DispatcherMetrics Dispatcher 分发器专用指标
type DispatcherMetrics struct {
	// 队列指标
	QueueLength      atomic.Int64 // 当前队列长度
	MaxQueueLength   atomic.Int64 // 最大队列长度（窗口内）
	QueueFullCount   atomic.Int64 // 队列满次数

	// 处理指标
	TasksProcessed   atomic.Int64 // 处理的任务总数
	TasksFailed      atomic.Int64 // 处理失败的任务数
	BatchProcessed   atomic.Int64 // 批量处理的任务数

	// 延迟指标
	ProcessingHistogram *Histogram // 处理延迟分布

	// 多队列指标（仅用于 MultiQueueDispatcher）
	QueueCount       atomic.Int64 // 队列数量
	MinQueueLength   atomic.Int64 // 最小队列长度
	MaxQueueLengthMQ atomic.Int64 // 最大队列长度（多队列）

	// 时间窗口
	windowStartTime atomic.Value // time.Time - 窗口开始时间
	windowDuration  time.Duration // 窗口大小
	mu              sync.RWMutex
}

// NewDispatcherMetrics 创建新的 Dispatcher 指标
func NewDispatcherMetrics() *DispatcherMetrics {
	dm := &DispatcherMetrics{
		ProcessingHistogram: NewHistogram(),
		windowDuration:      60 * time.Second,
	}
	dm.windowStartTime.Store(time.Now())
	return dm
}

// RecordTaskEnqueued 记录任务入队
func (dm *DispatcherMetrics) RecordTaskEnqueued(queueLength int) {
	dm.QueueLength.Store(int64(queueLength))

	// 更新最大队列长度
	for {
		current := dm.MaxQueueLength.Load()
		if int64(queueLength) <= current {
			break
		}
		if dm.MaxQueueLength.CompareAndSwap(current, int64(queueLength)) {
			break
		}
	}
}

// RecordTaskDequeued 记录任务出队
func (dm *DispatcherMetrics) RecordTaskDequeued(queueLength int) {
	dm.QueueLength.Store(int64(queueLength))
}

// RecordQueueFull 记录队列满事件
func (dm *DispatcherMetrics) RecordQueueFull() {
	dm.QueueFullCount.Add(1)
}

// RecordTaskProcessed 记录任务处理完成
func (dm *DispatcherMetrics) RecordTaskProcessed(latency time.Duration) {
	dm.TasksProcessed.Add(1)
	dm.ProcessingHistogram.Observe(latency.Milliseconds())
}

// RecordTaskFailed 记录任务处理失败
func (dm *DispatcherMetrics) RecordTaskFailed() {
	dm.TasksFailed.Add(1)
}

// RecordBatchProcessed 记录批量处理
func (dm *DispatcherMetrics) RecordBatchProcessed(count int) {
	dm.BatchProcessed.Add(1)
	dm.TasksProcessed.Add(int64(count))
}

// SetMultiQueueStats 设置多队列统计信息
func (dm *DispatcherMetrics) SetMultiQueueStats(queueCount, minLen, maxLen int) {
	dm.QueueCount.Store(int64(queueCount))
	dm.MinQueueLength.Store(int64(minLen))
	dm.MaxQueueLengthMQ.Store(int64(maxLen))
}

// GetSnapshot 获取 Dispatcher 指标快照
func (dm *DispatcherMetrics) GetSnapshot() *DispatcherMetricsSnapshot {
	now := time.Now()
	windowStart := dm.windowStartTime.Load().(time.Time)

	return &DispatcherMetricsSnapshot{
		// 队列指标
		QueueLength:    dm.QueueLength.Load(),
		MaxQueueLength: dm.MaxQueueLength.Load(),
		QueueFullCount: dm.QueueFullCount.Load(),

		// 处理指标
		TasksProcessed: dm.TasksProcessed.Load(),
		TasksFailed:    dm.TasksFailed.Load(),
		BatchProcessed: dm.BatchProcessed.Load(),

		// 延迟指标
		AvgProcessingLatency: float64(dm.ProcessingHistogram.Mean()),
		P50ProcessingLatency: float64(dm.ProcessingHistogram.Percentile(0.50)),
		P95ProcessingLatency: float64(dm.ProcessingHistogram.Percentile(0.95)),
		P99ProcessingLatency: float64(dm.ProcessingHistogram.Percentile(0.99)),

		// 多队列指标
		QueueCount:       dm.QueueCount.Load(),
		MinQueueLength:   dm.MinQueueLength.Load(),
		MaxQueueLengthMQ: dm.MaxQueueLengthMQ.Load(),

		// 时间指标
		WindowStartTime: windowStart,
		WindowDuration:  now.Sub(windowStart).Milliseconds(),
		Timestamp:       now.UnixMilli(),
	}
}

// ResetWindow 重置指标窗口
func (dm *DispatcherMetrics) ResetWindow() {
	dm.MaxQueueLength.Store(0)
	dm.MaxQueueLengthMQ.Store(0)
	dm.windowStartTime.Store(time.Now())
}

// DispatcherMetricsSnapshot Dispatcher 指标快照
type DispatcherMetricsSnapshot struct {
	// 队列指标
	QueueLength    int64 `json:"queue_length"`
	MaxQueueLength int64 `json:"max_queue_length"`
	QueueFullCount int64 `json:"queue_full_count"`

	// 处理指标
	TasksProcessed int64 `json:"tasks_processed"`
	TasksFailed    int64 `json:"tasks_failed"`
	BatchProcessed int64 `json:"batch_processed"`

	// 延迟指标（毫秒）
	AvgProcessingLatency float64 `json:"avg_processing_latency_ms"`
	P50ProcessingLatency float64 `json:"p50_processing_latency_ms"`
	P95ProcessingLatency float64 `json:"p95_processing_latency_ms"`
	P99ProcessingLatency float64 `json:"p99_processing_latency_ms"`

	// 多队列指标
	QueueCount       int64 `json:"queue_count,omitempty"`
	MinQueueLength   int64 `json:"min_queue_length,omitempty"`
	MaxQueueLengthMQ int64 `json:"max_queue_length_mq,omitempty"`

	// 时间指标
	WindowStartTime time.Time `json:"window_start_time"`
	WindowDuration  int64     `json:"window_duration_ms"`
	Timestamp       int64     `json:"timestamp"`
}

// SessionManagerMetrics SessionManager 会话管理器专用指标
type SessionManagerMetrics struct {
	// 会话指标
	SessionCount     atomic.Int64 // 当前会话数
	TotalSessions    atomic.Int64 // 总会话数（累计）
	SessionsAdded    atomic.Int64 // 新增会话数（窗口内）
	SessionsRemoved  atomic.Int64 // 移除会话数（窗口内）

	// 心跳指标
	HeartbeatChecks     atomic.Int64 // 心跳检查次数
	HeartbeatTimeouts   atomic.Int64 // 心跳超时次数
	HeartbeatRecoveries atomic.Int64 // 心跳恢复次数

	// 分片指标（仅用于 ShardedSessionManager）
	ShardCount     atomic.Int64 // 分片数量
	MinShardSize   atomic.Int64 // 最小分片大小
	MaxShardSize   atomic.Int64 // 最大分片大小
	AvgShardSize   atomic.Int64 // 平均分片大小（x100）

	// 时间窗口
	windowStartTime atomic.Value
	mu              sync.RWMutex
}

// NewSessionManagerMetrics 创建新的 SessionManager 指标
func NewSessionManagerMetrics() *SessionManagerMetrics {
	sm := &SessionManagerMetrics{}
	sm.windowStartTime.Store(time.Now())
	return sm
}

// RecordSessionAdded 记录会话添加
func (sm *SessionManagerMetrics) RecordSessionAdded() {
	sm.SessionCount.Add(1)
	sm.TotalSessions.Add(1)
	sm.SessionsAdded.Add(1)
}

// RecordSessionRemoved 记录会话移除
func (sm *SessionManagerMetrics) RecordSessionRemoved() {
	sm.SessionCount.Add(-1)
	sm.SessionsRemoved.Add(1)
}

// RecordHeartbeatCheck 记录心跳检查
func (sm *SessionManagerMetrics) RecordHeartbeatCheck() {
	sm.HeartbeatChecks.Add(1)
}

// RecordHeartbeatTimeout 记录心跳超时
func (sm *SessionManagerMetrics) RecordHeartbeatTimeout() {
	sm.HeartbeatTimeouts.Add(1)
}

// RecordHeartbeatRecovery 记录心跳恢复
func (sm *SessionManagerMetrics) RecordHeartbeatRecovery() {
	sm.HeartbeatRecoveries.Add(1)
}

// SetShardStats 设置分片统计信息
func (sm *SessionManagerMetrics) SetShardStats(shardCount, minSize, maxSize, avgSize int) {
	sm.ShardCount.Store(int64(shardCount))
	sm.MinShardSize.Store(int64(minSize))
	sm.MaxShardSize.Store(int64(maxSize))
	sm.AvgShardSize.Store(int64(avgSize * 100)) // 保留两位小数
}

// GetSnapshot 获取 SessionManager 指标快照
func (sm *SessionManagerMetrics) GetSnapshot() *SessionManagerMetricsSnapshot {
	now := time.Now()
	windowStart := sm.windowStartTime.Load().(time.Time)

	avgSize := sm.AvgShardSize.Load()
	snapshot := &SessionManagerMetricsSnapshot{
		// 会话指标
		SessionCount:    sm.SessionCount.Load(),
		TotalSessions:   sm.TotalSessions.Load(),
		SessionsAdded:   sm.SessionsAdded.Load(),
		SessionsRemoved: sm.SessionsRemoved.Load(),

		// 心跳指标
		HeartbeatChecks:     sm.HeartbeatChecks.Load(),
		HeartbeatTimeouts:   sm.HeartbeatTimeouts.Load(),
		HeartbeatRecoveries: sm.HeartbeatRecoveries.Load(),

		// 分片指标
		ShardCount:     sm.ShardCount.Load(),
		MinShardSize:   sm.MinShardSize.Load(),
		MaxShardSize:   sm.MaxShardSize.Load(),
		AvgShardSize:   float64(avgSize) / 100,

		// 时间指标
		WindowStartTime: windowStart,
		WindowDuration:  now.Sub(windowStart).Milliseconds(),
		Timestamp:       now.UnixMilli(),
	}

	return snapshot
}

// ResetWindow 重置指标窗口
func (sm *SessionManagerMetrics) ResetWindow() {
	sm.SessionsAdded.Store(0)
	sm.SessionsRemoved.Store(0)
	sm.HeartbeatChecks.Store(0)
	sm.HeartbeatTimeouts.Store(0)
	sm.HeartbeatRecoveries.Store(0)
	sm.windowStartTime.Store(time.Now())
}

// SessionManagerMetricsSnapshot SessionManager 指标快照
type SessionManagerMetricsSnapshot struct {
	// 会话指标
	SessionCount    int64 `json:"session_count"`
	TotalSessions   int64 `json:"total_sessions"`
	SessionsAdded   int64 `json:"sessions_added"`
	SessionsRemoved int64 `json:"sessions_removed"`

	// 心跳指标
	HeartbeatChecks     int64 `json:"heartbeat_checks"`
	HeartbeatTimeouts   int64 `json:"heartbeat_timeouts"`
	HeartbeatRecoveries int64 `json:"heartbeat_recoveries"`

	// 分片指标
	ShardCount   int64   `json:"shard_count,omitempty"`
	MinShardSize int64   `json:"min_shard_size,omitempty"`
	MaxShardSize int64   `json:"max_shard_size,omitempty"`
	AvgShardSize float64 `json:"avg_shard_size,omitempty"`

	// 时间指标
	WindowStartTime time.Time `json:"window_start_time"`
	WindowDuration  int64     `json:"window_duration_ms"`
	Timestamp       int64     `json:"timestamp"`
}
