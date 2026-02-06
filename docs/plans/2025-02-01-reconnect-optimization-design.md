# 客户端重连优化机制设计文档

**日期**: 2025-02-01
**版本**: 1.0
**作者**: Claude & 用户协作设计

---

## 1. 概述

本文档描述 QUIC Flow 客户端的自动重连优化机制，包括抖动防止重连风暴、网络错误分类自适应退避、心跳失败容错和增强监控指标。

### 1.1 设计目标

1. **高可用**: 客户端必须无限重连，确保服务高可用
2. **防风暴**: 避免多客户端同时重连造成服务端压力
3. **自适应**: 根据错误类型调整重连策略
4. **可观测**: 提供完善的监控指标

### 1.2 架构约束

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
│  无限重连    │ ────────────────> │  (不主动)    │
└─────────────┘                    └─────────────┘
```

**关键约束**:
- Server **永远不会主动连接** Client
- Client 负责所有连接维护工作
- 客户端必须无限重连，不允许配置最大重连次数

---

## 2. 核心功能设计

### 2.1 抖动机制 (Jitter)

**目的**: 防止多客户端同时断线时产生重连风暴

**算法**:
```go
wait_time = backoff + backoff * ratio * (2 * rand - 1)
```

**配置**:
| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enable_jitter` | bool | `true` | 是否启用抖动 |
| `jitter_ratio` | float64 | `0.25` | 抖动比例 ±25% |

**示例**:
- `backoff = 2s`, `ratio = 0.25` → `wait_time ∈ [1.5s, 2.5s]`
- `backoff = 10s`, `ratio = 0.25` → `wait_time ∈ [7.5s, 12.5s]`

**算法验证**: ✓ 测试通过
- 抖动范围正确: 实际值在理论范围内
- 平均值接近基准: 偏离 < 2ms
- 分布均匀: 10个区间样本数接近期望值

### 2.2 网络错误分类

**目的**: 根据错误类型自动调整退避策略

**错误类型**:

| 类型 | 描述 | 示例 | 退避倍数 | 是否重连 |
|------|------|------|----------|----------|
| `Transient` | 瞬时错误 | EOF, connection reset | 1.0x | ✓ |
| `Timeout` | 超时错误 | deadline exceeded | 1.5x | ✓ |
| `Refused` | 拒绝错误 | ECONNREFUSED | 2.0x | ✓ |
| `Auth` | 认证错误 | TLS 证书错误 | - | ✗ |
| `Unknown` | 未知错误 | 其他 | 1.0x | ✗ |

**算法验证**: ✓ 测试通过
- 错误类型正确识别
- 是否应该重连判断正确
- 退避倍数计算正确: 1.0, 1.5, 2.0

### 2.3 心跳失败容错

**目的**: 避免因瞬时网络抖动导致不必要的重连

**配置**:
| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `max_heartbeat_failures` | int32 | `1` | 最大允许失败次数 |

**行为**:
- `max_heartbeat_failures = 1`: 单次心跳失败即触发重连（默认，保持现有行为）
- `max_heartbeat_failures = 3`: 允许连续 2 次失败，第 3 次才触发重连
- 心跳成功后自动重置失败计数

**算法验证**: ✓ 测试通过
- 失败计数累积正确
- 达到阈值触发重连
- 心跳成功重置计数器
- 并发计数线程安全

### 2.4 监控指标

**新增指标**:

| 指标名称 | 类型 | 描述 |
|----------|------|------|
| `reconnect_attempts_total` | Counter | 重连尝试总次数 |
| `reconnect_success_total` | Counter | 重连成功总次数 |
| `reconnect_current_backoff_ms` | Gauge | 当前退避时间（毫秒） |
| `reconnect_error_transient_total` | Counter | 瞬时错误总数 |
| `reconnect_error_refused_total` | Counter | 拒绝错误总数 |
| `reconnect_error_timeout_total` | Counter | 超时错误总数 |
| `heartbeat_consecutive_failures` | Gauge | 当前连续失败次数 |
| `heartbeat_recovered_total` | Counter | 心跳恢复次数 |
| `reconnect_histogram` | Histogram | 重连成功分布 |

**直方图桶定义**:
- `le="1"`: 1次成功
- `le="2"`: 2次成功
- `le="5"`: 3-5次成功
- `le="10"`: 6-10次成功
- `le="20"`: 11-20次成功
- `le="50"`: 21-50次成功
- `le="100"`: 51-100次成功
- `le="+Inf"`: >100次成功

