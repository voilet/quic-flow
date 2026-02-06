# M1: 实时告警系统设计文档

> **版本**: v1.0
> **日期**: 2025-02-06
> **状态**: 设计阶段

---

## 一、概述

### 1.1 项目背景

QUIC Flow 实时告警系统是一个功能完善的监控告警平台，支持：

- **灵活的规则引擎**: 基于 CEL 表达式的强大规则定义
- **多渠道通知**: 钉钉、企业微信、飞书、Slack、邮件等
- **智能告警处理**: 分组聚合、抑制规则、路由分发
- **值班轮换**: 支持复杂的值班表配置和通知偏好
- **实时推送**: 基于 SSE 的实时告警事件推送

### 1.2 设计目标

- **功能完善**: 多渠道通知 + 告警分组 + 抑制规则
- **易于使用**: 友好的 Web UI 和 API
- **高可靠**: 告警不丢失、通知必达
- **可扩展**: 支持自定义通知渠道和规则

---

## 二、系统架构

### 2.1 整体架构

```
┌──────────────────────────────────────────────────────────────────────────┐
│                        QUIC Flow 告警系统                                  │
│                                                                            │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │                         告警规则引擎                                 │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────────────┐│  │
│  │  │   规则管理   │  │   规则解析   │  │          表达式引擎             ││  │
│  │  │   CRUD      │  │  (Parser)   │  │       (CEL/Golang)            ││  │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────────────┘│  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                     │                                      │
│                                     ▼                                      │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │                         告警评估引擎                                 │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────────────┐│  │
│  │  │   指标采集   │  │   条件评估   │  │          持续时间检测           ││  │
│  │  │  (Collector)│  │  (Evaluator)│  │         (For Duration)          ││  │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────────────┘│  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                     │                                      │
│                                     ▼                                      │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │                         告警处理引擎                                 │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────────────┐│  │
│  │  │   分组聚合   │  │   抑制规则   │  │          路由分发               ││  │
│  │  │ (Grouping)  │  │ (Silencing) │  │         (Routing)               ││  │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────────────┘│  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                     │                                      │
│                                     ▼                                      │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │                         通知发送引擎                                 │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────────────┐│  │
│  │  │  Webhook    │  │   邮件      │  │          即时通讯               ││  │
│  │  │  (通用)     │  │  (SMTP)     │  │   (钉钉/企微/飞书/Slack)        ││  │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────────────┘│  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

---

## 三、核心数据模型

### 3.1 告警规则

```go
// AlertRule 告警规则
type AlertRule struct {
    ID          uint      `gorm:"primaryKey"`
    Name        string    `gorm:"size:128;uniqueIndex"`
    Description string    `gorm:"size:512"`
    Enabled     bool      `gorm:"default:true"`
    Priority    int       `gorm:"default:0"`

    // 规则定义 (CEL 表达式)
    Condition   string    `gorm:"type:text"`
    ForDuration time.Duration
    Severity    string    // critical | warning | info

    // 标签和注解
    Labels      map[string]string `gorm:"type:json"`
    Annotations map[string]string `gorm:"type:json"`

    // 通知配置
    NotifyChannels []uint  `gorm:"type:json"`
    NotifyGroup     string  `gorm:"size:64"`

    // 统计
    TriggeredCount int
    LastTriggered  *time.Time

    CreatedBy   string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### 3.2 告警实例

```go
// AlertInstance 告警实例
type AlertInstance struct {
    ID          uint      `gorm:"primaryKey"`
    RuleID      uint      `gorm:"index"`
    RuleName    string
    Status      string    // firing | resolved | silenced
    Severity    string
    Labels      map[string]string `gorm:"type:json"`
    Annotations map[string]string `gorm:"type:json"`
    Summary     string    `gorm:"size:512"`
    Description string    `gorm:"type:text"`
    StartedAt   time.Time
    FiredAt     time.Time
    ResolvedAt  *time.Time
    MetricValues map[string]float64 `gorm:"type:json"`
    Notified    bool      `gorm:"default:false"`
    NotifyCount int       `gorm:"default:0"`
    Fingerprint string    `gorm:"size:64;index"`
    GroupKey    string    `gorm:"size:128;index"`
}
```

### 3.3 抑制规则

```go
// SilenceRule 抑制规则
type SilenceRule struct {
    ID          uint      `gorm:"primaryKey"`
    Name        string    `gorm:"size:128"`
    Comment     string    `gorm:"size:512"`
    RuleID      *uint     `gorm:"index"`
    MatchLabels map[string]string `gorm:"type:json"`
    MatchRegex  map[string]string `gorm:"type:json"`
    StartAt     time.Time
    EndAt       time.Time
    Enabled     bool      `gorm:"default:true"`
    CreatedBy   string
    CreatedAt   time.Time
}
```

### 3.4 通知渠道

```go
// NotifyChannel 通知渠道
type NotifyChannel struct {
    ID          uint      `gorm:"primaryKey"`
    Name        string    `gorm:"size:128;uniqueIndex"`
    Type        string    // webhook | email | dingtalk | wechat | feishu | slack
    Config      map[string]interface{} `gorm:"type:json"`
    Enabled     bool      `gorm:"default:true"`
    MaxRetries  int       `gorm:"default:3"`
    RetryInterval time.Duration
    CreatedBy   string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### 3.5 值班管理

```go
// OnCallSchedule 值班表
type OnCallSchedule struct {
    ID          uint      `gorm:"primaryKey"`
    Name        string    `gorm:"size:128"`
    Description string    `gorm:"size:512"`
    TimeZone    string    `gorm:"size:64"`
    Config      map[string]interface{} `gorm:"type:json"`
    CurrentOnCall string `gorm:"size:128"`
    Enabled     bool      `gorm:"default:true"`
}

// OnCallUser 值班用户
type OnCallUser struct {
    ID          uint      `gorm:"primaryKey"`
    Name        string    `gorm:"size:128"`
    Email       string    `gorm:"size:256"`
    Phone       string    `gorm:"size:64"`
    NotifyChannels []string `gorm:"type:json"`
    Constraints  map[string]interface{} `gorm:"type:json"`
}
```

---

## 四、告警规则表达式

### 4.1 CEL 表达式语法

```
# ========== 简单比较 ==========

# 数值比较
cpu_usage > 80
memory_usage >= 90
disk_free < 10
error_rate > 0.05

# 字符串比较
status == "error"
level != "info"

# ========== 复合条件 ==========

# 逻辑运算
cpu_usage > 80 && memory_usage > 70
cpu_usage > 90 || memory_usage > 90
!(status == "ok")

# ========== 范围匹配 ==========

# 列表匹配
status in ["error", "timeout", "refused"]
code in [500, 502, 503, 504]

# 范围匹配
response_time in (200, 1000]  # 200 < x <= 1000

# ========== 正则匹配 ==========

message =~ ".*error.*"
url =~ "^https?://.*"

# ========== 函数调用 ==========

# 聚合函数
avg(cpu_usage, 5m) > 80        # 5分钟内平均值
max(memory_usage, 10m) > 90     # 10分钟内最大值
count(errors, 1m) > 10          # 1分钟内错误计数

# 变化率检测
rate(cpu_usage, 5m) > 0.5      # 5分钟内变化率
delta(disk_free, 1h) < 0       # 1小时内变化量

# ========== 标签查询 ==========

{service="api", env="prod"}
{service=~"api.*"}
cpu_usage > 80 && {service="api"}
```

### 4.2 规则示例

```yaml
# CPU 告警规则
name: "CPU过高告警"
condition: "avg(cpu_usage, 5m) > 80"
for_duration: "5m"
severity: "warning"
labels:
  service: "api"
  env: "prod"
annotations:
  summary: "服务器 CPU 使用率过高"
  description: "{{ $labels.host }} CPU 使用率超过 80%，当前 {{ $values.cpu_usage }}%"
  runbook: "https://docs.example.com/runbooks/high-cpu"

# 内存告警规则
name: "内存不足告警"
condition: "memory_usage > 90 && {env='prod'}"
for_duration: "3m"
severity: "critical"
labels:
  env: "prod"
annotations:
  summary: "生产环境内存不足"

# 接口错误率告警
name: "API 错误率过高"
condition: "rate(http_errors, 5m) > 0.05"
for_duration: "2m"
severity: "critical"
labels:
  service: "api"
annotations:
  summary: "API 错误率超过 5%"
```

---

## 五、告警处理流程

### 5.1 完整生命周期

```
┌──────────────┐
│  指标数据源   │
│ - Prometheus │
│ - QUIC 客户端 │
│ - 自定义上报  │
└──────┬───────┘
       │
       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. 规则匹配阶段                                                  │
│ - 遍历启用的告警规则                                             │
│ - 解析条件表达式                                                  │
│ - 评估当前指标值                                                  │
│ - 匹配标签                                                        │
└─────────────────────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. 持续时间检测                                                  │
│ - 检查条件是否持续满足 ForDuration                                │
│ - 例如: 5分钟内持续 CPU > 80%                                     │
└─────────────────────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. 告警分组与去重                                                │
│ - 计算告警指纹 (Fingerprint)                                      │
│ - 相同 RuleID + 相同关键标签 → 同一告警                           │
└─────────────────────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. 抑制规则检查                                                  │
│ - 检查是否有匹配的抑制规则                                        │
│ - 维护窗口、特定环境抑制等                                        │
└─────────────────────────────────────────────────────────────────┘
       │
       ▼ (未被抑制)
┌─────────────────────────────────────────────────────────────────┐
│ 5. 通知路由                                                      │
│ - 根据规则配置路由到不同接收者                                    │
│ - 查询值班轮换表                                                  │
└─────────────────────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 6. 通知发送                                                      │
│ - 渲染通知模板                                                    │
│ - 多渠道并发发送                                                  │
│ - 发送失败重试                                                    │
└─────────────────────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 7. 告警解决                                                      │
│ - 条件不再满足                                                    │
│ - 状态从 firing 变为 resolved                                     │
│ - 发送解决通知                                                    │
└─────────────────────────────────────────────────────────────────┘
```

---

## 六、通知模板设计

### 6.1 钉钉通知

```go
type DingTalkAlertMessage struct {
    MsgType string `json:"msgtype"`
    Text struct {
        Content string `json:"content"`
        At struct {
            AtMobiles []string `json:"atMobiles"`
            IsAtAll   bool     `json:"isAtAll"`
        } `json:"at"`
    } `json:"text"`
}

// 渲染示例:
// 【🚨 CRITICAL 告警】
//
// 告警名称: CPU过高
// 严重程度: 严重
//
// 告警详情:
// server1 CPU使用率超过80%，当前85%
//
// 标签:
// - host: server1
// - service: api
//
// 开始时间: 2025-01-10 10:30:00
// 持续时长: 15分钟
//
// @13800138000
```

### 6.2 企业微信通知

```go
type WeChatAlertMessage struct {
    MsgType string `json:"msgtype"`
    Text struct {
        Content     string   `json:"content"`
        MentionedList []string `json:"mentioned_list"`
    } `json:"text"`
}

// 渲染示例:
// <font color="warning">【严重告警】</font>
//
// >告警名称: CPU过高
// >严重程度: <font color="warning">严重</font>
// >
// >告警详情:
// server1 CPU使用率超过80%，当前85%
// >
// >开始时间: 2025-01-10 10:30:00
//
// <@user1>
```

### 6.3 邮件通知

```html
Subject: [🚨 CRITICAL] CPU过高 - server1

<html>
<body>
  <h2 style="color: #d32f2f;">🚨 严重告警</h2>

  <table>
    <tr><td><strong>告警名称:</strong></td><td>CPU过高</td></tr>
    <tr><td><strong>严重程度:</strong></td><td><span style="color: #d32f2f;">严重</span></td></tr>
  </table>

  <h3>告警详情</h3>
  <p>server1 CPU使用率超过80%，当前85%</p>

  <h3>标签</h3>
  <ul>
    <li>host: server1</li>
    <li>service: api</li>
    <li>env: prod</li>
  </ul>

  <p><a href="https://quic-flow.example.com/alerts/123">查看详情</a></p>
</body>
</html>
```

---

## 七、HTTP API 设计

### 7.1 告警规则

```yaml
# 创建告警规则
POST /api/alert/rules
Request:
  name: string
  condition: string              # CEL 表达式
  for_duration: string           # e.g., "5m"
  severity: string               # critical | warning | info
  labels: object
  annotations: object
  notify_channels: number[]

# 获取告警规则详情
GET /api/alert/rules/:id

# 列出告警规则
GET /api/alert/rules?enabled=true&severity=critical

# 启用/禁用告警规则
PUT /api/alert/rules/:id/toggle

# 测试告警规则
POST /api/alert/rules/:id/test
Request:
  metrics: object  # 模拟的指标值
```

### 7.2 告警实例

```yaml
# 获取活跃告警列表
GET /api/alerts?status=firing&severity=critical

# 获取告警详情
GET /api/alerts/:id

# 解决告警
POST /api/alerts/:id/resolve
Request:
  comment?: string

# 抑制告警
POST /api/alerts/:id/silence
Request:
  duration: string
  comment?: string
```

### 7.3 通知渠道

```yaml
# 创建通知渠道
POST /api/alert/channels
Request:
  name: string
  type: string
  config: object

# 测试通知渠道
POST /api/alert/channels/:id/test
Request:
  alert: object

# 列出通知渠道
GET /api/alert/channels
```

### 7.4 SSE 实时推送

```yaml
# 订阅告警事件 (SSE)
GET /api/alert/events
Response (text/event-stream):
  # 新告警触发
  event: firing
  data: {
    "id": 123,
    "rule_name": "CPU过高",
    "severity": "critical",
    "summary": "server1 CPU使用率85%"
  }

  # 告警解决
  event: resolved
  data: {
    "id": 123,
    "duration": "15m"
  }
```

---

## 八、实施计划

### 8.1 阶段划分

| 阶段 | 内容 | 预计工时 |
|-----|------|---------|
| **Phase 1** | 数据模型 + 规则引擎 | 4 天 |
| **Phase 2** | 告警评估 + 处理引擎 | 3 天 |
| **Phase 3** | 通知渠道实现 | 3 天 |
| **Phase 4** | 值班轮换功能 | 2 天 |
| **Phase 5** | Web UI 开发 | 3 天 |
| **Phase 6** | 测试与文档 | 2 天 |

**总预计工时**: ~17 天

### 8.2 优先级

| 功能 | 优先级 | 说明 |
|-----|-------|------|
| 规则引擎 | 🔴 P0 | 核心功能 |
| 告警评估 | 🔴 P0 | 核心功能 |
| 钉钉通知 | 🔴 P0 | 国内主流 |
| 企业微信通知 | 🔴 P0 | 国内主流 |
| 邮件通知 | 🟡 P1 | 基础渠道 |
| 抑制规则 | 🟡 P1 | 重要功能 |
| 飞书通知 | 🟡 P1 | 国内主流 |
| 值班轮换 | 🟡 P1 | 企业需求 |
| Slack 通知 | 🟢 P2 | 国际化 |
| 告警分组 | 🟢 P2 | 高级功能 |

---

## 九、附录

### 9.1 CEL 表达式快速参考

```
# 比较运算符
==, !=, <, >, <=, >=

# 逻辑运算符
&&, ||, !

# 成员运算符
in, not in

# 正则匹配
=~ (匹配), !~ (不匹配)

# 聚合函数
avg(metric, duration)    # 平均值
max(metric, duration)    # 最大值
min(metric, duration)    # 最小值
sum(metric, duration)    # 求和
count(metric, duration)  # 计数

# 变化率函数
rate(metric, duration)   # 变化率
delta(metric, duration)  # 变化量
```

### 9.2 通知渠道配置示例

```yaml
# 钉钉
dingtalk:
  webhook: "https://oapi.dingtalk.com/robot/send?access_token=xxx"
  secret: "SEC***"
  at_mobiles: ["13800138000"]
  at_all: false

# 企业微信
wechat:
  webhook: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
  mentioned_list: ["user1", "user2"]

# 邮件
email:
  smtp_host: "smtp.example.com"
  smtp_port: 587
  username: "alert@example.com"
  password: "***"
  from: "alert@example.com"
  to: ["team@example.com"]
```

---

**文档结束**
