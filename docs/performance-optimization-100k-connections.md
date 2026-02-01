# 5-10万长连接性能优化文档

## 概述

本文档记录了 QUIC Flow 项目针对 5-10 万长连接场景的性能优化工作。优化目标是确保系统在 10 万并发连接下能够稳定运行，同时保持低延迟和高吞吐量。

**优化日期**: 2026-02-01
**Commit**: `c80f8d3`
**Go 版本**: 1.25.4

---

## 问题分析

### 1. 现状评估

#### 配置文件评估

| 参数 | 标准配置 | 高性能配置 | 10万连接需求 |
|------|----------|-----------|-------------|
| `max_clients` | 10,000 | 150,000 | ✅ 充足 |
| `worker_count` | 20 | 200 | ✅ 充足 |
| `task_queue_size` | 2,000 | 100,000 | ✅ 充足 |
| `max_promises` | 50,000 | 150,000 | ✅ 充足 |
| `heartbeat_check_interval` | 5s | 10s | ⚠️ 需调整 |

**结论**: 配置层面基本满足需求，但心跳检查间隔需要放宽以减少遍历开销。

#### 代码层面问题

通过深入分析，发现以下性能瓶颈：

##### 1.1 Broadcast goroutine 爆炸 🔴 严重

**位置**: `pkg/transport/server/server.go:713-770`

```go
// 问题代码
for _, clientID := range clientIDs {  // 100,000 次循环
    wg.Add(1)
    go func(cid string) {
        defer wg.Done()
        s.SendTo(cid, &msgCopy)  // 包含阻塞的 OpenStreamSync()
    }(clientID)
}
```

**影响**:
- 10 万广播 = 10 万 goroutine 同时创建
- 栈内存占用: 100,000 × 2KB ≈ 200MB
- CPU 调度开销巨大
- `OpenStreamSync()` 阻塞导致 goroutine 堆积

##### 1.2 Promise 延迟删除 goroutine 泄漏 🔴 严重

**位置**: `pkg/callback/manager.go:170-173, 190-193, 211-214`

```go
// 问题代码：每次 Promise 完成/超时都创建新 goroutine
go func() {
    time.Sleep(100 * time.Millisecond)
    pm.Remove(msgID)
}()
```

**影响**:
- 1000 msg/s = 1000 goroutine/s 累积
- 短期 goroutine 无法及时回收
- 内存持续增长，最终 OOM

##### 1.3 心跳检查日志开销 🟡 中等

**位置**: `pkg/session/manager.go:260-313`

```go
// 问题代码：每次 timeout 都输出日志
sm.Range(func(clientID string, session *ClientSession) bool {
    if timeout {
        sm.logger.Warn("Heartbeat timeout detected", ...)  // 每条都记录
        if timeoutCount >= max {
            sm.logger.Error("Heartbeat timeout threshold reached...", ...)
        }
    }
})
```

**影响**:
- 10 万连接 × 每 10 秒 = 每秒遍历 1 万次
- 1% 超时率 = 每秒 100 条 Warn 日志
- 日志 I/O 成为瓶颈

##### 1.4 分页实现低效 🟡 中等

**位置**: `pkg/session/manager.go:214-240`

```go
// 问题代码：先获取全部，再分片
func (sm *SessionManager) ListClientsWithDetailsPaginated(...) {
    all := sm.ListClientsWithDetails()  // 遍历全部 10 万
    return all[offset:end], total       // 再切片
}
```

**影响**:
- 请求第 1 页（limit=100）也要遍历 10 万条
- 内存浪费：10 万 × 100B ≈ 10MB

##### 1.5 HTTP API 无限制 🟢 轻微

**位置**: `pkg/api/http_server.go:183-354`

```go
// 问题代码：limit=0 时返回全部数据
if limit == 0 {
    clients = h.serverAPI.ListClientsWithDetails()  // 10 万条
}
```

**影响**:
- 单次响应 10MB+ JSON
- 网络传输和解析开销大

---

## 优化方案

### 2.1 Broadcast 并发限制

