package session

import (
	"sync"
	"testing"
	"time"
)

// BenchmarkShardedSessionManager_Add 分片管理器添加会话基准测试
func BenchmarkShardedSessionManager_Add(b *testing.B) {
	sm := NewShardedSessionManager(ShardedManagerConfig{
		ShardCount:         32,
		HeartbeatTimeout:   45 * time.Second,
		MaxTimeoutCount:    3,
		Logger:             nil,
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		session := &ClientSession{
			ClientID:    string(rune('a'+(i%26))) + "-client",
			connectedAt: time.Now().UnixMilli(),
		}
		session.lastHeartbeat.Store(time.Now().UnixMilli())
		sm.Add(session)
	}
}

// BenchmarkShardedSessionManager_Get 分片管理器获取会话基准测试
func BenchmarkShardedSessionManager_Get(b *testing.B) {
	sm := NewShardedSessionManager(ShardedManagerConfig{
		ShardCount:         32,
		HeartbeatTimeout:   45 * time.Second,
		MaxTimeoutCount:    3,
		Logger:             nil,
	})

	// 预填充 10000 个会话
	for i := 0; i < 10000; i++ {
		session := &ClientSession{
			ClientID:    string(rune('a'+(i%26))) + "-client-" + string(rune('0'+(i%10))),
			connectedAt: time.Now().UnixMilli(),
		}
		session.lastHeartbeat.Store(time.Now().UnixMilli())
		sm.Add(session)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		clientID := string(rune('a'+(i%26))) + "-client-" + string(rune('0'+(i%10)))
		sm.Get(clientID)
	}
}

// BenchmarkShardedSessionManager_Remove 分片管理器移除会话基准测试
func BenchmarkShardedSessionManager_Remove(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		sm := NewShardedSessionManager(ShardedManagerConfig{
			ShardCount:       32,
			HeartbeatTimeout: 45 * time.Second,
			MaxTimeoutCount:  3,
			Logger:           nil,
		})

		// 添加 1000 个会话
		sessions := make([]*ClientSession, 1000)
		for j := 0; j < 1000; j++ {
			sessions[j] = &ClientSession{
				ClientID:    string(rune('a'+(j%26))) + "-client-" + string(rune('0'+(j%10))),
				connectedAt: time.Now().UnixMilli(),
			}
			sessions[j].lastHeartbeat.Store(time.Now().UnixMilli())
			sm.Add(sessions[j])
		}
		b.StartTimer()

		// 移除所有会话
		for j := 0; j < 1000; j++ {
			sm.Remove(sessions[j].ClientID)
		}
	}
}

// BenchmarkShardedSessionManager_ListPaginated 分页列表基准测试
func BenchmarkShardedSessionManager_ListPaginated(b *testing.B) {
	sm := NewShardedSessionManager(ShardedManagerConfig{
		ShardCount:       32,
		HeartbeatTimeout: 45 * time.Second,
		MaxTimeoutCount:  3,
		Logger:           nil,
	})

	// 预填充 100000 个会话
	for i := 0; i < 100000; i++ {
		session := &ClientSession{
			ClientID:    string(rune('a'+(i%26))) + "-client-" + string(rune('0'+(i%10))) + "-" + string(rune('0'+(i%100))),
			connectedAt: time.Now().UnixMilli(),
		}
		session.lastHeartbeat.Store(time.Now().UnixMilli())
		sm.Add(session)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sm.ListClientsWithDetailsPaginated(0, 100)
	}
}

// BenchmarkClientSessionMemory 内存分配基准测试
func BenchmarkClientSessionMemory(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = &ClientSession{
			ClientID: "test-client",
		}
	}
}

// BenchmarkInt64VsAtomicValueTimestamps int64 vs atomic.Value 时间戳性能对比
func BenchmarkInt64VsAtomicValueTimestamps(b *testing.B) {
	// 测试 int64 时间戳（优化后）
	b.Run("Int64", func(b *testing.B) {
		session := &ClientSession{
			connectedAt: time.Now().UnixMilli(),
		}
		session.lastHeartbeat.Store(time.Now().UnixMilli())

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = session.GetLastHeartbeat()
			session.UpdateLastHeartbeat()
		}
	})
}

// BenchmarkLazyMetadata 懒加载元数据性能测试
func BenchmarkLazyMetadata(b *testing.B) {
	session := &ClientSession{
		ClientID: "test-client",
	}

	b.Run("GetBeforeInit", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			session.GetMetadata("key")
		}
	})

	b.Run("GetAfterInit", func(b *testing.B) {
		session.SetMetadata("key", "value")
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			session.GetMetadata("key")
		}
	})

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			session.SetMetadata("key", i)
		}
	})
}

