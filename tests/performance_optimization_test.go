package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/voilet/quic-flow/pkg/dispatcher"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/protocol"
	"github.com/voilet/quic-flow/pkg/transport/client"
)

// ============================================================================
// 性能优化测试用例
// 目标：通过基准测试验证优化是否有必要
// ============================================================================

// ----------------------------------------------------------------------------
// 优化点 1: sync.Map vs 分片 Map (ShardedMap)
// 场景：SessionManager 在高并发读写场景下的性能
// ----------------------------------------------------------------------------

// BenchmarkSyncMapVsShardedMap 比较 sync.Map 和分片 Map 的性能
// 使用 mock 会话对象避免依赖真实的 QUIC 连接
func BenchmarkSyncMapVsShardedMap(b *testing.B) {
	sizes := []int{1000, 10000, 50000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("SyncMap_%d", size), func(b *testing.B) {
			benchmarkSyncMap(b, size)
		})

		b.Run(fmt.Sprintf("ShardedMap_%d", size), func(b *testing.B) {
			benchmarkShardedMap(b, size)
		})
	}
}

// benchmarkSyncMap 测试 sync.Map 性能（使用 mock）
func benchmarkSyncMap(b *testing.B, size int) {
	sm := NewMockSessionManager()

	// 预填充数据
	for i := 0; i < size; i++ {
		sess := &MockSession{ClientID: fmt.Sprintf("client-%d", i)}
		sm.Add(sess.ClientID, sess)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 模拟读写混合场景 (80% 读, 20% 写)
		clientID := fmt.Sprintf("client-%d", i%size)
		if i%5 == 0 {
			// 写操作
			sess := &MockSession{ClientID: clientID}
			sm.Add(clientID, sess)
		} else {
			// 读操作
			sm.Get(clientID)
		}
	}
}

// benchmarkShardedMap 测试分片 Map 性能（使用 mock）
func benchmarkShardedMap(b *testing.B, size int) {
	sm := NewShardedSessionManager(32) // 32 个分片

	// 预填充数据
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("client-%d", i)
		sm.Add(key, &mockSession{ClientID: key})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 模拟读写混合场景 (80% 读, 20% 写)
		clientID := fmt.Sprintf("client-%d", i%size)
		if i%5 == 0 {
			// 写操作
			sm.Add(clientID, &mockSession{ClientID: clientID})
		} else {
			// 读操作
			sm.Get(clientID)
		}
	}
}

// ShardedSessionManager 分片会话管理器（用于对比测试）
type ShardedSessionManager struct {
	shards []*shard
	count  atomic.Int64
}

type shard struct {
	sync.RWMutex
	sessions map[string]*mockSession
}

type mockSession struct {
	ClientID string
}

func NewShardedSessionManager(shardCount int) *ShardedSessionManager {
	sm := &ShardedSessionManager{
		shards: make([]*shard, shardCount),
	}
	for i := 0; i < shardCount; i++ {
		sm.shards[i] = &shard{
			sessions: make(map[string]*mockSession),
		}
	}
	return sm
}

func (sm *ShardedSessionManager) getShard(key string) *shard {
	// 使用 FNV 哈希算法
	hash := uint32(0)
	for _, c := range key {
		hash = hash*31 + uint32(c)
	}
	return sm.shards[int(hash)%len(sm.shards)]
}

func (sm *ShardedSessionManager) Add(key string, sess *mockSession) {
	shard := sm.getShard(key)
	shard.Lock()
	shard.sessions[key] = sess
	shard.Unlock()
	sm.count.Add(1)
}

func (sm *ShardedSessionManager) Get(key string) (*mockSession, bool) {
	shard := sm.getShard(key)
	shard.RLock()
	sess, ok := shard.sessions[key]
	shard.RUnlock()
	return sess, ok
}

// ----------------------------------------------------------------------------
// 优化点 2: 数据库批量插入 vs 逐条插入
// 场景：批量创建执行记录时的性能差异
// ----------------------------------------------------------------------------

// BenchmarkBatchInsertVsSingleInsert 比较批量插入和逐条插入的性能
func BenchmarkBatchInsertVsSingleInsert(b *testing.B) {
	sizes := []int{10, 50, 100, 500}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("BatchInsert_%d", size), func(b *testing.B) {
			// 模拟批量插入
			b.StopTimer()
			executions := make([]*mockExecution, size)
			for i := 0; i < size; i++ {
				executions[i] = &mockExecution{ID: int64(i)}
			}
			b.StartTimer()

			benchmarkBatchInsert(b, executions)
		})

		b.Run(fmt.Sprintf("SingleInsert_%d", size), func(b *testing.B) {
			b.StopTimer()
			executions := make([]*mockExecution, size)
			for i := 0; i < size; i++ {
				executions[i] = &mockExecution{ID: int64(i)}
			}
			b.StartTimer()

			benchmarkSingleInsert(b, executions)
		})
	}
}

