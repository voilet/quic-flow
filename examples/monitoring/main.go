package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/protocol"
	"github.com/voilet/quic-flow/pkg/transport/server"
)

// 监控示例程序 (T049)
// 展示如何使用 EventHooks、GetMetrics 和 Prometheus 导出

var (
	addr           = flag.String("addr", ":8474", "QUIC server address")
	certFile       = flag.String("cert", "certs/server.crt", "TLS certificate file")
	keyFile        = flag.String("key", "certs/server.key", "TLS key file")
	metricsAddr    = flag.String("metrics", ":9090", "Prometheus metrics address")
	metricsEnabled = flag.Bool("enable-metrics", true, "Enable Prometheus metrics endpoint")
)

func main() {
	flag.Parse()

	log.Println("=== QUIC Backbone Monitoring Example ===")

	// 创建自定义 Logger
	logger := monitoring.NewLogger(monitoring.LogLevelInfo, "text")

	// 创建 Event Hooks
	hooks := &monitoring.EventHooks{
		OnConnect: func(clientID string) {
			log.Printf("[HOOK] Client connected: %s", clientID)
		},
		OnDisconnect: func(clientID string, reason error) {
			log.Printf("[HOOK] Client disconnected: %s, reason: %v", clientID, reason)
		},
		OnHeartbeatTimeout: func(clientID string) {
			log.Printf("[HOOK] Heartbeat timeout for client: %s", clientID)
		},
		OnReconnect: func(clientID string, attemptCount int) {
			log.Printf("[HOOK] Client reconnected: %s (attempt %d)", clientID, attemptCount)
		},
		OnMessageSent: func(msgID, clientID string, err error) {
			if err != nil {
				log.Printf("[HOOK] Message send failed: %s to %s, error: %v", msgID, clientID, err)
			} else {
				log.Printf("[HOOK] Message sent: %s to %s", msgID, clientID)
			}
		},
		OnMessageReceived: func(msgID, clientID string) {
			log.Printf("[HOOK] Message received: %s from %s", msgID, clientID)
		},
	}

	// 创建服务器配置
	config := &server.ServerConfig{
		TLSCertFile: *certFile,
		TLSKeyFile:  *keyFile,

		MaxIdleTimeout:     30 * time.Second,
		MaxIncomingStreams: 1000,

		HeartbeatCheckInterval: 5 * time.Second,
		HeartbeatTimeout:       45 * time.Second,
		MaxTimeoutCount:        3,

		MaxClients:  10000,
		MaxPromises: 50000,

		Logger: logger,
		Hooks:  hooks,
	}

	// 创建服务器
	srv, err := server.NewServer(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// 启动 Prometheus 指标端点（如果启用）
	if *metricsEnabled {
		go startMetricsServer(srv, *metricsAddr)
	}

	// 启动服务器
	if err := srv.Start(*addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Printf("✅ Server started on %s", *addr)
	if *metricsEnabled {
		log.Printf("✅ Metrics endpoint available at http://localhost%s/metrics", *metricsAddr)
	}

	// 启动定期指标输出
	go printMetricsPeriodically(srv)

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}

// startMetricsServer 启动 Prometheus 指标服务器
func startMetricsServer(srv *server.Server, addr string) {
	// 获取服务器的 Metrics 实例
	metricsSnapshot := srv.GetMetrics()

	// 注意：这里我们需要从 Server 获取 Metrics 实例
	// 由于 Server 没有直接暴露 Metrics，我们使用一个 wrapper
	handler := &metricsHandler{server: srv}

	http.Handle("/metrics", handler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "QUIC Backbone Monitoring\n\n")
		fmt.Fprintf(w, "Metrics endpoint: /metrics\n")
		fmt.Fprintf(w, "Connected clients: %d\n", metricsSnapshot.ConnectedClients)
	})

	log.Printf("Starting metrics server on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Printf("Metrics server error: %v", err)
	}
}

// metricsHandler wrapper for PrometheusHandler
type metricsHandler struct {
	server *server.Server
}

