---
name: quic-flow
description: QUIC Flow 项目专用助手。帮助理解 QUIC 通信系统架构、开发新功能、调试连接问题、添加消息处理器、配置发布任务等。当询问项目结构、QUIC 协议实现、客户端连接、消息传输或发布系统时使用。
---

# QUIC Flow 项目助手

QUIC Flow 是一个基于 QUIC 协议（HTTP/3）的高性能工业级通信骨干网络系统。

## 项目架构

### 核心组件

**传输层** (`pkg/transport/`)
- `server.go` - QUIC 服务器，支持 10K-100K+ 并发连接
- `client.go` - QUIC 客户端，自动重连（指数退避）
- `quic_config.go` - QUIC 协议配置（TLS、连接参数、Keep-alive）

**会话管理** (`pkg/session/`)
- `manager.go` - 会话生命周期管理（Idle → Connecting → Connected）
- `session.go` - 单个会话状态维护
- 心跳检测：15s 间隔，45s 超时，3 次未响应断开

**消息系统** (`pkg/dispatcher/`, `pkg/callback/`)
- `dispatcher.go` - Worker Pool 模式，支持 200+ 并发处理
- `promise.go` - 异步请求-响应模式（类似 JavaScript Promise）
- 消息类型：单播、广播、异步回调

**发布管理** (`pkg/release/`)
- 三层配置：项目级 → 版本级 → 任务级
- 部署方式：脚本、容器、Git、Kubernetes
- 支持金丝雀发布和 Webhook 自动部署

### 目录结构

```
quic-flow/
├── cmd/                    # 可执行程序
│   ├── server/            # QUIC 服务器主程序
│   ├── client/            # 客户端主程序
│   ├── ctl/               # 管理工具
│   ├── cli/               # 命令行接口
│   └── loadtest/          # 负载测试工具（1万并发）
├── pkg/                   # 核心库代码
│   ├── transport/         # QUIC 传输层
│   ├── session/           # 会话管理
│   ├── dispatcher/        # 消息分发
│   ├── callback/          # Promise 机制
│   ├── protocol/          # Protobuf 定义
│   ├── auth/              # 认证系统（JWT + Casbin）
│   ├── release/           # 发布管理
│   └── monitoring/        # Prometheus 监控
├── web/                   # Vue 3 管理界面
└── config/                # 配置文件
```

## 技术栈

**后端：**
- Go 1.25
- quic-go (RFC 9000)
- Protocol Buffers
- Gin (HTTP API)
- GORM (ORM)
- Prometheus (监控)
- Cobra (CLI)

**前端：**
- Vue 3 + Element Plus
- Vite
- Pinia (状态管理)
- Monaco Editor

**数据库：**
- PostgreSQL (生产) / MySQL / SQLite (开发)

## 开发指南

### 添加新的消息类型

1. 在 `pkg/protocol/` 定义 Protobuf 消息
2. 在 `pkg/dispatcher/` 注册处理器
3. 实现处理逻辑：

```go
// 注册消息处理器
dispatcher.RegisterHandler(MessageType_CUSTOM, func(msg *Message) error {
    // 处理消息
    return nil
})
```

### 实现自定义命令

客户端命令处理在 `pkg/commands/`：

```go
type CommandHandler interface {
    Handle(ctx context.Context, args []string) (string, error)
    Name() string
}
```

### 心跳和重连机制

- 客户端心跳：每 15 秒发送一次
- 服务端超时：45 秒未响应视为离线
- 重连策略：指数退避（1s → 2s → 4s → 8s → 最大 60s）

### 监控指标

27+ Prometheus 指标：
- `quic_flow_connections_total` - 总连接数
- `quic_flow_messages_sent` - 发送消息数
- `quic_flow_latency_seconds` - 延迟统计（P50/P95/P99）
- `quic_flow_errors_total` - 错误计数

访问：`http://localhost:8080/metrics`

### 发布任务配置

在 Web 界面配置发布任务：
1. 创建项目（配置环境、部署方式）
2. 创建版本（关联 Git commit）
3. 创建任务（脚本/容器/K8s YAML）
4. 配置 Webhook 触发器

## 常见问题

### 连接失败排查

1. 检查 TLS 证书：`certs/` 目录
2. 验证防火墙：QUIC 使用 UDP（默认端口 8443）
3. 查看日志：`--log-level debug`
4. 检查配置文件：`config/server.yaml`

### 性能调优

标准配置（10K 连接）：
```yaml
server:
  max_connections: 10000
  workers: 200
```

高性能配置（100K+ 连接）：
```yaml
server:
  max_connections: 100000
  workers: 1000
  read_buffer_size: 65536
  write_buffer_size: 65536
```

### 消息发送模式

```go
// 单播
session.SendData(message)

// 广播
sessionManager.Broadcast(message)

// 异步请求-响应
promise := callbackManager.CreatePromise()
session.SendRequest(promise.ID(), request)
response := promise.Wait(timeout)
```

## 构建和运行

```bash
# 服务器
go run cmd/server/main.go -config config/server.yaml

# 客户端
go run cmd/client/main.go -server localhost:8443

# 负载测试
go run cmd/loadtest/main.go -clients 10000 -server localhost:8443

# Web 界面
cd web && npm run dev
```

## 参考资源

- Protobuf 定义：`pkg/protocol/*.proto`
- API 文档：`docs/api.md`
- 部署指南：`docs/deployment.md`
- 监控指标：`pkg/monitoring/metrics.go`
