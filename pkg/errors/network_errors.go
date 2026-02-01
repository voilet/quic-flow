package errors

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
)

// NetworkErrorType 网络错误类型
type NetworkErrorType int

const (
	// ErrorTypeTransient 瞬时错误 - 网络暂时不可用，应该快速重试
	ErrorTypeTransient NetworkErrorType = iota

	// ErrorTypeRefused 拒绝错误 - 服务端拒绝连接，应该慢速重试
	ErrorTypeRefused

	// ErrorTypeTimeout 超时错误 - 网络超时，中等退避
	ErrorTypeTimeout

	// ErrorTypeAuth 认证错误 - 不应该重试
	ErrorTypeAuth

	// ErrorTypeUnknown 未知错误
	ErrorTypeUnknown
)

// String 返回错误类型的字符串表示
func (t NetworkErrorType) String() string {
	switch t {
	case ErrorTypeTransient:
		return "Transient"
	case ErrorTypeRefused:
		return "Refused"
	case ErrorTypeTimeout:
		return "Timeout"
	case ErrorTypeAuth:
		return "Auth"
	default:
		return "Unknown"
	}
}

// ShouldReconnect 返回是否应该重连
func (t NetworkErrorType) ShouldReconnect() bool {
	switch t {
	case ErrorTypeAuth:
		return false // 认证错误不重连
	case ErrorTypeUnknown:
		return false // 未知错误不重连
	default:
		return true // 瞬时、拒绝、超时错误都应该重连
	}
}

// GetBackoffMultiplier 返回退避时间倍数
// 返回值应用于: newBackoff = baseBackoff * multiplier
func (t NetworkErrorType) GetBackoffMultiplier() float64 {
	switch t {
	case ErrorTypeTransient:
		return 1.0 // 瞬时错误，使用基础退避
	case ErrorTypeTimeout:
		return 1.5 // 超时错误，使用 1.5 倍退避
	case ErrorTypeRefused:
		return 2.0 // 拒绝错误，使用 2 倍退避（服务端可能压力大）
	default:
		return 1.0 // 未知错误，使用基础退避
	}
}

// ClassifyNetworkError 分类网络错误
func ClassifyNetworkError(err error) NetworkErrorType {
	if err == nil {
		return ErrorTypeUnknown
	}

	// 瞬时错误检测
	if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, io.EOF) {
		return ErrorTypeTransient
	}

	// 超时错误检测
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ErrorTypeTimeout
	}

	// 连接拒绝错误检测
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Op == "dial" {
			// 检查是否是 ECONNREFUSED
			if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
				return ErrorTypeRefused
			}
		}
		// read/write 操作错误通常是瞬时的
		if opErr.Op == "read" || opErr.Op == "write" {
			return ErrorTypeTransient
		}
	}

	// 直接检查 syscall 错误
	if errors.Is(err, syscall.ECONNREFUSED) {
		return ErrorTypeRefused
	}

	return ErrorTypeUnknown
}
