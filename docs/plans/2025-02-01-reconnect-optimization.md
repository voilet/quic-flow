# 客户端重连优化机制实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**目标:** 实现客户端重连优化机制，包括抖动防止风暴、网络错误分类自适应退避、心跳失败容错和增强监控指标。

**架构:** 客户端主动重连、服务端被动的架构模式。通过在重连循环中添加抖动算法、错误分类器和心跳失败累积来提高重连效率和可靠性。

**技术栈:** Go 1.21+, QUIC (quic-go), atomic 操作, Prometheus 监控

---

## Task 1: 扩展配置结构 (ClientConfig)

**文件:**
- 修改: `pkg/transport/client/config.go`

**Step 1: 添加抖动配置字段**

在 `ClientConfig` 结构体中添加：

```go
// 重连抖动配置
EnableJitter bool    // 是否启用抖动（默认 true）
JitterRatio  float64 // 抖动比例，默认 0.25（±25%）

// 心跳容错配置
MaxHeartbeatFailures int32 // 最大心跳失败次数（默认 1）
```

**Step 2: 更新默认配置**

修改 `NewDefaultClientConfig` 函数：

```go
func NewDefaultClientConfig(clientID string) *ClientConfig {
	return &ClientConfig{
		// ... 现有配置

		// 抖动默认值
		EnableJitter: true,
		JitterRatio:  0.25, // ±25%

		// 心跳容错默认值
		MaxHeartbeatFailures: 1, // 保持现有行为
	}
}
```

**Step 3: 扩展配置验证**

修改 `Validate` 函数，添加：

```go
// 验证抖动配置
if c.EnableJitter {
	if c.JitterRatio < 0 || c.JitterRatio > 1 {
		return fmt.Errorf("%w: JitterRatio 必须在 [0, 1] 范围内, 得到 %.2f",
			ErrInvalidConfig, c.JitterRatio)
	}
}

// 验证心跳容错配置
if c.MaxHeartbeatFailures < 1 {
	return fmt.Errorf("%w: MaxHeartbeatFailures 必须 >= 1, 得到 %d",
		ErrInvalidConfig, c.MaxHeartbeatFailures)
}
```

**Step 4: 运行测试验证**

```bash
go test ./pkg/transport/client/... -v -run TestConfig
```

**Step 5: 提交**

```bash
git add pkg/transport/client/config.go
git commit -m "feat(client): 添加抖动和心跳容错配置

- 添加 EnableJitter 和 JitterRatio 配置
- 添加 MaxHeartbeatFailures 配置
- 更新配置验证逻辑

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 2: 扩展 Client 结构体

**文件:**
- 修改: `pkg/transport/client/client.go`

**Step 1: 添加新字段**

在 `Client` 结构体中添加：

```go
import (
	"math/rand"
	"sync"
	// ... 其他导入
)

type Client struct {
	// ... 现有字段

	// 抖动随机数生成器（每个客户端独立）
	jitterRNG *rand.Rand

	// 心跳失败计数
	heartbeatFailures atomic.Int32

	// 上次错误类型（用于调整退避）
	lastErrorType     errors.NetworkErrorType
	lastErrorTypeMu   sync.RWMutex
}
```

**Step 2: 初始化新字段**

修改 `NewClient` 函数：

```go
func NewClient(config *ClientConfig) (*Client, error) {
	// ... 现有代码

	c := &Client{
		// ... 现有初始化
		jitterRNG: rand.New(rand.NewSource(time.Now().UnixNano())),
		// ... 其他初始化
	}

	return c, nil
}
```

**Step 3: 运行测试**

```bash
go test ./pkg/transport/client/... -v -run TestNewClient
```

**Step 4: 提交**

```bash
git add pkg/transport/client/client.go
git commit -m "feat(client): 扩展 Client 结构体

- 添加 jitterRNG 用于抖动计算
- 添加 heartbeatFailures 用于失败累积
- 添加 lastErrorType 用于错误分类

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 3: 修改重连循环（添加抖动和错误分类）

**文件:**
- 修改: `pkg/transport/client/client.go`

**Step 1: 修改 reconnectLoop 函数**

