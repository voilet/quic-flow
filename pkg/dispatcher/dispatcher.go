package dispatcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/protocol"
)

// Dispatcher 消息分发器 (T035, T036)
// 负责将收到的消息路由到对应的 MessageHandler
type Dispatcher struct {
	// Handler 注册表（按消息类型）
	handlers sync.Map // map[protocol.MessageType]MessageHandler

	// Worker pool
	workerCount int
	taskQueue   chan *DispatchTask
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc

	// 监控
	logger  *monitoring.Logger
	metrics *monitoring.Metrics

	// 配置
	config *DispatcherConfig
}

// DispatcherConfig Dispatcher 配置
type DispatcherConfig struct {
	WorkerCount      int           // Worker 数量（默认 10）
	TaskQueueSize    int           // 任务队列大小（默认 1000）
	HandlerTimeout   time.Duration // Handler 处理超时（默认 30s）
	Logger           *monitoring.Logger
	Metrics          *monitoring.Metrics
	DefaultHandler   MessageHandler // 默认 Handler（可选，处理未注册的消息类型）
}

// DispatchTask 分发任务
type DispatchTask struct {
	Message    *protocol.DataMessage
	Context    context.Context
	ResponseCh chan<- *DispatchResponse // 响应通道（可选）
}

// DispatchResponse 分发响应
type DispatchResponse struct {
	Response *protocol.DataMessage
	Error    error
}

// NewDispatcher 创建新的 Dispatcher
func NewDispatcher(config *DispatcherConfig) *Dispatcher {
	if config == nil {
		config = &DispatcherConfig{}
	}

	// 默认值
	if config.WorkerCount <= 0 {
		config.WorkerCount = 10
	}
	if config.TaskQueueSize <= 0 {
		config.TaskQueueSize = 1000
	}
	if config.HandlerTimeout <= 0 {
		config.HandlerTimeout = 30 * time.Second
	}
	if config.Logger == nil {
		config.Logger = monitoring.NewLogger(monitoring.LogLevelInfo, "text")
	}
	if config.Metrics == nil {
		config.Metrics = monitoring.NewMetrics()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Dispatcher{
		workerCount: config.WorkerCount,
		taskQueue:   make(chan *DispatchTask, config.TaskQueueSize),
		ctx:         ctx,
		cancel:      cancel,
		logger:      config.Logger,
		metrics:     config.Metrics,
		config:      config,
	}
}

// RegisterHandler 注册消息处理器 (T036)
// msgType: 消息类型
// handler: 处理器实现
func (d *Dispatcher) RegisterHandler(msgType protocol.MessageType, handler MessageHandler) {
	d.handlers.Store(msgType, handler)
	d.logger.Debug("Handler registered", "message_type", msgType)
}

// UnregisterHandler 注销消息处理器
func (d *Dispatcher) UnregisterHandler(msgType protocol.MessageType) {
	d.handlers.Delete(msgType)
	d.logger.Debug("Handler unregistered", "message_type", msgType)
}

// Start 启动 Dispatcher (T035)
func (d *Dispatcher) Start() {
	d.logger.Info("Starting dispatcher", "workers", d.workerCount, "queue_size", d.config.TaskQueueSize)

	// 启动 worker goroutines
	for i := 0; i < d.workerCount; i++ {
		d.wg.Add(1)
		go d.worker(i)
	}

	d.logger.Info("Dispatcher started", "workers", d.workerCount)
}

// Stop 停止 Dispatcher
func (d *Dispatcher) Stop() {
	d.logger.Info("Stopping dispatcher...")

	// 取消上下文
	d.cancel()

	// 等待所有 worker 完成
	d.wg.Wait()

	d.logger.Info("Dispatcher stopped")
}

// Dispatch 分发消息（异步）(T036)
// msg: 要分发的消息
// responseCh: 可选的响应通道，如果提供，则会将处理结果发送到此通道
func (d *Dispatcher) Dispatch(ctx context.Context, msg *protocol.DataMessage, responseCh chan<- *DispatchResponse) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	task := &DispatchTask{
		Message:    msg,
		Context:    ctx,
		ResponseCh: responseCh,
	}

	select {
	case d.taskQueue <- task:
		d.metrics.RecordMessageReceived(int64(len(msg.Payload)))
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-d.ctx.Done():
		return fmt.Errorf("dispatcher is stopped")
	}
}

// DispatchSync 分发消息（同步）
// 等待处理完成并返回结果
func (d *Dispatcher) DispatchSync(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
	responseCh := make(chan *DispatchResponse, 1)

	if err := d.Dispatch(ctx, msg, responseCh); err != nil {
		return nil, err
	}

	select {
	case resp := <-responseCh:
		return resp.Response, resp.Error
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// worker Worker goroutine (T035)
func (d *Dispatcher) worker(id int) {
	defer d.wg.Done()

	d.logger.Debug("Worker started", "worker_id", id)

	for {
		select {
		case <-d.ctx.Done():
			d.logger.Debug("Worker stopped", "worker_id", id)
			return

		case task := <-d.taskQueue:
			d.processTask(task)
		}
	}
}

// processTask 处理单个任务 (T036)
func (d *Dispatcher) processTask(task *DispatchTask) {
	startTime := time.Now()

	// 查找对应的 Handler
	handler := d.findHandler(task.Message.Type)
	if handler == nil {
		d.logger.Warn("No handler registered for message type", "type", task.Message.Type)
		d.sendResponse(task.ResponseCh, nil, fmt.Errorf("no handler for message type: %v", task.Message.Type))
		return
	}

	// 创建带超时的 context
	ctx, cancel := context.WithTimeout(task.Context, d.config.HandlerTimeout)
	defer cancel()

	// 调用 Handler
	response, err := handler.OnMessage(ctx, task.Message)

	// 记录延迟
	duration := time.Since(startTime)
	d.metrics.RecordLatency(duration)

	if err != nil {
		d.logger.Error("Handler failed", "type", task.Message.Type, "error", err, "duration", duration)
		d.metrics.RecordDecodingError() // 复用此指标表示处理错误
	} else {
		d.logger.Debug("Message processed", "type", task.Message.Type, "duration", duration)
	}

	// 发送响应（如果需要）
	d.sendResponse(task.ResponseCh, response, err)
}

// findHandler 查找 Handler (T036)
func (d *Dispatcher) findHandler(msgType protocol.MessageType) MessageHandler {
	if handler, ok := d.handlers.Load(msgType); ok {
		return handler.(MessageHandler)
	}

	// 尝试使用默认 Handler
	if d.config.DefaultHandler != nil {
		return d.config.DefaultHandler
	}

	return nil
}

// sendResponse 发送响应
func (d *Dispatcher) sendResponse(responseCh chan<- *DispatchResponse, response *protocol.DataMessage, err error) {
	if responseCh == nil {
		return
	}

	select {
	case responseCh <- &DispatchResponse{
		Response: response,
		Error:    err,
	}:
	default:
		d.logger.Warn("Failed to send response: channel full or closed")
	}
}

// GetQueueLength 获取当前队列长度（监控用）
func (d *Dispatcher) GetQueueLength() int {
	return len(d.taskQueue)
}