func (h *metricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	snapshot := h.server.GetMetrics()

	// 手动生成 Prometheus 格式
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	output := formatPrometheusMetrics(snapshot)
	w.Write([]byte(output))
}

// formatPrometheusMetrics 格式化指标为 Prometheus 文本格式
func formatPrometheusMetrics(s *protocol.MetricsSnapshot) string {
	return fmt.Sprintf(`# HELP quic_backbone_connected_clients Current number of connected clients
# TYPE quic_backbone_connected_clients gauge
quic_backbone_connected_clients %d

# HELP quic_backbone_total_connections Total number of connections
# TYPE quic_backbone_total_connections counter
quic_backbone_total_connections %d

# HELP quic_backbone_messages_sent_total Total number of messages sent
# TYPE quic_backbone_messages_sent_total counter
quic_backbone_messages_sent_total %d

# HELP quic_backbone_messages_received_total Total number of messages received
# TYPE quic_backbone_messages_received_total counter
quic_backbone_messages_received_total %d

# HELP quic_backbone_message_throughput Message throughput (messages/second)
# TYPE quic_backbone_message_throughput gauge
quic_backbone_message_throughput %d

# HELP quic_backbone_latency_average_milliseconds Average latency in milliseconds
# TYPE quic_backbone_latency_average_milliseconds gauge
quic_backbone_latency_average_milliseconds %d

# HELP quic_backbone_latency_p99_milliseconds P99 latency in milliseconds
# TYPE quic_backbone_latency_p99_milliseconds gauge
quic_backbone_latency_p99_milliseconds %d

# HELP quic_backbone_heartbeat_timeouts_total Total heartbeat timeouts
# TYPE quic_backbone_heartbeat_timeouts_total counter
quic_backbone_heartbeat_timeouts_total %d

# HELP quic_backbone_promise_active Currently active promises
# TYPE quic_backbone_promise_active gauge
quic_backbone_promise_active %d

# HELP quic_backbone_encoding_errors_total Total encoding errors
# TYPE quic_backbone_encoding_errors_total counter
quic_backbone_encoding_errors_total %d

# HELP quic_backbone_uptime_seconds System uptime in seconds
# TYPE quic_backbone_uptime_seconds gauge
quic_backbone_uptime_seconds %d
`,
		s.ConnectedClients,
		s.TotalConnections,
		s.MessagesSent,
		s.MessagesReceived,
		s.MessageThroughput,
		s.AverageLatency,
		s.P99Latency,
		s.HeartbeatTimeouts,
		s.PromiseActive,
		s.EncodingErrors,
		s.UptimeSeconds,
	)
}

// printMetricsPeriodically 定期打印指标
func printMetricsPeriodically(srv *server.Server) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		snapshot := srv.GetMetrics()

		log.Println("\n=== Metrics Snapshot ===")
		log.Printf("Connected Clients: %d", snapshot.ConnectedClients)
		log.Printf("Total Connections: %d", snapshot.TotalConnections)
		log.Printf("Messages Sent: %d", snapshot.MessagesSent)
		log.Printf("Messages Received: %d", snapshot.MessagesReceived)
		log.Printf("Throughput: %d msg/s", snapshot.MessageThroughput)
		log.Printf("Avg Latency: %d ms", snapshot.AverageLatency)
		log.Printf("P50 Latency: %d ms", snapshot.P50Latency)
		log.Printf("P95 Latency: %d ms", snapshot.P95Latency)
		log.Printf("P99 Latency: %d ms", snapshot.P99Latency)
		log.Printf("Heartbeat Timeouts: %d", snapshot.HeartbeatTimeouts)
		log.Printf("Active Promises: %d", snapshot.PromiseActive)
		log.Printf("Encoding Errors: %d", snapshot.EncodingErrors)
		log.Printf("Decoding Errors: %d", snapshot.DecodingErrors)
		log.Printf("Network Errors: %d", snapshot.NetworkErrors)
		log.Printf("Uptime: %d seconds", snapshot.UptimeSeconds)
		log.Println("========================")
	}
}