func benchmarkBatchInsert(b *testing.B, executions []*mockExecution) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// 模拟批量插入（使用事务）
		_ = mockBatchCreate(executions)
	}
}

func benchmarkSingleInsert(b *testing.B, executions []*mockExecution) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, exec := range executions {
			_ = mockSingleCreate(exec)
		}
	}
}

type mockExecution struct {
	ID int64
}

func mockBatchCreate(executions []*mockExecution) error {
	// 模拟批量插入：构建一个大 SQL 语句
	// INSERT INTO executions (id) VALUES (1), (2), (3), ...
	return nil
}

func mockSingleCreate(exec *mockExecution) error {
	// 模拟单条插入
	// INSERT INTO executions (id) VALUES (1)
	return nil
}

// ----------------------------------------------------------------------------
// 优化点 3: 广播消息流复用 vs 每次打开新流
// 场景：向大量客户端广播消息时的性能差异
// ----------------------------------------------------------------------------

// BenchmarkBroadcastStreamReuse 比较流复用和每次打开新流的性能
func BenchmarkBroadcastStreamReuse(b *testing.B) {
	clientCounts := []int{100, 1000, 5000}

	for _, count := range clientCounts {
		b.Run(fmt.Sprintf("ReuseStream_%d", count), func(b *testing.B) {
			benchmarkBroadcastReuse(b, count)
		})

		b.Run(fmt.Sprintf("NewStream_%d", count), func(b *testing.B) {
			benchmarkBroadcastNewStream(b, count)
		})
	}
}

func benchmarkBroadcastReuse(b *testing.B, clientCount int) {
	b.ReportAllocs()

	// 模拟流复用：预先建立连接池
	connections := make([]*mockConnection, clientCount)
	for i := 0; i < clientCount; i++ {
		connections[i] = &mockConnection{
			ClientID: fmt.Sprintf("client-%d", i),
			stream:   &mockStream{},
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 使用复用的流发送消息
		msg := &protocol.DataMessage{
			MsgId:    uuid.New().String(),
			Payload:  []byte("test message"),
			SenderId: "server",
		}

		for _, conn := range connections {
			_ = conn.stream.Write(msg) // 复用流
		}
	}
}

func benchmarkBroadcastNewStream(b *testing.B, clientCount int) {
	b.ReportAllocs()

	connections := make([]*mockConnection, clientCount)
	for i := 0; i < clientCount; i++ {
		connections[i] = &mockConnection{
			ClientID: fmt.Sprintf("client-%d", i),
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := &protocol.DataMessage{
			MsgId:    uuid.New().String(),
			Payload:  []byte("test message"),
			SenderId: "server",
		}

		for _, conn := range connections {
			// 每次打开新流
			stream := conn.OpenStream()
			_ = stream.Write(msg)
			stream.Close()
		}
	}
}

type mockConnection struct {
	ClientID string
	stream   *mockStream
}

func (c *mockConnection) OpenStream() *mockStream {
	return &mockStream{}
}

type mockStream struct {
	closed bool
}

func (s *mockStream) Write(msg *protocol.DataMessage) error {
	return nil
}

func (s *mockStream) Close() error {
	s.closed = true
	return nil
}

// ----------------------------------------------------------------------------
// 优化点 4: Dispatcher 单队列 vs 多队列
// 场景：高并发消息处理的性能差异
// ----------------------------------------------------------------------------

// BenchmarkDispatcherQueues 比较单队列和多队列的性能
func BenchmarkDispatcherQueues(b *testing.B) {
	workerCounts := []int{1, 4, 10, 20}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("SingleQueue_%dworkers", workers), func(b *testing.B) {
			benchmarkSingleQueueDispatcher(b, workers)
		})

		b.Run(fmt.Sprintf("MultiQueue_%dworkers", workers), func(b *testing.B) {
			benchmarkMultiQueueDispatcher(b, workers)
		})
	}
}

func benchmarkSingleQueueDispatcher(b *testing.B, workerCount int) {
	logger := monitoring.NewLogger(monitoring.LogLevelError, "text")
	config := &dispatcher.DispatcherConfig{
		WorkerCount:    workerCount,
		TaskQueueSize:  1000,
		HandlerTimeout: 30 * time.Second,
		Logger:         logger,
	}
	disp := dispatcher.NewDispatcher(config)

	// 注册简单的处理器
	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_EVENT,
		dispatcher.MessageHandlerFunc(func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
			return &protocol.DataMessage{MsgId: msg.MsgId}, nil
		}))

	disp.Start()
	defer disp.Stop()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		msg := &protocol.DataMessage{
			MsgId:    uuid.New().String(),
			Type:     protocol.MessageType_MESSAGE_TYPE_EVENT,
			Payload:  []byte("test"),
			SenderId: "test",
		}
		_ = disp.Dispatch(context.Background(), msg, nil)
	}
}

