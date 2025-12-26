package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"

	"github.com/voilet/QuicFlow/pkg/callback"
	"github.com/voilet/QuicFlow/pkg/dispatcher"
	pkgerrors "github.com/voilet/QuicFlow/pkg/errors"
	"github.com/voilet/QuicFlow/pkg/monitoring"
	"github.com/voilet/QuicFlow/pkg/protocol"
	"github.com/voilet/QuicFlow/pkg/session"
	"github.com/voilet/QuicFlow/pkg/transport/codec"
)

// Server QUIC 服务器
type Server struct {
	config  *ServerConfig
	quicCfg *quic.Config
	tlsCfg  *tls.Config

	listener *quic.Listener

	// 会话管理
	sessions *session.SessionManager

	// Promise 管理（异步回调）
	promises *callback.PromiseManager

	// 消息分发器（路由）
	dispatcher *dispatcher.Dispatcher

	// 监控
	metrics *monitoring.Metrics
	hooks   *monitoring.EventHooks
	logger  *monitoring.Logger

	// 编解码
	codec codec.Codec

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 状态
	mu      sync.RWMutex
	running bool
}

// NewServer 创建新的服务器实例
func NewServer(config *ServerConfig) (*Server, error) {
	if config == nil {
		return nil, fmt.Errorf("%w: config is nil", pkgerrors.ErrInvalidConfig)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// 构建 TLS 配置
	tlsCfg, err := config.BuildTLSConfig()
	if err != nil {
		return nil, err
	}

	// 构建 QUIC 配置
	quicCfg := config.BuildQUICConfig()

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 创建会话管理器
	sessionMgr := session.NewSessionManager(session.SessionManagerConfig{
		HeartbeatCheckInterval: config.HeartbeatCheckInterval,
		HeartbeatTimeout:       config.HeartbeatTimeout,
		MaxTimeoutCount:        config.MaxTimeoutCount,
		Hooks:                  config.Hooks,
		Logger:                 config.Logger,
	})

	// 创建 Promise 管理器
	promiseMgr := callback.NewPromiseManager(&callback.PromiseManagerConfig{
		MaxCapacity:     config.MaxPromises,
		CleanupInterval: 10 * time.Second,
		DefaultTimeout:  30 * time.Second,
		Logger:          config.Logger,
		Metrics:         monitoring.NewMetrics(),
		Hooks:           config.Hooks, // 传递钩子 (T050)
	})

	s := &Server{
		config:   config,
		quicCfg:  quicCfg,
		tlsCfg:   tlsCfg,
		sessions: sessionMgr,
		promises: promiseMgr,
		metrics:  monitoring.NewMetrics(),
		hooks:    config.Hooks,
		logger:   config.Logger,
		codec:    codec.NewProtobufCodec(),
		ctx:      ctx,
		cancel:   cancel,
		running:  false,
	}

	return s, nil
}

// Start 启动服务器
func (s *Server) Start(listenAddr string) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("%w: server is already running", pkgerrors.ErrInvalidConfig)
	}
	s.running = true
	s.mu.Unlock()

	// 启动监听
	listener, err := quic.ListenAddr(listenAddr, s.tlsCfg, s.quicCfg)
	if err != nil {
		return fmt.Errorf("failed to start QUIC listener: %w", err)
	}

	s.listener = listener
	s.logger.Info("Server started", "listen_addr", listenAddr)

	// 启动会话管理器
	s.sessions.Start()

	// 启动 Promise 管理器
	s.promises.Start()

	// 启动连接接受循环
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping server...")

	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	// 取消上下文
	s.cancel()

	// 关闭监听器
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.logger.Error("Failed to close listener", "error", err)
		}
	}

	// 停止会话管理器
	s.sessions.Stop()

	// 停止 Promise 管理器
	s.promises.Stop()

	// 关闭所有会话
	s.sessions.CloseAll("server shutdown")

	// 等待所有 goroutine 完成
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("Server stopped gracefully")
		return nil
	case <-ctx.Done():
		s.logger.Warn("Server stop timeout, forcing shutdown")
		return ctx.Err()
	}
}

