package server

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/quic-go/quic-go"

	pkgerrors "github.com/voilet/quic-flow/pkg/errors"
	"github.com/voilet/quic-flow/pkg/monitoring"
	tlsutil "github.com/voilet/quic-flow/pkg/transport/tls"
)

// ServerConfig 服务器配置
type ServerConfig struct {
	// TLS 配置
	TLSCertFile string // TLS 证书文件路径
	TLSKeyFile  string // TLS 私钥文件路径
	TLSConfig   *tls.Config // 自定义 TLS 配置（可选，优先级高于文件路径）

	// 网络配置
	ListenAddr string // 监听地址（例如 ":8474"）

	// QUIC 配置
	MaxIdleTimeout                 time.Duration // 空闲超时（默认 60 秒）
	MaxIncomingStreams             int64         // 每连接最大并发流数（默认 1000）
	MaxIncomingUniStreams          int64         // 单向流数量（默认 100）
	InitialStreamReceiveWindow     uint64        // 初始流接收窗口（默认 512KB）
	MaxStreamReceiveWindow         uint64        // 最大流接收窗口（默认 6MB）
	InitialConnectionReceiveWindow uint64        // 初始连接接收窗口（默认 1MB）
	MaxConnectionReceiveWindow     uint64        // 最大连接接收窗口（默认 15MB）

	// 会话管理配置
	MaxClients            int64         // 最大客户端数（默认 10000）
	HeartbeatInterval     time.Duration // 心跳间隔（默认 15 秒）
	HeartbeatTimeout      time.Duration // 心跳超时（默认 45 秒）
	HeartbeatCheckInterval time.Duration // 心跳检查间隔（默认 5 秒）
	MaxTimeoutCount       int32         // 最大超时次数（默认 3 次）

	// Promise 管理配置
	MaxPromises           int64         // 最大 Promise 数量（默认 50000）
	PromiseWarnThreshold  int64         // Promise 警告阈值（默认 40000）
	DefaultMessageTimeout time.Duration // 默认消息超时（默认 30 秒）

	// 监控配置
	Hooks  *monitoring.EventHooks // 事件钩子（可选）
	Logger *monitoring.Logger     // 日志实例（可选）
}

// NewDefaultServerConfig 创建默认服务器配置
func NewDefaultServerConfig(certFile, keyFile, listenAddr string) *ServerConfig {
	return &ServerConfig{
		TLSCertFile: certFile,
		TLSKeyFile:  keyFile,
		ListenAddr:  listenAddr,

		// QUIC 默认值
		MaxIdleTimeout:                 60 * time.Second,
		MaxIncomingStreams:             1000,
		MaxIncomingUniStreams:          100,
		InitialStreamReceiveWindow:     512 * 1024,      // 512KB
		MaxStreamReceiveWindow:         6 * 1024 * 1024, // 6MB
		InitialConnectionReceiveWindow: 1024 * 1024,     // 1MB
		MaxConnectionReceiveWindow:     15 * 1024 * 1024, // 15MB

		// 会话管理默认值
		MaxClients:             10000,
		HeartbeatInterval:      15 * time.Second,
		HeartbeatTimeout:       45 * time.Second,
		HeartbeatCheckInterval: 5 * time.Second,
		MaxTimeoutCount:        3,

		// Promise 默认值
		MaxPromises:           50000,
		PromiseWarnThreshold:  40000,
		DefaultMessageTimeout: 30 * time.Second,

		// 默认日志
		Logger: monitoring.NewDefaultLogger(),
	}
}

// Validate 验证配置的有效性
func (c *ServerConfig) Validate() error {
	// 验证 TLS 配置
	if c.TLSConfig == nil {
		if c.TLSCertFile == "" || c.TLSKeyFile == "" {
			return fmt.Errorf("%w: TLS certificate and key files are required", pkgerrors.ErrMissingTLSConfig)
		}
	}

	// 验证监听地址
	if c.ListenAddr == "" {
		return fmt.Errorf("%w: listen address is required", pkgerrors.ErrInvalidAddress)
	}

	// 验证超时配置
	if c.MaxIdleTimeout <= 0 {
		return fmt.Errorf("%w: MaxIdleTimeout must be positive", pkgerrors.ErrInvalidConfig)
	}

	if c.HeartbeatInterval <= 0 {
		return fmt.Errorf("%w: HeartbeatInterval must be positive", pkgerrors.ErrInvalidConfig)
	}

	if c.HeartbeatTimeout < c.HeartbeatInterval {
		return fmt.Errorf("%w: HeartbeatTimeout must be >= HeartbeatInterval", pkgerrors.ErrInvalidConfig)
	}

	// 验证容量配置
	if c.MaxClients <= 0 {
		return fmt.Errorf("%w: MaxClients must be positive", pkgerrors.ErrInvalidConfig)
	}

	if c.MaxPromises <= 0 {
		return fmt.Errorf("%w: MaxPromises must be positive", pkgerrors.ErrInvalidConfig)
	}

	return nil
}

// BuildTLSConfig 构建 TLS 配置
func (c *ServerConfig) BuildTLSConfig() (*tls.Config, error) {
	if c.TLSConfig != nil {
		// 验证自定义 TLS 配置
		if err := tlsutil.ValidateTLSConfig(c.TLSConfig); err != nil {
			return nil, err
		}
		return c.TLSConfig, nil
	}

	// 从文件加载 TLS 配置
	return tlsutil.LoadServerTLSConfig(c.TLSCertFile, c.TLSKeyFile)
}

// BuildQUICConfig 构建 QUIC 配置
func (c *ServerConfig) BuildQUICConfig() *quic.Config {
	return &quic.Config{
		MaxIdleTimeout:                 c.MaxIdleTimeout,
		MaxIncomingStreams:             c.MaxIncomingStreams,
		MaxIncomingUniStreams:          c.MaxIncomingUniStreams,
		KeepAlivePeriod:                0, // 禁用 QUIC 层 keep-alive（使用应用层心跳）
		InitialStreamReceiveWindow:     c.InitialStreamReceiveWindow,
		MaxStreamReceiveWindow:         c.MaxStreamReceiveWindow,
		InitialConnectionReceiveWindow: c.InitialConnectionReceiveWindow,
		MaxConnectionReceiveWindow:     c.MaxConnectionReceiveWindow,
		Allow0RTT:                      false, // 禁用 0-RTT（安全考虑）
	}
}