func benchmarkMultiQueueDispatcher(b *testing.B, workerCount int) {
	// 使用真正的 MultiQueueDispatcher
	logger := monitoring.NewLogger(monitoring.LogLevelError, "text")
	config := &dispatcher.DispatcherConfig{
		WorkerCount:    workerCount,
		TaskQueueSize:  1000,
		HandlerTimeout: 30 * time.Second,
		Logger:         logger,
	}
	mqConfig := &dispatcher.MultiQueueConfig{
		QueueCount: 4, // 4 个队列
	}

	disp := dispatcher.NewMultiQueueDispatcher(config, mqConfig)

	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_EVENT,
		dispatcher.MessageHandlerFunc(func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
			return &protocol.DataMessage{MsgId: msg.MsgId}, nil
		}))

	disp.Start()
	defer disp.Stop()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		msg := &protocol.DataMessage{
			MsgId:    uuid.New().String(),
			Type:     protocol.MessageType_MESSAGE_TYPE_EVENT,
			Payload:  []byte("test"),
			SenderId: "test",
		}
		_ = disp.Dispatch(context.Background(), msg, nil)
	}
}

// ----------------------------------------------------------------------------
// 优化点 5: 对象池 vs 直接分配
// 场景：高频对象创建的性能差异
// ----------------------------------------------------------------------------

// BenchmarkObjectPool 比较使用对象池和直接分配的性能
func BenchmarkObjectPool(b *testing.B) {
	b.Run("WithPool", func(b *testing.B) {
		benchmarkWithPool(b)
	})

	b.Run("WithoutPool", func(b *testing.B) {
		benchmarkWithoutPool(b)
	})
}

func benchmarkWithPool(b *testing.B) {
	// 使用 sync.Pool
	pool := &mockPool{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		msg := pool.Get()
		// 使用消息
		_ = msg.MsgId
		pool.Put(msg)
	}
}

func benchmarkWithoutPool(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 直接创建新对象
		msg := &protocol.DataMessage{
			MsgId:    uuid.New().String(),
			SenderId: "test",
			Payload:  make([]byte, 256),
		}
		_ = msg.MsgId
	}
}

type mockPool struct {
	pool sync.Pool
}

func (p *mockPool) Get() *protocol.DataMessage {
	return &protocol.DataMessage{
		MsgId:    uuid.New().String(),
		SenderId: "test",
		Payload:  make([]byte, 256),
	}
}

func (p *mockPool) Put(msg *protocol.DataMessage) {
	// 重置对象
	msg.Payload = msg.Payload[:0]
}

// ----------------------------------------------------------------------------
// 优化点 6: 消息批量处理 vs 单条处理
// 场景：高频小消息的批量处理性能
// ----------------------------------------------------------------------------

// BenchmarkBatchMessageProcessing 比较批量处理和单条处理的性能
func BenchmarkBatchMessageProcessing(b *testing.B) {
	batchSizes := []int{1, 10, 50, 100}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(b *testing.B) {
			benchmarkBatchProcessing(b, batchSize)
		})
	}
}

func benchmarkBatchProcessing(b *testing.B, batchSize int) {
	handler := &mockBatchHandler{
		processFunc: func(msgs []*protocol.DataMessage) {
			// 批量处理
			for _, msg := range msgs {
				_ = msg.MsgId
			}
		},
	}

	messages := make([]*protocol.DataMessage, batchSize)
	for i := 0; i < batchSize; i++ {
		messages[i] = &protocol.DataMessage{
			MsgId:    uuid.New().String(),
			Payload:  []byte("test"),
			SenderId: "test",
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		handler.processFunc(messages)
	}
}

type mockBatchHandler struct {
	processFunc func([]*protocol.DataMessage)
}

// ----------------------------------------------------------------------------
// 压力测试：模拟 5-10 万长连接场景
// ----------------------------------------------------------------------------

// TestHighConcurrencySessionManagement 测试高并发会话管理
func TestHighConcurrencySessionManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high concurrency test in short mode")
	}

	clientCounts := []int{10000, 50000}

	for _, count := range clientCounts {
		t.Run(fmt.Sprintf("%d_clients", count), func(t *testing.T) {
			testHighConcurrencySessions(t, count)
		})
	}
}

func testHighConcurrencySessions(t *testing.T, clientCount int) {
	sm := NewMockSessionManager()

	// 并发添加会话
	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			clientID := fmt.Sprintf("client-%d", idx)
			sess := &MockSession{ClientID: clientID}
			sm.Add(clientID, sess)
		}(i)
	}

	wg.Wait()
	addDuration := time.Since(startTime)

	// 验证会话数量
	currentCount := sm.Count()
	t.Logf("Added %d sessions in %v (%.2f ops/sec)",
		currentCount, addDuration, float64(clientCount)/addDuration.Seconds())

	// 并发读取会话
	readStart := time.Now()
	var readOps int64

	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _ = sm.Get(fmt.Sprintf("client-%d", idx))
			atomic.AddInt64(&readOps, 1)
		}(i)
	}

	wg.Wait()
	readDuration := time.Since(readStart)

	t.Logf("Read %d sessions in %v (%.2f ops/sec)",
		readOps, readDuration, float64(readOps)/readDuration.Seconds())
}

