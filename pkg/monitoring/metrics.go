package monitoring

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/voilet/quic-flow/pkg/protocol"
)

// Metrics 定义系统指标结构，使用 atomic 保证并发安全
type Metrics struct {
	// 连接相关指标
	ConnectedClients atomic.Int64 // 当前连接的客户端数量
	TotalConnections atomic.Int64 // 总连接数（累计）
	TotalDisconnects atomic.Int64 // 总断开数（累计）

	// 消息相关指标
	MessagesSent     atomic.Int64 // 发送的消息总数
	MessagesReceived atomic.Int64 // 接收的消息总数
	MessagesFailed   atomic.Int64 // 发送失败的消息数
	BytesSent        atomic.Int64 // 发送的字节总数
	BytesReceived    atomic.Int64 // 接收的字节总数

	// 心跳相关指标
	HeartbeatsSent    atomic.Int64 // 发送的心跳数
	HeartbeatsReceived atomic.Int64 // 接收的心跳数
	HeartbeatTimeouts atomic.Int64 // 心跳超时次数

	// 回调相关指标
	PromiseCreated   atomic.Int64 // 创建的 Promise 数量
	PromiseCompleted atomic.Int64 // 完成的 Promise 数量
	PromiseTimeouts  atomic.Int64 // 超时的 Promise 数量
	PromiseRejections atomic.Int64 // 因容量满被拒绝的 Promise 数量
	PromiseWarnTriggered atomic.Int64 // Promise 容量警告触发次数
	PromiseCleanups  atomic.Int64 // Promise 清理次数

	// 广播相关指标
	BroadcastsSent   atomic.Int64 // 广播消息次数
	BroadcastTargets atomic.Int64 // 广播目标客户端总数

	// 错误相关指标
	EncodingErrors atomic.Int64 // 编码错误次数
	DecodingErrors atomic.Int64 // 解码错误次数
	NetworkErrors  atomic.Int64 // 网络错误次数

	// 延迟统计（由 Histogram 计算）
	latencyHistogram *Histogram

	// 时间窗口（用于计算吞吐量）
	lastResetTime atomic.Value // time.Time - 最后一次重置时间
	startTime     time.Time    // 启动时间
}

// NewMetrics 创建新的 Metrics 实例
func NewMetrics() *Metrics {
	m := &Metrics{
		latencyHistogram: NewHistogram(),
		startTime:        time.Now(),
	}
	m.lastResetTime.Store(time.Now())
	return m
}

// RecordMessageSent 记录发送消息
func (m *Metrics) RecordMessageSent(size int64) {
	m.MessagesSent.Add(1)
	m.BytesSent.Add(size)
}

// RecordMessageReceived 记录接收消息
func (m *Metrics) RecordMessageReceived(size int64) {
	m.MessagesReceived.Add(1)
	m.BytesReceived.Add(size)
}

// RecordMessageFailed 记录消息发送失败
func (m *Metrics) RecordMessageFailed() {
	m.MessagesFailed.Add(1)
}

// RecordLatency 记录消息延迟
func (m *Metrics) RecordLatency(latency time.Duration) {
	m.latencyHistogram.Observe(latency.Milliseconds())
}

// RecordConnection 记录新连接
func (m *Metrics) RecordConnection() {
	m.ConnectedClients.Add(1)
	m.TotalConnections.Add(1)
}

// RecordDisconnection 记录断开连接
func (m *Metrics) RecordDisconnection() {
	m.ConnectedClients.Add(-1)
	m.TotalDisconnects.Add(1)
}

// RecordHeartbeatSent 记录发送心跳
func (m *Metrics) RecordHeartbeatSent() {
	m.HeartbeatsSent.Add(1)
}

// RecordHeartbeatReceived 记录接收心跳
func (m *Metrics) RecordHeartbeatReceived() {
	m.HeartbeatsReceived.Add(1)
}

// RecordHeartbeatTimeout 记录心跳超时
func (m *Metrics) RecordHeartbeatTimeout() {
	m.HeartbeatTimeouts.Add(1)
}

// RecordBroadcast 记录广播消息
func (m *Metrics) RecordBroadcast(targetCount int64) {
	m.BroadcastsSent.Add(1)
	m.BroadcastTargets.Add(targetCount)
}

// RecordPromiseCreated 记录 Promise 创建
func (m *Metrics) RecordPromiseCreated() {
	m.PromiseCreated.Add(1)
}