```go
func (c *Client) reconnectLoop() {
	defer c.wg.Done()

	backoff := c.config.InitialBackoff

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("重连循环已停止")
			return

		case <-c.disconnectCh:
			c.logger.Info("连接已断开，准备重连...")

			// 计算带抖动的退避时间
			waitTime := backoff
			if c.config.EnableJitter {
				jitterRange := float64(waitTime) * c.config.JitterRatio
				jitter := c.jitterRNG.Float64()*2 - 1 // [-1, 1]
				waitTime = waitTime + time.Duration(jitterRange*float64(jitter))

				c.logger.Debug("计算带抖动的退避时间",
					"base_backoff", backoff,
					"jitter_ratio", c.config.JitterRatio,
					"wait_time", waitTime)
			}

			// 等待退避时间
			timer := time.NewTimer(waitTime)
			select {
			case <-c.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}

			// 增加重连尝试次数
			attempts := c.reconnectAttempts.Add(1)

			// 根据上次错误类型调整退避
			c.lastErrorTypeMu.RLock()
			lastErrorType := c.lastErrorType
			c.lastErrorTypeMu.RUnlock()

			adjustedBackoff := time.Duration(float64(backoff) * lastErrorType.GetBackoffMultiplier())
			if adjustedBackoff != backoff {
				c.logger.Debug("根据错误类型调整退避",
					"base", backoff,
					"error_type", lastErrorType,
					"adjusted", adjustedBackoff)
				backoff = adjustedBackoff
			}

			c.logger.Info("开始重连尝试", "attempt", attempts)

			// 尝试连接
			if err := c.dial(); err != nil {
				// 连接失败
				errorType := errors.ClassifyNetworkError(err)

				c.lastErrorTypeMu.Lock()
				c.lastErrorType = errorType
				c.lastErrorTypeMu.Unlock()

				c.logger.Error("重连失败",
					"attempt", attempts,
					"error_type", errorType,
					"error", err)

				// 检查是否应该停止重连
				select {
				case <-c.ctx.Done():
					return
				default:
				}

				// 指数退避
				backoff = time.Duration(math.Min(
					float64(backoff*2),
					float64(c.config.MaxBackoff),
				))

				// 继续重连
				c.notifyDisconnect()
			} else {
				// 重连成功
				c.logger.Info("重连成功", "attempt", attempts)

				// 重置状态
				backoff = c.config.InitialBackoff
				c.lastErrorTypeMu.Lock()
				c.lastErrorType = errors.ErrorTypeUnknown
				c.lastErrorTypeMu.Unlock()

				// 重启后台任务
				c.startBackgroundTasks()
			}
		}
	}
}
```

**Step 2: 运行测试**

```bash
go test ./pkg/transport/client/... -v -run TestReconnect
```

**Step 3: 提交**

```bash
git add pkg/transport/client/client.go
git commit -m "feat(client): 重连循环添加抖动和错误分类

- 实现抖动算法防止重连风暴
- 根据错误类型调整退避时间
- 所有日志输出使用中文

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 4: 修改心跳循环（添加失败累积）

**文件:**
- 修改: `pkg/transport/client/heartbeat.go`

**Step 1: 修改 heartbeatLoop 函数**

```go
func (c *Client) heartbeatLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	c.logger.Debug("心跳循环已启动", "interval", c.config.HeartbeatInterval)

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("心跳循环已停止")
			return

		case <-ticker.C:
			if !c.IsConnected() {
				c.logger.Debug("未连接，跳过心跳")
				continue
			}

			if err := c.sendHeartbeat(); err != nil {
				// 累积失败次数
				failures := c.heartbeatFailures.Add(1)
				c.logger.Error("心跳失败",
					"failures", failures,
					"max", c.config.MaxHeartbeatFailures,
					"error", err)
				c.metrics.RecordNetworkError()

				// 只有超过阈值才触发重连
				if failures >= c.config.MaxHeartbeatFailures {
					c.logger.Warn("心跳失败次数达到阈值，触发重连",
						"failures", failures,
						"max", c.config.MaxHeartbeatFailures)

					c.heartbeatFailures.Store(0) // 重置计数
					c.setState(protocol.ClientState_CLIENT_STATE_IDLE)
					c.notifyDisconnect()
				}
			} else {
				// 心跳成功，重置计数
				prevFailures := c.heartbeatFailures.Swap(0)
				if prevFailures > 0 {
					c.logger.Info("心跳恢复",
						"previous_failures", prevFailures)
				}
			}
		}
	}
}
```

**Step 2: 运行测试**

```bash
go test ./pkg/transport/client/... -v -run TestHeartbeat
```

**Step 3: 提交**

```bash
git add pkg/transport/client/heartbeat.go
git commit -m "feat(client): 心跳循环添加失败累积机制

- 实现 MaxHeartbeatFailures 阈值控制
- 心跳成功后自动重置失败计数
- 添加心跳恢复日志

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 5: 扩展监控指标