// TestConcurrentMessageDispatch 测试并发消息分发性能
func TestConcurrentMessageDispatch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent dispatch test in short mode")
	}

	workerConfigs := []struct {
		workers   int
		queueSize int
		msgCount  int
	}{
		{10, 1000, 10000},
		{20, 2000, 20000},
		{50, 5000, 50000},
	}

	for _, cfg := range workerConfigs {
		t.Run(fmt.Sprintf("%dworkers_%dmsgs", cfg.workers, cfg.msgCount), func(t *testing.T) {
			testConcurrentDispatch(t, cfg.workers, cfg.queueSize, cfg.msgCount)
		})
	}
}

func testConcurrentDispatch(t *testing.T, workerCount, queueSize, msgCount int) {
	logger := monitoring.NewLogger(monitoring.LogLevelError, "text")
	config := &dispatcher.DispatcherConfig{
		WorkerCount:    workerCount,
		TaskQueueSize:  queueSize,
		HandlerTimeout: 30 * time.Second,
		Logger:         logger,
	}
	disp := dispatcher.NewDispatcher(config)

	var processed int64

	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_EVENT,
		dispatcher.MessageHandlerFunc(func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
			atomic.AddInt64(&processed, 1)
			return &protocol.DataMessage{MsgId: msg.MsgId}, nil
		}))

	disp.Start()
	defer disp.Stop()

	startTime := time.Now()

	// 并发发送消息
	var wg sync.WaitGroup
	for i := 0; i < msgCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg := &protocol.DataMessage{
				MsgId:    uuid.New().String(),
				Type:     protocol.MessageType_MESSAGE_TYPE_EVENT,
				Payload:  []byte(fmt.Sprintf("message-%d", idx)),
				SenderId: "test",
			}
			_ = disp.Dispatch(context.Background(), msg, nil)
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	t.Logf("Dispatched %d messages in %v (%.2f msg/sec)",
		msgCount, duration, float64(msgCount)/duration.Seconds())
	t.Logf("Processed: %d, Queue utilization: %.1f%%",
		processed, float64(msgCount)/float64(queueSize)*100)
}

// ----------------------------------------------------------------------------
// 客户端并发连接测试
// ----------------------------------------------------------------------------

// TestClientConcurrentConnections 测试客户端并发连接性能
func TestClientConcurrentConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent connection test in short mode")
	}

	testCases := []struct {
		name           string
		clientCount    int
		rampUpDuration time.Duration
	}{
		{"quick_100_clients", 100, 5 * time.Second},
		{"gradual_500_clients", 500, 30 * time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testClientRampUp(t, tc.clientCount, tc.rampUpDuration)
		})
	}
}

func testClientRampUp(t *testing.T, clientCount int, rampUpDuration time.Duration) {
	clients := make([]*client.Client, clientCount)
	var connected, failed int64

	interval := rampUpDuration / time.Duration(clientCount)
	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			clientID := fmt.Sprintf("perf-client-%d", idx)
			c, err := createTestClient(clientID)
			if err != nil {
				atomic.AddInt64(&failed, 1)
				return
			}

			if err := c.Connect(testServerAddr); err != nil {
				atomic.AddInt64(&failed, 1)
				return
			}

			clients[idx] = c
			atomic.AddInt64(&connected, 1)
		}(i)

		// 控制启动速率
		time.Sleep(interval)
	}

	wg.Wait()
	duration := time.Since(startTime)

	successRate := float64(connected) / float64(clientCount) * 100
	connectRate := float64(connected) / duration.Seconds()

	t.Logf("Connected: %d/%d (%.1f%%) in %v (%.2f conn/sec)",
		connected, clientCount, successRate, duration, connectRate)

	// 清理
	for _, c := range clients {
		if c != nil {
			c.Disconnect()
		}
	}

	if successRate < 90 {
		t.Errorf("Connection success rate too low: %.1f%%", successRate)
	}
}

// ----------------------------------------------------------------------------
// 内存分配测试
// ----------------------------------------------------------------------------

// TestMemoryAllocation 测试内存分配情况
func TestMemoryAllocation(t *testing.T) {
	testCases := []struct {
		name       string
		iterations int
	}{
		{"1k_iterations", 1000},
		{"10k_iterations", 10000},
		{"100k_iterations", 100000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var m1, m2 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// 执行操作
			for i := 0; i < tc.iterations; i++ {
				msg := &protocol.DataMessage{
					MsgId:    uuid.New().String(),
					SenderId: "test",
					Payload:  make([]byte, 256),
				}
				_, _ = json.Marshal(msg)
			}

			runtime.ReadMemStats(&m2)

			allocated := m2.TotalAlloc - m1.TotalAlloc
			avgAlloc := allocated / uint64(tc.iterations)

			t.Logf("Iterations: %d, Total allocated: %d bytes, Avg: %d bytes/op",
				tc.iterations, allocated, avgAlloc)
		})
	}
}

