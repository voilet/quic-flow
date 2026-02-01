# QUIC Backbone 参数配置指南

本文档详细说明了 QUIC Backbone 项目中所有组件的参数配置方式，包括命令行参数、配置文件参数和环境变量。

## 目录

- [服务器 (quic-server)](#服务器-quic-server)
- [客户端 (quic-client)](#客户端-quic-client)
- [CLI 工具 (quic-ctl)](#cli-工具-quic-ctl)
- [高级配置](#高级配置)
- [配置示例](#配置示例)
- [环境变量](#环境变量)
- [最佳实践](#最佳实践)

---

## 服务器 (quic-server)

### 命令行参数

服务器支持以下命令行参数：

#### 基础参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `-addr` | string | `:8474` | QUIC 服务器监听地址（格式：`host:port` 或 `:port`） |
| `-api` | string | `:8475` | HTTP API 服务器监听地址 |
| `-cert` | string | `certs/server-cert.pem` | TLS 服务器证书文件路径 |
| `-key` | string | `certs/server-key.pem` | TLS 服务器私钥文件路径 |

#### 使用示例

```bash
# 使用默认参数启动
./bin/quic-server

# 自定义端口和证书路径
./bin/quic-server -addr :9000 -api :9001 -cert /path/to/cert.pem -key /path/to/key.pem

# 监听所有网络接口
./bin/quic-server -addr 0.0.0.0:8474

# 仅监听本地回环
./bin/quic-server -addr 127.0.0.1:8474
```

### 高级配置（编程方式）

通过代码创建服务器时，可以配置更多高级参数：

#### 网络配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `ListenAddr` | string | - | 监听地址（必需） |
| `MaxIdleTimeout` | time.Duration | 60s | 连接空闲超时时间 |

#### TLS 配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `TLSCertFile` | string | - | TLS 证书文件路径 |
| `TLSKeyFile` | string | - | TLS 私钥文件路径 |
| `TLSConfig` | *tls.Config | nil | 自定义 TLS 配置（优先级高于文件路径） |

#### QUIC 配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `MaxIncomingStreams` | int64 | 1000 | 每个连接最大并发流数 |
| `MaxIncomingUniStreams` | int64 | 100 | 单向流最大数量 |
| `InitialStreamReceiveWindow` | uint64 | 512KB | 初始流接收窗口大小 |
| `MaxStreamReceiveWindow` | uint64 | 6MB | 最大流接收窗口大小 |
| `InitialConnectionReceiveWindow` | uint64 | 1MB | 初始连接接收窗口大小 |
| `MaxConnectionReceiveWindow` | uint64 | 15MB | 最大连接接收窗口大小 |

#### 会话管理配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `MaxClients` | int64 | 10000 | 最大并发客户端数 |
| `HeartbeatInterval` | time.Duration | 15s | 心跳间隔时间 |
| `HeartbeatTimeout` | time.Duration | 45s | 心跳超时时间 |
| `HeartbeatCheckInterval` | time.Duration | 5s | 心跳检查间隔 |
| `MaxTimeoutCount` | int32 | 3 | 最大连续超时次数 |

#### Promise 管理配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `MaxPromises` | int64 | 50000 | 最大 Promise 数量 |
| `PromiseWarnThreshold` | int64 | 40000 | Promise 数量警告阈值 |
| `DefaultMessageTimeout` | time.Duration | 30s | 默认消息超时时间 |

#### 监控配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `Hooks` | *monitoring.EventHooks | nil | 事件钩子（可选） |
| `Logger` | *monitoring.Logger | 默认日志器 | 日志实例（可选） |

#### 编程示例

```go
config := server.NewDefaultServerConfig("cert.pem", "key.pem", ":8474")

// 修改默认配置
config.MaxClients = 20000
config.HeartbeatInterval = 10 * time.Second
config.MaxIncomingStreams = 2000

// 设置事件钩子
config.Hooks = &monitoring.EventHooks{
    OnConnect: func(clientID string) {
        log.Printf("Client connected: %s", clientID)
    },
    OnDisconnect: func(clientID string, reason error) {
        log.Printf("Client disconnected: %s, reason: %v", clientID, reason)
    },
}

// 创建服务器
srv, err := server.NewServer(config)
if err != nil {
    log.Fatal(err)
}

// 启动服务器
if err := srv.Start(config.ListenAddr); err != nil {
    log.Fatal(err)
}
```

---

## 客户端 (quic-client)

### 命令行参数

客户端支持以下命令行参数：

#### 基础参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `-server` | string | `localhost:8474` | 服务器地址（格式：`host:port`） |
| `-id` | string | `client-001` | 客户端唯一标识符 |
| `-insecure` | bool | `true` | 跳过 TLS 证书验证（**仅开发环境使用**） |

#### 使用示例

```bash
# 使用默认参数连接
./bin/quic-client

# 连接到远程服务器
./bin/quic-client -server 192.168.1.100:8474 -id client-prod-001

# 生产环境（启用证书验证）
./bin/quic-client -server prod.example.com:8474 -id client-prod-001 -insecure=false

# 连接到本地服务器
./bin/quic-client -server localhost:9000 -id test-client
```

### 高级配置（编程方式）

通过代码创建客户端时，可以配置更多高级参数：

#### 客户端标识

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `ClientID` | string | - | 客户端唯一标识（必需） |

#### TLS 配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `TLSCertFile` | string | - | 客户端证书文件路径（双向 TLS，可选） |
| `TLSKeyFile` | string | - | 客户端私钥文件路径（双向 TLS，可选） |
| `CACertFile` | string | - | CA 证书文件路径（用于验证服务器） |
| `InsecureSkipVerify` | bool | false | 跳过服务器证书验证（**仅开发环境**） |
| `TLSConfig` | *tls.Config | nil | 自定义 TLS 配置（优先级高于文件路径） |

#### QUIC 配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `MaxIdleTimeout` | time.Duration | 60s | 连接空闲超时时间 |
| `MaxIncomingStreams` | int64 | 1000 | 每个连接最大并发流数 |
| `MaxIncomingUniStreams` | int64 | 100 | 单向流最大数量 |
| `InitialStreamReceiveWindow` | uint64 | 512KB | 初始流接收窗口大小 |
| `MaxStreamReceiveWindow` | uint64 | 6MB | 最大流接收窗口大小 |
| `InitialConnectionReceiveWindow` | uint64 | 1MB | 初始连接接收窗口大小 |
| `MaxConnectionReceiveWindow` | uint64 | 15MB | 最大连接接收窗口大小 |

#### 重连配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `ReconnectEnabled` | bool | true | 是否启用自动重连 |
| `InitialBackoff` | time.Duration | 1s | 首次重试延迟时间 |
| `MaxBackoff` | time.Duration | 60s | 最大重试延迟时间 |

#### 重连抖动配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `EnableJitter` | bool | true | 是否启用抖动（强烈推荐） |
| `JitterRatio` | float64 | 0.25 | 抖动比例，±25% |

**抖动作用**: 防止多客户端同时断线时产生重连风暴，在退避时间上添加随机化。

**抖动公式**: `wait_time = backoff + backoff * ratio * (2 * rand - 1)`

**示例**:
- `backoff = 2s`, `ratio = 0.25` → `wait_time ∈ [1.5s, 2.5s]`
- `backoff = 10s`, `ratio = 0.25` → `wait_time ∈ [7.5s, 12.5s]`

#### 心跳配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `HeartbeatInterval` | time.Duration | 15s | 心跳间隔时间 |
| `HeartbeatTimeout` | time.Duration | 5s | 心跳响应超时时间 |
| `MaxHeartbeatFailures` | int32 | 1 | 最大允许失败次数 |

**心跳容错机制**: 允许连续多次心跳失败后再触发重连，避免因瞬时网络抖动导致不必要的重连。

- `MaxHeartbeatFailures = 1`: 单次心跳失败即触发重连（默认，保持现有行为）
- `MaxHeartbeatFailures = 3`: 允许连续 2 次失败，第 3 次才触发重连
- 心跳成功后自动重置失败计数

#### 消息配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `DefaultMessageTimeout` | time.Duration | 30s | 默认消息超时时间 |

#### 监控配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `Hooks` | *monitoring.EventHooks | nil | 事件钩子（可选） |
| `Logger` | *monitoring.Logger | 默认日志器 | 日志实例（可选） |

#### 编程示例

```go
config := client.NewDefaultClientConfig("client-prod-001")

// 修改默认配置
config.InsecureSkipVerify = false // 生产环境必须为 false
config.ReconnectEnabled = true
config.InitialBackoff = 2 * time.Second
config.MaxBackoff = 120 * time.Second

// 配置 TLS
config.CACertFile = "/path/to/ca.pem"
config.TLSCertFile = "/path/to/client-cert.pem"
config.TLSKeyFile = "/path/to/client-key.pem"

// 设置事件钩子
config.Hooks = &monitoring.EventHooks{
    OnConnect: func(clientID string) {
        log.Printf("Connected: %s", clientID)
    },
    OnDisconnect: func(clientID string, reason error) {
        log.Printf("Disconnected: %s, reason: %v", clientID, reason)
    },
    OnReconnect: func(clientID string, attemptCount int) {
        log.Printf("Reconnected: %s after %d attempts", clientID, attemptCount)
    },
}

// 创建客户端
c, err := client.NewClient(config)
if err != nil {
    log.Fatal(err)
}

// 连接到服务器
if err := c.Connect("server.example.com:8474"); err != nil {
    log.Fatal(err)
}
```

---

## CLI 工具 (quic-ctl)

CLI 工具用于管理服务器和客户端。

### 命令概览

```
quic-ctl <command> [options]

Commands:
  list        列出所有在线客户端
  send        向指定客户端发送消息
  broadcast   向所有客户端广播消息
  help        显示帮助信息
```

### 1. list 命令

列出所有在线客户端。

#### 参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `-api` | string | `http://localhost:8475` | API 服务器地址 |

#### 示例

```bash
# 使用默认 API 地址
./bin/quic-ctl list

# 指定 API 地址
./bin/quic-ctl list -api http://192.168.1.100:8475
```

#### 输出示例

```
Connected Clients: 3

CLIENT ID   REMOTE ADDRESS   UPTIME  CONNECTED AT
---------   --------------   ------  ------------
client-001  127.0.0.1:52104  9s      2025-12-24 16:59:40
client-002  127.0.0.1:58931  6s      2025-12-24 16:59:42
client-003  127.0.0.1:58822  3s      2025-12-24 16:59:45
```

### 2. send 命令

向指定客户端发送消息。

#### 参数

| 参数 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `-api` | string | `http://localhost:8475` | 否 | API 服务器地址 |
| `-client` | string | - | **是** | 目标客户端 ID |
| `-type` | string | `command` | 否 | 消息类型（`command`\|`event`\|`query`\|`response`） |
| `-payload` | string | - | **是** | 消息内容（JSON 格式） |
| `-wait-ack` | bool | `false` | 否 | 是否等待客户端确认 |

#### 消息类型说明

- **command**: 命令消息，用于执行操作
- **event**: 事件消息，用于通知状态变化
- **query**: 查询消息，用于请求信息
- **response**: 响应消息，用于回复请求

#### 示例

```bash
# 发送命令消息
./bin/quic-ctl send -client client-001 -type command -payload '{"action":"restart","timeout":30}'

# 发送事件消息
./bin/quic-ctl send -client client-002 -type event -payload '{"event":"config_changed","path":"/etc/app/config.json"}'

# 发送查询消息
./bin/quic-ctl send -client client-003 -type query -payload '{"query":"status","fields":["cpu","memory"]}'

# 等待客户端确认
./bin/quic-ctl send -client client-001 -type command -payload '{"action":"backup"}' -wait-ack

# 指定 API 服务器
./bin/quic-ctl send -api http://192.168.1.100:8475 -client client-001 -type command -payload '{"action":"restart"}'
```

#### 输出示例

```
✅ Message sent successfully
   Client ID: client-001
   Message ID: 491e0615-ca0e-48cd-a24b-80f760cc5f69
   Type: command
   Payload: {"action":"restart","timeout":30}
```

### 3. broadcast 命令

向所有在线客户端广播消息。

#### 参数

| 参数 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `-api` | string | `http://localhost:8475` | 否 | API 服务器地址 |
| `-type` | string | `event` | 否 | 消息类型（`command`\|`event`\|`query`\|`response`） |
| `-payload` | string | - | **是** | 消息内容（JSON 格式） |

#### 示例

```bash
# 广播系统更新事件
./bin/quic-ctl broadcast -type event -payload '{"event":"update_available","version":"1.2.0"}'

# 广播维护通知
./bin/quic-ctl broadcast -type event -payload '{"event":"maintenance","start_time":"2025-12-25T00:00:00Z","duration":"2h"}'

# 广播命令（谨慎使用）
./bin/quic-ctl broadcast -type command -payload '{"action":"refresh_config"}'

# 指定 API 服务器
./bin/quic-ctl broadcast -api http://192.168.1.100:8475 -type event -payload '{"event":"alert","message":"Critical update"}'
```

#### 输出示例

```
✅ Message broadcast completed
   Message ID: c6eeb4eb-35bb-475c-b661-f74fca7c941b
   Type: event
   Payload: {"event":"update_available","version":"1.2.0"}
   Total Clients: 3
   Success: 3
   Failed: 0
```

---

## 高级配置

### 日志级别配置

可以通过环境变量或代码配置日志级别。

#### 代码配置

```go
// 服务器
logger := monitoring.NewLogger(monitoring.LogLevelDebug, "json")
config.Logger = logger

// 客户端
logger := monitoring.NewLogger(monitoring.LogLevelInfo, "text")
config.Logger = logger
```

#### 日志级别

- `LogLevelDebug`: 调试级别（详细日志）
- `LogLevelInfo`: 信息级别（默认）
- `LogLevelWarn`: 警告级别
- `LogLevelError`: 错误级别

#### 日志格式

- `text`: 文本格式（易读）
- `json`: JSON 格式（便于解析）

### 性能调优

#### 高并发场景

```go
config := server.NewDefaultServerConfig("cert.pem", "key.pem", ":8474")

// 增加最大客户端数
config.MaxClients = 50000

// 增加流数量
config.MaxIncomingStreams = 5000

// 增加接收窗口
config.MaxStreamReceiveWindow = 10 * 1024 * 1024 // 10MB
config.MaxConnectionReceiveWindow = 30 * 1024 * 1024 // 30MB

// 增加 Promise 容量
config.MaxPromises = 100000
```

#### 低延迟场景

```go
config := client.NewDefaultClientConfig("client-001")

// 减少心跳间隔
config.HeartbeatInterval = 5 * time.Second

// 减少超时时间
config.DefaultMessageTimeout = 10 * time.Second

// 减少空闲超时
config.MaxIdleTimeout = 30 * time.Second
```

#### 不稳定网络

```go
config := client.NewDefaultClientConfig("client-001")

// 启用自动重连
config.ReconnectEnabled = true

// 增加重试延迟
config.InitialBackoff = 3 * time.Second
config.MaxBackoff = 180 * time.Second

// 增加超时时间
config.MaxIdleTimeout = 120 * time.Second
config.HeartbeatTimeout = 10 * time.Second
```

---

## 配置示例

### 生产环境服务器配置

```go
package main

import (
    "log"
    "github.com/voilet/QuicFlow/pkg/monitoring"
    "github.com/voilet/QuicFlow/pkg/transport/server"
)

func main() {
    // 创建生产环境配置
    config := server.NewDefaultServerConfig(
        "/etc/quic/server-cert.pem",
        "/etc/quic/server-key.pem",
        ":8474",
    )

    // 生产环境调优
    config.MaxClients = 10000
    config.HeartbeatTimeout = 60 * time.Second
    config.MaxPromises = 50000

    // JSON 日志格式（便于日志收集系统解析）
    config.Logger = monitoring.NewLogger(monitoring.LogLevelInfo, "json")

    // 事件钩子（集成监控系统）
    config.Hooks = &monitoring.EventHooks{
        OnConnect: func(clientID string) {
            // 发送到监控系统
            metrics.IncrementCounter("client.connected")
        },
        OnDisconnect: func(clientID string, reason error) {
            metrics.IncrementCounter("client.disconnected")
        },
    }

    srv, err := server.NewServer(config)
    if err != nil {
        log.Fatal(err)
    }

    if err := srv.Start(config.ListenAddr); err != nil {
        log.Fatal(err)
    }

    // ... 等待关闭信号
}
```

### 生产环境客户端配置

```go
package main

import (
    "log"
    "github.com/voilet/QuicFlow/pkg/monitoring"
    "github.com/voilet/QuicFlow/pkg/transport/client"
)

func main() {
    config := client.NewDefaultClientConfig("prod-client-001")

    // 启用证书验证
    config.InsecureSkipVerify = false
    config.CACertFile = "/etc/quic/ca.pem"

    // 启用自动重连
    config.ReconnectEnabled = true
    config.InitialBackoff = 2 * time.Second
    config.MaxBackoff = 120 * time.Second

    // JSON 日志
    config.Logger = monitoring.NewLogger(monitoring.LogLevelInfo, "json")

    // 事件钩子
    config.Hooks = &monitoring.EventHooks{
        OnConnect: func(clientID string) {
            log.Printf("Connected to server")
        },
        OnDisconnect: func(clientID string, reason error) {
            log.Printf("Disconnected: %v", reason)
        },
        OnReconnect: func(clientID string, attemptCount int) {
            log.Printf("Reconnected after %d attempts", attemptCount)
        },
    }

    c, err := client.NewClient(config)
    if err != nil {
        log.Fatal(err)
    }

    if err := c.Connect("server.example.com:8474"); err != nil {
        log.Fatal(err)
    }

    // ... 等待关闭信号
}
```

---

## 环境变量

虽然当前版本不直接支持环境变量配置，但可以通过代码实现：

```go
package main

import (
    "os"
    "strconv"
    "time"
)

func loadServerConfigFromEnv() *server.ServerConfig {
    addr := getEnv("QUIC_SERVER_ADDR", ":8474")
    cert := getEnv("QUIC_TLS_CERT", "certs/server-cert.pem")
    key := getEnv("QUIC_TLS_KEY", "certs/server-key.pem")

    config := server.NewDefaultServerConfig(cert, key, addr)

    // 从环境变量读取最大客户端数
    if maxClients := getEnv("QUIC_MAX_CLIENTS", ""); maxClients != "" {
        if val, err := strconv.ParseInt(maxClients, 10, 64); err == nil {
            config.MaxClients = val
        }
    }

    // 从环境变量读取心跳间隔
    if hbInterval := getEnv("QUIC_HEARTBEAT_INTERVAL", ""); hbInterval != "" {
        if val, err := time.ParseDuration(hbInterval); err == nil {
            config.HeartbeatInterval = val
        }
    }

    return config
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

### 建议的环境变量

#### 服务器

- `QUIC_SERVER_ADDR`: 服务器监听地址（默认 `:8474`）
- `QUIC_API_ADDR`: API 服务器监听地址（默认 `:8475`）
- `QUIC_TLS_CERT`: TLS 证书路径
- `QUIC_TLS_KEY`: TLS 私钥路径
- `QUIC_MAX_CLIENTS`: 最大客户端数
- `QUIC_LOG_LEVEL`: 日志级别（debug/info/warn/error）
- `QUIC_LOG_FORMAT`: 日志格式（text/json）

#### 客户端

- `QUIC_SERVER_ADDR`: 服务器地址
- `QUIC_CLIENT_ID`: 客户端 ID
- `QUIC_INSECURE_SKIP_VERIFY`: 跳过证书验证（true/false）
- `QUIC_CA_CERT`: CA 证书路径
- `QUIC_LOG_LEVEL`: 日志级别
- `QUIC_RECONNECT_ENABLED`: 启用自动重连（true/false）

---

## 最佳实践

### 1. 开发环境

```bash
# 服务器：使用默认配置，跳过复杂设置
./bin/quic-server

# 客户端：跳过证书验证
./bin/quic-client -server localhost:8474 -id dev-client -insecure=true
```

### 2. 测试环境

```bash
# 服务器：使用测试证书
./bin/quic-server -cert test-certs/cert.pem -key test-certs/key.pem -addr :8474

# 客户端：连接到测试服务器
./bin/quic-client -server test-server:8474 -id test-client-001 -insecure=false
```

### 3. 生产环境

```bash
# 服务器：使用生产证书，监听所有接口
./bin/quic-server \
  -cert /etc/ssl/quic/server-cert.pem \
  -key /etc/ssl/quic/server-key.pem \
  -addr 0.0.0.0:8474 \
  -api 127.0.0.1:8475  # API 仅监听本地

# 客户端：启用证书验证
./bin/quic-client \
  -server prod-server.example.com:8474 \
  -id prod-client-001 \
  -insecure=false
```

### 4. 安全建议

1. **生产环境必须禁用 `-insecure` 参数**
   ```bash
   # ❌ 错误
   ./bin/quic-client -server prod:8474 -insecure=true

   # ✅ 正确
   ./bin/quic-client -server prod:8474 -insecure=false
   ```

2. **API 端口仅监听本地**
   ```bash
   # ✅ 安全：API 仅本地访问
   ./bin/quic-server -addr 0.0.0.0:8474 -api 127.0.0.1:8475

   # ❌ 不安全：API 暴露到公网
   ./bin/quic-server -addr 0.0.0.0:8474 -api 0.0.0.0:8475
   ```

3. **使用有效的 TLS 证书**
   - 开发环境：自签名证书
   - 生产环境：CA 签发的证书

4. **限制客户端 ID 格式**
   - 建议使用有意义的命名：`service-region-instance`
   - 例如：`webapp-us-east-001`

### 5. 监控和日志

```bash
# 生产环境使用 JSON 日志，便于日志收集系统解析
# 通过代码配置：
config.Logger = monitoring.NewLogger(monitoring.LogLevelInfo, "json")
```

### 6. 性能调优

根据实际场景调整参数：

- **高并发场景**：增加 `MaxClients`、`MaxIncomingStreams`
- **低延迟场景**：减少 `HeartbeatInterval`、超时时间
- **不稳定网络**：增加重连参数、超时时间

---

## 故障排查

### 常见问题

#### 1. 连接失败

```bash
Error: failed to dial server: timeout: no recent network activity
```

**解决方法：**
- 检查服务器是否运行
- 检查防火墙规则
- 确认服务器地址和端口正确

#### 2. 证书验证失败

```bash
Error: tls: failed to verify certificate
```

**解决方法：**
- 开发环境：使用 `-insecure=true`
- 生产环境：确保证书有效且未过期

#### 3. API 连接失败

```bash
Error: failed to connect to API server: connection refused
```

**解决方法：**
- 确认 API 服务器已启动
- 检查 API 地址和端口
- 确认网络可达性

---

## 相关文档

- [快速开始](../quickstart.md)
- [CLI 使用指南](cli-guide.md)
- [HTTP API 文档](http-api.md)
- [网络可靠性设计](network-reliability.md)

---

**最后更新：** 2025-12-24
**版本：** 1.0.0
