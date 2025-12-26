package monitoring

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/voilet/quic-flow/pkg/protocol"
)

// PrometheusHandler HTTP handler for Prometheus metrics export (T048)
// 生成 Prometheus 文本格式的指标输出
type PrometheusHandler struct {
	metrics *Metrics
	prefix  string // 指标名称前缀（默认 "quic_backbone_"）
}

// NewPrometheusHandler 创建新的 Prometheus Handler
func NewPrometheusHandler(metrics *Metrics, prefix string) *PrometheusHandler {
	if prefix == "" {
		prefix = "quic_backbone_"
	}
	return &PrometheusHandler{
		metrics: metrics,
		prefix:  prefix,
	}
}

// ServeHTTP 实现 http.Handler 接口
func (h *PrometheusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	snapshot := h.metrics.GetSnapshot()

	// 设置 Content-Type
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// 生成 Prometheus 格式的指标
	metrics := h.formatMetrics(snapshot)
	w.Write([]byte(metrics))
}

// formatMetrics 格式化指标为 Prometheus 文本格式
func (h *PrometheusHandler) formatMetrics(snapshot *protocol.MetricsSnapshot) string {
	var sb strings.Builder

	// 连接指标
	h.writeGauge(&sb, "connected_clients", "Current number of connected clients", snapshot.ConnectedClients)
	h.writeCounter(&sb, "total_connections", "Total number of connections", snapshot.TotalConnections)
	h.writeCounter(&sb, "total_disconnects", "Total number of disconnects", snapshot.TotalDisconnects)

	// 消息指标
	h.writeCounter(&sb, "messages_sent_total", "Total number of messages sent", snapshot.MessagesSent)
	h.writeCounter(&sb, "messages_received_total", "Total number of messages received", snapshot.MessagesReceived)
	h.writeCounter(&sb, "messages_failed_total", "Total number of failed messages", snapshot.MessagesFailed)
	h.writeCounter(&sb, "bytes_sent_total", "Total bytes sent", snapshot.BytesSent)
	h.writeCounter(&sb, "bytes_received_total", "Total bytes received", snapshot.BytesReceived)
	h.writeGauge(&sb, "message_throughput", "Message throughput (messages/second)", snapshot.MessageThroughput)

	// 延迟指标
	h.writeGauge(&sb, "latency_average_milliseconds", "Average latency in milliseconds", snapshot.AverageLatency)
	h.writeGauge(&sb, "latency_p50_milliseconds", "P50 latency in milliseconds", snapshot.P50Latency)
	h.writeGauge(&sb, "latency_p95_milliseconds", "P95 latency in milliseconds", snapshot.P95Latency)
	h.writeGauge(&sb, "latency_p99_milliseconds", "P99 latency in milliseconds", snapshot.P99Latency)

	// 心跳指标
	h.writeCounter(&sb, "heartbeats_sent_total", "Total heartbeats sent", snapshot.HeartbeatsSent)
	h.writeCounter(&sb, "heartbeats_received_total", "Total heartbeats received", snapshot.HeartbeatsReceived)
	h.writeCounter(&sb, "heartbeat_timeouts_total", "Total heartbeat timeouts", snapshot.HeartbeatTimeouts)

	// Promise 指标
	h.writeCounter(&sb, "promise_created_total", "Total promises created", snapshot.PromiseCreated)
	h.writeCounter(&sb, "promise_completed_total", "Total promises completed", snapshot.PromiseCompleted)
	h.writeCounter(&sb, "promise_timeouts_total", "Total promise timeouts", snapshot.PromiseTimeouts)
	h.writeGauge(&sb, "promise_active", "Currently active promises", snapshot.PromiseActive)

	// 广播指标
	h.writeCounter(&sb, "broadcasts_sent_total", "Total broadcast messages sent", snapshot.BroadcastsSent)
	h.writeCounter(&sb, "broadcast_targets_total", "Total broadcast target clients", snapshot.BroadcastTargets)

	// 错误指标
	h.writeCounter(&sb, "encoding_errors_total", "Total encoding errors", snapshot.EncodingErrors)
	h.writeCounter(&sb, "decoding_errors_total", "Total decoding errors", snapshot.DecodingErrors)
	h.writeCounter(&sb, "network_errors_total", "Total network errors", snapshot.NetworkErrors)

	// 系统指标
	h.writeGauge(&sb, "uptime_seconds", "System uptime in seconds", snapshot.UptimeSeconds)

	return sb.String()
}

// writeGauge 写入 Gauge 类型指标
func (h *PrometheusHandler) writeGauge(sb *strings.Builder, name, help string, value int64) {
	fullName := h.prefix + name
	sb.WriteString(fmt.Sprintf("# HELP %s %s\n", fullName, help))
	sb.WriteString(fmt.Sprintf("# TYPE %s gauge\n", fullName))
	sb.WriteString(fmt.Sprintf("%s %d\n", fullName, value))
}

// writeCounter 写入 Counter 类型指标
func (h *PrometheusHandler) writeCounter(sb *strings.Builder, name, help string, value int64) {
	fullName := h.prefix + name
	sb.WriteString(fmt.Sprintf("# HELP %s %s\n", fullName, help))
	sb.WriteString(fmt.Sprintf("# TYPE %s counter\n", fullName))
	sb.WriteString(fmt.Sprintf("%s %d\n", fullName, value))
}

// writeHistogram 写入 Histogram 类型指标（保留，未来可能使用）
func (h *PrometheusHandler) writeHistogram(sb *strings.Builder, name, help string, buckets map[string]int64, sum, count int64) {
	fullName := h.prefix + name
	sb.WriteString(fmt.Sprintf("# HELP %s %s\n", fullName, help))
	sb.WriteString(fmt.Sprintf("# TYPE %s histogram\n", fullName))

	// 写入桶
	for le, count := range buckets {
		sb.WriteString(fmt.Sprintf("%s_bucket{le=\"%s\"} %d\n", fullName, le, count))
	}

	// 写入 +Inf 桶
	sb.WriteString(fmt.Sprintf("%s_bucket{le=\"+Inf\"} %d\n", fullName, count))

	// 写入 sum 和 count
	sb.WriteString(fmt.Sprintf("%s_sum %d\n", fullName, sum))
	sb.WriteString(fmt.Sprintf("%s_count %d\n", fullName, count))
}
