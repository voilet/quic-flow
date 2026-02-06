# Client 重连机制优化任务清单

> **设计原则**: 客户端必须无限重连，确保服务高可用
>
> **优化目标**: 减少重连风暴、提升重连效率、增强网络适应能力

---

## 架构约束 ⚠️

**本系统采用客户端主动、服务端被动的架构模式**：

```
┌─────────────┐                    ┌─────────────┐
│   Client    │                    │   Server    │
│  (主动侧)    │                    │  (被动侧)    │
├─────────────┤                    ├─────────────┤
│             │ ① Dial()          │             │
│   Connect   │ ────────────────> │  Accept()   │
│             │                   │             │
│             │ ② Ping/Pong       │  心跳检测    │
│  Heartbeat  │ <═════════════════│  (只检测)    │
│             │                   │             │
│             │ ③ Reconnect       │             │
│  断线重连    │ ────────────────> │  (不主动)    │
│  (无限循环)  │                   │             │
└─────────────┘                    └─────────────┘
```

| 特性 | Client | Server |
|-----|--------|--------|
| 连接建立 | ✅ 主动 Dial | ❌ 被动 Accept |
| 心跳发送 | ✅ 主动 Ping | ✅ 响应 Pong |
| 断线检测 | ✅ 主动发现 | ✅ 心跳超时清理会话 |
| 重连行为 | ✅ 无限重连 | ❌ **不会主动连接** |
| 连接恢复 | ✅ 重新 Dial | ✅ 接受新连接 |

**关键约束**：
1. **Server 永远不会主动连接 Client** - Server 只能被动接受连接
2. **Client 负责所有连接维护** - 包括首次连接、断线检测、重连
3. **Server 只做心跳检测** - 检测到超时后清理会话，等待 Client 重连

---

## 一、添加抖动 (Jitter) 避免重连风暴

**问题**: 多客户端同时断线时会同时重连，造成服务端压力

**优先级**: 🔴 高

**实现方案**:

```go
// ClientConfig 新增配置
type ClientConfig struct {
    // ... 现有配置

    // 重连抖动配置
    EnableJitter     bool          // 是否启用抖动（默认 true）
    JitterRatio      float64       // 抖动比例（默认 0.25，即 ±25%）
}

// reconnectLoop 修改
func (c *Client) reconnectLoop() {
    backoff := c.config.InitialBackoff
    rng := rand.New(rand.NewSource(time.Now().UnixNano()))

    for {
        select {
        case <-c.disconnectCh:
            // 计算带抖动的退避时间
            waitTime := backoff
            if c.config.EnableJitter {
                jitter := time.Duration(float64(waitTime) * c.config.JitterRatio * (rng.Float64()*2 - 1))
                waitTime = waitTime + jitter
            }

            c.logger.Debug("Waiting before reconnect",
                "base_backoff", backoff,
                "with_jitter", waitTime)

            timer := time.NewTimer(waitTime)
            // ...
        }
    }
}
```

**验收标准**:
- [ ] 添加 `EnableJitter` 和 `JitterRatio` 配置项
- [ ] 修改 `reconnectLoop()` 应用抖动
- [ ] 单元测试验证抖动范围在 ±25% 内
- [ ] 压力测试：1000 个客户端同时断线，重连时间分布均匀

---

## 二、网络质量感知 - 区分错误类型

**问题**: 所有错误使用相同的退避策略，不够灵活

**优先级**: 🟡 中

**实现方案**:

```go
// pkg/errors/network_errors.go - 新增文件
package errors

import (
    "errors"
    "io"
    "net"
    "syscall"
)

// 网络错误分类
type NetworkErrorType int

const (
    // 瞬时错误 - 网络暂时不可用，应该快速重试
    ErrorTypeTransient NetworkErrorType = iota
    // 拒绝错误 - 服务端拒绝连接，应该慢速重试
    ErrorTypeRefused
    // 超时错误 - 网络超时，中等退避
    ErrorTypeTimeout
    // 认证错误 - 不应该重试
    ErrorTypeAuth
    // 未知错误
    ErrorTypeUnknown
)

// ClassifyNetworkError 分类网络错误
func ClassifyNetworkError(err error) NetworkErrorType {
    if err == nil {
        return ErrorTypeUnknown
    }

    // TLS 证书/认证错误
    if errors.Is(err, io.ErrClosedPipe) ||
       errors.Is(err, io.EOF) {
        return ErrorTypeTransient
    }

    // 网络超时
    if errors.Is(err, context.DeadlineExceeded) ||
       errors.Is(err, context.Canceled) {
        return ErrorTypeTimeout
    }

    // 连接被拒绝
    var opErr *net.OpError
    if errors.As(err, &opErr) {
        if opErr.Op == "dial" {
            if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
                return ErrorTypeRefused
            }
        }
        // DNS 错误通常是瞬时的
        if opErr.Op == "read" || opErr.Op == "write" {
            return ErrorTypeTransient
        }
    }

    return ErrorTypeUnknown
}

// reconnectLoop 修改
func (c *Client) reconnectLoop() {
    backoff := c.config.InitialBackoff
    lastErrorType := errors.ErrorTypeUnknown

    for {
        select {
        case <-c.disconnectCh:
            // 根据上次的错误类型调整初始退避
            if lastErrorType == errors.ErrorTypeRefused {
                // 连接被拒绝，从较长退避开始
                backoff = max(backoff, 5*time.Second)
            } else if lastErrorType == errors.ErrorTypeTransient {
                // 瞬时错误，使用较短退避
                backoff = c.config.InitialBackoff
            }

            // ... 重连逻辑

            if err := c.dial(); err != nil {
                lastErrorType = errors.ClassifyNetworkError(err)
                c.logger.Error("Reconnect failed",
                    "error_type", lastErrorType,
                    "error", err)

                // ... 继续重连
            }
        }
    }
}
```

**验收标准**:
- [ ] 新增 `pkg/errors/network_errors.go` 文件
- [ ] 实现错误分类逻辑
- [ ] 修改 `reconnectLoop()` 根据错误类型调整退避
- [ ] 单元测试覆盖各种错误类型

---

## 三、心跳超时累积机制

**问题**: 单次心跳失败立即触发重连，可能过于敏感

**优先级**: 🟢 低

**实现方案**:

```go
// Client 新增字段
type Client struct {
    // ... 现有字段

    // 心跳失败计数
    heartbeatFailures atomic.Int32
}

// ClientConfig 新增配置
type ClientConfig struct {
    // ... 现有配置

    // 心跳容错配置
    MaxHeartbeatFailures int32 // 最大心跳失败次数（默认 1，保持现有行为）
}

// heartbeatLoop 修改
func (c *Client) heartbeatLoop() {
    ticker := time.NewTicker(c.config.HeartbeatInterval)
    defer ticker.Stop()

    for {
        select {
        case <-c.ctx.Done():
            return
        case <-ticker.C:
            if !c.IsConnected() {
                continue
            }

            if err := c.sendHeartbeat(); err != nil {
                failures := c.heartbeatFailures.Add(1)
                c.logger.Error("Heartbeat failed",
                    "failures", failures,
                    "max", c.config.MaxHeartbeatFailures)

                // 只有超过阈值才触发重连
                if failures >= c.config.MaxHeartbeatFailures {
                    c.heartbeatFailures.Store(0)
                    c.setState(protocol.ClientState_CLIENT_STATE_IDLE)
                    c.notifyDisconnect()
                }
            } else {
                // 心跳成功，重置计数
                c.heartbeatFailures.Store(0)
            }
        }
    }
}
```