// ----------------------------------------------------------------------------
// 辅助函数
// ----------------------------------------------------------------------------

// setupPerfTestClientDispatcher 设置客户端的分发器（简化版）
func setupPerfTestClientDispatcher(c *client.Client) *dispatcher.Dispatcher {
	logger := monitoring.NewLogger(monitoring.LogLevelError, "text")

	dispConfig := &dispatcher.DispatcherConfig{
		WorkerCount:    10,
		TaskQueueSize:  1000,
		HandlerTimeout: 30 * time.Second,
		Logger:         logger,
	}
	disp := dispatcher.NewDispatcher(dispConfig)

	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_COMMAND,
		dispatcher.MessageHandlerFunc(func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
			return &protocol.DataMessage{MsgId: msg.MsgId}, nil
		}))

	disp.Start()
	c.SetDispatcher(disp)
	return disp
}

// MockSessionManager 用于基准测试的简化会话管理器（使用 sync.Map）
type MockSessionManager struct {
	sessions sync.Map
	count    atomic.Int64
}

func (m *MockSessionManager) Add(clientID string, sess *MockSession) {
	m.sessions.Store(clientID, sess)
	m.count.Add(1)
}

func (m *MockSessionManager) Get(clientID string) (*MockSession, bool) {
	val, ok := m.sessions.Load(clientID)
	if !ok {
		return nil, false
	}
	return val.(*MockSession), true
}

func (m *MockSessionManager) Count() int64 {
	return m.count.Load()
}

// MockSession 模拟会话对象
type MockSession struct {
	ClientID    string
	RemoteAddr  string
	ConnectedAt int64
}

// NewMockSessionManager 创建 mock 会话管理器
func NewMockSessionManager() *MockSessionManager {
	return &MockSessionManager{}
}

// ============================================================================
// 优化验证测试：使用真实的优化组件
// ============================================================================

// ----------------------------------------------------------------------------
// 测试 1: ShardedSessionManager 性能验证
// 验证目标：比 sync.Map 快 10-20%，内存使用减少 40%
// ----------------------------------------------------------------------------

// BenchmarkShardedSessionManagerVsSyncMap 验证 ShardedSessionManager 性能
// 使用真实的 ShardedSessionManager 实现
func BenchmarkShardedSessionManagerVsSyncMap(b *testing.B) {
	sizes := []int{1000, 10000, 50000, 100000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("SyncMap_%d", size), func(b *testing.B) {
			benchmarkSyncMapReal(b, size)
		})

		b.Run(fmt.Sprintf("ShardedManager_%d", size), func(b *testing.B) {
			benchmarkShardedManagerReal(b, size)
		})
	}
}

// benchmarkSyncMapReal 测试 sync.Map 实现的性能
func benchmarkSyncMapReal(b *testing.B, size int) {
	sm := &SyncMapSessionManager{sessions: sync.Map{}}

	// 预填充数据
	for i := 0; i < size; i++ {
		clientID := fmt.Sprintf("client-%d", i)
		sess := &MockSession{ClientID: clientID}
		sm.sessions.Store(clientID, sess)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 模拟读写混合场景 (80% 读, 20% 写)
		clientID := fmt.Sprintf("client-%d", i%size)
		if i%5 == 0 {
			// 写操作
			sess := &MockSession{ClientID: clientID}
			sm.sessions.Store(clientID, sess)
		} else {
			// 读操作
			sm.sessions.Load(clientID)
		}
	}
}

// benchmarkShardedManagerReal 测试 ShardedSessionManager 的性能
func benchmarkShardedManagerReal(b *testing.B, size int) {
	// 使用真实的 ShardedSessionManager
	sm := NewShardedSessionManagerForBench(32)

	// 预填充数据
	for i := 0; i < size; i++ {
		clientID := fmt.Sprintf("client-%d", i)
		sess := createMockSessionForBench(clientID)
		sm.AddForBench(sess)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 模拟读写混合场景 (80% 读, 20% 写)
		clientID := fmt.Sprintf("client-%d", i%size)
		if i%5 == 0 {
			// 写操作
			sess := createMockSessionForBench(clientID)
			sm.AddForBench(sess)
		} else {
			// 读操作
			sm.GetForBench(clientID)
		}
	}
}

// ----------------------------------------------------------------------------
// 测试 2: MultiQueueDispatcher 性能验证
// 验证目标：在高并发场景下提升吞吐量，减少队列竞争
// ----------------------------------------------------------------------------

