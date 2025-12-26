package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	pkgerrors "github.com/voilet/quic-flow/pkg/errors"
)

const (
	// ALPN 协议标识
	ALPNProtocol = "quic-backbone-v1"
)

// LoadServerTLSConfig 加载服务器端 TLS 配置
// certFile: 证书文件路径
// keyFile: 私钥文件路径
func LoadServerTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	// 加载证书和私钥
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to load certificate: %v", pkgerrors.ErrMissingTLSConfig, err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{ALPNProtocol}, // ALPN 协议标识
		MinVersion:   tls.VersionTLS13,       // 强制 TLS 1.3
		MaxVersion:   tls.VersionTLS13,
	}

	return tlsConfig, nil
}

// LoadClientTLSConfig 加载客户端 TLS 配置
// certFile: 客户端证书文件路径（双向 TLS 时使用，可选）
// keyFile: 客户端私钥文件路径（双向 TLS 时使用，可选）
// caCertFile: CA 证书文件路径（用于验证服务器证书，可选）
// insecureSkipVerify: 是否跳过服务器证书验证（仅用于开发环境）
func LoadClientTLSConfig(certFile, keyFile, caCertFile string, insecureSkipVerify bool) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		NextProtos:         []string{ALPNProtocol}, // ALPN 协议标识
		MinVersion:         tls.VersionTLS13,       // 强制 TLS 1.3
		MaxVersion:         tls.VersionTLS13,
		InsecureSkipVerify: insecureSkipVerify, // 生产环境必须为 false
	}

	// 加载客户端证书（双向 TLS）
	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to load client certificate: %v", pkgerrors.ErrMissingTLSConfig, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// 加载 CA 证书（用于验证服务器）
	if caCertFile != "" {
		caCert, err := os.ReadFile(caCertFile)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to read CA certificate: %v", pkgerrors.ErrMissingTLSConfig, err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("%w: failed to parse CA certificate", pkgerrors.ErrMissingTLSConfig)
		}

		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// NewInsecureClientTLSConfig 创建不验证服务器证书的客户端 TLS 配置
// ⚠️ 警告：仅用于开发环境，生产环境禁止使用
func NewInsecureClientTLSConfig() *tls.Config {
	return &tls.Config{
		NextProtos:         []string{ALPNProtocol},
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true, // 跳过证书验证
	}
}

// ValidateTLSConfig 验证 TLS 配置的有效性
func ValidateTLSConfig(config *tls.Config) error {
	if config == nil {
		return fmt.Errorf("%w: TLS config is nil", pkgerrors.ErrMissingTLSConfig)
	}

	// 检查 ALPN 协议
	found := false
	for _, proto := range config.NextProtos {
		if proto == ALPNProtocol {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%w: ALPN protocol %s not found", pkgerrors.ErrInvalidConfig, ALPNProtocol)
	}

	// 检查 TLS 版本
	if config.MinVersion != tls.VersionTLS13 {
		return fmt.Errorf("%w: TLS 1.3 is required, got min version %d", pkgerrors.ErrInvalidConfig, config.MinVersion)
	}

	return nil
}
