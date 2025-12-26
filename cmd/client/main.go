package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/voilet/quic-flow/pkg/command"
	"github.com/voilet/quic-flow/pkg/dispatcher"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/protocol"
	"github.com/voilet/quic-flow/pkg/router"
	"github.com/voilet/quic-flow/pkg/transport/client"
)

func main() {
	// 命令行参数
	serverAddr := flag.String("server", "localhost:8474", "服务器地址")
	clientID := flag.String("id", "client-001", "客户端 ID")
	insecure := flag.Bool("insecure", true, "跳过 TLS 证书验证（仅开发环境）")
	flag.Parse()

	// 创建日志器
	logger := monitoring.NewLogger(monitoring.LogLevelInfo, "text")

	logger.Info("=== QUIC Backbone Client ===")
	logger.Info("Connecting to server", "server", *serverAddr, "client_id", *clientID)

	// 创建客户端配置
	config := client.NewDefaultClientConfig(*clientID)
	config.InsecureSkipVerify = *insecure
	config.Logger = logger

	// 设置事件钩子
	config.Hooks = &monitoring.EventHooks{
		OnConnect: func(clientID string) {
			logger.Info("✅ Connected to server", "client_id", clientID)
		},
		OnDisconnect: func(clientID string, reason error) {
			logger.Warn("❌ Disconnected from server", "client_id", clientID, "reason", reason)
		},
		OnReconnect: func(clientID string, attemptCount int) {
			logger.Info("🔄 Reconnected to server", "client_id", clientID, "attempts", attemptCount)
		},
	}

	// 创建客户端
	c, err := client.NewClient(config)
	if err != nil {
		logger.Error("Failed to create client", "error", err)
		os.Exit(1)
	}

	// ========================================
	// 设置命令路由器（zinx风格）
	// 路由注册在 router.go 中
	// ========================================
	cmdRouter := SetupClientRouter(logger)

	// ========================================
	// 创建 Dispatcher 并注册消息处理器
	// ========================================
	disp := setupDispatcher(logger, c, cmdRouter)

	// 设置 Dispatcher 到客户端（必须在连接之前设置）
	c.SetDispatcher(disp)
	logger.Info("✅ Dispatcher attached to client")

	// 连接到服务器
	if err := c.Connect(*serverAddr); err != nil {
		logger.Error("Failed to connect", "error", err)
		// 不退出，因为启用了自动重连
	}

	logger.Info("Client started (auto-reconnect enabled)")
	logger.Info("🎯 Ready to receive and execute commands")
	logger.Info("Press Ctrl+C to stop")

	// 定期打印状态
	go printStatus(c, cmdRouter)

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// 优雅关闭
	shutdown(logger, disp, c)
}

// setupDispatcher 设置消息分发器
func setupDispatcher(logger *monitoring.Logger, c *client.Client, cmdRouter *router.Router) *dispatcher.Dispatcher {
	dispatcherConfig := &dispatcher.DispatcherConfig{
		WorkerCount:    10,
		TaskQueueSize:  1000,
		HandlerTimeout: 30 * time.Second,
		Logger:         logger,
	}
	disp := dispatcher.NewDispatcher(dispatcherConfig)

	// 创建命令处理器（使用路由器作为执行器）
	commandHandler := command.NewCommandHandler(c, cmdRouter, logger)

	// 注册 MESSAGE_TYPE_COMMAND 处理器
	// Server 下发的命令会被路由到对应的处理函数
	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_COMMAND, dispatcher.MessageHandlerFunc(func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
		return commandHandler.HandleCommand(ctx, msg)
	}))

	// 可以注册其他消息类型的处理器
	// 例如：处理 Server 推送的事件
	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_EVENT, dispatcher.MessageHandlerFunc(func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
		logger.Info("Received event from server", "msg_id", msg.MsgId)
		// TODO: 处理事件逻辑
		return nil, nil
	}))

	// 启动 Dispatcher
	disp.Start()
	logger.Info("✅ Dispatcher started with command handler")

	return disp
}

// printStatus 定期打印状态
func printStatus(c *client.Client, cmdRouter interface{ ListCommands() []string }) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		state := c.GetState()
		metrics := c.GetMetrics()
		lastPong := c.GetTimeSinceLastPong()

		fmt.Printf("\n=== Client Status ===\n")
		fmt.Printf("State: %v\n", state)
		fmt.Printf("Connected: %v\n", c.IsConnected())
		fmt.Printf("Last Pong: %v ago\n", lastPong.Round(time.Second))
		fmt.Printf("Heartbeats Sent: %d\n", metrics.ConnectedClients)
		fmt.Printf("Registered Commands: %v\n", cmdRouter.ListCommands())
		fmt.Println()
	}
}

// shutdown 优雅关闭
func shutdown(logger *monitoring.Logger, disp *dispatcher.Dispatcher, c *client.Client) {
	logger.Info("Shutting down client...")

	// 停止 Dispatcher
	disp.Stop()
	logger.Info("Dispatcher stopped")

	// 断开连接
	logger.Info("Disconnecting from server...")
	if err := c.Disconnect(); err != nil {
		logger.Error("Error during disconnect", "error", err)
		os.Exit(1)
	}

	logger.Info("Client stopped gracefully")
}