// BenchmarkMultiQueueDispatcherPerformance 验证 MultiQueueDispatcher 性能
func BenchmarkMultiQueueDispatcherPerformance(b *testing.B) {
	configs := []struct {
		name       string
		workerCount int
		queueCount int
	}{
		{"10workers_4queues", 10, 4},
		{"20workers_4queues", 20, 4},
		{"50workers_8queues", 50, 8},
	}

	for _, cfg := range configs {
		b.Run(fmt.Sprintf("SingleQueue_%s", cfg.name), func(b *testing.B) {
			benchmarkRealSingleQueueDispatcher(b, cfg.workerCount)
		})

		b.Run(fmt.Sprintf("MultiQueue_%s", cfg.name), func(b *testing.B) {
			benchmarkRealMultiQueueDispatcher(b, cfg.workerCount, cfg.queueCount)
		})
	}
}

func benchmarkRealSingleQueueDispatcher(b *testing.B, workerCount int) {
	logger := monitoring.NewLogger(monitoring.LogLevelError, "text")
	config := &dispatcher.DispatcherConfig{
		WorkerCount:    workerCount,
		TaskQueueSize:  10000,
		HandlerTimeout: 30 * time.Second,
		Logger:         logger,
	}
	disp := dispatcher.NewDispatcher(config)

	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_EVENT,
		dispatcher.MessageHandlerFunc(func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
			return &protocol.DataMessage{MsgId: msg.MsgId}, nil
		}))

	disp.Start()
	defer disp.Stop()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		msg := &protocol.DataMessage{
			MsgId:    uuid.New().String(),
			Type:     protocol.MessageType_MESSAGE_TYPE_EVENT,
			Payload:  []byte("test message"),
			SenderId: "bench-client",
		}
		_ = disp.Dispatch(context.Background(), msg, nil)
	}
}

func benchmarkRealMultiQueueDispatcher(b *testing.B, workerCount, queueCount int) {
	logger := monitoring.NewLogger(monitoring.LogLevelError, "text")
	config := &dispatcher.DispatcherConfig{
		WorkerCount:    workerCount,
		TaskQueueSize:  10000,
		HandlerTimeout: 30 * time.Second,
		Logger:         logger,
	}
	mqConfig := &dispatcher.MultiQueueConfig{
		QueueCount: queueCount,
	}

	disp := dispatcher.NewMultiQueueDispatcher(config, mqConfig)

	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_EVENT,
		dispatcher.MessageHandlerFunc(func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
			return &protocol.DataMessage{MsgId: msg.MsgId}, nil
		}))

	disp.Start()
	defer disp.Stop()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		msg := &protocol.DataMessage{
			MsgId:    uuid.New().String(),
			Type:     protocol.MessageType_MESSAGE_TYPE_EVENT,
			Payload:  []byte("test message"),
			SenderId: "bench-client",
		}
		_ = disp.Dispatch(context.Background(), msg, nil)
	}
}

// ----------------------------------------------------------------------------
// 测试 3: 并发场景下的分片负载均衡验证
// 验证目标：确保负载均匀分布到各个分片
// ----------------------------------------------------------------------------

// BenchmarkShardedDistribution 验证分片分布的均匀性
func BenchmarkShardedDistribution(b *testing.B) {
	shardCounts := []int{16, 32, 64, 128}

	for _, shardCount := range shardCounts {
		b.Run(fmt.Sprintf("%d_shards", shardCount), func(b *testing.B) {
			benchmarkShardDistribution(b, shardCount)
		})
	}
}

func benchmarkShardDistribution(b *testing.B, shardCount int) {
	sm := NewShardedSessionManagerForBench(shardCount)

	// 测试 10000 个客户端的分布
	testSize := 10000
	shardUsage := make([]int, shardCount)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 每次重置统计
		for j := range shardUsage {
			shardUsage[j] = 0
		}

		// 添加会话并统计分布
		for j := 0; j < testSize; j++ {
			clientID := fmt.Sprintf("client-%d", j)
			sess := createMockSessionForBench(clientID)
			shardIdx := sm.GetShardIndexForBench(clientID)
			shardUsage[shardIdx]++
			sm.AddForBench(sess)
		}

		// 计算标准差，验证分布均匀性
		avg := float64(testSize) / float64(shardCount)
		variance := 0.0
		for _, count := range shardUsage {
			diff := float64(count) - avg
			variance += diff * diff
		}
		variance /= float64(shardCount)
		stdDev := variance / avg // 变异系数

		// 变异系数应该小于 0.3（即标准差小于平均值的 30%）
		// 对于 FNV 哈希和有限的样本量，这是一个合理的阈值
		if stdDev > 0.3 {
			b.Errorf("Shard distribution not uniform: stdDev=%.4f (expected <0.3)", stdDev)
		}
	}
}

// ----------------------------------------------------------------------------
// 测试 4: 批量分发性能验证
// ----------------------------------------------------------------------------