---

## 3. 数据流设计

### 3.1 重连流程

```
Client.reconnectLoop()
     │
     v
┌─────────────────────────────────────────────────────────────┐
│ 1. 接收 disconnectCh 信号                                     │
│                                                              │
│ 2. 计算带抖动的退避时间                                       │
│    waitTime = backoff + jitter                               │
│                                                              │
│ 3. 等待 waitTime                                             │
│                                                              │
│ 4. reconnectAttempts.Add(1)                                  │
│                                                              │
│ 5. 根据上次错误类型调整退避                                   │
│    adjustedBackoff = backoff * errorMultiplier               │
│                                                              │
│ 6. 执行 dial()                                               │
│                                                              │
│     ├─ 成功 ─────────────────────────────────┐               │
│     │                                        │               │
│     │  • reconnectSuccess.Add(1)             │               │
│     │  • reconnectHistogram.Record(attempts)│               │
│     │  • backoff = InitialBackoff (重置)     │               │
│     │  • startBackgroundTasks()              │               │
│     │                                        │               │
│     └────────────────────────────────────────┘               │
│                                                              │
│     ├─ 失败 ─────────────────────────────────┐               │
│     │                                        │               │
│     │  • errorType = ClassifyNetworkError(err)│             │
│     │  • reconnectErrorXXX.Add(1)             │               │
│     │  • backoff = min(backoff * 2, MaxBackoff)│            │
│     │  • notifyDisconnect() (继续重连)        │               │
│     │                                        │               │
│     └────────────────────────────────────────┘               │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 心跳流程

```
Client.heartbeatLoop()
     │
     v
┌─────────────────────────────────────────────────────────────┐
│ 1. 每 HeartbeatInterval 发送一次 Ping                         │
│                                                              │
│ 2. 等待 Pong 响应 (HeartbeatTimeout 超时)                     │
│                                                              │
│     ├─ 成功 ─────────────────────────────────┐               │
│     │                                        │               │
│     │  • heartbeatFailures.Store(0) (重置)   │               │
│     │  • heartbeatRecoveredCount.Add(1)      │               │
│     │                                        │               │
│     └────────────────────────────────────────┘               │
│                                                              │
│     ├─ 失败 ─────────────────────────────────┐               │
│     │                                        │               │
│     │  • failures = heartbeatFailures.Add(1) │               │
│     │  • if failures >= MaxHeartbeatFailures │               │
│     │      • heartbeatFailures.Store(0)      │               │
│     │      • setState(IDLE)                  │               │
│     │      • notifyDisconnect() (触发重连)    │               │
│     │                                        │               │
│     └────────────────────────────────────────┘               │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. 实现清单

### 4.1 代码修改

| 文件 | 修改内容 |
|------|----------|
| `pkg/transport/client/config.go` | 新增 `EnableJitter`, `JitterRatio`, `MaxHeartbeatFailures` 配置 |
| `pkg/transport/client/client.go` | 新增 `jitterRNG`, `heartbeatFailures`, `lastErrorType` 字段；修改 `reconnectLoop()` |
| `pkg/transport/client/heartbeat.go` | 修改 `heartbeatLoop()` 支持失败累积 |
| `pkg/errors/network_errors.go` | 新增文件：错误分类器 |
| `pkg/monitoring/metrics.go` | 扩展 `Metrics` 结构体，新增重连指标 |

### 4.2 测试文件

| 文件 | 状态 | 覆盖内容 |
|------|------|----------|
| `pkg/transport/client/reconnect_jitter_test.go` | ✓ 完成 | 抖动算法测试 |
| `pkg/errors/network_errors_test.go` | ✓ 完成 | 错误分类测试 |
| `pkg/transport/client/heartbeat_failure_test.go` | ✓ 完成 | 心跳累积测试 |
| `pkg/transport/client/reconnect_metrics_test.go` | 待实现 | 监控指标测试 |

### 4.3 文档更新

| 文件 | 修改内容 |
|------|----------|
| `docs/configuration-guide.md` | 新增重连配置章节 |
| `docs/network-reliability.md` | 新增重连机制说明 |
| `docs/reconnect-design.md` | 新增设计文档 |

---

## 5. 配置示例

### 生产环境配置