// BenchmarkTimeWheelOperations 时间轮操作基准测试
func BenchmarkTimeWheelOperations(b *testing.B) {
	manager := newMockManager()
	tw := NewTimeWheelHeartbeatChecker(manager, TimeWheelConfig{
		SlotSize:         60,
		TickInterval:     1 * time.Second,
		HeartbeatTimeout: 45 * time.Second,
		MaxTimeoutCount:  3,
		Logger:           nil,
	})

	b.Run("Register", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			clientID := "client-" + string(rune('0'+(i%10)))
			tw.Register(clientID)
		}
	})

	b.Run("UpdateHeartbeat", func(b *testing.B) {
		clientID := "test-client"
		tw.Register(clientID)
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			tw.UpdateHeartbeat(clientID)
		}
	})

	b.Run("Unregister", func(b *testing.B) {
		b.StopTimer()
		clientIDs := make([]string, 1000)
		for i := 0; i < 1000; i++ {
			clientIDs[i] = "client-" + string(rune('0'+(i%10))) + "-" + string(rune('0'+(i%100)))
			tw.Register(clientIDs[i])
		}
		b.StartTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			idx := i % 1000
			tw.Unregister(clientIDs[idx])
			tw.Register(clientIDs[idx])
		}
	})
}

// BenchmarkMemoryComparison 内存占用对比测试
func BenchmarkMemoryComparison(b *testing.B) {
	const sessionCount = 100000

	b.Run("Optimized", func(b *testing.B) {
		b.ReportAllocs()
		sessions := make([]ClientSession, sessionCount)
		for i := 0; i < sessionCount; i++ {
			sessions[i] = ClientSession{
				ClientID: "test-client",
			}
			sessions[i].lastHeartbeat.Store(time.Now().UnixMilli())
		}
	})

	b.Run("WithMetadata", func(b *testing.B) {
		b.ReportAllocs()
		sessions := make([]ClientSession, sessionCount)
		for i := 0; i < sessionCount; i++ {
			sessions[i] = ClientSession{
				ClientID: "test-client",
			}
			sessions[i].SetMetadata("key", "value") // 触发 map 创建
		}
	})
}

// BenchmarkConcurrentAccess 并发访问基准测试
func BenchmarkConcurrentAccess(b *testing.B) {
	sm := NewShardedSessionManager(ShardedManagerConfig{
		ShardCount:       32,
		HeartbeatTimeout: 45 * time.Second,
		MaxTimeoutCount:  3,
		Logger:           nil,
	})

	// 预填充会话
	for i := 0; i < 10000; i++ {
		session := &ClientSession{
			ClientID:    "client-" + string(rune('0'+(i%10))) + "-" + string(rune('0'+(i%100))),
			connectedAt: time.Now().UnixMilli(),
		}
		session.lastHeartbeat.Store(time.Now().UnixMilli())
		sm.Add(session)
	}

	b.Run("ConcurrentGet", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				clientID := "client-" + string(rune('0'+(i%10))) + "-" + string(rune('0'+(i%100)))
				sm.Get(clientID)
				i++
			}
		})
	})

	b.Run("ConcurrentAddRemove", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				clientID := "temp-client-" + string(rune('0'+(i%100)))
				session := &ClientSession{
					ClientID: clientID,
				}
				session.lastHeartbeat.Store(time.Now().UnixMilli())
				session.connectedAt = time.Now().UnixMilli()
				sm.Add(session)
				sm.Remove(clientID)
				i++
			}
		})
	})
}

// BenchmarkShardSelection 分片选择性能测试
func BenchmarkShardSelection(b *testing.B) {
	sm := NewShardedSessionManager(ShardedManagerConfig{
		ShardCount:       32,
		HeartbeatTimeout: 45 * time.Second,
		MaxTimeoutCount:  3,
		Logger:           nil,
	})

	clientIDs := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		clientIDs[i] = "client-" + string(rune('a'+(i%26))) + "-" + string(rune('0'+(i%100)))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sm.getShard(clientIDs[i%10000])
	}
}

// BenchmarkSyncMapVsShard sync.Map vs 分片 map 性能对比
func BenchmarkSyncMapVsShard(b *testing.B) {
	const sessionCount = 10000

	// sync.Map 实现
	var syncMap sync.Map
	sessions := make([]ClientSession, sessionCount)
	for i := 0; i < sessionCount; i++ {
		sessions[i] = ClientSession{
			ClientID: "sync-client-" + string(rune('0'+(i%100))),
		}
		syncMap.Store(sessions[i].ClientID, &sessions[i])
	}

	// 分片实现
	sm := NewShardedSessionManager(ShardedManagerConfig{
		ShardCount:       32,
		HeartbeatTimeout: 45 * time.Second,
		MaxTimeoutCount:  3,
		Logger:           nil,
	})

	for i := 0; i < sessionCount; i++ {
		session := &ClientSession{
			ClientID: "shard-client-" + string(rune('0'+(i%100))),
		}
		sm.Add(session)
	}

	b.Run("SyncMap_Get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			clientID := "sync-client-" + string(rune('0'+(i%100)))
			syncMap.Load(clientID)
		}
	})

	b.Run("Shard_Get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			clientID := "shard-client-" + string(rune('0'+(i%100)))
			sm.Get(clientID)
		}
	})
}