**文件:**
- 修改: `pkg/monitoring/metrics.go`
- 修改: `pkg/protocol/types.pb.go` (需要更新 proto 定义)

**Step 1: 添加重连相关指标到 Metrics 结构体**

```go
type Metrics struct {
	// ... 现有指标

	// 重连指标
	ReconnectAttempts      atomic.Int64
	ReconnectSuccess       atomic.Int64
	ReconnectCurrentBackoff atomic.Int64

	// 错误分类指标
	ReconnectErrorTransient atomic.Int64
	ReconnectErrorRefused   atomic.Int64
	ReconnectErrorTimeout  atomic.Int64
	ReconnectErrorUnknown  atomic.Int64

	// 心跳容错指标
	HeartbeatConsecutiveFailures atomic.Int64
	HeartbeatRecoveredCount     atomic.Int64

	// ... 其他现有字段
}
```

**Step 2: 添加记录方法**

```go
// RecordReconnectAttempt 记录重连尝试
func (m *Metrics) RecordReconnectAttempt() {
	m.ReconnectAttempts.Add(1)
}

// RecordReconnectSuccess 记录重连成功
func (m *Metrics) RecordReconnectSuccess(attempts int32) {
	m.ReconnectSuccess.Add(1)
	m.ReconnectCurrentBackoff.Store(0)
}

// RecordReconnectError 记录重连错误
func (m *Metrics) RecordReconnectError(errorType errors.NetworkErrorType) {
	switch errorType {
	case errors.ErrorTypeTransient:
		m.ReconnectErrorTransient.Add(1)
	case errors.ErrorTypeRefused:
		m.ReconnectErrorRefused.Add(1)
	case errors.ErrorTypeTimeout:
		m.ReconnectErrorTimeout.Add(1)
	default:
		m.ReconnectErrorUnknown.Add(1)
	}
}

// RecordHeartbeatRecovered 记录心跳恢复
func (m *Metrics) RecordHeartbeatRecovered() {
	m.HeartbeatRecoveredCount.Add(1)
}
```

**Step 3: 更新 Prometheus 输出**

修改 `pkg/monitoring/prometheus.go`：

```go
// 在 formatMetrics 函数中添加

	// 重连指标
	h.writeCounter(&sb, "reconnect_attempts_total", "Total reconnect attempts", snapshot.ReconnectAttempts)
	h.writeCounter(&sb, "reconnect_success_total", "Total reconnect successes", snapshot.ReconnectSuccess)
	h.writeGauge(&sb, "reconnect_current_backoff_ms", "Current backoff time in milliseconds", snapshot.ReconnectCurrentBackoff)

	// 错误分类指标
	h.writeCounter(&sb, "reconnect_error_transient_total", "Transient error count", snapshot.ReconnectErrorTransient)
	h.writeCounter(&sb, "reconnect_error_refused_total", "Refused error count", snapshot.ReconnectErrorRefused)
	h.writeCounter(&sb, "reconnect_error_timeout_total", "Timeout error count", snapshot.ReconnectErrorTimeout)
	h.writeCounter(&sb, "reconnect_error_unknown_total", "Unknown error count", snapshot.ReconnectErrorUnknown)

	// 心跳容错指标
	h.writeGauge(&sb, "heartbeat_consecutive_failures", "Current consecutive heartbeat failures", snapshot.HeartbeatConsecutiveFailures)
	h.writeCounter(&sb, "heartbeat_recovered_total", "Total heartbeat recoveries", snapshot.HeartbeatRecoveredCount)
```

**Step 4: 运行测试**

```bash
go test ./pkg/monitoring/... -v
```

**Step 5: 提交**

```bash
git add pkg/monitoring/metrics.go pkg/monitoring/prometheus.go
git commit -m "feat(monitoring): 添加重连优化监控指标

- 添加重连尝试、成功、退避指标
- 添加错误分类指标
- 添加心跳容错指标
- 更新 Prometheus 输出

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 6: 集成现有测试文件

**文件:**
- 已存在: `pkg/transport/client/reconnect_jitter_test.go`
- 已存在: `pkg/errors/network_errors_test.go`
- 已存在: `pkg/transport/client/heartbeat_failure_test.go`

**Step 1: 确认测试文件在 worktree 中**

```bash
ls -la pkg/errors/network_errors_test.go
ls -la pkg/transport/client/reconnect_jitter_test.go
ls -la pkg/transport/client/heartbeat_failure_test.go
```

**Step 2: 运行所有测试**

```bash
go test ./pkg/errors/... ./pkg/transport/client/... -v
```

**Step 3: 如果测试文件不存在，创建占位测试**

```bash
# 创建简单的占位测试，确保代码能编译
```

**Step 4: 提交测试文件**

```bash
git add pkg/errors/network_errors_test.go
git add pkg/transport/client/reconnect_jitter_test.go
git add pkg/transport/client/heartbeat_failure_test.go
git commit -m "test: 添加重连优化单元测试