// RecordPromiseCompleted 记录 Promise 完成
func (m *Metrics) RecordPromiseCompleted() {
	m.PromiseCompleted.Add(1)
}

// RecordPromiseTimeout 记录 Promise 超时
func (m *Metrics) RecordPromiseTimeout() {
	m.PromiseTimeouts.Add(1)
}

// RecordEncodingError 记录编码错误
func (m *Metrics) RecordEncodingError() {
	m.EncodingErrors.Add(1)
}

// RecordDecodingError 记录解码错误
func (m *Metrics) RecordDecodingError() {
	m.DecodingErrors.Add(1)
}

// RecordNetworkError 记录网络错误
func (m *Metrics) RecordNetworkError() {
	m.NetworkErrors.Add(1)
}

// GetSnapshot 获取当前指标快照 (T047 增强版)
func (m *Metrics) GetSnapshot() *protocol.MetricsSnapshot {
	now := time.Now()
	lastReset := m.lastResetTime.Load().(time.Time)
	duration := now.Sub(lastReset).Seconds()

	// 计算吞吐量（消息/秒）
	var throughput int64
	if duration > 0 {
		totalMessages := m.MessagesSent.Load() + m.MessagesReceived.Load()
		throughput = int64(float64(totalMessages) / duration)
	}

	return &protocol.MetricsSnapshot{
		// 连接指标
		ConnectedClients: m.ConnectedClients.Load(),
		TotalConnections: m.TotalConnections.Load(),
		TotalDisconnects: m.TotalDisconnects.Load(),

		// 消息指标
		MessagesSent:     m.MessagesSent.Load(),
		MessagesReceived: m.MessagesReceived.Load(),
		MessagesFailed:   m.MessagesFailed.Load(),
		BytesSent:        m.BytesSent.Load(),
		BytesReceived:    m.BytesReceived.Load(),
		MessageThroughput: throughput,

		// 延迟指标
		AverageLatency: m.latencyHistogram.Mean(),
		P50Latency:     m.latencyHistogram.Percentile(0.50),
		P95Latency:     m.latencyHistogram.Percentile(0.95),
		P99Latency:     m.latencyHistogram.Percentile(0.99),

		// 心跳指标
		HeartbeatsSent:     m.HeartbeatsSent.Load(),
		HeartbeatsReceived: m.HeartbeatsReceived.Load(),
		HeartbeatTimeouts:  m.HeartbeatTimeouts.Load(),

		// Promise 指标
		PromiseCreated:   m.PromiseCreated.Load(),
		PromiseCompleted: m.PromiseCompleted.Load(),
		PromiseTimeouts:  m.PromiseTimeouts.Load(),
		PromiseActive:    m.PromiseCreated.Load() - m.PromiseCompleted.Load() - m.PromiseTimeouts.Load(),

		// 广播指标
		BroadcastsSent:   m.BroadcastsSent.Load(),
		BroadcastTargets: m.BroadcastTargets.Load(),

		// 错误指标
		EncodingErrors: m.EncodingErrors.Load(),
		DecodingErrors: m.DecodingErrors.Load(),
		NetworkErrors:  m.NetworkErrors.Load(),

		// 系统指标
		UptimeSeconds: int64(m.GetUptime().Seconds()),
		Timestamp:     now.UnixMilli(),
	}
}

// Reset 重置可重置的指标（用于时间窗口计算）
func (m *Metrics) Reset() {
	m.lastResetTime.Store(time.Now())
	// 注意：不重置累计指标（如 TotalConnections），仅重置时间窗口
}

// GetUptime 获取系统运行时间
func (m *Metrics) GetUptime() time.Duration {
	return time.Since(m.startTime)
}

// String 返回指标的字符串表示（用于日志输出）
func (m *Metrics) String() string {
	snapshot := m.GetSnapshot()
	return formatMetricsSnapshot(snapshot)
}

// formatMetricsSnapshot 格式化指标快照为字符串
func formatMetricsSnapshot(s *protocol.MetricsSnapshot) string {
	return fmt.Sprintf(
		"Metrics[Clients=%d, Connections=%d, Throughput=%d msg/s, AvgLatency=%dms, P99Latency=%dms]",
		s.ConnectedClients,
		s.TotalConnections,
		s.MessageThroughput,
		s.AverageLatency,
		s.P99Latency,
	)
}
