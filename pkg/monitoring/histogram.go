package monitoring

import (
	"sync/atomic"
)

// Histogram 轻量级延迟统计直方图
// 使用固定桶（buckets）统计延迟分布
type Histogram struct {
	// 延迟桶（毫秒）
	// bucket[0]: 0-10ms
	// bucket[1]: 10-50ms
	// bucket[2]: 50-100ms
	// bucket[3]: 100-200ms
	// bucket[4]: 200ms+
	buckets [5]atomic.Int64

	count atomic.Int64 // 总样本数
	sum   atomic.Int64 // 总延迟（微秒，为了更高精度）
}

// NewHistogram 创建新的 Histogram 实例
func NewHistogram() *Histogram {
	return &Histogram{}
}

// Observe 记录一个延迟样本（毫秒）
func (h *Histogram) Observe(latencyMs int64) {
	h.count.Add(1)
	h.sum.Add(latencyMs * 1000) // 转为微秒存储

	// 更新对应的桶
	switch {
	case latencyMs < 10:
		h.buckets[0].Add(1)
	case latencyMs < 50:
		h.buckets[1].Add(1)
	case latencyMs < 100:
		h.buckets[2].Add(1)
	case latencyMs < 200:
		h.buckets[3].Add(1)
	default:
		h.buckets[4].Add(1)
	}
}

// Mean 返回平均延迟（毫秒）
func (h *Histogram) Mean() int64 {
	count := h.count.Load()
	if count == 0 {
		return 0
	}

	sum := h.sum.Load()
	return (sum / count) / 1000 // 转回毫秒
}

// Count 返回样本总数
func (h *Histogram) Count() int64 {
	return h.count.Load()
}

// Sum 返回总延迟（毫秒）
func (h *Histogram) Sum() int64 {
	return h.sum.Load() / 1000 // 转回毫秒
}

// Percentile 计算指定百分位的延迟（毫秒）
// p: 百分位值（0.0-1.0），例如 0.99 表示 P99
//
// 简化实现：基于桶边界估算百分位
// 实际生产环境可以使用更精确的算法（如 t-digest）
func (h *Histogram) Percentile(p float64) int64 {
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}

	totalCount := h.count.Load()
	if totalCount == 0 {
		return 0
	}

	// 计算目标样本位置
	targetCount := int64(float64(totalCount) * p)

	// 累计计数，找到目标所在的桶
	var cumulativeCount int64
	for i := 0; i < len(h.buckets); i++ {
		cumulativeCount += h.buckets[i].Load()
		if cumulativeCount >= targetCount {
			// 返回桶的上边界作为估算值
			return h.getBucketUpperBound(i)
		}
	}

	// 如果没有找到（理论上不应该发生），返回最大桶的上边界
	return h.getBucketUpperBound(len(h.buckets) - 1)
}

// getBucketUpperBound 获取桶的上边界值（毫秒）
func (h *Histogram) getBucketUpperBound(bucketIndex int) int64 {
	switch bucketIndex {
	case 0:
		return 10 // 0-10ms
	case 1:
		return 50 // 10-50ms
	case 2:
		return 100 // 50-100ms
	case 3:
		return 200 // 100-200ms
	case 4:
		return 500 // 200ms+ （估算为 500ms）
	default:
		return 1000 // 默认值
	}
}

// GetBucketCounts 获取所有桶的计数（用于调试或可视化）
func (h *Histogram) GetBucketCounts() [5]int64 {
	var counts [5]int64
	for i := 0; i < len(h.buckets); i++ {
		counts[i] = h.buckets[i].Load()
	}
	return counts
}

// Reset 重置直方图（清空所有数据）
func (h *Histogram) Reset() {
	h.count.Store(0)
	h.sum.Store(0)
	for i := 0; i < len(h.buckets); i++ {
		h.buckets[i].Store(0)
	}
}

// Snapshot 获取当前直方图的快照
type HistogramSnapshot struct {
	Count        int64    // 样本总数
	Sum          int64    // 总延迟（毫秒）
	Mean         int64    // 平均延迟（毫秒）
	P50          int64    // P50 延迟（毫秒）
	P95          int64    // P95 延迟（毫秒）
	P99          int64    // P99 延迟（毫秒）
	BucketCounts [5]int64 // 各桶计数
}

// GetSnapshot 获取直方图快照
func (h *Histogram) GetSnapshot() *HistogramSnapshot {
	return &HistogramSnapshot{
		Count:        h.Count(),
		Sum:          h.Sum(),
		Mean:         h.Mean(),
		P50:          h.Percentile(0.50),
		P95:          h.Percentile(0.95),
		P99:          h.Percentile(0.99),
		BucketCounts: h.GetBucketCounts(),
	}
}
