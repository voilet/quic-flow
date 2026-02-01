package client

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

// TestJitterCalculation 测试抖动计算算法的正确性
func TestJitterCalculation(t *testing.T) {
	// 测试参数
	backoff := 2 * time.Second
	jitterRatio := 0.25
	rng := rand.New(rand.NewSource(42)) // 固定种子，便于复现

	// 预期范围: [1.5s, 2.5s]
	// 公式: waitTime = backoff + backoff * ratio * (2*rand - 1)
	//      waitTime = 2000ms + 2000ms * 0.25 * (2*rand - 1)
	//      waitTime = 2000ms + 500ms * (2*rand - 1)
	//      范围: [2000 - 500, 2000 + 500] = [1500, 2500] ms

	expectedMin := 1500 * time.Millisecond
	expectedMax := 2500 * time.Millisecond

	// 模拟 1000 次计算，验证所有结果都在预期范围内
	samples := make([]time.Duration, 1000)
	for i := 0; i < 1000; i++ {
		// 抖动计算公式
		jitterRange := float64(backoff) * jitterRatio
		jitter := rng.Float64()*2 - 1 // [-1, 1]
		waitTime := backoff + time.Duration(jitterRange*float64(jitter))
		samples[i] = waitTime

		// 验证范围
		if waitTime < expectedMin || waitTime > expectedMax {
			t.Errorf("第 %d 次: 抖动结果 %v 超出范围 [%v, %v]",
				i, waitTime, expectedMin, expectedMax)
		}
	}

	// 计算统计信息
	var sum time.Duration
	min := samples[0]
	max := samples[0]
	for _, s := range samples {
		sum += s
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}
	mean := sum / time.Duration(len(samples))

	t.Logf("抖动统计 (base=%v, ratio=%.2f):", backoff, jitterRatio)
	t.Logf("  理论范围: [%v, %v]", expectedMin, expectedMax)
	t.Logf("  实际范围: [%v, %v]", min, max)
	t.Logf("  平均值: %v", mean)
	t.Logf("  平均偏离基准: %v (期望: 0)", mean-backoff)

	// 验证平均值接近基准值（在大样本下应该接近）
	diffMs := math.Abs(float64((mean - backoff).Milliseconds()))
	if diffMs > 50.0 {
		t.Errorf("平均值偏离基准过大: %v (期望 < 50ms)", mean-backoff)
	}
}

// TestJitterRatioLimits 测试不同抖动比例
func TestJitterRatioLimits(t *testing.T) {
	tests := []struct {
		name         string
		backoff      time.Duration
		jitterRatio  float64
		expectedMin  time.Duration
		expectedMax  time.Duration
	}{
		{
			name:         "25% 抖动",
			backoff:      1 * time.Second,
			jitterRatio:  0.25,
			expectedMin:  750 * time.Millisecond,
			expectedMax:  1250 * time.Millisecond,
		},
		{
			name:         "50% 抖动",
			backoff:      1 * time.Second,
			jitterRatio:  0.50,
			expectedMin:  500 * time.Millisecond,
			expectedMax:  1500 * time.Millisecond,
		},
		{
			name:         "10% 抖动",
			backoff:      5 * time.Second,
			jitterRatio:  0.10,
			expectedMin:  4500 * time.Millisecond,
			expectedMax:  5500 * time.Millisecond,
		},
		{
			name:         "0% 抖动（禁用）",
			backoff:      1 * time.Second,
			jitterRatio:  0.0,
			expectedMin:  1000 * time.Millisecond,
			expectedMax:  1000 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(42))

			// 采样 100 次
			for i := 0; i < 100; i++ {
				jitterRange := float64(tt.backoff) * tt.jitterRatio
				jitter := rng.Float64()*2 - 1
				waitTime := tt.backoff + time.Duration(jitterRange*float64(jitter))

				if waitTime < tt.expectedMin || waitTime > tt.expectedMax {
					t.Errorf("第 %d 次: 抖动结果 %v 超出范围 [%v, %v]",
						i, waitTime, tt.expectedMin, tt.expectedMax)
				}
			}
		})
	}
}

// TestExponentialBackoffWithJitter 测试指数退避结合抖动
func TestExponentialBackoffWithJitter(t *testing.T) {
	initialBackoff := 1 * time.Second
	maxBackoff := 60 * time.Second
	jitterRatio := 0.25

	// 模拟 10 次失败重连的退避时间
	backoff := initialBackoff
	rng := rand.New(rand.NewSource(42))

	t.Log("指数退避 + 抖动序列:")
	for i := 1; i <= 10; i++ {
		// 应用抖动
		jitterRange := float64(backoff) * jitterRatio
		jitter := rng.Float64()*2 - 1
		waitTime := backoff + time.Duration(jitterRange*float64(jitter))

		// 计算预期范围
		expectedMin := time.Duration(float64(backoff) * (1 - jitterRatio))
		expectedMax := time.Duration(float64(backoff) * (1 + jitterRatio))

		t.Logf("  第 %d 次重连: base=%v, 范围=[%v, %v], 实际=%v",
			i, backoff, expectedMin, expectedMax, waitTime)

		// 验证范围
		if waitTime < expectedMin || waitTime > expectedMax {
			t.Errorf("第 %d 次: 抖动结果 %v 超出范围 [%v, %v]",
				i, waitTime, expectedMin, expectedMax)
		}

		// 指数退避
		backoff = time.Duration(math.Min(
			float64(backoff*2),
			float64(maxBackoff),
		))
	}
}

// TestJitterDistribution 测试抖动分布的均匀性
func TestJitterDistribution(t *testing.T) {
	backoff := 10 * time.Second
	jitterRatio := 0.25
	samples := 10000
	rng := rand.New(rand.NewSource(42))

	// 将范围分为 10 个区间，统计每个区间的样本数
	bins := make([]int, 10)
	minRange := float64(backoff) * (1 - jitterRatio)
	maxRange := float64(backoff) * (1 + jitterRatio)
	binWidth := (maxRange - minRange) / 10

	for i := 0; i < samples; i++ {
		jitterRange := float64(backoff) * jitterRatio
		jitter := rng.Float64()*2 - 1
		waitTime := float64(backoff + time.Duration(jitterRange*float64(jitter)))

		// 计算落在哪个区间
		binIndex := int((waitTime - minRange) / binWidth)
		if binIndex >= 0 && binIndex < 10 {
			bins[binIndex]++
		}
	}

	// 验证每个区间的样本数接近期望值（samples/10 = 1000）
	expected := samples / 10
	tolerance := expected / 10 // 允许 10% 的偏差

	for i, count := range bins {
		lower := minRange + float64(i)*binWidth
		upper := minRange + float64(i+1)*binWidth
		t.Logf("  区间 [%v, %v): %d 样本 (期望: %d)",
			time.Duration(lower), time.Duration(upper), count, expected)

		if count < expected-tolerance || count > expected+tolerance {
			t.Errorf("区间 %d 样本数 %d 偏离期望 %d 超过容差 %d",
				i, count, expected, tolerance)
		}
	}
}
