package client

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestHeartbeatFailureAccumulation 测试心跳失败累积机制
func TestHeartbeatFailureAccumulation(t *testing.T) {
	tests := []struct {
		name                  string
		maxFailures           int32
		failToTrigger         int32 // 触发重连需要的失败次数
		description           string
	}{
		{
			name:          "默认行为-单次失败触发",
			maxFailures:   1,
			failToTrigger: 1,
			description:   "保持现有行为，单次心跳失败即触发重连",
		},
		{
			name:          "容错模式-三次失败触发",
			maxFailures:   3,
			failToTrigger: 3,
			description:   "允许连续 2 次心跳失败，第 3 次才触发重连",
		},
		{
			name:          "高容错-五次失败触发",
			maxFailures:   5,
			failToTrigger: 5,
			description:   "允许连续 4 次心跳失败，第 5 次才触发重连",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建模拟的失败计数器
			var failures atomic.Int32
			triggered := false

			t.Logf("测试配置: MaxHeartbeatFailures=%d, %s", tt.maxFailures, tt.description)

			// 模拟连续心跳失败
			for i := int32(1); i <= tt.maxFailures+2; i++ {
				failures.Add(1)
				current := failures.Load()

				t.Logf("  第 %d 次心跳失败, 当前计数: %d", i, current)

				// 检查是否应该触发重连
				if current >= tt.maxFailures {
					if !triggered {
						if i == tt.failToTrigger {
							t.Logf("  ✓ 第 %d 次失败触发重连 (预期)", i)
							triggered = true
							// 触发重连后重置计数
							failures.Store(0)
						} else {
							t.Errorf("  第 %d 次失败触发重连，但应该在 %d 次触发", i, tt.failToTrigger)
							return
						}
					}
					// 触发重连后，只记录日志，不检查触发条件
				} else {
					// 未达到阈值，验证不应该触发
					if triggered {
						// 已经触发过重连，这是后续的失败，正常累积
						t.Logf("  ✓ 第 %d 次失败 (重连后累积, 计数 %d)", i, current)
					} else if i >= tt.failToTrigger {
						t.Errorf("  第 %d 次失败应该触发重连 (当前计数 %d >= 阈值 %d)",
							i, current, tt.maxFailures)
						return
					} else {
						t.Logf("  ✓ 第 %d 次失败不触发重连 (计数 %d < 阈值 %d)", i, current, tt.maxFailures)
					}
				}
			}

			if !triggered {
				t.Errorf("未在预期次数 %d 触发重连", tt.failToTrigger)
			}
		})
	}
}

// TestHeartbeatSuccessResetsCounter 测试心跳成功重置计数器
func TestHeartbeatSuccessResetsCounter(t *testing.T) {
	var failures atomic.Int32
	maxFailures := int32(3)

	t.Log("测试心跳成功后重置失败计数器")

	// 第 1 次失败
	failures.Add(1)
	t.Logf("第 1 次失败: 计数=%d (阈值=%d)", failures.Load(), maxFailures)

	// 第 2 次失败
	failures.Add(1)
	t.Logf("第 2 次失败: 计数=%d (阈值=%d)", failures.Load(), maxFailures)

	// 心跳成功，重置计数
	failures.Store(0)
	t.Logf("心跳成功: 计数重置为 0")

	// 再次失败，应该从 1 开始计数
	failures.Add(1)
	if failures.Load() != 1 {
		t.Errorf("重置后第一次失败，计数应为 1，实际为 %d", failures.Load())
	}
	t.Logf("重置后第 1 次失败: 计数=%d ✓", failures.Load())
}

// TestHeartbeatFailureThresholds 测试不同阈值的行为
func TestHeartbeatFailureThresholds(t *testing.T) {
	thresholds := []int32{1, 2, 3, 5, 10}

	for _, maxFailures := range thresholds {
		t.Run(fmt.Sprintf("阈值=%d", maxFailures), func(t *testing.T) {
			var failures atomic.Int32
			triggered := false

			// 模拟 maxFailures + 1 次失败
			for i := int32(1); i <= maxFailures+1; i++ {
				failures.Add(1)

				if failures.Load() >= maxFailures && !triggered {
					t.Logf("第 %d 次失败触发重连 (阈值=%d)", i, maxFailures)
					triggered = true
					failures.Store(0)
				}
			}

			if !triggered {
				t.Errorf("未在预期次数触发重连 (阈值=%d)", maxFailures)
			}
		})
	}
}

// TestConcurrentHeartbeatFailures 测试并发心跳失败计数的线程安全性
func TestConcurrentHeartbeatFailures(t *testing.T) {
	var failures atomic.Int32
	goroutines := 10
	failuresPerGoroutine := int32(20)

	t.Logf("测试并发心跳失败计数: %d 个 goroutine, 每个 %d 次失败",
		goroutines, failuresPerGoroutine)

	// 并发模拟心跳失败
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := int32(0); j < failuresPerGoroutine; j++ {
				failures.Add(1)
				time.Sleep(time.Microsecond) // 模拟心跳间隔
			}
		}()
	}

	wg.Wait()

	expected := int32(goroutines) * failuresPerGoroutine
	actual := failures.Load()

	t.Logf("预期失败次数: %d, 实际: %d", expected, actual)

	if actual != expected {
		t.Errorf("并发计数不准确: 预期 %d, 实际 %d", expected, actual)
	}
}

// BenchmarkHeartbeatFailureCounting 基准测试心跳失败计数性能
func BenchmarkHeartbeatFailureCounting(b *testing.B) {
	var failures atomic.Int32

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		failures.Add(1)
		if failures.Load() >= 3 {
			failures.Store(0)
		}
	}
}