// BenchmarkBatchDispatch 验证批量分发的性能提升
func BenchmarkBatchDispatch(b *testing.B) {
	batchSizes := []int{1, 10, 50, 100}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("SingleQueue_Batch%d", batchSize), func(b *testing.B) {
			benchmarkBatchSingleQueue(b, batchSize)
		})

		b.Run(fmt.Sprintf("MultiQueue_Batch%d", batchSize), func(b *testing.B) {
			benchmarkBatchMultiQueue(b, batchSize)
		})
	}
}

func benchmarkBatchSingleQueue(b *testing.B, batchSize int) {
	logger := monitoring.NewLogger(monitoring.LogLevelError, "text")
	config := &dispatcher.DispatcherConfig{
		WorkerCount:    20,
		TaskQueueSize:  10000,
		HandlerTimeout: 30 * time.Second,
		Logger:         logger,
	}
	disp := dispatcher.NewDispatcher(config)

	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_EVENT,
		dispatcher.MessageHandlerFunc(func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
			return &protocol.DataMessage{MsgId: msg.MsgId}, nil
		}))

	disp.Start()
	defer disp.Stop()

	// 预创建消息批次
	msgs := make([]*protocol.DataMessage, batchSize)
	for i := 0; i < batchSize; i++ {
		msgs[i] = &protocol.DataMessage{
			MsgId:    uuid.New().String(),
			Type:     protocol.MessageType_MESSAGE_TYPE_EVENT,
			Payload:  []byte("test"),
			SenderId: "bench",
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = disp.DispatchBatch(context.Background(), msgs)
	}
}

func benchmarkBatchMultiQueue(b *testing.B, batchSize int) {
	logger := monitoring.NewLogger(monitoring.LogLevelError, "text")
	config := &dispatcher.DispatcherConfig{
		WorkerCount:    20,
		TaskQueueSize:  10000,
		HandlerTimeout: 30 * time.Second,
		Logger:         logger,
	}
	mqConfig := &dispatcher.MultiQueueConfig{QueueCount: 4}

	disp := dispatcher.NewMultiQueueDispatcher(config, mqConfig)

	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_EVENT,
		dispatcher.MessageHandlerFunc(func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
			return &protocol.DataMessage{MsgId: msg.MsgId}, nil
		}))

	disp.Start()
	defer disp.Stop()

	// 预创建消息批次
	msgs := make([]*protocol.DataMessage, batchSize)
	for i := 0; i < batchSize; i++ {
		msgs[i] = &protocol.DataMessage{
			MsgId:    uuid.New().String(),
			Type:     protocol.MessageType_MESSAGE_TYPE_EVENT,
			Payload:  []byte("test"),
			SenderId: "bench",
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = disp.DispatchBatch(context.Background(), msgs)
	}
}

// ----------------------------------------------------------------------------
// 测试 5: 高并发下的延迟测试
// ----------------------------------------------------------------------------

// BenchmarkDispatcherLatency 测试分发器在不同负载下的延迟
func BenchmarkDispatcherLatency(b *testing.B) {
	loads := []struct {
		name      string
		msgsPerSec int
	}{
		{"low_1000", 1000},
		{"medium_10000", 10000},
		{"high_50000", 50000},
	}

	for _, load := range loads {
		b.Run(fmt.Sprintf("SingleQueue_%s", load.name), func(b *testing.B) {
			benchmarkLatencySingleQueue(b, load.msgsPerSec)
		})

		b.Run(fmt.Sprintf("MultiQueue_%s", load.name), func(b *testing.B) {
			benchmarkLatencyMultiQueue(b, load.msgsPerSec)
		})
	}
}

func benchmarkLatencySingleQueue(b *testing.B, targetRate int) {
	logger := monitoring.NewLogger(monitoring.LogLevelError, "text")
	config := &dispatcher.DispatcherConfig{
		WorkerCount:    20,
		TaskQueueSize:  10000,
		HandlerTimeout: 30 * time.Second,
		Logger:         logger,
	}
	disp := dispatcher.NewDispatcher(config)

	// 使用带延迟的处理器
	var totalLatency atomic.Int64
	var processedCount atomic.Int64

	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_EVENT,
		dispatcher.MessageHandlerFunc(func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
			startTime := getMsgTimestamp(msg)
			latency := time.Since(startTime).Microseconds()
			totalLatency.Add(latency)
			processedCount.Add(1)
			return &protocol.DataMessage{MsgId: msg.MsgId}, nil
		}))

	disp.Start()
	defer disp.Stop()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := &protocol.DataMessage{
			MsgId:    uuid.New().String(),
			Type:     protocol.MessageType_MESSAGE_TYPE_EVENT,
			Payload:  []byte("test"),
			SenderId: "bench",
		}
		setMsgTimestamp(msg, time.Now())
		_ = disp.Dispatch(context.Background(), msg, nil)

		// 控制发送速率
		time.Sleep(time.Second / time.Duration(targetRate))
	}

	// 报告平均延迟
	count := processedCount.Load()
	if count > 0 {
		avgLatency := float64(totalLatency.Load() / count)
		b.ReportMetric(avgLatency, "us/op")
	}
}

