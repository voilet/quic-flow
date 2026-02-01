# QUIC 网络可靠性说明 (T046)

## 概述

本项目使用 QUIC 协议作为传输层，QUIC 协议本身提供了可靠的数据传输保证。本文档说明 QUIC 的可靠性特性以及我们的配置。

## QUIC 可靠传输特性

### 1. 内置可靠性

QUIC 协议在传输层提供以下可靠性保证：

- **流级别的可靠传输**：每个 QUIC 流都保证数据按序到达，不会丢失
- **自动重传机制**：丢失的数据包会自动重传，无需应用层处理
- **流量控制**：防止发送方压垮接收方，避免数据丢失
- **拥塞控制**：动态调整发送速率，适应网络条件

### 2. 弱网环境支持

QUIC 在弱网环境下的优势：

- **快速恢复**：使用改进的 TCP 拥塞控制算法（如 CUBIC、BBR）
- **连接迁移**：支持网络切换（WiFi ↔ 4G），不会断开连接
- **0-RTT 重连**：快速恢复已断开的连接
- **头部压缩**：减少带宽消耗（使用 QPACK）

### 3. 与 TCP 的对比

| 特性 | QUIC | TCP |
|------|------|-----|
| 连接建立 | 1-RTT（0-RTT 重连）| 3-RTT（握手 + TLS）|
| 队头阻塞 | 无（多路复用）| 有 |
| 连接迁移 | 支持 | 不支持 |
| 拥塞控制 | 可插拔（BBR, CUBIC）| 固定算法 |
| 丢包恢复 | 改进的重传机制 | 标准 TCP 重传 |

## 配置说明

### 服务器配置 (ServerConfig)

```go
// pkg/transport/server/config.go

type ServerConfig struct {
    // ...

    // QUIC 传输参数
    MaxIdleTimeout:     30 * time.Second,   // 空闲超时（防止连接长时间无数据）
    MaxIncomingStreams: 1000,               // 最大并发流数（防止资源耗尽）

    // 应用层心跳
    HeartbeatCheckInterval: 5 * time.Second,  // 心跳检查间隔
    HeartbeatTimeout:       45 * time.Second, // 心跳超时（3 次检查）
}

func (c *ServerConfig) BuildQUICConfig() *quic.Config {
    return &quic.Config{
        MaxIdleTimeout:                 c.MaxIdleTimeout,
        MaxIncomingStreams:             c.MaxIncomingStreams,
        MaxIncomingUniStreams:          100,

        // 可靠性相关配置
        KeepAlivePeriod:                15 * time.Second, // QUIC 层面的 Keep-Alive
        EnableDatagrams:                false,            // 禁用不可靠的 Datagram
        DisablePathMTUDiscovery:        false,            // 启用路径 MTU 发现（优化性能）
    }
}
```

### 客户端配置 (ClientConfig)

```go
// pkg/transport/client/config.go

type ClientConfig struct {
    // ...

    // QUIC 传输参数
    MaxIdleTimeout:     30 * time.Second,
    MaxIncomingStreams: 100,

    // 重连配置
    ReconnectEnabled: true,
    InitialBackoff:   1 * time.Second,  // 首次重连延迟
    MaxBackoff:       60 * time.Second, // 最大重连延迟（指数退避）

    // 心跳配置
    HeartbeatInterval: 15 * time.Second, // 发送心跳间隔
    HeartbeatTimeout:  45 * time.Second, // 等待 Pong 超时
}
```

## 可靠性验证

### 1. 正常网络条件

在正常网络下，QUIC 提供：

- **消息送达率**: 100%（流级别保证）
- **延迟**: P50 < 5ms, P99 < 50ms（局域网）
- **吞吐量**: 受限于网络带宽，通常可达到 1-10 Gbps

### 2. 弱网模拟

使用 `tc` (Linux) 或 `Network Link Conditioner` (macOS) 模拟弱网：

```bash
# 示例：模拟 100ms 延迟 + 5% 丢包
tc qdisc add dev eth0 root netem delay 100ms loss 5%
```

预期行为：

- **丢包率 5%**: QUIC 自动重传，应用层感知不到丢包
- **延迟 100ms**: 消息延迟增加，但不影响可靠性
- **带宽限制**: 拥塞控制自动调整发送速率

### 3. 连接断开与重连

测试场景：

1. **临时网络中断（< 30s）**: QUIC 连接保持，数据缓冲并在网络恢复后发送
2. **长时间中断（> 30s）**: 连接超时，客户端自动重连（指数退避）
3. **网络切换（WiFi ↔ 4G）**: QUIC 连接迁移，无需重新建立连接

