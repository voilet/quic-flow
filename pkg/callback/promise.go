package callback

import (
	"sync"
	"time"

	"github.com/voilet/quic-flow/pkg/protocol"
)

// Promise 异步请求的响应承诺 (T040)
// 用于追踪发出的消息并等待对应的 Ack 响应
type Promise struct {
	// 消息 ID
	MsgID string

	// 响应通道
	RespChan chan *PromiseResponse

	// 超时定时器
	Timer *time.Timer

	// 创建时间
	CreatedAt time.Time

	// 是否已完成
	completed bool
	mu        sync.Mutex
}

// PromiseResponse Promise 的响应结果
type PromiseResponse struct {
	AckMessage *protocol.AckMessage // Ack 消息（如果成功）
	Error      error                 // 错误（如果超时或失败）
}

// NewPromise 创建新的 Promise
// msgID: 消息 ID
// timeout: 超时时间
// onTimeout: 超时回调函数
func NewPromise(msgID string, timeout time.Duration, onTimeout func(msgID string)) *Promise {
	p := &Promise{
		MsgID:     msgID,
		RespChan:  make(chan *PromiseResponse, 1),
		CreatedAt: time.Now(),
		completed: false,
	}

	// 设置超时定时器
	p.Timer = time.AfterFunc(timeout, func() {
		if onTimeout != nil {
			onTimeout(msgID)
		}
		p.Timeout()
	})

	return p
}

// Complete 完成 Promise（收到响应）
// ack: Ack 消息
func (p *Promise) Complete(ack *protocol.AckMessage) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.completed {
		return false
	}

	p.completed = true

	// 停止定时器
	if p.Timer != nil {
		p.Timer.Stop()
	}

	// 发送响应
	select {
	case p.RespChan <- &PromiseResponse{
		AckMessage: ack,
		Error:      nil,
	}:
	default:
		// Channel 已满，忽略
	}

	return true
}

// Timeout 超时处理
func (p *Promise) Timeout() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.completed {
		return false
	}

	p.completed = true

	// 发送超时错误
	select {
	case p.RespChan <- &PromiseResponse{
		AckMessage: nil,
		Error:      ErrPromiseTimeout,
	}:
	default:
		// Channel 已满，忽略
	}

	return true
}

// Fail 失败处理
func (p *Promise) Fail(err error) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.completed {
		return false
	}

	p.completed = true

	// 停止定时器
	if p.Timer != nil {
		p.Timer.Stop()
	}

	// 发送错误
	select {
	case p.RespChan <- &PromiseResponse{
		AckMessage: nil,
		Error:      err,
	}:
	default:
		// Channel 已满，忽略
	}

	return true
}

// IsCompleted 检查是否已完成
func (p *Promise) IsCompleted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.completed
}

// Age 获取 Promise 存活时间
func (p *Promise) Age() time.Duration {
	return time.Since(p.CreatedAt)
}
