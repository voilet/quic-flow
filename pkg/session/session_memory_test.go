package session

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestClientSessionMemoryLayout 测试内存布局
func TestClientSessionMemoryLayout(t *testing.T) {
	// 验证优化后的字段类型
	session := &ClientSession{}

	// connectedAt 应该是 int64
	var _ int64 = session.connectedAt

	// lastHeartbeat 应该是 atomic.Int64
	_ = session.lastHeartbeat.Load()

	// metadata 应该初始为 nil（懒加载）
	assert.Nil(t, session.metadata, "metadata should be nil initially")

	// cachedRemoteAddr 应该初始为空
	assert.Empty(t, session.cachedRemoteAddr, "cachedRemoteAddr should be empty initially")
}

// TestClientSessionGetRemoteAddr 测试按需获取远程地址
func TestClientSessionGetRemoteAddr(t *testing.T) {
	// 注意：这个测试需要模拟 quic.Conn，在真实环境中使用 mock
	session := &ClientSession{
		ClientID:         "test-client",
		cachedRemoteAddr: "192.168.1.100:12345",
	}

	// 当连接为 nil 时，应该返回缓存的地址
	addr := session.GetRemoteAddr()
	assert.Equal(t, "192.168.1.100:12345", addr)
}

// TestLazyMetadataInitialization 测试元数据懒加载
func TestLazyMetadataInitialization(t *testing.T) {
	session := &ClientSession{
		ClientID: "test-client",
	}

	// 初始状态：metadata 为 nil
	assert.Nil(t, session.metadata)

	// 读取不存在的 key
	val, ok := session.GetMetadata("test-key")
	assert.False(t, ok)
	assert.Nil(t, val)

	// metadata 仍然应该是 nil（没有创建）
	assert.Nil(t, session.metadata)

	// 设置第一个值
	session.SetMetadata("key1", "value1")

	// 现在应该创建了 map
	assert.NotNil(t, session.metadata)

	// 验证值
	val, ok = session.GetMetadata("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)

	// 设置更多值
	session.SetMetadata("key2", 123)
	session.SetMetadata("key3", true)

	// 验证所有值
	val, ok = session.GetMetadata("key2")
	assert.True(t, ok)
	assert.Equal(t, 123, val)

	val, ok = session.GetMetadata("key3")
	assert.True(t, ok)
	assert.Equal(t, true, val)
}

// TestClientSessionInt64Timestamps 测试 int64 时间戳
func TestClientSessionInt64Timestamps(t *testing.T) {
	nowMs := time.Now().UnixMilli()
	session := &ClientSession{
		ClientID:    "test-client",
		connectedAt: nowMs,
	}
	session.lastHeartbeat.Store(nowMs)

	// 验证 connectedAt 是 int64
	assert.Equal(t, nowMs, session.connectedAt)

	// 验证 GetLastHeartbeat 返回正确的时间
	lastHB := session.GetLastHeartbeat()
	assert.InDelta(t, nowMs, lastHB.UnixMilli(), 1)

	// 验证 GetUptime 正确计算
	uptime := session.GetUptime()
	assert.True(t, uptime >= 0 && uptime < 100*time.Millisecond)

	// 验证 UpdateLastHeartbeat
	time.Sleep(10 * time.Millisecond)
	session.UpdateLastHeartbeat()

	newLastHB := session.GetLastHeartbeat()
	assert.True(t, newLastHB.After(lastHB) || newLastHB.Equal(lastHB))
}

// TestClientSessionConcurrentMetadata 并发测试
func TestClientSessionConcurrentMetadata(t *testing.T) {
	session := &ClientSession{
		ClientID: "test-client",
	}

	const goroutines = 100
	const operationsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2) // 读 + 写

	// 并发写入
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				key := "key"
				session.SetMetadata(key, idx)
			}
		}(i)
	}

	// 并发读取
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				session.GetMetadata("key")
			}
		}()
	}

	wg.Wait()

	// 验证没有 panic 且数据一致
	val, ok := session.GetMetadata("key")
	assert.True(t, ok)
	assert.NotNil(t, val)
}

// BenchmarkClientSessionMemoryAllocation 内存分配基准测试
func BenchmarkClientSessionMemoryAllocation(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = &ClientSession{
			ClientID:    "test-client",
			connectedAt: time.Now().UnixMilli(),
		}
	}
}

// BenchmarkInt64Timestamps int64 时间戳性能测试
func BenchmarkInt64Timestamps(b *testing.B) {
	session := &ClientSession{
		ClientID:    "test-client",
		connectedAt: time.Now().UnixMilli(),
	}
	session.lastHeartbeat.Store(time.Now().UnixMilli())

	b.Run("GetLastHeartbeat", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = session.GetLastHeartbeat()
		}
	})

	b.Run("UpdateLastHeartbeat", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			session.UpdateLastHeartbeat()
		}
	})

	b.Run("GetUptime", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = session.GetUptime()
		}
	})
}

// TestMemoryEstimation 内存估算测试
func TestMemoryEstimation(t *testing.T) {
	// 估算单个会话的内存占用
	// 基础结构体: ~70B
	// ClientID (平均20字符): ~36B
	// quic.Conn 指针: 8B
	// 总计: ~114B/会话

	// 10W 会话: ~11.4MB
	// 对比优化前: ~18.4MB
	// 节省: ~7MB

	const sessions = 100000
	const estimatedSize = 114 // bytes
	const totalEstimated = sessions * estimatedSize

	t.Logf("Estimated memory for %d sessions: %d bytes (%.2f MB)",
		sessions, totalEstimated, float64(totalEstimated)/(1024*1024))

	t.Logf("Compared to original (~18.4MB): savings of ~%.2f MB",
		18.4-float64(totalEstimated)/(1024*1024))
}