## 应用层增强

除了 QUIC 的可靠性外，我们在应用层增加了额外保障：

### 1. 心跳机制

- **目的**: 及时检测"僵尸连接"（连接看似正常但实际无法通信）
- **实现**: 客户端每 15s 发送 Ping，服务器响应 Pong
- **超时处理**: 3 次超时后清理会话

### 2. Promise 机制

- **目的**: 追踪消息的确认状态
- **实现**: 发送方创建 Promise，等待接收方的 Ack
- **超时**: 默认 30s，超时后返回错误

### 3. 自动重连

- **目的**: 应对长时间网络中断
- **实现**: 指数退避重连（1s → 2s → 4s → ... → 60s）
- **状态保持**: 重连成功后恢复正常通信

#### 重连触发条件

| 触发条件 | 描述 | 处理方式 |
|----------|------|----------|
| 心跳超时 | 连续 N 次心跳失败 | 触发重连 |
| 连接断开 | 网络中断、服务器关闭 | 触发重连 |

#### 重连策略

**指数退避 + 抖动**:

```
尝试次数    基础退避    带抖动范围 (±25%)
─────────────────────────────────────
  1         1s         [0.75s, 1.25s]
  2         2s         [1.5s, 2.5s]
  3         4s         [3.0s, 5.0s]
  4         8s         [6.0s, 10.0s]
  5        16s         [12.0s, 20.0s]
  6+       60s         [45.0s, 75.0s] (达到上限)
```

**抖动机制**: 防止多客户端同时断线时产生重连风暴，在退避时间上添加随机化。

#### 错误分类与退避策略

系统根据错误类型自动调整退避时间：

| 错误类型 | 示例 | 退避倍数 | 策略 |
|----------|------|----------|------|
| 瞬时错误 | EOF, connection reset | 1.0x | 快速重试 |
| 超时错误 | timeout, canceled | 1.5x | 中等退避 |
| 拒绝错误 | ECONNREFUSED | 2.0x | 慢速重试 |
| 认证错误 | TLS 证书错误 | - | **不重连** |

#### 心跳容错机制

允许连续多次心跳失败后再触发重连：

- `MaxHeartbeatFailures = 1`: 单次心跳失败即触发重连（默认）
- `MaxHeartbeatFailures = 3`: 允许连续 2 次失败，第 3 次才触发重连
- 心跳成功后自动重置失败计数

## 最佳实践

### 1. 消息大小限制

虽然 QUIC 支持大消息，但建议：

- **单条消息 < 1MB**: 避免阻塞其他消息（队头阻塞）
- **大文件传输**: 分块发送，每块 < 256KB

### 2. 超时配置

根据网络环境调整超时：

- **局域网**: HeartbeatInterval = 10s, Timeout = 30s
- **广域网**: HeartbeatInterval = 15s, Timeout = 45s
- **移动网络**: HeartbeatInterval = 20s, Timeout = 60s

### 3. 错误处理

应用层应处理以下错误：

- `ErrClientNotConnected`: 连接已断开，等待自动重连或手动重连
- `ErrHeartbeatTimeout`: 心跳超时，连接可能已失效
- `ErrPromiseTimeout`: 消息确认超时，可能需要重发

## 监控指标

建议监控以下指标来评估网络可靠性：

- **连接成功率**: `successful_connections / total_connection_attempts`
- **消息送达率**: `ack_received / messages_sent_with_ack`
- **平均延迟**: `sum(latency) / message_count`
- **P99 延迟**: 99% 消息的延迟上限
- **重连次数**: 每小时的重连次数
- **心跳超时率**: `heartbeat_timeouts / heartbeat_sent`

## 参考资料

- [QUIC 协议规范 (RFC 9000)](https://www.rfc-editor.org/rfc/rfc9000.html)
- [quic-go 库文档](https://github.com/quic-go/quic-go)
- [QUIC 与 TCP 性能对比](https://www.chromium.org/quic/playing-with-quic)

## 总结

QUIC 协议提供了强大的可靠性保证，适合各种网络环境。本项目通过合理的配置和应用层增强，进一步提升了系统的可靠性和容错能力。

**关键点**：

1. ✅ QUIC 保证流级别的可靠传输，无需应用层重传
2. ✅ 配置合理的超时和心跳参数
3. ✅ 实现自动重连和 Promise 机制作为额外保障
4. ✅ 弱网环境下依赖 QUIC 的拥塞控制和重传机制