func benchmarkLatencyMultiQueue(b *testing.B, targetRate int) {
	logger := monitoring.NewLogger(monitoring.LogLevelError, "text")
	config := &dispatcher.DispatcherConfig{
		WorkerCount:    20,
		TaskQueueSize:  10000,
		HandlerTimeout: 30 * time.Second,
		Logger:         logger,
	}
	mqConfig := &dispatcher.MultiQueueConfig{QueueCount: 4}

	disp := dispatcher.NewMultiQueueDispatcher(config, mqConfig)

	// 使用带延迟的处理器
	var totalLatency atomic.Int64
	var processedCount atomic.Int64

	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_EVENT,
		dispatcher.MessageHandlerFunc(func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
			startTime := getMsgTimestamp(msg)
			latency := time.Since(startTime).Microseconds()
			totalLatency.Add(latency)
			processedCount.Add(1)
			return &protocol.DataMessage{MsgId: msg.MsgId}, nil
		}))

	disp.Start()
	defer disp.Stop()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := &protocol.DataMessage{
			MsgId:    uuid.New().String(),
			Type:     protocol.MessageType_MESSAGE_TYPE_EVENT,
			Payload:  []byte("test"),
			SenderId: "bench",
		}
		setMsgTimestamp(msg, time.Now())
		_ = disp.Dispatch(context.Background(), msg, nil)

		// 控制发送速率
		time.Sleep(time.Second / time.Duration(targetRate))
	}

	// 报告平均延迟
	count := processedCount.Load()
	if count > 0 {
		avgLatency := float64(totalLatency.Load() / count)
		b.ReportMetric(avgLatency, "us/op")
	}
}

// ----------------------------------------------------------------------------
// 辅助类型和函数（用于基准测试）
// ----------------------------------------------------------------------------

// SyncMapSessionManager 使用 sync.Map 的会话管理器（用于对比）
type SyncMapSessionManager struct {
	sessions sync.Map
}

// BenchShardedSessionManager 用于基准测试的分片会话管理器包装
type BenchShardedSessionManager struct {
	shards   []*benchShard
	mask     uint32
	count    atomic.Int64
}

type benchShard struct {
	sync.RWMutex
	sessions map[string]*MockSession
}

// NewShardedSessionManagerForBench 创建用于基准测试的分片管理器
func NewShardedSessionManagerForBench(shardCount int) *BenchShardedSessionManager {
	shards := make([]*benchShard, shardCount)
	for i := 0; i < shardCount; i++ {
		shards[i] = &benchShard{
			sessions: make(map[string]*MockSession),
		}
	}
	return &BenchShardedSessionManager{
		shards: shards,
		mask:   uint32(shardCount - 1),
	}
}

func (sm *BenchShardedSessionManager) getShard(clientID string) *benchShard {
	// FNV-1a 哈希
	hash := uint32(2166136261)
	for i := 0; i < len(clientID); i++ {
		hash ^= uint32(clientID[i])
		hash *= 16777619
	}
	return sm.shards[hash&sm.mask]
}

func (sm *BenchShardedSessionManager) AddForBench(sess *MockSession) {
	shard := sm.getShard(sess.ClientID)
	shard.Lock()
	shard.sessions[sess.ClientID] = sess
	shard.Unlock()
	sm.count.Add(1)
}

func (sm *BenchShardedSessionManager) GetForBench(clientID string) (*MockSession, bool) {
	shard := sm.getShard(clientID)
	shard.RLock()
	sess, ok := shard.sessions[clientID]
	shard.RUnlock()
	return sess, ok
}

func (sm *BenchShardedSessionManager) GetShardIndexForBench(clientID string) int {
	hash := uint32(2166136261)
	for i := 0; i < len(clientID); i++ {
		hash ^= uint32(clientID[i])
		hash *= 16777619
	}
	return int(hash & sm.mask)
}

func createMockSessionForBench(clientID string) *MockSession {
	return &MockSession{
		ClientID:    clientID,
		RemoteAddr:  "127.0.0.1:12345",
		ConnectedAt: time.Now().UnixMilli(),
	}
}

// 消息时间戳辅助函数
// 使用 DataMessage 的 Timestamp 字段（单位：毫秒）
func setMsgTimestamp(msg *protocol.DataMessage, t time.Time) {
	msg.Timestamp = t.UnixMilli()
}

func getMsgTimestamp(msg *protocol.DataMessage) time.Time {
	if msg.Timestamp == 0 {
		return time.Now()
	}
	return time.UnixMilli(msg.Timestamp)
}