**验收标准**:
- [ ] 添加 `MaxHeartbeatFailures` 配置项（默认 1）
- [ ] 添加 `heartbeatFailures` 计数器
- [ ] 修改心跳逻辑支持累积失败
- [ ] 单元测试验证累积机制

---

## 四、监控指标增强

**优先级**: 🟡 中

**新增指标**:

```go
// pkg/monitoring/metrics.go - 扩展
type Metrics struct {
    // ... 现有指标

    // 重连指标
    reconnectAttempts      atomic.Int64
    reconnectSuccessAfter  atomic.Int64  // 重连成功（按尝试次数分布）
    reconnectCurrentBackoff atomic.Int64  // 当前退避时间（毫秒）

    // 心跳容错指标
    heartbeatConsecutiveFailures atomic.Int64
    heartbeatRecoveredCount     atomic.Int64
}

// 重连成功分布（直方图）
type ReconnectHistogram struct {
    buckets [10]atomic.Int64  // 1次, 2次, 3-5次, 6-10次, 11-20次, 21-50次, 51-100次, >100次
}

func (h *ReconnectHistogram) Record(attempts int32) {
    switch {
    case attempts == 1:
        h.buckets[0].Add(1)
    case attempts == 2:
        h.buckets[1].Add(1)
    case attempts <= 5:
        h.buckets[2].Add(1)
    case attempts <= 10:
        h.buckets[3].Add(1)
    case attempts <= 20:
        h.buckets[4].Add(1)
    case attempts <= 50:
        h.buckets[5].Add(1)
    case attempts <= 100:
        h.buckets[6].Add(1)
    default:
        h.buckets[7].Add(1)
    }
}
```

**验收标准**:
- [ ] 添加重连相关指标
- [ ] 添加心跳容错指标
- [ ] 添加重连成功分布直方图
- [ ] 暴露 Prometheus 指标接口

---

## 五、单元测试

**优先级**: 🔴 高

### 5.1 抖动测试

```go
// tests/client/reconnect_jitter_test.go
func TestReconnectJitter(t *testing.T) {
    config := client.NewDefaultClientConfig("test")
    config.EnableJitter = true
    config.JitterRatio = 0.25

    c, _ := client.NewClient(config)

    // 模拟 100 次重连，验证退避时间分布
    var backoffs []time.Duration
    for i := 0; i < 100; i++ {
        backoff := c.calculateBackoff(2 * time.Second)
        backoffs = append(backoffs, backoff)
    }

    // 验证范围在 [1.5s, 2.5s] 之间
    min, max := minMax(backoffs)
    assert.True(t, min >= 1500*time.Millisecond)
    assert.True(t, max <= 2500*time.Millisecond)
}
```

### 5.2 错误分类测试

```go
// tests/errors/network_error_classification_test.go
func TestClassifyNetworkError(t *testing.T) {
    tests := []struct {
        name     string
        err      error
        expected errors.NetworkErrorType
    }{
        {"timeout", context.DeadlineExceeded, errors.ErrorTypeTimeout},
        {"refused", &net.OpError{Err: syscall.ECONNREFUSED}, errors.ErrorTypeRefused},
        {"transient", io.ErrClosedPipe, errors.ErrorTypeTransient},
        // ...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := errors.ClassifyNetworkError(tt.err)
            assert.Equal(t, tt.expected, got)
        })
    }
}
```

### 5.3 心跳累积测试

```go
// tests/client/heartbeat_failure_test.go
func TestHeartbeatConsecutiveFailures(t *testing.T) {
    config := client.NewDefaultClientConfig("test")
    config.MaxHeartbeatFailures = 3

    c, _ := client.NewClient(config)

    // 模拟心跳失败
    for i := 0; i < 2; i++ {
        c.simulateHeartbeatFailure()
        assert.Equal(t, int32(i+1), c.GetHeartbeatFailures())
        assert.False(t, c.IsReconnecting())  // 不应触发重连
    }

    // 第 3 次失败触发重连
    c.simulateHeartbeatFailure()
    assert.True(t, c.IsReconnecting())
    assert.Equal(t, int32(0), c.GetHeartbeatFailures())  // 重置计数
}
```

