# QUIC Flow 配置中心系统设计文档

> **版本**: v1.0
> **日期**: 2025-02-06
> **状态**: 设计阶段

---

## 一、概述

### 1.1 项目背景

QUIC Flow 配置中心 (QFCC) 是一个基于 QUIC 长连接的轻量级分布式配置中心，旨在提供：

- **实时推送**: 利用 QUIC 长连接实现毫秒级配置推送
- **多语言 SDK**: 支持 Go、Python、Java、JavaScript 等主流语言
- **推拉结合**: 服务端主动推送 + 客户端主动拉取双模式
- **灰度发布**: 支持按标签、IP、百分比进行灰度发布
- **版本管理**: 完整的配置版本历史和回滚能力

### 1.2 与传统 Nacos 的对比

| 特性 | Nacos | QUIC Flow Config |
|------|-------|------------------|
| 推送方式 | HTTP 短轮询 (2-3秒延迟) | QUIC 长连接 (毫秒级) |
| 服务端推送 | 弱支持 | 原生支持 |
| SDK 复杂度 | 高 | 低 (复用 QUIC 连接) |
| 部署模式 | 独立部署 | 与堡垒机/发布系统集成 |
| 网络开销 | 高 (频繁轮询) | 低 (事件驱动) |

---

## 二、系统架构

### 2.1 整体架构图

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         QUIC Flow Server                                 │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │                    Configuration Service                           │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌────────────────────────────┐│  │
│  │  │   Config    │  │   Version   │  │      Change Log            ││  │
│  │  │   CRUD      │  │   Control   │  │       & Audit              ││  │
│  │  └─────────────┘  └─────────────┘  └────────────────────────────┘│  │
│  └────────────────────────────────────────────────────────────────────┘  │
│                              │                                           │
│                              ▼                                           │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │                   Distribution Engine                              │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌────────────────────────────┐│  │
│  │  │   Target    │  │    Gray     │  │       Rollback             ││  │
│  │  │  Selector   │  │  Release    │  │       Manager              ││  │
│  │  └─────────────┘  └─────────────┘  └────────────────────────────┘│  │
│  └────────────────────────────────────────────────────────────────────┘  │
│                              │                                           │
│                              ▼                                           │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │                   Push Engine (via QUIC)                           │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌────────────────────────────┐│  │
│  │  │  Session    │  │   Config    │  │      Ack & Retry           ││  │
│  │  │  Manager    │  │  Dispatcher │  │       Manager               ││  │
│  │  └─────────────┘  └─────────────┘  └────────────────────────────┘│  │
│  └────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────┘
                               │
                   ┌───────────┴───────────┐
                   ▼                       ▼
        ┌──────────────────┐      ┌─────────────────┐
        │  SDK Clients      │      │   PostgreSQL    │
        │  (Go/Py/Java/JS)  │      │   (Config DB)   │
        │  - 接收推送        │      └─────────────────┘
        │  - 主动拉取        │
        └──────────────────┘