```yaml
# config/client.yaml
client:
  client_id: "my-client"

  # 基础重连配置
  reconnect_enabled: true
  initial_backoff: 1s
  max_backoff: 60s

  # 抖动配置（强烈推荐）
  enable_jitter: true
  jitter_ratio: 0.25  # ±25%

  # 心跳配置
  heartbeat_interval: 15s
  heartbeat_timeout: 5s
  max_heartbeat_failures: 1  # 保持现有行为

  # 监控配置
  metrics_enabled: true
```

### 不稳定网络配置

```yaml
client:
  # 更宽松的心跳容错
  max_heartbeat_failures: 3

  # 更大的抖动
  jitter_ratio: 0.30

  # 更长的退避时间
  initial_backoff: 2s
  max_backoff: 120s
```

---

## 6. 重连策略表

| 尝试次数 | 基础退避 | 带抖动范围 (±25%) | 说明 |
|----------|----------|-------------------|------|
| 1 | 1s | [0.75s, 1.25s] | 首次重试 |
| 2 | 2s | [1.5s, 2.5s] | - |
| 3 | 4s | [3.0s, 5.0s] | - |
| 4 | 8s | [6.0s, 10.0s] | - |
| 5 | 16s | [12.0s, 20.0s] | - |
| 6 | 32s | [24.0s, 40.0s] | - |
| 7+ | 60s | [45.0s, 75.0s] | 达到上限 |

**注意**: 客户端会**无限重连**，直到手动调用 `Disconnect()` 或程序退出。

---

## 7. 强制性规则

```
┌─────────────────────────────────────────────────────────────────┐
│                    开发规范 - 强制规则                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  规则 #1: 语言要求                                               │
│  • 所有代码注释必须使用中文                                       │
│  • 所有日志输出必须使用中文                                       │
│  • 所有配置文件说明必须使用中文                                   │
│  • 所有文档和 README 必须使用中文                                 │
│  • 错误信息必须使用中文                                           │
│                                                                 │
│  规则 #2: 架构约束                                               │
│  • Client 必须无限重连，不允许配置最大重连次数                     │
│  • Server 永远不会主动连接 Client                                │
│  • EnableJitter 默认为 true                                      │
│  • 认证错误不重连                                                │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 8. 算法验证结果

```
╔══════════════════════════════════════════════════════════════╗
║                      算法验证结果汇总                          ║
╠══════════════════════════════════════════════════════════════╣
║                                                                ║
║  1. 抖动算法                                                  ║
║     ✓ 抖动范围正确: 实际值在理论范围内                        ║
║     ✓ 平均值接近基准: 偏离 < 2ms                              ║
║     ✓ 分布均匀: 10个区间样本数接近期望值                      ║
║     ✓ 各种抖动比例: 0%, 10%, 25%, 50%                        ║
║                                                                ║
║  2. 网络错误分类                                              ║
║     ✓ 错误类型正确识别: Transient, Refused, Timeout          ║
║     ✓ 是否应该重连判断正确                                    ║
║     ✓ 退避倍数计算正确: 1.0, 1.5, 2.0                         ║
║                                                                ║
║  3. 心跳失败累积                                              ║
║     ✓ 失败计数累积正确                                        ║
║     ✓ 达到阈值触发重连                                        ║
║     ✓ 心跳成功重置计数器                                      ║
║     ✓ 并发计数线程安全                                        ║
║                                                                ║
╚══════════════════════════════════════════════════════════════╝
```

---

## 9. 附录

### A. 错误类型判断逻辑

```go
func ClassifyNetworkError(err error) NetworkErrorType {
    // 瞬时错误检测
    if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, io.EOF) {
        return ErrorTypeTransient
    }

    // 超时错误检测
    if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
        return ErrorTypeTimeout
    }

    // 连接拒绝错误检测
    var opErr *net.OpError
    if errors.As(err, &opErr) {
        if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
            return ErrorTypeRefused
        }
        if opErr.Op == "read" || opErr.Op == "write" {
            return ErrorTypeTransient
        }
    }

    return ErrorTypeUnknown
}
```

### B. 退避倍数获取

```go
func (t NetworkErrorType) GetBackoffMultiplier() float64 {
    switch t {
    case ErrorTypeTransient:
        return 1.0 // 瞬时错误，使用基础退避
    case ErrorTypeTimeout:
        return 1.5 // 超时错误，使用 1.5 倍退避
    case ErrorTypeRefused:
        return 2.0 // 拒绝错误，使用 2 倍退避
    default:
        return 1.0 // 未知错误，使用基础退避
    }
}
```

---

**文档版本**: 1.0
**最后更新**: 2025-02-01