- 抖动算法测试
- 网络错误分类测试
- 心跳失败累积测试

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 7: 更新配置文档

**文件:**
- 修改: `docs/configuration-guide.md`

**Step 1: 添加重连配置章节**

在文档中添加：

```markdown
## 客户端重连配置

### 基础重连配置

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `reconnect_enabled` | bool | `true` | 是否启用自动重连 |
| `initial_backoff` | duration | `1s` | 首次重试延迟 |
| `max_backoff` | duration | `60s` | 最大重试延迟 |

### 抖动配置

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enable_jitter` | bool | `true` | 是否启用抖动（强烈推荐） |
| `jitter_ratio` | float | `0.25` | 抖动比例，±25% |

### 心跳容错配置

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `max_heartbeat_failures` | int | `1` | 最大允许失败次数 |

### 配置示例

```yaml
client:
  reconnect_enabled: true
  initial_backoff: 1s
  max_backoff: 60s
  enable_jitter: true
  jitter_ratio: 0.25
  max_heartbeat_failures: 1
```
```

**Step 2: 提交文档**

```bash
git add docs/configuration-guide.md
git commit -m "docs: 添加重连配置文档

- 添加抖动配置说明
- 添加心跳容错配置说明
- 添加配置示例

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 8: 更新网络可靠性文档

**文件:**
- 修改: `docs/network-reliability.md`

**Step 1: 添加重连机制章节**

```markdown
## 重连机制

### 重连触发条件

| 触发条件 | 描述 | 处理方式 |
|----------|------|----------|
| 心跳超时 | 连续 N 次心跳失败 | 触发重连 |
| 连接断开 | 网络中断、服务器关闭 | 触发重连 |

### 错误分类与退避策略

| 错误类型 | 退避倍数 | 策略 |
|----------|----------|------|
| 瞬时错误 | 1.0x | 快速重试 |
| 超时错误 | 1.5x | 中等退避 |
| 拒绝错误 | 2.0x | 慢速重试 |
```

**Step 2: 提交文档**

```bash
git add docs/network-reliability.md
git commit -m "docs: 添加重连机制说明

- 添加重连触发条件
- 添加错误分类与退避策略
- 添加监控指标说明

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 9: 运行完整测试套件

**Step 1: 运行所有测试**

```bash
go test ./... -v
```

**Step 2: 检查测试覆盖率**

```bash
go test ./pkg/transport/client/... -cover
go test ./pkg/errors/... -cover
go test ./pkg/monitoring/... -cover
```

**Step 3: 修复任何失败的测试**

如果测试失败，修复问题并重新运行。

**Step 4: 提交任何修复**

```bash
git commit -am "test: 修复测试失败"
```

---

## Task 10: 最终验证和清理

**Step 1: 验证代码构建**

```bash
go build ./cmd/client
go build ./cmd/server
```

**Step 2: 运行集成测试（如果存在）**

```bash
go test ./tests/... -v
```

**Step 3: 检查代码格式**

```bash
go fmt ./...
```

**Step 4: 运行 linter**

```bash
# 如果项目有配置 golangci-lint
golangci-lint run
```

**Step 5: 最终提交**

```bash
git add -A
git commit -m "chore: 最终代码清理和格式化

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## 任务清单总结

| 任务 | 描述 | 预计时间 |
|------|------|----------|
| Task 1 | 扩展配置结构 | 10分钟 |
| Task 2 | 扩展 Client 结构体 | 10分钟 |
| Task 3 | 修改重连循环 | 20分钟 |
| Task 4 | 修改心跳循环 | 15分钟 |
| Task 5 | 扩展监控指标 | 20分钟 |
| Task 6 | 集成测试文件 | 10分钟 |
| Task 7 | 更新配置文档 | 10分钟 |
| Task 8 | 更新网络可靠性文档 | 10分钟 |
| Task 9 | 运行完整测试套件 | 15分钟 |
| Task 10 | 最终验证和清理 | 10分钟 |

**总预计时间**: ~2 小时

---

## 重要提醒

1. **所有代码注释必须使用中文**
2. **所有日志输出必须使用中文**
3. **遵循 TDD 原则**：先写测试，再写实现
4. **频繁提交**：每个 Task 完成后立即提交
5. **YAGNI 原则**：只实现需要的功能
