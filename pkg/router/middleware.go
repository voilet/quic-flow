package router

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/voilet/quic-flow/pkg/monitoring"
)

// LoggingMiddleware 日志中间件
// 记录命令执行的开始、结束和耗时
func LoggingMiddleware(logger *monitoring.Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
			start := time.Now()

			// 从context获取命令类型（如果有）
			cmdType := "unknown"
			if v := ctx.Value(ContextKeyCommandType); v != nil {
				cmdType = v.(string)
			}

			logger.Debug("Command execution started", "command_type", cmdType)

			// 执行下一个处理器
			result, err := next(ctx, payload)

			duration := time.Since(start)
			if err != nil {
				logger.Error("Command execution failed",
					"command_type", cmdType,
					"duration", duration,
					"error", err,
				)
			} else {
				logger.Info("Command execution completed",
					"command_type", cmdType,
					"duration", duration,
				)
			}

			return result, err
		}
	}
}

// RecoveryMiddleware panic恢复中间件
// 捕获Handler中的panic，返回错误而不是崩溃
func RecoveryMiddleware(logger *monitoring.Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, payload json.RawMessage) (result json.RawMessage, err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := string(debug.Stack())
					logger.Error("Command handler panic recovered",
						"panic", r,
						"stack", stack,
					)
					err = fmt.Errorf("internal error: %v", r)
					result = nil
				}
			}()

			return next(ctx, payload)
		}
	}
}

// TimeoutMiddleware 超时中间件
// 为命令执行设置超时时间
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
			// 如果context已有更短的deadline，则使用已有的
			if deadline, ok := ctx.Deadline(); ok {
				if time.Until(deadline) < timeout {
					return next(ctx, payload)
				}
			}

			// 创建带超时的context
			timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// 使用channel接收结果
			type result struct {
				data json.RawMessage
				err  error
			}
			done := make(chan result, 1)

			go func() {
				data, err := next(timeoutCtx, payload)
				done <- result{data, err}
			}()

			select {
			case <-timeoutCtx.Done():
				return nil, fmt.Errorf("command execution timeout after %v", timeout)
			case r := <-done:
				return r.data, r.err
			}
		}
	}
}

// MetricsMiddleware 指标收集中间件
// 记录命令执行次数、成功/失败率、延迟等
func MetricsMiddleware(metrics *monitoring.Metrics) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
			start := time.Now()

			result, err := next(ctx, payload)

			// 记录延迟
			metrics.RecordLatency(time.Since(start))

			// 记录消息处理
			if err != nil {
				metrics.RecordDecodingError() // 复用错误计数
			} else {
				metrics.RecordMessageReceived(int64(len(result)))
			}

			return result, err
		}
	}
}

// RateLimitMiddleware 限流中间件
// 使用令牌桶算法限制命令执行频率
func RateLimitMiddleware(rate int, burst int) Middleware {
	limiter := newTokenBucket(rate, burst)

	return func(next Handler) Handler {
		return func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
			if !limiter.Allow() {
				return nil, fmt.Errorf("rate limit exceeded")
			}
			return next(ctx, payload)
		}
	}
}

// tokenBucket 令牌桶实现
type tokenBucket struct {
	rate       int       // 每秒产生的令牌数
	burst      int       // 桶容量
	tokens     float64   // 当前令牌数
	lastUpdate time.Time // 上次更新时间
}

func newTokenBucket(rate, burst int) *tokenBucket {
	return &tokenBucket{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastUpdate: time.Now(),
	}
}

func (tb *tokenBucket) Allow() bool {
	now := time.Now()
	elapsed := now.Sub(tb.lastUpdate).Seconds()
	tb.lastUpdate = now

	// 补充令牌
	tb.tokens += elapsed * float64(tb.rate)
	if tb.tokens > float64(tb.burst) {
		tb.tokens = float64(tb.burst)
	}

	// 检查是否有可用令牌
	if tb.tokens < 1 {
		return false
	}

	tb.tokens--
	return true
}

// ValidationMiddleware 参数验证中间件
// 验证payload是否为有效JSON
func ValidationMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
			// 检查是否为有效JSON
			if len(payload) > 0 && !json.Valid(payload) {
				return nil, fmt.Errorf("invalid JSON payload")
			}
			return next(ctx, payload)
		}
	}
}

// RetryMiddleware 重试中间件
// 在命令失败时自动重试
func RetryMiddleware(maxRetries int, delay time.Duration) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
			var lastErr error
			for i := 0; i <= maxRetries; i++ {
				result, err := next(ctx, payload)
				if err == nil {
					return result, nil
				}
				lastErr = err

				// 检查是否应该重试
				if i < maxRetries {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(delay):
						// 继续重试
					}
				}
			}
			return nil, fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
		}
	}
}

// ChainMiddleware 组合多个中间件为一个
func ChainMiddleware(middlewares ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// Context keys
type contextKey string

const (
	ContextKeyCommandType  contextKey = "command_type"
	ContextKeyCommandID    contextKey = "command_id"
	ContextKeyClientID     contextKey = "client_id"
	ContextKeyNeedCallback contextKey = "need_callback"
	ContextKeyCallbackID   contextKey = "callback_id"
	ContextKeyCallbackFunc contextKey = "callback_func"
)

// CallbackFunc 回调函数类型
type CallbackFunc func(callbackID, commandType string, success bool, result []byte, errMsg string)

// WithCommandContext 创建带命令信息的context
func WithCommandContext(ctx context.Context, commandType, commandID, clientID string) context.Context {
	ctx = context.WithValue(ctx, ContextKeyCommandType, commandType)
	ctx = context.WithValue(ctx, ContextKeyCommandID, commandID)
	ctx = context.WithValue(ctx, ContextKeyClientID, clientID)
	return ctx
}

// WithCallbackContext 创建带回调信息的context
func WithCallbackContext(ctx context.Context, needCallback bool, callbackID string, callbackFunc CallbackFunc) context.Context {
	ctx = context.WithValue(ctx, ContextKeyNeedCallback, needCallback)
	ctx = context.WithValue(ctx, ContextKeyCallbackID, callbackID)
	if callbackFunc != nil {
		ctx = context.WithValue(ctx, ContextKeyCallbackFunc, callbackFunc)
	}
	return ctx
}

// GetCallbackInfo 从context中获取回调信息
func GetCallbackInfo(ctx context.Context) (needCallback bool, callbackID string) {
	if v := ctx.Value(ContextKeyNeedCallback); v != nil {
		needCallback = v.(bool)
	}
	if v := ctx.Value(ContextKeyCallbackID); v != nil {
		callbackID = v.(string)
	}
	return
}

// GetCallbackFunc 从context中获取回调函数
func GetCallbackFunc(ctx context.Context) CallbackFunc {
	if v := ctx.Value(ContextKeyCallbackFunc); v != nil {
		return v.(CallbackFunc)
	}
	return nil
}

// DoCallback 执行回调（如果需要）
func DoCallback(ctx context.Context, success bool, result []byte, errMsg string) {
	needCallback, callbackID := GetCallbackInfo(ctx)
	if !needCallback {
		return
	}

	callbackFunc := GetCallbackFunc(ctx)
	if callbackFunc == nil {
		return
	}

	commandType := ""
	if v := ctx.Value(ContextKeyCommandType); v != nil {
		commandType = v.(string)
	}

	callbackFunc(callbackID, commandType, success, result, errMsg)
}
