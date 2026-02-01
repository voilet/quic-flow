package errors

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
	"testing"
)

// TestClassifyNetworkError 测试网络错误分类
func TestClassifyNetworkError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected NetworkErrorType
	}{
		// 瞬时错误 - 应该快速重试
		{
			name:     "EOF 错误",
			err:      io.EOF,
			expected: ErrorTypeTransient,
		},
		{
			name:     "ErrClosedPipe 错误",
			err:      io.ErrClosedPipe,
			expected: ErrorTypeTransient,
		},
		{
			name:     "read 操作网络错误",
			err:      &net.OpError{Op: "read", Err: errors.New("connection reset")},
			expected: ErrorTypeTransient,
		},
		{
			name:     "write 操作网络错误",
			err:      &net.OpError{Op: "write", Err: errors.New("broken pipe")},
			expected: ErrorTypeTransient,
		},

		// 超时错误 - 中等退避
		{
			name:     "Context 超时",
			err:      context.DeadlineExceeded,
			expected: ErrorTypeTimeout,
		},
		{
			name:     "Context 取消",
			err:      context.Canceled,
			expected: ErrorTypeTimeout,
		},

		// 连接拒绝错误 - 慢速重试
		{
			name:     "连接被拒绝 (ECONNREFUSED)",
			err:      &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED},
			expected: ErrorTypeRefused,
		},
		{
			name:     "连接被拒绝 (简单的 ECONNREFUSED)",
			err:      syscall.ECONNREFUSED,
			expected: ErrorTypeRefused,
		},

		// 未知错误
		{
			name:     "nil 错误",
			err:      nil,
			expected: ErrorTypeUnknown,
		},
		{
			name:     "普通错误",
			err:      errors.New("some unknown error"),
			expected: ErrorTypeUnknown,
		},
		{
			name:     "dial 操作但非 ECONNREFUSED",
			err:      &net.OpError{Op: "dial", Err: errors.New("network unreachable")},
			expected: ErrorTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyNetworkError(tt.err)
			if result != tt.expected {
				t.Errorf("ClassifyNetworkError(%v) = %v, 期望 %v",
					tt.err, result, tt.expected)
			}
		})
	}
}

// TestErrorTypeString 测试错误类型字符串表示
func TestNetworkErrorTypeString(t *testing.T) {
	tests := []struct {
		typ      NetworkErrorType
		expected string
	}{
		{ErrorTypeTransient, "Transient"},
		{ErrorTypeRefused, "Refused"},
		{ErrorTypeTimeout, "Timeout"},
		{ErrorTypeAuth, "Auth"},
		{ErrorTypeUnknown, "Unknown"},
		{NetworkErrorType(999), "Unknown"}, // 未知类型
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.typ.String(); got != tt.expected {
				t.Errorf("%v.String() = %v, 期望 %v", tt.typ, got, tt.expected)
			}
		})
	}
}

// TestShouldReconnect 测试是否应该重连
func TestShouldReconnect(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		// 应该重连的错误
		{"EOF", io.EOF, true},
		{"ErrClosedPipe", io.ErrClosedPipe, true},
		{"Timeout", context.DeadlineExceeded, true},
		{"Canceled", context.Canceled, true},
		{"Connection Refused", syscall.ECONNREFUSED, true},
		{"Transient network error", &net.OpError{Op: "read", Err: errors.New("reset")}, true},

		// 不应该重连的错误
		{"Nil error", nil, false},
		{"Unknown error", errors.New("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errType := ClassifyNetworkError(tt.err)
			result := errType.ShouldReconnect()
			if result != tt.expected {
				t.Errorf("ShouldReconnect(%v) = %v, 期望 %v", tt.err, result, tt.expected)
			}
		})
	}
}

// TestGetBackoffMultiplier 测试退避时间倍数
func TestGetBackoffMultiplier(t *testing.T) {
	tests := []struct {
		name               string
		err                error
		expectedMultiplier float64
	}{
		{"Transient error - 快速重试", io.EOF, 1.0},
		{"Timeout error - 正常退避", context.DeadlineExceeded, 1.5},
		{"Refused error - 慢速重试", syscall.ECONNREFUSED, 2.0},
		{"Unknown error - 默认", errors.New("unknown"), 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errType := ClassifyNetworkError(tt.err)
			result := errType.GetBackoffMultiplier()
			if result != tt.expectedMultiplier {
				t.Errorf("GetBackoffMultiplier(%v) = %v, 期望 %v",
					tt.err, result, tt.expectedMultiplier)
			}
		})
	}
}

// BenchmarkClassifyNetworkError 基准测试错误分类性能
func BenchmarkClassifyNetworkError(b *testing.B) {
	errs := []error{
		io.EOF,
		context.DeadlineExceeded,
		syscall.ECONNREFUSED,
		&net.OpError{Op: "read", Err: errors.New("reset")},
		errors.New("unknown"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ClassifyNetworkError(errs[i%len(errs)])
	}
}