// acceptLoop 连接接受循环 (T023)
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	s.logger.Info("Accept loop started")

	for {
		conn, err := s.listener.Accept(s.ctx)
		if err != nil {
			select {
			case <-s.ctx.Done():
				// 服务器关闭
				s.logger.Debug("Accept loop stopped")
				return
			default:
				s.logger.Error("Failed to accept connection", "error", err)
				s.metrics.RecordNetworkError()
				continue
			}
		}

		// 为每个连接启动处理 goroutine
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection 处理单个连接 (T024)
func (s *Server) handleConnection(conn *quic.Conn) {
	defer s.wg.Done()

	remoteAddr := conn.RemoteAddr().String()
	s.logger.Debug("New connection accepted", "remote_addr", remoteAddr)

	// 等待第一个消息（包含客户端 ID）
	stream, err := conn.AcceptStream(s.ctx)
	if err != nil {
		s.logger.Error("Failed to accept first stream", "remote_addr", remoteAddr, "error", err)
		conn.CloseWithError(1, "failed to accept first stream")
		return
	}

	// 读取第一个帧（应该是 Ping 帧，包含客户端 ID）
	firstFrame, err := s.codec.ReadFrame(stream)
	if err != nil {
		s.logger.Error("Failed to read first frame", "remote_addr", remoteAddr, "error", err)
		stream.Close()
		conn.CloseWithError(1, "invalid first frame")
		return
	}

	// 解析客户端 ID
	var clientID string
	if firstFrame.Type == protocol.FrameType_FRAME_TYPE_PING {
		pingFrame, err := codec.DecodePingFrame(firstFrame)
		if err != nil {
			s.logger.Error("Failed to decode ping frame", "remote_addr", remoteAddr, "error", err)
			stream.Close()
			conn.CloseWithError(1, "invalid ping frame")
			return
		}
		clientID = pingFrame.ClientId
	} else {
		s.logger.Error("First frame must be PING", "remote_addr", remoteAddr, "frame_type", firstFrame.Type)
		stream.Close()
		conn.CloseWithError(1, "first frame must be PING")
		return
	}

	if clientID == "" {
		s.logger.Error("Client ID is empty", "remote_addr", remoteAddr)
		stream.Close()
		conn.CloseWithError(1, "client ID is required")
		return
	}

	// 创建会话
	sess := session.NewClientSession(clientID, conn)
	if err := s.sessions.Add(sess); err != nil {
		s.logger.Error("Failed to add session", "client_id", clientID, "error", err)
		stream.Close()
		conn.CloseWithError(1, "failed to create session")
		return
	}

	// 记录指标
	s.metrics.RecordConnection()

	// 触发连接事件
	if s.hooks != nil {
		s.hooks.SafeOnConnect(clientID)
	}

	s.logger.Info("Client connected", "client_id", clientID, "remote_addr", remoteAddr)

	// 响应 Pong
	pongFrame, err := codec.EncodePongFrame(&protocol.PongFrame{
		ServerTime: time.Now().UnixMilli(),
	}, time.Now().UnixMilli())
	if err == nil {
		s.codec.WriteFrame(stream, pongFrame)
	}
	stream.Close()

	// 处理后续流
	for {
		stream, err := conn.AcceptStream(s.ctx)
		if err != nil {
			select {
			case <-s.ctx.Done():
				// 服务器关闭
				return
			default:
				s.logger.Debug("Connection closed", "client_id", clientID, "error", err)
				s.handleDisconnect(clientID, err)
				return
			}
		}

		// 为每个流启动处理 goroutine
		s.wg.Add(1)
		go s.handleStream(clientID, conn, stream)
	}
}

// handleStream 处理单个流 (T025)
func (s *Server) handleStream(clientID string, conn *quic.Conn, stream *quic.Stream) {
	defer s.wg.Done()
	defer stream.Close()

	// 读取帧
	frame, err := s.codec.ReadFrame(stream)
	if err != nil {
		if err != io.EOF {
			s.logger.Error("Failed to read frame", "client_id", clientID, "error", err)
			s.metrics.RecordDecodingError()
		}
		return
	}

	// 根据帧类型处理
	switch frame.Type {
	case protocol.FrameType_FRAME_TYPE_PING:
		s.handlePing(clientID, stream, frame)

	case protocol.FrameType_FRAME_TYPE_DATA:
		s.handleData(clientID, stream, frame)

	case protocol.FrameType_FRAME_TYPE_ACK:
		s.handleAck(clientID, frame)

	default:
		s.logger.Warn("Unknown frame type", "client_id", clientID, "frame_type", frame.Type)
	}
}

// handlePing 处理心跳请求（详细实现在 heartbeat.go）
func (s *Server) handlePing(clientID string, stream *quic.Stream, frame *protocol.Frame) {
	// 更新会话心跳时间
	sess, err := s.sessions.Get(clientID)
	if err != nil {
		s.logger.Error("Session not found for ping", "client_id", clientID)
		return
	}

	sess.UpdateLastHeartbeat()
	s.metrics.RecordHeartbeatReceived()

	// 响应 Pong
	pongFrame, err := codec.EncodePongFrame(&protocol.PongFrame{
		ServerTime: time.Now().UnixMilli(),
	}, time.Now().UnixMilli())

	if err != nil {
		s.logger.Error("Failed to encode pong", "client_id", clientID, "error", err)
		return
	}

	if err := s.codec.WriteFrame(stream, pongFrame); err != nil {
		s.logger.Error("Failed to send pong", "client_id", clientID, "error", err)
	}

	s.logger.Debug("Pong sent", "client_id", clientID)
}

// handleData 处理数据消息
func (s *Server) handleData(clientID string, stream *quic.Stream, frame *protocol.Frame) {
	s.logger.Debug("Data message received", "client_id", clientID)
	s.metrics.RecordMessageReceived(int64(len(frame.Payload)))

	// 解析消息
	dataMsg, err := codec.DecodeDataMessage(frame)
	if err != nil {
		s.logger.Error("Failed to decode data message", "client_id", clientID, "error", err)
		s.metrics.RecordDecodingError()
		return
	}

	// 触发事件
	if s.hooks != nil {
		s.hooks.SafeOnMessageReceived(dataMsg.MsgId, clientID)
	}

	// 使用 Dispatcher 分发消息（如果已设置）
	if s.dispatcher != nil {
		// 创建响应通道
		responseCh := make(chan *dispatcher.DispatchResponse, 1)

		// 异步分发
		if err := s.dispatcher.Dispatch(s.ctx, dataMsg, responseCh); err != nil {
			s.logger.Error("Failed to dispatch message", "client_id", clientID, "msg_id", dataMsg.MsgId, "error", err)
			return
		}

		// 等待响应（如果需要）
		if dataMsg.WaitAck {
			select {
			case resp := <-responseCh:
				if resp.Response != nil {
					// 发送响应回客户端
					s.sendResponse(clientID, stream, resp.Response)
				}
			case <-s.ctx.Done():
				return
			}
		}
	} else {
		s.logger.Warn("No dispatcher set, message not processed", "client_id", clientID, "msg_id", dataMsg.MsgId)
	}
}

// handleAck 处理确认消息 (T043)
func (s *Server) handleAck(clientID string, frame *protocol.Frame) {
	s.logger.Debug("Ack message received", "client_id", clientID)

	// 解码 Ack 消息
	ackMsg, err := codec.DecodeAckMessage(frame)
	if err != nil {
		s.logger.Error("Failed to decode ack message", "client_id", clientID, "error", err)
		s.metrics.RecordDecodingError()
		return
	}

	// 完成对应的 Promise
	if err := s.promises.Complete(ackMsg.MsgId, ackMsg); err != nil {
		s.logger.Warn("Failed to complete promise", "msg_id", ackMsg.MsgId, "error", err)
	} else {
		s.logger.Debug("Promise completed", "msg_id", ackMsg.MsgId, "status", ackMsg.Status)
	}
}

// handleDisconnect 处理客户端断开
func (s *Server) handleDisconnect(clientID string, reason error) {
	s.logger.Info("Client disconnected", "client_id", clientID, "reason", reason)

	// 移除会话
	if err := s.sessions.Remove(clientID); err != nil {
		s.logger.Error("Failed to remove session", "client_id", clientID, "error", err)
	}

	// 记录指标
	s.metrics.RecordDisconnection()

	// 触发断开事件
	if s.hooks != nil {
		s.hooks.SafeOnDisconnect(clientID, reason)
	}
}

// ListClients 获取所有在线客户端 ID 列表 (T033)
func (s *Server) ListClients() []string {
	return s.sessions.ListClientIDs()
}

// GetClientInfo 获取客户端详细信息 (T034)
func (s *Server) GetClientInfo(clientID string) (*protocol.ClientInfo, error) {
	sess, err := s.sessions.Get(clientID)
	if err != nil {
		return nil, err
	}

	return sess.ToClientInfo(), nil
}

// GetMetrics 获取服务器指标快照
func (s *Server) GetMetrics() *protocol.MetricsSnapshot {
	return s.metrics.GetSnapshot()
}

// SetDispatcher 设置消息分发器（路由）
// 必须在 Start() 之前调用
func (s *Server) SetDispatcher(d *dispatcher.Dispatcher) {
	s.dispatcher = d
	s.logger.Info("Dispatcher set for server")
}

// GetDispatcher 获取消息分发器
func (s *Server) GetDispatcher() *dispatcher.Dispatcher {
	return s.dispatcher
}

// sendResponse 发送响应消息到客户端
func (s *Server) sendResponse(clientID string, stream *quic.Stream, msg *protocol.DataMessage) {
	if msg == nil {
		return
	}

	// 编码响应消息
	responseFrame, err := codec.EncodeDataMessage(msg)
	if err != nil {
		s.logger.Error("Failed to encode response", "client_id", clientID, "error", err)
		return
	}

	// 发送响应
	if err := s.codec.WriteFrame(stream, responseFrame); err != nil {
		s.logger.Error("Failed to send response", "client_id", clientID, "error", err)
		return
	}

	s.logger.Debug("Response sent", "client_id", clientID, "msg_id", msg.MsgId)
}

// SendTo 发送消息到指定客户端（单播）(T038)
// clientID: 目标客户端 ID
// msg: 要发送的消息
// 返回错误如果客户端不存在或发送失败
func (s *Server) SendTo(clientID string, msg *protocol.DataMessage) error {
	if msg == nil {
		return fmt.Errorf("%w: message is nil", pkgerrors.ErrInvalidConfig)
	}

	// 验证客户端是否存在
	sess, err := s.sessions.Get(clientID)
	if err != nil {
		return fmt.Errorf("%w: %v", pkgerrors.ErrClientNotConnected, err)
	}

	// 设置时间戳（如果没有）
	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().UnixMilli()
	}

	// 编码消息为 DATA 帧
	dataFrame, err := codec.EncodeDataMessage(msg)
	if err != nil {
		s.metrics.RecordEncodingError()
		return fmt.Errorf("failed to encode message: %w", err)
	}

	// 打开新流
	s.logger.Info("Opening stream to client", "client_id", clientID, "msg_id", msg.MsgId)
	stream, err := sess.Conn.OpenStreamSync(s.ctx)
	if err != nil {
		s.logger.Error("Failed to open stream", "client_id", clientID, "error", err)
		s.metrics.RecordNetworkError()
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	s.logger.Info("Stream opened, writing frame", "client_id", clientID, "msg_id", msg.MsgId)

	// 发送消息
	if err := s.codec.WriteFrame(stream, dataFrame); err != nil {
		s.logger.Error("Failed to write frame", "client_id", clientID, "error", err)
		s.metrics.RecordNetworkError()
		return fmt.Errorf("failed to send message: %w", err)
	}

	s.logger.Info("Frame written, closing stream", "client_id", clientID, "msg_id", msg.MsgId)

	// 关闭写端
	if err := stream.Close(); err != nil {
		s.logger.Error("Failed to close stream", "client_id", clientID, "error", err)
		return fmt.Errorf("failed to close stream: %w", err)
	}

	// 记录指标
	s.metrics.RecordMessageSent(int64(len(msg.Payload)))
	s.logger.Info("✅ Message sent successfully", "client_id", clientID, "msg_id", msg.MsgId)

	// 触发事件
	if s.hooks != nil {
		s.hooks.SafeOnMessageSent(msg.MsgId, clientID, nil)
	}

	return nil
}

// SendToWithPromise 发送消息到指定客户端并创建Promise等待响应
// 用于需要等待客户端执行结果的场景（如命令下发）
func (s *Server) SendToWithPromise(clientID string, msg *protocol.DataMessage, timeout time.Duration) (*callback.Promise, error) {
	if msg == nil {
		return nil, fmt.Errorf("%w: message is nil", pkgerrors.ErrInvalidConfig)
	}

	// 验证客户端是否存在
	sess, err := s.sessions.Get(clientID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", pkgerrors.ErrClientNotConnected, err)
	}

	// 设置时间戳（如果没有）
	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().UnixMilli()
	}

	// 编码消息为 DATA 帧
	dataFrame, err := codec.EncodeDataMessage(msg)
	if err != nil {
		s.metrics.RecordEncodingError()
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	// 创建Promise（在发送前创建，确保不会错过响应）
	promise, err := s.promises.Create(msg.MsgId, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create promise: %w", err)
	}

	// 打开新流
	s.logger.Info("Opening stream to client (with promise)", "client_id", clientID, "msg_id", msg.MsgId)
	stream, err := sess.Conn.OpenStreamSync(s.ctx)
	if err != nil {
		s.logger.Error("Failed to open stream", "client_id", clientID, "error", err)
		s.promises.Remove(msg.MsgId)
		s.metrics.RecordNetworkError()
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}

	s.logger.Info("Stream opened, writing frame", "client_id", clientID, "msg_id", msg.MsgId)

	// 发送消息（使用长度前缀协议，不需要关闭写端来标识消息边界）
	if err := s.codec.WriteFrame(stream, dataFrame); err != nil {
		s.logger.Error("Failed to write frame", "client_id", clientID, "error", err)
		stream.Close()
		s.promises.Remove(msg.MsgId)
		s.metrics.RecordNetworkError()
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	s.logger.Info("Frame written, waiting for ACK response", "client_id", clientID, "msg_id", msg.MsgId)
	s.metrics.RecordMessageSent(int64(len(msg.Payload)))

	// 启动goroutine在同一个流上读取ACK响应
	// 使用长度前缀协议，不需要关闭写端，客户端可以通过长度前缀知道消息边界
	go func() {
		defer stream.Close()

		s.logger.Info("开始读取ACK响应", "client_id", clientID, "msg_id", msg.MsgId)

		// 读取ACK响应（使用长度前缀协议）
		ackFrame, err := s.codec.ReadFrame(stream)
		if err != nil {
			s.logger.Error("Failed to read ACK response", "client_id", clientID, "msg_id", msg.MsgId, "error", err)
			s.promises.Fail(msg.MsgId, fmt.Errorf("failed to read ACK: %w", err))
			return
		}

		if ackFrame == nil {
			s.logger.Error("ACK frame is nil", "client_id", clientID, "msg_id", msg.MsgId)
			s.promises.Fail(msg.MsgId, fmt.Errorf("ACK frame is nil"))
			return
		}

		// 验证帧类型
		if ackFrame.Type != protocol.FrameType_FRAME_TYPE_ACK {
			s.logger.Error("Expected ACK frame, got different type", "client_id", clientID, "msg_id", msg.MsgId, "type", ackFrame.Type)
			s.promises.Fail(msg.MsgId, fmt.Errorf("expected ACK frame, got %v", ackFrame.Type))
			return
		}

		// 解码ACK消息
		ackMsg, err := codec.DecodeAckMessage(ackFrame)
		if err != nil {
			s.logger.Error("Failed to decode ACK message", "client_id", clientID, "msg_id", msg.MsgId, "error", err)
			s.promises.Fail(msg.MsgId, fmt.Errorf("failed to decode ACK: %w", err))
			return
		}

		s.logger.Info("✅ ACK received from client", "client_id", clientID, "msg_id", msg.MsgId, "status", ackMsg.Status, "has_result", len(ackMsg.Result) > 0)

		// 完成Promise
		if err := s.promises.Complete(msg.MsgId, ackMsg); err != nil {
			s.logger.Warn("Failed to complete promise", "msg_id", msg.MsgId, "error", err)
		}
	}()

	s.logger.Info("✅ Message sent, waiting for ACK in background", "client_id", clientID, "msg_id", msg.MsgId, "timeout", timeout)

	return promise, nil
}

// Broadcast 广播消息到所有客户端 (T039)
// msg: 要广播的消息
// 返回成功发送的客户端数量和错误列表
func (s *Server) Broadcast(msg *protocol.DataMessage) (int, []error) {
	if msg == nil {
		return 0, []error{fmt.Errorf("%w: message is nil", pkgerrors.ErrInvalidConfig)}
	}

	// 获取所有客户端 ID
	clientIDs := s.sessions.ListClientIDs()
	if len(clientIDs) == 0 {
		return 0, nil
	}

	s.logger.Debug("Broadcasting message", "msg_id", msg.MsgId, "recipients", len(clientIDs))

	// 并发发送到所有客户端
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex
	var errors []error

	for _, clientID := range clientIDs {
		wg.Add(1)
		go func(cid string) {
			defer wg.Done()

			// 创建消息副本（设置接收方 ID）
			msgCopy := *msg
			msgCopy.ReceiverId = cid

			if err := s.SendTo(cid, &msgCopy); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to send to %s: %w", cid, err))
				mu.Unlock()
				s.logger.Warn("Broadcast failed for client", "client_id", cid, "error", err)
			} else {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(clientID)
	}

	wg.Wait()

	s.logger.Info("Broadcast completed", "msg_id", msg.MsgId, "total", len(clientIDs), "success", successCount, "failed", len(errors))

	// 记录广播指标 (T051)
	s.metrics.RecordBroadcast(int64(len(clientIDs)))

	// 触发广播事件 (T050)
	if s.hooks != nil {
		s.hooks.SafeOnBroadcast(msg.MsgId, len(clientIDs), successCount)
	}

	return successCount, errors
}

// AsyncSend 异步发送消息并等待 Ack (T042)
// clientID: 目标客户端 ID
// msg: 要发送的消息
// timeout: 等待 Ack 的超时时间（0 表示使用默认值 30s）
// 返回 Promise 的响应通道
func (s *Server) AsyncSend(clientID string, msg *protocol.DataMessage, timeout time.Duration) (<-chan *callback.PromiseResponse, error) {
	if msg == nil {
		return nil, fmt.Errorf("%w: message is nil", pkgerrors.ErrInvalidConfig)
	}

	// 生成消息 ID（如果没有）
	if msg.MsgId == "" {
		msg.MsgId = uuid.New().String()
	}

	// 设置 WaitAck 标志
	msg.WaitAck = true

	// 创建 Promise
	promise, err := s.promises.Create(msg.MsgId, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create promise: %w", err)
	}

	// 发送消息
	if err := s.SendTo(clientID, msg); err != nil {
		s.promises.Remove(msg.MsgId)
		return nil, err
	}

	s.logger.Debug("Async send initiated", "client_id", clientID, "msg_id", msg.MsgId)

	return promise.RespChan, nil
}

// AsyncBroadcast 异步广播消息（不等待 Ack）
// msg: 要广播的消息
// 返回成功发送的客户端数量和错误列表
func (s *Server) AsyncBroadcast(msg *protocol.DataMessage) (int, []error) {
	// 广播不支持 WaitAck，因为无法追踪多个客户端的响应
	if msg != nil {
		msg.WaitAck = false
	}
	return s.Broadcast(msg)
}
