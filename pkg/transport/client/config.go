package client

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/quic-go/quic-go"

	pkgerrors "github.com/voilet/quic-flow/pkg/errors"
	"github.com/voilet/quic-flow/pkg/monitoring"
	tlsutil "github.com/voilet/quic-flow/pkg/transport/tls"
)

// ClientConfig 客户端配置
type ClientConfig struct {
	// 客户端标识
	ClientID string // 客户端唯一标识（必须）

	// TLS 配置
	TLSCertFile        string      // 客户端证书文件路径（双向 TLS，可选）
	TLSKeyFile         string      // 客户端私钥文件路径（双向 TLS，可选）
	CACertFile         string      // CA 证书文件路径（用于验证服务器，可选）
	InsecureSkipVerify bool        // 跳过服务器证书验证（仅开发环境）
	TLSConfig          *tls.Config // 自定义 TLS 配置（可选，优先级高于文件路径）

	// QUIC 配置
	MaxIdleTimeout                 time.Duration // 空闲超时（默认 60 秒）
	MaxIncomingStreams             int64         // 每连接最大并发流数（默认 1000）
	MaxIncomingUniStreams          int64         // 单向流数量（默认 100）
	InitialStreamReceiveWindow     uint64        // 初始流接收窗口（默认 512KB）
	MaxStreamReceiveWindow         uint64        // 最大流接收窗口（默认 6MB）
	InitialConnectionReceiveWindow uint64        // 初始连接接收窗口（默认 1MB）
	MaxConnectionReceiveWindow     uint64        // 最大连接接收窗口（默认 15MB）

	// 重连配置
	ReconnectEnabled bool          // 是否启用自动重连（默认 true）
	InitialBackoff   time.Duration // 首次重试延迟（默认 1 秒）
	MaxBackoff       time.Duration // 最大重试延迟（默认 60 秒）

	// 心跳配置
	HeartbeatInterval time.Duration // 心跳间隔（默认 15 秒）
	HeartbeatTimeout  time.Duration // 心跳超时（默认 5 秒）

	// 消息配置
	DefaultMessageTimeout time.Duration // 默认消息超时（默认 30 秒）

	// 监控配置
	Hooks  *monitoring.EventHooks // 事件钩子（可选）
	Logger *monitoring.Logger     // 日志实例（可选）
}

// NewDefaultClientConfig 创建默认客户端配置
func NewDefaultClientConfig(clientID string) *ClientConfig {
	return &ClientConfig{
		ClientID:           clientID,
		InsecureSkipVerify: false, // 生产环境必须为 false

		// QUIC 默认值
		MaxIdleTimeout:                 60 * time.Second,
		MaxIncomingStreams:             1000,
		MaxIncomingUniStreams:          100,
		InitialStreamReceiveWindow:     512 * 1024,      // 512KB
		MaxStreamReceiveWindow:         6 * 1024 * 1024, // 6MB
		InitialConnectionReceiveWindow: 1024 * 1024,     // 1MB
		MaxConnectionReceiveWindow:     15 * 1024 * 1024, // 15MB

		// 重连默认值
		ReconnectEnabled: true,
		InitialBackoff:   1 * time.Second,
		MaxBackoff:       60 * time.Second,

		// 心跳默认值
		HeartbeatInterval: 15 * time.Second,
		HeartbeatTimeout:  5 * time.Second,

		// 消息默认值
		DefaultMessageTimeout: 30 * time.Second,

		// 默认日志
		Logger: monitoring.NewDefaultLogger(),
	}
}

// Validate 验证配置的有效性
func (c *ClientConfig) Validate() error {
	// 验证客户端 ID
	if c.ClientID == "" {
		return pkgerrors.ErrInvalidClientID
	}

	// 验证 TLS 配置
	if c.TLSConfig == nil && !c.InsecureSkipVerify {
		// 如果不跳过验证，检查是否提供了证书文件
		if c.TLSCertFile != "" && c.TLSKeyFile == "" {
			return fmt.Errorf("%w: TLS key file is required when cert file is provided", pkgerrors.ErrMissingTLSConfig)
		}
		if c.TLSCertFile == "" && c.TLSKeyFile != "" {
			return fmt.Errorf("%w: TLS cert file is required when key file is provided", pkgerrors.ErrMissingTLSConfig)
		}
	}

	// 验证超时配置
	if c.MaxIdleTimeout <= 0 {
		return fmt.Errorf("%w: MaxIdleTimeout must be positive", pkgerrors.ErrInvalidConfig)
	}

	if c.HeartbeatInterval <= 0 {
		return fmt.Errorf("%w: HeartbeatInterval must be positive", pkgerrors.ErrInvalidConfig)
	}

	// 验证重连配置
	if c.ReconnectEnabled {
		if c.InitialBackoff <= 0 {
			return fmt.Errorf("%w: InitialBackoff must be positive", pkgerrors.ErrInvalidConfig)
		}
		if c.MaxBackoff < c.InitialBackoff {
			return fmt.Errorf("%w: MaxBackoff must be >= InitialBackoff", pkgerrors.ErrInvalidConfig)
		}
	}

	return nil
}

// BuildTLSConfig 构建 TLS 配置
func (c *ClientConfig) BuildTLSConfig() (*tls.Config, error) {
	if c.TLSConfig != nil {
		// 验证自定义 TLS 配置
		if err := tlsutil.ValidateTLSConfig(c.TLSConfig); err != nil {
			return nil, err
		}
		return c.TLSConfig, nil
	}

	// 开发环境：跳过证书验证
	if c.InsecureSkipVerify {
		return tlsutil.NewInsecureClientTLSConfig(), nil
	}

	// 从文件加载 TLS 配置
	return tlsutil.LoadClientTLSConfig(
		c.TLSCertFile,
		c.TLSKeyFile,
		c.CACertFile,
		c.InsecureSkipVerify,
	)
}

// BuildQUICConfig 构建 QUIC 配置
func (c *ClientConfig) BuildQUICConfig() *quic.Config {
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