```

### 2.2 配置类型支持

系统支持两种配置类型：

**1. 应用配置 (application)**
- 用途：业务应用运行时配置
- 格式：JSON/YAML/Properties
- 示例：数据库连接、API 端点、功能开关
- 目标：运行中的应用进程

**2. 系统参数 (system)**
- 用途：QUIC Flow 客户端/服务器参数
- 格式：JSON 结构体
- 示例：心跳间隔、日志级别、重连策略
- 目标：QUIC Client 自身配置

---

## 三、核心数据模型

### 3.1 配置管理

```go
// Config 配置项
type Config struct {
    ID          uint      `gorm:"primaryKey"`
    Namespace   string    `gorm:"size:64;index"`   // 命名空间 (环境隔离)
    Group       string    `gorm:"size:64;index"`   // 配置分组
    DataID      string    `gorm:"size:128;index"`  // 配置标识
    ConfigType  string    `gorm:"size:32"`         // application | system
    Content     string    `gorm:"type:text"`       // 配置内容
    Format      string    `gorm:"size:32"`         // json | yaml | properties | text
    Encrypted   bool      `gorm:"default:false"`   // 是否加密存储
    Tags        []string  `gorm:"type:text[]"`     // 标签 (用于灰度选择)
    Description string    `gorm:"size:512"`        // 描述
    Version     int       `gorm:"default:1"`       // 当前版本号
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### 3.2 版本管理

```go
// ConfigRelease 配置发布记录
type ConfigRelease struct {
    ID          uint      `gorm:"primaryKey"`
    ConfigID    uint      `gorm:"index"`
    Namespace   string
    Group       string
    DataID      string
    Content     string    `gorm:"type:text"`
    ReleaseType string    // full | rollback | gray
    Status      string    // pending | publishing | success | failed
    ReleasedBy  string
    ReleasedAt  time.Time
    TotalTargets int
    SuccessCount int
    FailedCount  int
    IsGray      bool
    GrayRuleID  *uint
}
```

### 3.3 灰度规则

```go
// GrayRule 灰度发布规则
type GrayRule struct {
    ID          uint      `gorm:"primaryKey"`
    ConfigID    uint      `gorm:"index"`
    RuleName    string    `gorm:"size:128"`
    RuleType    string    // tag | ip | client_id | percentage
    RuleValue   string    `gorm:"type:text"`
    Enabled     bool      `gorm:"default:true"`
    Priority    int       `gorm:"default:0"`
}
```

### 3.4 客户端订阅

```go
// ConfigSubscriber 配置订阅者
type ConfigSubscriber struct {
    ID          uint      `gorm:"primaryKey"`
    ClientID    string    `gorm:"size:128;index"`
    SDKType     string    `gorm:"size:32"`
    Namespace   string    `gorm:"size:64;index"`
    Subscriptions []string `gorm:"type:text[]"`
    ClientIP    string    `gorm:"size:64"`
    ClientTags  []string  `gorm:"type:text[]"`
    LastActive  time.Time
    Status      string    // online | offline
}
```

---

## 四、配置推送协议

### 4.1 Protobuf 定义

```protobuf
syntax = "proto3";
package quic_flow.config;

enum ConfigMessageType {
  // 客户端 → 服务端
  CONFIG_REGISTER = 0;
  CONFIG_PULL = 1;
  CONFIG_ACK = 2;
  // 服务端 → 客户端
  CONFIG_PUSH = 10;
  CONFIG_SYNC = 11;
}

message ConfigRegisterRequest {
  string client_id = 1;
  string sdk_type = 2;
  string namespace = 3;
  repeated string subscriptions = 4;
  map<string, string> labels = 5;
}

message ConfigPullRequest {
  string client_id = 1;
  string namespace = 2;
  repeated string config_keys = 3;
  int64 client_version = 4;
}

message ConfigPushNotify {
  string request_id = 1;
  string namespace = 2;
  repeated ConfigItem configs = 3;
  int64 server_time = 4;
}

message ConfigItem {
  string group = 1;
  string data_id = 2;
  string content = 3;
  string format = 4;
  int32 version = 5;
  bool encrypted = 6;
  map<string, string> metadata = 7;
}
```

### 4.2 推送流程

```
┌────────┐   1.发布配置    ┌─────────────┐
│ Web UI │ ──────────────> │ QUIC Flow   │
└────────┘                 └──────┬──────┘
                                 │ 2.保存DB + 推送
                                 ▼
                          ┌─────────────┐
                          │ 匹配目标     │
                          │ 客户端       │
                          └──────┬──────┘
                                 │ 3.通过QUIC推送
                                 ▼
┌────────┐   4.config.push  ┌─────────────┐
│ SDK    │ <──────────────── │   Server    │
└────────┘                 └─────────────┘
    │
    │ 5.应用配置
    │
    │ 6.config.ack
    ───────────────────>
```

---

## 五、多语言 SDK 设计

### 5.1 SDK 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      Application Layer                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │  Config     │  │  Listener   │  │      Annotation         │  │
│  │  Service    │  │  Callback   │  │      (Java/Go)          │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       SDK Core Layer                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   Config    │  │   Cache     │  │      Watcher            │  │
│  │  Manager    │  │  (Local)    │  │    (Long Polling)       │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Transport Layer                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   QUIC      │  │   HTTP      │  │      Reconnect          │  │
│  │   Client    │  │   Fallback  │  │      Manager            │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 5.2 Go SDK 使用示例

```go
package main

import (
    "context"
    "github.com/quic-flow/config-sdk-go"
)

func main() {
    // 创建客户端
    client := config_sdk.NewClient(&config_sdk.ClientConfig{
        ServerAddr:    "localhost:8474",
        Namespace:     "production",
        AutoReconnect: true,
        EnableCache:   true,
    })

    ctx := context.Background()
    client.Start(ctx)
    defer client.Close()

    // 获取配置
    value := client.GetString("app", "database.url")
    fmt.Println("Database URL:", value)

    // 获取 JSON 配置
    type DBConfig struct {
        Host     string `json:"host"`
        Port     int    `json:"port"`
        Username string `json:"username"`
    }
    var dbConfig DBConfig
    client.GetJSON("app", "database", &dbConfig)

    // 监听配置变更
    watcher, _ := client.Listen(
        config_sdk.ConfigKey{Group: "app", DataID: "config.yaml"},
    )

    go func() {
        for {
            event, _ := watcher.Next(ctx)
            fmt.Printf("Config changed: %s.%s, Type: %v\n",
                event.Group, event.DataID, event.Type)

            if event.Type == config_sdk.ChangeTypeModified {
                reloadApplicationConfig()
            }
        }
    }()
}
```

### 5.3 Python SDK 使用示例

```python
from quic_flow_config import ConfigClient, ConfigWatcher

async def main():
    client = ConfigClient(
        server_addr="localhost:8474",
        namespace="production"
    )

    await client.start()

    # 获取配置
    db_url = client.get_string("app", "database.url")
    print(f"Database URL: {db_url}")

    # 监听配置变更
    watcher = client.listen(
        ConfigKey("app", "config.yaml")
    )

    async for event in watcher.events():
        print(f"Config changed: {event.group}.{event.data_id}")
        if event.new_value:
            print(f"New content: {event.new_value.content}")
```

### 5.4 Java SDK 使用示例

```java
public class Example {
    public static void main(String[] args) {
        ConfigClient client = new QuicFlowConfigClient.Builder()
            .serverAddr("localhost:8474")
            .namespace("production")
            .build();

        client.start();

        // 注解方式
        @ConfigValue(group = "app", dataId = "database.url")
        private String databaseUrl;

        // 监听配置变更
        @ConfigListener({
            @ConfigKey(group = "app", dataId = "database.url")
        })
        public void onConfigChange(ConfigChangeEvent event) {
            System.out.println("Config changed: " + event);
            reinitDatabase();
        }
    }
}
```

---

## 六、HTTP API 设计

### 6.1 配置 CRUD

```yaml
# 创建配置
POST /api/config
Request:
  namespace: string
  group: string
  data_id: string
  config_type: string
  content: string
  format: string
  tags?: string[]

# 更新配置
PUT /api/config/:id

# 删除配置
DELETE /api/config/:id

# 获取配置详情
GET /api/config/:id

# 列出配置
GET /api/config?namespace={ns}&group={group}
```

### 6.2 配置发布

```yaml
# 发布配置
POST /api/config/:id/release
Request:
  target_selector:
    client_ids?: string[]
    tags?: string[]
    ip_range?: string[]
    percentage?: int
  gray_rule_id?: uint
  comment?: string

# 查询发布状态
GET /api/config/release/:release_id

# 发布进度推送 (SSE)
GET /api/config/release/:release_id/events
```

### 6.3 灰度规则

```yaml
# 创建灰度规则
POST /api/config/:id/gray-rule
Request:
  rule_name: string
  rule_type: string  # tag | ip | client_id | percentage
  rule_value: string
  priority: int

# 列出灰度规则
GET /api/config/:id/gray-rules

# 启用/禁用灰度规则
PUT /api/config/:id/gray-rule/:rule_id
```

### 6.4 配置回滚

```yaml
# 回滚到指定版本
POST /api/config/:id/rollback
Request:
  to_version: int
  comment?: string

# 配置版本对比
GET /api/config/:id/diff?from_version={v1}&to_version={v2}
```

---

## 七、Web UI 界面设计

### 7.1 配置管理页面

```
┌─────────────────────────────────────────────────────────────┐
│ QUIC Flow - 配置中心                        [user] [logout] │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ ┌───────────┐  ┌─────────────────────────────────────────┐ │
│ │ 侧边栏     │  │ 主内容区                                │ │
│ │           │  │                                         │ │
│ │ 配置列表   │  │ [环境: ▼production] [分组: ▼全部]        │ │
│ │ 创建配置   │  │ [搜索配置...] [+ 新建配置] [导入] [导出] │ │
│ │ 发布管理   │  │                                         │ │
│ │ 灰度规则   │  │ ┌─────────────────────────────────────┐ │ │
│ │ 订阅者     │  │ │ 📄 app:config.yaml                   │ │ │
│ │ 变更历史   │  │ │    版本: 6 | 标签: [core] [prod]     │ │ │
│ │ 系统设置   │  │ │    更新: 5分钟前                      │ │ │
│ └───────────┘  │ │    [查看] [编辑] [发布] [历史] [删除] │ │ │
│               │ └─────────────────────────────────────┘ │ │
│               │                                         │ │
└───────────────┴─────────────────────────────────────────┘
```

### 7.2 配置编辑页面

```
┌─────────────────────────────────────────────────────────────┐
│ 编辑配置 - app:config.yaml                        [保存][取消]│
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ 基本信息                                                    │
│ 命名空间: production (只读)                                  │
│ 分组: app (只读)                                            │
│ DataID: config.yaml (只读)                                  │
│ 描述: [应用主配置文件                               ]       │
│ 标签: [core] [+ 添加标签]                                    │
│                                                             │
│ 配置内容                                                    │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ 1│ server:                                             │ │
│ │ 2│   port: 8080                                        │ │
│ │ 3│   host: 0.0.0.0                                      │ │
│ │ 4│                                                      │ │
│ │ 5│ database:                                           │ │
│ │ 6│   url: postgresql://localhost:5432/mydb             │ │
│ │ 7│   pool_size: 100                                    │ │
│ └─────────────────────────────────────────────────────────┘ │
│                                                             │
│ 版本对比                                                    │
│ 对比版本: [v5 (当前) ▼] → [v6 (编辑中) ▼]  [显示差异]       │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ -  port: 8080                                          │ │
│ │ +  port: 9090                                          │ │
│ └─────────────────────────────────────────────────────────┘ │
│                                                             │
│ 发布配置                                                    │
│ ○ 全量发布  ● 灰度发布  ○ 指定客户端                         │
│ 灰度规则: [按标签 ▼]                                         │
│ 标签: [test] [+ 添加标签]                                    │
│ 百分比: [10] %                                               │
│                                                             │
│                         [预览] [取消] [发布]                  │
└─────────────────────────────────────────────────────────────┘
```

---

## 八、实施计划

### 8.1 阶段划分

| 阶段 | 内容 | 预计工时 |
|-----|------|---------|
| **Phase 1** | 数据模型 + API 服务 | 5 天 |
| **Phase 2** | 配置推送引擎 | 5 天 |
| **Phase 3** | Go SDK 开发 | 3 天 |
| **Phase 4** | Python/Java SDK | 4 天 |
| **Phase 5** | Web UI 开发 | 5 天 |
| **Phase 6** | 测试与文档 | 3 天 |

**总预计工时**: ~25 天

### 8.2 优先级

| 功能 | 优先级 | 说明 |
|-----|-------|------|
| 配置 CRUD | 🔴 P0 | 基础功能 |
| 配置推送 | 🔴 P0 | 核心功能 |
| Go SDK | 🔴 P0 | 官方 SDK |
| 版本管理 | 🟡 P1 | 重要功能 |
| 灰度发布 | 🟡 P1 | 高级功能 |
| Python SDK | 🟡 P1 | 常用语言 |
| Web UI | 🟡 P1 | 用户体验 |
| 回滚功能 | 🟢 P2 | 运维必备 |
| Java SDK | 🟢 P2 | 企业应用 |
| 配置加密 | 🟢 P2 | 安全增强 |

---

## 九、附录

### 9.1 配置示例

```yaml
# 应用配置示例 (app:config.yaml)
server:
  port: 8080
  host: 0.0.0.0

database:
  url: postgresql://localhost:5432/mydb
  pool_size: 100
  timeout: 30s

cache:
  enabled: true
  ttl: 3600
  max_size: 10000

features:
  new_ui: true
  beta_api: false
```

```json
// 系统参数示例 (system:client.yaml)
{
  "heartbeat_interval": "15s",
  "heartbeat_timeout": "45s",
  "log_level": "info",
  "max_retries": 3,
  "reconnect_interval": "5s"
}
```

### 9.2 SDK 包名规划

| 语言 | 包名/模块名 |
|-----|-----------|
| Go | `github.com/quic-flow/config-sdk-go` |
| Python | `quic-flow-config` |
| Java | `com.quic.flow.config` |
| JavaScript | `@quic-flow/config-sdk` |

---

**文档结束**
