package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"

	"github.com/voilet/quic-flow/pkg/dispatcher"
	pkgerrors "github.com/voilet/quic-flow/pkg/errors"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/protocol"
	"github.com/voilet/quic-flow/pkg/transport/codec"
)

// Client QUIC 客户端
type Client struct {
	config  *ClientConfig
	quicCfg *quic.Config
	tlsCfg  *tls.Config

	// 连接
	conn       *quic.Conn
	serverAddr string

	// 状态（原子操作）
	state atomic.Value // protocol.ClientState

	// 监控
	metrics *monitoring.Metrics
	hooks   *monitoring.EventHooks
	logger  *monitoring.Logger

	// 编解码
	codec codec.Codec

	// 消息分发
	dispatcher *dispatcher.Dispatcher
	dispMu     sync.RWMutex

	// 控制
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	disconnectCh chan struct{} // 连接断开信号

	// 重连
	reconnectAttempts atomic.Int32 // 重连尝试次数

	// 心跳
	lastPongTime atomic.Value // time.Time - 最后收到 Pong 的时间
}

// NewClient 创建新的客户端实例 (T027)
func NewClient(config *ClientConfig) (*Client, error) {
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

	c := &Client{
		config:       config,
		quicCfg:      quicCfg,
		tlsCfg:       tlsCfg,
		metrics:      monitoring.NewMetrics(),
		hooks:        config.Hooks,
		logger:       config.Logger,
		codec:        codec.NewProtobufCodec(),
		ctx:          ctx,
		cancel:       cancel,
		disconnectCh: make(chan struct{}, 1), // 缓冲区大小为1，确保重连信号不会丢失
	}

	// 初始化状态
	c.state.Store(protocol.ClientState_CLIENT_STATE_IDLE)
	c.lastPongTime.Store(time.Now())

	return c, nil
}

// Connect 连接到服务器
func (c *Client) Connect(serverAddr string) error {
	c.serverAddr = serverAddr

	// 如果启用了自动重连，启动重连循环（即使首次连接成功也需要启动，以便处理后续断开）
	if c.config.ReconnectEnabled {
		c.wg.Add(1)
		go c.reconnectLoop()
	}

	// 首次连接
	if err := c.dial(); err != nil {
		// 如果启用了自动重连，不返回错误（重连循环会处理）
		if c.config.ReconnectEnabled {
			c.logger.Warn("Initial connection failed, will retry via reconnect loop", "error", err)
			// 触发首次重连尝试
			c.notifyDisconnect()
			return nil
		}
		return err
	}

	// 启动后台任务
	c.startBackgroundTasks()

	return nil
}

// dial 执行单次连接尝试
func (c *Client) dial() error {
	c.setState(protocol.ClientState_CLIENT_STATE_CONNECTING)

	conn, err := quic.DialAddr(c.ctx, c.serverAddr, c.tlsCfg, c.quicCfg)
	if err != nil {
		c.setState(protocol.ClientState_CLIENT_STATE_IDLE)
		return fmt.Errorf("failed to dial server: %w", err)
	}

	c.conn = conn
	c.setState(protocol.ClientState_CLIENT_STATE_CONNECTED)

	// 发送第一个 Ping（包含客户端 ID）
	if err := c.sendInitialPing(); err != nil {
		conn.CloseWithError(1, "failed to send initial ping")
		c.setState(protocol.ClientState_CLIENT_STATE_IDLE)
		return fmt.Errorf("failed to send initial ping: %w", err)
	}

	c.logger.Info("Connected to server", "server_addr", c.serverAddr, "client_id", c.config.ClientID)

	// 触发连接事件
	if c.hooks != nil {
		attemptCount := c.reconnectAttempts.Load()
		if attemptCount == 0 {
			c.hooks.SafeOnConnect(c.config.ClientID)
		} else {
			c.hooks.SafeOnReconnect(c.config.ClientID, int(attemptCount))
		}
	}

	// 重置重连计数
	c.reconnectAttempts.Store(0)

	return nil
}