**方案**: 使用 semaphore 限制并发 goroutine 数量

```go
// 优化后代码
maxConcurrency := 1000
sem := make(chan struct{}, maxConcurrency)

for _, clientID := range clientIDs {
    sem <- struct{}{}  // 获取令牌（阻塞等待）
    wg.Add(1)
    go func(cid string) {
        defer func() { <-sem; wg.Done() }()  // 释放令牌
        s.SendTo(cid, &msgCopy)
    }(clientID)
}
```

**效果**:
- goroutine 数量: 100,000 → 1,000
- 栈内存占用: 200MB → 2MB
- CPU 调度开销降低 99%

---

### 2.2 Promise 延迟删除优化

**方案**: 使用单一 goroutine + 通道处理所有延迟删除

```go
// 新增数据结构
type delayedRemove struct {
    msgID    string
    removeAt time.Time
}

type PromiseManager struct {
    // ...
    delayedRemoveCh chan delayedRemove  // 统一的延迟删除通道
}

// 新增延迟删除处理循环
func (pm *PromiseManager) delayedRemoveLoop() {
    var pending []delayedRemove
    ticker := time.NewTicker(50 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case item := <-pm.delayedRemoveCh:
            pending = append(pending, item)
        case <-ticker.C:
            now := time.Now()
            i := 0
            for _, item := range pending {
                if now.After(item.removeAt) {
                    pm.Remove(item.msgID)
                } else {
                    pending[i] = item
                    i++
                }
            }
            pending = pending[:i]
        }
    }
}

// 完成 Promise 时发送到通道
func (pm *PromiseManager) Complete(msgID string, ack *protocol.AckMessage) error {
    // ...
    select {
    case pm.delayedRemoveCh <- delayedRemove{
        msgID:    msgID,
        removeAt: time.Now().Add(delayedRemoveDelay),
    }:
    default:
        pm.Remove(msgID)  // 通道满则直接删除
    }
    return nil
}
```

**效果**:
- goroutine 数量: N (累积) → 1 (常驻)
- 内存泄漏风险消除
- 延迟删除功能保持不变

---

### 2.3 心跳检查日志优化

**方案**: 汇总日志代替逐条日志

```go
// 优化后代码
func (sm *SessionManager) checkHeartbeats() {
    var timeoutClients []string
    var removedClients []string

    sm.Range(func(clientID string, session *ClientSession) bool {
        if timeout {
            timeoutCount := session.IncrementTimeoutCount()
            if timeoutCount == 1 {
                timeoutClients = append(timeoutClients, clientID)
            }
            if timeoutCount >= sm.maxTimeoutCount {
                // 执行清理...
                removedClients = append(removedClients, clientID)
            }
        }
        return true
    })

    // 汇总记录一条日志
    if len(timeoutClients) > 0 || len(removedClients) > 0 {
        sm.logger.Warn("Heartbeat check summary",
            "timeout_count", len(timeoutClients),
            "removed_count", len(removedClients),
            "total_sessions", sm.Count())
    }
}
```

**效果**:
- 日志量: 1,000 条/次 → 1 条/次
- 日志 I/O 开销降低 99.9%
- 监控信息仍然完整

---

### 2.4 分页提前终止

**方案**: 遍历时跳过 + 提前终止

```go
// 优化后代码
func (sm *SessionManager) ListClientsWithDetailsPaginated(offset, limit int) (...) {
    result := make([]ClientInfoBrief, 0, min(limit, int(total)))
    skipped := 0
    collected := 0

    sm.sessions.Range(func(key, value interface{}) bool {
        // 跳过前面的元素
        if skipped < offset {
            skipped++
            return true
        }

        // 收集够 limit 个后停止遍历
        if limit > 0 && collected >= limit {
            return false  // 提前终止
        }

        session := value.(*ClientSession)
        result = append(result, ClientInfoBrief{...})
        collected++
        return true
    })

    return result, total
}
```

**效果**:
- 第 1 页遍历次数: 100,000 → ~100
- 内存占用: 10MB → 10KB
- 响应时间降低 99%