**验收标准**:
- [ ] 抖动测试通过
- [ ] 错误分类测试通过
- [ ] 心跳累积测试通过
- [ ] 覆盖率 > 80%

---

## 六、集成测试

**优先级**: 🟡 中

### 6.1 大规模重连测试

```bash
# tests/integration/reconnect_storm_test.sh
#!/bin/bash

# 启动服务器
./server &
SERVER_PID=$!

# 启动 1000 个客户端
for i in {1..1000}; do
    ./client --id="client-$i" &
done

# 等待 10 秒
sleep 10

# 杀掉服务器（模拟崩溃）
kill -9 $SERVER_PID

# 重启服务器
./server &
SERVER_PID=$!

# 监控重连分布（应该在 1-60 秒内均匀分布）
# 验证服务器没有因重连风暴而崩溃

# 清理
kill -9 $SERVER_PID
pkill -9 client
```

**验收标准**:
- [ ] 服务器正常重启
- [ ] 所有客户端最终重连成功
- [ ] 重连时间分布符合预期（带抖动）
- [ ] 服务器 CPU/内存没有峰值

---

## 七、文档更新

**优先级**: 🟢 低

- [ ] 更新 `docs/network-reliability.md` 添加重连优化说明
- [ ] 更新 `docs/configuration-guide.md` 添加新配置项文档
- [ ] 添加重连机制设计文档 `docs/reconnect-design.md`
- [ ] 更新 README.md 添加重连最佳实践

---

## 八、任务优先级总结

| 任务 | 优先级 | 预计工时 | 依赖 |
|-----|-------|---------|-----|
| 添加抖动 (Jitter) | 🔴 高 | 4h | 无 |
| 单元测试 | 🔴 高 | 6h | 添加抖动、错误分类 |
| 网络质量感知 | 🟡 中 | 6h | 错误分类 |
| 监控指标增强 | 🟡 中 | 3h | 无 |
| 集成测试 | 🟡 中 | 4h | 所有功能 |
| 心跳超时累积 | 🟢 低 | 3h | 无 |
| 文档更新 | 🟢 低 | 2h | 所有功能 |

**总预计工时**: ~28 小时

---

## 九、实施计划

### Phase 1: 核心优化 (Week 1)
1. 添加抖动机制
2. 网络错误分类
3. 单元测试

### Phase 2: 增强功能 (Week 2)
1. 心跳累积机制
2. 监控指标
3. 集成测试

### Phase 3: 文档与收尾 (Week 3)
1. 文档更新
2. 性能验证
3. Code Review

---

## 十、不实施项

| 项目 | 原因 |
|-----|------|
| 最大重连次数限制 | **业务要求**: 客户端必须无限重连，确保服务高可用 |
| 服务端主动通知重连 | **架构约束**: Server 不会主动连接 Client，只能被动接受 |
| 退避时间上限降低 | 当前 60s 上限合理，过短会增加服务端压力 |
| 自适应退避算法 | 过于复杂，当前指数退避已足够 |
| 双向心跳检测 | **架构约束**: Server 只做被动检测，Client 负责主动维护连接 |

---

## 附录：配置示例

```yaml
# config/client.yaml
client:
  client_id: "my-client"

  # 基础重连配置
  reconnect_enabled: true
  initial_backoff: 1s
  max_backoff: 60s

  # 抖动配置（新增）
  enable_jitter: true
  jitter_ratio: 0.25  # ±25%

  # 心跳配置
  heartbeat_interval: 15s
  heartbeat_timeout: 5s
  max_heartbeat_failures: 1  # 新增：累积失败次数阈值

  # 监控配置
  metrics_enabled: true
  hooks:
    on_reconnect: "log_reconnect_event"
```