// sendInitialPing 发送初始 Ping 帧（包含客户端 ID）
func (c *Client) sendInitialPing() error {
	stream, err := c.conn.OpenStreamSync(c.ctx)
	if err != nil {
		return err
	}

	pingFrame, err := codec.EncodePingFrame(&protocol.PingFrame{
		ClientId: c.config.ClientID,
	}, time.Now().UnixMilli())
	if err != nil {
		stream.Close()
		return err
	}

	if err := c.codec.WriteFrame(stream, pingFrame); err != nil {
		stream.Close()
		return err
	}

	// 关闭写端，通知服务器数据发送完毕（但保持读端打开）
	if err := stream.Close(); err != nil {
		return err
	}

	// 等待 Pong 响应
	pongFrame, err := c.codec.ReadFrame(stream)
	if err != nil {
		return err
	}

	if pongFrame.Type != protocol.FrameType_FRAME_TYPE_PONG {
		return fmt.Errorf("expected PONG, got %v", pongFrame.Type)
	}

	c.lastPongTime.Store(time.Now())
	return nil
}

// reconnectLoop 自动重连循环 (T028)
func (c *Client) reconnectLoop() {
	defer c.wg.Done()

	backoff := c.config.InitialBackoff

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("Reconnect loop stopped")
			return
		case <-c.disconnectCh:
			// 连接断开，尝试重连
			c.logger.Info("Connection lost, attempting to reconnect...")

			// 指数退避
			c.logger.Debug("Waiting before reconnect", "backoff", backoff)
			time.Sleep(backoff)

			// 增加重连尝试次数
			c.reconnectAttempts.Add(1)

			// 尝试重连
			if err := c.dial(); err != nil {
				c.logger.Error("Reconnect failed", "attempt", c.reconnectAttempts.Load(), "error", err)

				// 增加退避时间（指数退避）
				backoff = time.Duration(math.Min(
					float64(backoff*2),
					float64(c.config.MaxBackoff),
				))

				// 重连失败，继续尝试
				c.notifyDisconnect()
			} else {
				// 重连成功
				c.logger.Info("Reconnected successfully", "attempt", c.reconnectAttempts.Load())

				// 重置退避时间
				backoff = c.config.InitialBackoff

				// 重启后台任务
				c.startBackgroundTasks()
			}
		}
	}
}

// startBackgroundTasks 启动后台任务
func (c *Client) startBackgroundTasks() {
	// 启动心跳循环
	c.wg.Add(1)
	go c.heartbeatLoop()

	// 启动接收循环 (T045)
	c.wg.Add(1)
	go c.receiveLoop()
}

// setState 设置连接状态 (T029)
func (c *Client) setState(state protocol.ClientState) {
	c.state.Store(state)
	c.logger.Debug("State changed", "state", state)
}

// GetState 获取当前连接状态 (T029)
func (c *Client) GetState() protocol.ClientState {
	return c.state.Load().(protocol.ClientState)
}

// IsConnected 检查是否已连接 (T029)
func (c *Client) IsConnected() bool {
	return c.GetState() == protocol.ClientState_CLIENT_STATE_CONNECTED
}

// Disconnect 断开连接
func (c *Client) Disconnect() error {
	c.logger.Info("Disconnecting...")

	// 取消上下文
	c.cancel()

	// 关闭连接
	if c.conn != nil {
		if err := c.conn.CloseWithError(0, "client disconnect"); err != nil {
			c.logger.Error("Failed to close connection", "error", err)
		}
	}

	// 等待所有 goroutine 完成
	c.wg.Wait()

	c.setState(protocol.ClientState_CLIENT_STATE_IDLE)
	c.logger.Info("Disconnected")

	return nil
}

// GetClientID 获取客户端 ID
func (c *Client) GetClientID() string {
	return c.config.ClientID
}

// SetDispatcher 设置消息分发器
// 设置后，客户端接收到的消息将自动分发给 Dispatcher 处理
func (c *Client) SetDispatcher(d *dispatcher.Dispatcher) {
	c.dispMu.Lock()
	defer c.dispMu.Unlock()
	c.dispatcher = d
}

// GetDispatcher 获取消息分发器
func (c *Client) GetDispatcher() *dispatcher.Dispatcher {
	c.dispMu.RLock()
	defer c.dispMu.RUnlock()
	return c.dispatcher
}

// GetMetrics 获取客户端指标快照
func (c *Client) GetMetrics() *protocol.MetricsSnapshot {
	return c.metrics.GetSnapshot()
}

// notifyDisconnect 通知连接断开
func (c *Client) notifyDisconnect() {
	select {
	case c.disconnectCh <- struct{}{}:
	default:
		// Channel 已满，忽略
	}
}
