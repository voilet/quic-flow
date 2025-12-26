package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/voilet/quic-flow/pkg/command"
	"github.com/voilet/quic-flow/pkg/dispatcher"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/protocol"
	"github.com/voilet/quic-flow/pkg/transport/client"
)

func main() {
	// 命令行参数
	serverAddr := flag.String("server", "localhost:8474", "Server address")
	clientID := flag.String("id", "client-001", "Client ID")
	insecure := flag.Bool("insecure", true, "Skip TLS verification")
	flag.Parse()

	// 创建 logger
	logger := monitoring.NewLogger(monitoring.LogLevelInfo, "text")
	logger.Info("Starting command example client", "client_id", *clientID, "server", *serverAddr)

	// 创建客户端配置
	config := &client.ClientConfig{
		ClientID:           *clientID,
		InsecureSkipVerify: *insecure,
		ReconnectEnabled:   true,
		InitialBackoff:     1 * time.Second,
		MaxBackoff:         60 * time.Second,
		HeartbeatInterval:  15 * time.Second,
		HeartbeatTimeout:   45 * time.Second,
		Logger:             logger,
		Hooks:              nil,
	}

	// 创建客户端
	c, err := client.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// 创建命令执行器
	executor := &SimpleCommandExecutor{}

	// 创建命令处理器
	commandHandler := command.NewCommandHandler(c, executor, logger)

	// 创建 dispatcher 并注册命令处理器
	dispatcherConfig := &dispatcher.DispatcherConfig{
		WorkerCount:    10,
		TaskQueueSize:  1000,
		HandlerTimeout: 30 * time.Second,
		Logger:         logger,
	}
	disp := dispatcher.NewDispatcher(dispatcherConfig)

	// 注册命令处理器（包装为 MessageHandler）
	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_COMMAND, dispatcher.MessageHandlerFunc(func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
		return commandHandler.HandleCommand(ctx, msg)
	}))

	// 启动 dispatcher
	disp.Start()

	// 连接到服务器
	if err := c.Connect(*serverAddr); err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}

	// 设置 Dispatcher，这样客户端接收到的消息会自动分发处理
	c.SetDispatcher(disp)

	logger.Info("Client connected and ready to receive commands")

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down client...")

	// 停止 dispatcher
	disp.Stop()

	// 关闭客户端
	if err := c.Disconnect(); err != nil {
		logger.Error("Failed to disconnect client", "error", err)
	}

	logger.Info("Client stopped")
}
