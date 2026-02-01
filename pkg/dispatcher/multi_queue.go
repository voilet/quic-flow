package dispatcher

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/protocol"
)

// MultiQueueDispatcher 多队列分发器
// 将任务分散到多个队列中，每个队列有独立的 worker pool
// 在高并发场景下减少队列竞争，提升吞吐量
type MultiQueueDispatcher struct {
	// Handler 注册表（按消息类型）
	handlers sync.Map // map[protocol.MessageType]MessageHandler

	// 多队列配置
	queueCount int              // 队列数量
	queues     []*dispatchQueue // 分发队列数组

	// 轮询计数器（用于负载均衡）
	dispatchIndex atomic.Uint64

	// 监控
	logger  *monitoring.Logger
	metrics *monitoring.Metrics

	// 配置
	config *DispatcherConfig

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// dispatchQueue 单个分发队列
type dispatchQueue struct {
	taskQueue chan *DispatchTask
	workers   int // 该队列的 worker 数量
}

// MultiQueueConfig 多队列配置
type MultiQueueConfig struct {
	QueueCount int // 队列数量（默认 4）
}

// NewMultiQueueDispatcher 创建多队列分发器
func NewMultiQueueDispatcher(config *DispatcherConfig, mqConfig *MultiQueueConfig) *MultiQueueDispatcher {
	if config == nil {
		config = &DispatcherConfig{}
	}
	if mqConfig == nil {
		mqConfig = &MultiQueueConfig{}
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

	// 队列数量默认为 4，但不超过 worker 数量
	if mqConfig.QueueCount <= 0 {
		mqConfig.QueueCount = 4
	}
	if mqConfig.QueueCount > config.WorkerCount {
		mqConfig.QueueCount = config.WorkerCount
	}
	if mqConfig.QueueCount > 32 {
		mqConfig.QueueCount = 32 // 最多 32 个队列
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 计算每个队列的 worker 数量
	workersPerQueue := config.WorkerCount / mqConfig.QueueCount
	remainingWorkers := config.WorkerCount % mqConfig.QueueCount

	// 创建队列
	queues := make([]*dispatchQueue, mqConfig.QueueCount)
	for i := 0; i < mqConfig.QueueCount; i++ {
		workerCount := workersPerQueue
		if i < remainingWorkers {
			workerCount++ // 前几个队列多分配一个 worker
		}

		queues[i] = &dispatchQueue{
			taskQueue: make(chan *DispatchTask, config.TaskQueueSize/mqConfig.QueueCount+1),
			workers:   workerCount,
		}
	}

	return &MultiQueueDispatcher{
		queueCount: mqConfig.QueueCount,
		queues:     queues,
		logger:     config.Logger,
		metrics:    config.Metrics,
		config:     config,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// RegisterHandler 注册消息处理器
func (d *MultiQueueDispatcher) RegisterHandler(msgType protocol.MessageType, handler MessageHandler) {
	d.handlers.Store(msgType, handler)
	d.logger.Debug("Handler registered", "message_type", msgType, "dispatcher", "multi-queue")
}

// UnregisterHandler 注销消息处理器
func (d *MultiQueueDispatcher) UnregisterHandler(msgType protocol.MessageType) {
	d.handlers.Delete(msgType)
	d.logger.Debug("Handler unregistered", "message_type", msgType, "dispatcher", "multi-queue")
}

// Start 启动多队列分发器
func (d *MultiQueueDispatcher) Start() {
	totalWorkers := 0
	for i, q := range d.queues {
		for j := 0; j < q.workers; j++ {
			d.wg.Add(1)
			go d.multiQueueWorker(i, j)
			totalWorkers++
		}
	}

	d.logger.Info("Multi-queue dispatcher started",
		"queues", d.queueCount,
		"total_workers", totalWorkers,
		"workers_per_queue", d.config.WorkerCount/d.queueCount)
}

// Stop 停止分发器
func (d *MultiQueueDispatcher) Stop() {
	d.logger.Info("Stopping multi-queue dispatcher...")

	d.cancel()
	d.wg.Wait()

	d.logger.Info("Multi-queue dispatcher stopped")
}

// Dispatch 分发消息（异步）
// 使用轮询方式选择队列，实现负载均衡
func (d *MultiQueueDispatcher) Dispatch(ctx context.Context, msg *protocol.DataMessage, responseCh chan<- *DispatchResponse) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	task := &DispatchTask{
		Message:    msg,
		Context:    ctx,
		ResponseCh: responseCh,
	}

	// 轮询选择队列（减少竞争，实现负载均衡）
	index := d.dispatchIndex.Add(1) % uint64(d.queueCount)
	queue := d.queues[index]

	select {
	case queue.taskQueue <- task:
		d.metrics.RecordMessageReceived(int64(len(msg.Payload)))
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-d.ctx.Done():
		return fmt.Errorf("dispatcher is stopped")
	}
}

// DispatchByHash 根据消息 ID 哈希分发消息
// 相同的消息总是分发到同一个队列，保证顺序性
func (d *MultiQueueDispatcher) DispatchByHash(ctx context.Context, msg *protocol.DataMessage, responseCh chan<- *DispatchResponse) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	task := &DispatchTask{
		Message:    msg,
		Context:    ctx,
		ResponseCh: responseCh,
	}

	// 使用消息 ID 的哈希值选择队列
	hash := fnvHash(msg.MsgId)
	index := hash % uint32(d.queueCount)
	queue := d.queues[index]

	select {
	case queue.taskQueue <- task:
		d.metrics.RecordMessageReceived(int64(len(msg.Payload)))
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-d.ctx.Done():
		return fmt.Errorf("dispatcher is stopped")
	}
}

// DispatchSync 分发消息（同步）
func (d *MultiQueueDispatcher) DispatchSync(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
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

// multiQueueWorker 多队列 worker
func (d *MultiQueueDispatcher) multiQueueWorker(queueIdx, workerIdx int) {
	defer d.wg.Done()

	d.logger.Debug("Multi-queue worker started",
		"queue_id", queueIdx,
		"worker_id", workerIdx)

	queue := d.queues[queueIdx]

	for {
		select {
		case <-d.ctx.Done():
			d.logger.Debug("Multi-queue worker stopped",
				"queue_id", queueIdx,
				"worker_id", workerIdx)
			return

		case task := <-queue.taskQueue:
			d.processTask(task)
		}
	}
}

// processTask 处理单个任务
func (d *MultiQueueDispatcher) processTask(task *DispatchTask) {
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
		d.metrics.RecordDecodingError()
	} else {
		d.logger.Debug("Message processed", "type", task.Message.Type, "duration", duration)
	}

	// 发送响应
	d.sendResponse(task.ResponseCh, response, err)
}

// findHandler 查找 Handler
func (d *MultiQueueDispatcher) findHandler(msgType protocol.MessageType) MessageHandler {
	if handler, ok := d.handlers.Load(msgType); ok {
		return handler.(MessageHandler)
	}

	if d.config.DefaultHandler != nil {
		return d.config.DefaultHandler
	}

	return nil
}

// sendResponse 发送响应
func (d *MultiQueueDispatcher) sendResponse(responseCh chan<- *DispatchResponse, response *protocol.DataMessage, err error) {
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

// GetQueueLengths 获取所有队列的当前长度（监控用）
func (d *MultiQueueDispatcher) GetQueueLengths() []int {
	lengths := make([]int, d.queueCount)
	for i, q := range d.queues {
		lengths[i] = len(q.taskQueue)
	}
	return lengths
}

// GetTotalQueueLength 获取所有队列的总长度
func (d *MultiQueueDispatcher) GetTotalQueueLength() int {
	total := 0
	for _, q := range d.queues {
		total += len(q.taskQueue)
	}
	return total
}

// GetStats 获取分发器统计信息
func (d *MultiQueueDispatcher) GetStats() *MultiQueueStats {
	lengths := d.GetQueueLengths()
	totalLength := 0
	minLength := lengths[0]
	maxLength := lengths[0]

	for _, l := range lengths {
		totalLength += l
		if l < minLength {
			minLength = l
		}
		if l > maxLength {
			maxLength = l
		}
	}

	return &MultiQueueStats{
		QueueCount:       d.queueCount,
		TotalQueueLength: totalLength,
		MinQueueLength:   minLength,
		MaxQueueLength:   maxLength,
		AvgQueueLength:   float64(totalLength) / float64(d.queueCount),
	}
}

// MultiQueueStats 多队列统计信息
type MultiQueueStats struct {
	QueueCount       int     // 队列数量
	TotalQueueLength int     // 总队列长度
	MinQueueLength   int     // 最小队列长度
	MaxQueueLength   int     // 最大队列长度
	AvgQueueLength   float64 // 平均队列长度
}

// fnvHash FNV-1a 哈希算法
func fnvHash(s string) uint32 {
	hash := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		hash ^= uint32(s[i])
		hash *= 16777619
	}
	return hash
}

// ============================================================================
// 批量处理 API
// ============================================================================

// DispatchBatch 批量分发消息（异步）
func (d *MultiQueueDispatcher) DispatchBatch(ctx context.Context, msgs []*protocol.DataMessage) error {
	if len(msgs) == 0 {
		return nil
	}

	for _, msg := range msgs {
		if err := d.Dispatch(ctx, msg, nil); err != nil {
			return err
		}
	}
	return nil
}

// DispatchBatchSync 批量分发消息（同步）
func (d *MultiQueueDispatcher) DispatchBatchSync(ctx context.Context, msgs []*protocol.DataMessage) (*BatchDispatchResult, error) {
	if len(msgs) == 0 {
		return &BatchDispatchResult{}, nil
	}

	responseChs := make([]chan *DispatchResponse, len(msgs))
	for i := range responseChs {
		responseChs[i] = make(chan *DispatchResponse, 1)
	}

	// 批量分发
	for i, msg := range msgs {
		if err := d.Dispatch(ctx, msg, responseChs[i]); err != nil {
			for j := 0; j < i; j++ {
				close(responseChs[j])
			}
			return nil, err
		}
	}

	// 收集结果
	result := &BatchDispatchResult{
		Responses: make([]*protocol.DataMessage, len(msgs)),
		Errors:    make([]error, len(msgs)),
	}

	for i, ch := range responseChs {
		select {
		case resp := <-ch:
			if resp.Error != nil {
				result.FailedCount++
				result.Errors[i] = resp.Error
			} else {
				result.SuccessCount++
				result.Responses[i] = resp.Response
			}
		case <-ctx.Done():
			return result, ctx.Err()
		}
	}

	return result, nil
}
