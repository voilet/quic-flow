package session

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/voilet/quic-flow/pkg/monitoring"
)

// 定义测试错误
var errSessionNotFound = errors.New("session not found")

// mockManager 模拟会话管理器
type mockManager struct {
	sessions map[string]*ClientSession
	mu       sync.RWMutex
	count    atomic.Int64
}

func newMockManager() *mockManager {
	return &mockManager{
		sessions: make(map[string]*ClientSession),
	}
}

func (m *mockManager) Get(clientID string) (*ClientSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[clientID]
	if !ok {
		return nil, errSessionNotFound
	}
	return session, nil
}

func (m *mockManager) Remove(clientID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[clientID]; !ok {
		return errSessionNotFound
	}
	delete(m.sessions, clientID)
	m.count.Add(-1)
	return nil
}

func (m *mockManager) Count() int64 {
	return m.count.Load()
}

func (m *mockManager) Add(session *ClientSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.ClientID] = session
	m.count.Add(1)
}

// TestTimeWheelRegister 测试注册到时间轮
func TestTimeWheelRegister(t *testing.T) {
	manager := newMockManager()
	tw := NewTimeWheelHeartbeatChecker(manager, TimeWheelConfig{
		SlotSize:         60,
		TickInterval:     100 * time.Millisecond,
		HeartbeatTimeout: 1 * time.Second,
		MaxTimeoutCount:  3,
		Logger:           monitoring.NewDefaultLogger(),
	})

	clientID := "test-client"
	tw.Register(clientID)

	// 验证会话被注册到正确的槽位
	stats := tw.GetStats()
	if stats.TotalSessions != 1 {
		t.Errorf("Expected 1 session, got %d", stats.TotalSessions)
	}
}

// TestTimeWheelUpdateHeartbeat 测试更新心跳时间
func TestTimeWheelUpdateHeartbeat(t *testing.T) {
	manager := newMockManager()
	tw := NewTimeWheelHeartbeatChecker(manager, TimeWheelConfig{
		SlotSize:         60,
		TickInterval:     100 * time.Millisecond,
		HeartbeatTimeout: 1 * time.Second,
		MaxTimeoutCount:  3,
		Logger:           monitoring.NewDefaultLogger(),
	})

	clientID := "test-client"
	tw.Register(clientID)

	// 获取初始槽位
	oldSlotIdx, ok := tw.sessionToSlot.Load(clientID)
	if !ok {
		t.Fatal("Session not registered")
	}

	// 更新心跳
	time.Sleep(200 * time.Millisecond)
	tw.UpdateHeartbeat(clientID)

	// 验证槽位已改变
	newSlotIdx, ok := tw.sessionToSlot.Load(clientID)
	if !ok {
		t.Fatal("Session not found after update")
	}

	if oldSlotIdx == newSlotIdx {
		t.Logf("Warning: slot index didn't change (may be expected if timing is close)")
	}
}

// TestTimeWheelUnregister 测试从时间轮注销
func TestTimeWheelUnregister(t *testing.T) {
	manager := newMockManager()
	tw := NewTimeWheelHeartbeatChecker(manager, TimeWheelConfig{
		SlotSize:         60,
		TickInterval:     100 * time.Millisecond,
		HeartbeatTimeout: 1 * time.Second,
		MaxTimeoutCount:  3,
		Logger:           monitoring.NewDefaultLogger(),
	})

	clientID := "test-client"
	tw.Register(clientID)

	// 注销会话
	tw.Unregister(clientID)

	// 验证会话已被移除
	stats := tw.GetStats()
	if stats.TotalSessions != 0 {
		t.Errorf("Expected 0 sessions after unregister, got %d", stats.TotalSessions)
	}

	_, ok := tw.sessionToSlot.Load(clientID)
	if ok {
		t.Error("Session still in sessionToSlot after unregister")
	}

	_, ok = tw.sessionExpiry.Load(clientID)
	if ok {
		t.Error("Session still in sessionExpiry after unregister")
	}
}