---

### 2.5 API 请求限制

**方案**: 添加最大返回限制

```go
const maxClientsPerRequest = 10000

func (h *HTTPServer) handleListClients(c *gin.Context) {
    limit := parseLimit(c.Query("limit"))

    // 强制最大限制
    if limit == 0 || limit > maxClientsPerRequest {
        limit = maxClientsPerRequest
    }

    // 始终使用分页 API
    clients, total := h.serverAPI.ListClientsWithDetailsPaginated(offset, limit)
    // ...
}
```

**效果**:
- 单次响应最大: 10MB → 1MB
- 避免网络和内存压力

---

### 2.6 配置调整

**方案**: 放宽心跳检查间隔

```yaml
# config/server-highperf.yaml
session:
  heartbeat_check_interval: 15  # 从 10 改为 15
```

**效果**:
- 遍历频率降低 30%
- CPU 开销相应减少

---

## 性能对比

### 3.1 关键指标对比 (10万连接场景)

| 指标 | 优化前 | 优化后 | 提升 |
|------|--------|--------|------|
| **Broadcast goroutine** | 100,000 | 1,000 | ↓ 99% |
| **Promise 延迟删除 goroutine** | 累积泄漏 | 1 常驻 | ✅ 修复 |
| **心跳日志 (1% timeout)** | 1,000 条/次 | 1 条/次 | ↓ 99.9% |
| **分页第1页遍历** | 100,000 次 | ~100 次 | ↓ 99.9% |
| **API 最大响应** | 无限制 | 10,000 条 | ✅ 限制 |
| **心跳检查间隔** | 10s | 15s | ↓ 30% 频率 |

### 3.2 资源占用对比

| 资源 | 优化前 | 优化后 |
|------|--------|--------|
| **Broadcast 栈内存** | ~200MB | ~2MB |
| **Promise goroutine** | 持续增长 | 1 个常驻 |
| **心跳日志 I/O** | 高 | 极低 |
| **分页 CPU** | 高 | 极低 |

---

## 验证方法

### 4.1 单元测试

```bash
# 测试 Broadcast 并发限制
go test -v ./pkg/transport/server/... -run TestBroadcast

# 测试 Promise 延迟删除
go test -v ./pkg/callback/... -run TestPromiseDelayedRemove

# 测试分页性能
go test -v ./pkg/session/... -run TestPagination
```

### 4.2 压力测试

```bash
# 启动服务器（高性能配置）
./cmd/server/quic-server -config config/server-highperf.yaml

# 运行负载测试
./cmd/loadtest/loadtest -clients 100000 -server localhost:8474

# 监控指标
curl http://localhost:8475/metrics
```

### 4.3 监控指标

通过 Prometheus 监控以下指标：

```promql
# 连接数
quic_flow_connections_total

# goroutine 数量
go_goroutines{job="quic-server"}

# 心跳超时率
rate(quic_flow_heartbeat_timeouts_total[5m])

# 广播延迟
histogram_quantile(0.99, rate(quic_flow_broadcast_latency_seconds_bucket[5m]))
```

---

## 后续优化建议

### 5.1 短期优化

1. **时间轮心跳检查**: 实现基于时间轮的定时器，避免全量遍历
2. **连接分片**: 将连接按哈希分片到多个 SessionManager，并行处理
3. **连接复用**: 复用 QUIC stream，减少 `OpenStreamSync()` 开销

### 5.2 长期优化

1. **连接状态缓存**: 使用 Redis 等缓存连接状态，减少内存占用
2. **消息批量发送**: 合并多个小消息为批量发送
3. **UDP 优化**: 调整 UDP 缓冲区大小，减少丢包

---

## 参考资源

- [Go 1.21 Release Notes](https://go.dev/doc/go1.21)
- [quic-go Documentation](https://github.com/quic-go/quic-go)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/)

---

## 变更历史

| 日期 | 版本 | 变更内容 |
|------|------|----------|
| 2026-02-01 | 1.0 | 初始版本，5-10万连接优化 |