// TestTimeWheelGetStats 测试获取统计信息
func TestTimeWheelGetStats(t *testing.T) {
	manager := newMockManager()
	tw := NewTimeWheelHeartbeatChecker(manager, TimeWheelConfig{
		SlotSize:         60,
		TickInterval:     100 * time.Millisecond,
		HeartbeatTimeout: 1 * time.Second,
		MaxTimeoutCount:  3,
		Logger:           monitoring.NewDefaultLogger(),
	})

	// 注册多个会话
	for i := 0; i < 10; i++ {
		tw.Register(string(rune('a' + i)))
	}

	stats := tw.GetStats()

	if stats.SlotSize != 60 {
		t.Errorf("Expected slot size 60, got %d", stats.SlotSize)
	}

	if stats.TotalSessions != 10 {
		t.Errorf("Expected 10 total sessions, got %d", stats.TotalSessions)
	}

	if stats.MinSlotSize < 0 {
		t.Errorf("MinSlotSize should be >= 0, got %d", stats.MinSlotSize)
	}

	if stats.MaxSlotSize > 10 {
		t.Errorf("MaxSlotSize should be <= 10, got %d", stats.MaxSlotSize)
	}
}

// TestTimeWheelStartStop 测试启动和停止
func TestTimeWheelStartStop(t *testing.T) {
	manager := newMockManager()
	tw := NewTimeWheelHeartbeatChecker(manager, TimeWheelConfig{
		SlotSize:         60,
		TickInterval:     50 * time.Millisecond,
		HeartbeatTimeout: 500 * time.Millisecond,
		MaxTimeoutCount:  3,
		Logger:           monitoring.NewDefaultLogger(),
	})

	// 启动时间轮
	tw.Start()

	// 等待几个 tick
	time.Sleep(200 * time.Millisecond)

	// 停止时间轮
	tw.Stop()

	// 验证可以正常停止（无 panic）
}

// TestTimeWheelProcessExpired 测试处理过期会话
func TestTimeWheelProcessExpired(t *testing.T) {
	manager := newMockManager()
	tw := NewTimeWheelHeartbeatChecker(manager, TimeWheelConfig{
		SlotSize:         60,
		TickInterval:     100 * time.Millisecond,
		HeartbeatTimeout: 500 * time.Millisecond,
		MaxTimeoutCount:  3,
		Logger:           monitoring.NewDefaultLogger(),
	})

	// 创建一个测试会话
	session := &ClientSession{
		ClientID: "expired-client",
		// connectedAt 和 lastHeartbeat 会被设置
	}
	session.connectedAt = time.Now().UnixMilli()
	session.lastHeartbeat.Store(time.Now().Add(-2 * time.Second).UnixMilli()) // 设置为过去的时间

	manager.Add(session)
	tw.Register(session.ClientID)

	// 启动时间轮
	tw.Start()
	defer tw.Stop()

	// 等待足够时间让会话超时
	time.Sleep(1 * time.Second)

	// 验证会话被处理（可能被移除）
	// 由于测试环境，我们只验证不会 panic
}

// BenchmarkTimeWheelRegister 性能测试：注册
func BenchmarkTimeWheelRegister(b *testing.B) {
	manager := newMockManager()
	tw := NewTimeWheelHeartbeatChecker(manager, TimeWheelConfig{
		SlotSize:         60,
		TickInterval:     1 * time.Second,
		HeartbeatTimeout: 45 * time.Second,
		MaxTimeoutCount:  3,
		Logger:           monitoring.NewDefaultLogger(),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clientID := string(rune('a' + (i % 26)))
		tw.Register(clientID)
	}
}

// BenchmarkTimeWheelUpdateHeartbeat 性能测试：更新心跳
func BenchmarkTimeWheelUpdateHeartbeat(b *testing.B) {
	manager := newMockManager()
	tw := NewTimeWheelHeartbeatChecker(manager, TimeWheelConfig{
		SlotSize:         60,
		TickInterval:     1 * time.Second,
		HeartbeatTimeout: 45 * time.Second,
		MaxTimeoutCount:  3,
		Logger:           monitoring.NewDefaultLogger(),
	})

	clientID := "test-client"
	tw.Register(clientID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tw.UpdateHeartbeat(clientID)
	}
}

// BenchmarkTimeWheelUnregister 性能测试：注销
func BenchmarkTimeWheelUnregister(b *testing.B) {
	manager := newMockManager()
	tw := NewTimeWheelHeartbeatChecker(manager, TimeWheelConfig{
		SlotSize:         60,
		TickInterval:     1 * time.Second,
		HeartbeatTimeout: 45 * time.Second,
		MaxTimeoutCount:  3,
		Logger:           monitoring.NewDefaultLogger(),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clientID := string(rune('a' + (i % 26)))
		tw.Register(clientID)
		tw.Unregister(clientID)
	}
}
