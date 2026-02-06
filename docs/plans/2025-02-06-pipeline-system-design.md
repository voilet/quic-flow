# M3: 可视化流水线系统设计文档

> **版本**: v1.0
> **日期**: 2025-02-06
> **状态**: 设计阶段

---

## 一、概述

### 1.1 项目背景

QUIC Flow 可视化流水线系统是一个支持**运维 + 开发双场景**的 CI/CD 平台：

- **运维场景 (Ops)**: 简单脚本执行、批量命令、发布部署
- **开发场景 (DevOps)**: 代码拉取、构建、测试、部署、通知

### 1.2 核心特性

- **可视化编排**: 拖拽式 DAG 编辑器
- **丰富任务类型**: Shell、容器、Git、QUIC、HTTP、通知、审批、条件、循环
- **灵活触发**: Webhook、定时任务、Git 事件、手动触发
- **实时监控**: SSE 实时推送执行状态
- **模板复用**: 预置运维和 CI/CD 模板

---

## 二、系统架构

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Pipeline 系统架构                                   │
│                                                                             │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                         Web UI 层                                       │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────────────┐ │  │
│  │  │  流水线编辑器│  │  可视化看板  │  │          执行历史               │  │  │
│  │  │  (拖拽编排)  │  │  (实时状态)  │  │         (日志回放)              │  │  │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────────────┘ │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                    │                                        │
│                                    ▼                                        │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                        API 服务层                                      │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────────────┐ │  │
│  │  │  Pipeline   │  │   Task      │  │          Execution              │  │  │
│  │  │    API      │  │   API       │  │           API                   │  │  │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────────────┘ │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                    │                                        │
│                                    ▼                                        │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                        编排引擎层                                       │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────────────┐ │  │
│  │  │  DAG 编译器  │  │  条件分支   │  │          并行调度               │  │  │
│  │  │  (解析DAG)  │  │  (if/else)  │  │         (Worker Pool)           │  │  │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────────────┘ │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                    │                                        │
│                                    ▼                                        │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                        任务执行层                                       │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────────────┐ │  │
│  │  │  Shell      │  │  Container  │  │          HTTP                   │  │  │
│  │  │  Executor   │  │  Executor   │  │          Executor               │  │  │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────────────┘ │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────────────┐ │  │
│  │  │  QUIC       │  │  Git        │  │          Notify                 │  │  │
│  │  │  Executor   │  │  Executor   │  │          Executor               │  │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────────────┘ │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 双场景设计

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          运维场景 (Ops)                                     │
│                                                                             │
│  简单脚本执行 → 批量命令 → 发布部署 → 监控告警                              │
│  📋 脚本任务     🔄 循环任务     📦 发布任务    📊 监控任务                  │
│                                                                             │
│  特点: 低代码、拖拽式、快速上手                                               │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                        开发场景 (DevOps)                                     │
│                                                                             │
│  代码拉取 → 构建 → 测试 → 部署 → 通知                                       │
│  🔀 Git任务     🏗️构建任务   🧪测试任务  🚀部署任务 📢通知任务             │
│                                                                             │
│  特点: CI/CD 完整流程、代码质量检查、集成第三方服务                            │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 三、核心数据模型

### 3.1 流水线定义

```go
// Pipeline 流水线定义
type Pipeline struct {
    ID          uint      `gorm:"primaryKey"`
    Name        string    `gorm:"size:128;uniqueIndex"`
    Description string    `gorm:"size:512"`
    Namespace   string    `gorm:"size:64;index"`
    Type        string    // ops | cicd
    Definition  string    `gorm:"type:text"`     // YAML 格式
    DAG         *PipelineDAG `gorm:"type:json"`
    Triggers    []Trigger  `gorm:"type:json"`
    Variables   []PipelineVariable `gorm:"type:json"`
    Resources   *Resources  `gorm:"type:json"`
    Enabled     bool       `gorm:"default:true"`
    Version     int        `gorm:"default:1"`
    CreatedBy   string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### 3.2 DAG 结构

```go
// PipelineDAG 流水线 DAG
type PipelineDAG struct {
    Name        string                    `json:"name"`
    Description string                    `json:"description"`
    Tasks       []PipelineTask            `json:"tasks"`
    Edges       []PipelineEdge            `json:"edges"`
    Config      *PipelineConfig           `json:"config"`
    Variables   map[string]string         `json:"variables"`
}

// PipelineTask 任务节点
type PipelineTask struct {
    ID          string                    `json:"id"`
    Name        string                    `json:"name"`
    Type        string                    `json:"type"`
    Description string                    `json:"description"`
    Config      map[string]interface{}    `json:"config"`
    When        string                    `json:"when"`
    RetryPolicy *RetryPolicy              `json:"retry_policy"`
    Timeout     time.Duration             `json:"timeout"`
    OnFailure   string                    `json:"on_failure"`
    Labels      map[string]string         `json:"labels"`
}

// PipelineEdge 依赖边
type PipelineEdge struct {
    From        string                    `json:"from"`
    To          string                    `json:"to"`
    Condition   string                    `json:"condition"`
}
```

### 3.3 执行实例

```go
// PipelineExecution 流水线执行
type PipelineExecution struct {
    ID          uint      `gorm:"primaryKey"`
    PipelineID  uint      `gorm:"index"`
    ExecutionID string    `gorm:"size:64;uniqueIndex"`
    TriggeredBy string    // manual | webhook | git_event | schedule
    TriggeredByUser string
    TriggerEvent map[string]interface{} `gorm:"type:json"`
    Inputs      map[string]string `gorm:"type:json"`
    Status      string    // pending | running | success | failed | cancelled
    StartedAt   time.Time
    CompletedAt *time.Time
    Duration    time.Duration
    Outputs     map[string]string `gorm:"type:json"`
    Error       string    `gorm:"type:text"`
    TaskExecutions []TaskExecution `gorm:"foreignKey:ExecutionID"`
}

// TaskExecution 任务执行
type TaskExecution struct {
    ID          uint      `gorm:"primaryKey"`
    ExecutionID uint      `gorm:"index"`
    TaskID      string
    Name        string
    Type        string
    Status      string    // pending | running | success | failed | cancelled
    StartedAt   *time.Time
    CompletedAt *time.Time
    Duration    time.Duration
    RetryCount  int
    MaxRetries  int
    Outputs     map[string]string `gorm:"type:json"`
    Logs        string    `gorm:"type:text"`
    Error       string    `gorm:"type:text"`
    ApprovalInfo *ApprovalInfo `gorm:"type:json"`
}
```

---

## 四、任务类型详解

### 4.1 Shell 任务

```yaml
type: shell
config:
  # 执行方式
  run_type: remote  # local | remote (QUIC)

  # 本地执行
  script: |
    #!/bin/bash
    echo "Hello World"

  # 远程执行
  target_selector:
    tags: [prod, web]
  script: |
    #!/bin/bash
    systemctl restart myapp
```

### 4.2 容器任务

```yaml
type: container
config:
  image: node:18-alpine
  command: [npm]
  args: [test]
  work_dir: /workspace
  env:
    NODE_ENV: test
  resources:
    limits:
      cpu: "500m"
      memory: "512Mi"
  volumes:
    - name: source
      source: ./src
      target: /workspace
```

### 4.3 Git 任务

```yaml
type: git
config:
  url: "https://github.com/myorg/repo.git"
  branch: main
  auth_type: token
  token: "${{GITHUB_TOKEN}}"
  depth: 100
  target_dir: ./src
```

### 4.4 QUIC 批量命令

```yaml
type: quic
config:
  command: "config.push"
  payload:
    version: "v1.0.0"
    files:
      - source: "./config/app.yaml"
        target: "/etc/myapp/app.yaml"
  target_selector:
    tags: [prod]
  max_concurrency: 50
  continue_on_error: true
```

### 4.5 HTTP 任务

```yaml
type: http
config:
  url: "https://api.example.com/health"
  method: GET
  headers:
    Authorization: "Bearer ${{TOKEN}}"
  expect_status: 200
  output_as: json
```

### 4.6 通知任务

```yaml
type: notify
config:
  channels: [dingtalk, email]
  template: deploy_result
  title: "部署完成"
  body: |
    流水线: {{ .pipeline.name }}
    状态: {{ .execution.status }}
  on_status: [success, failure]
```

### 4.7 人工审批

```yaml
type: approval
config:
  approvers: ["team-lead", "ops-manager"]
  timeout: 24h
  message: "请审批部署到生产环境"
  form_fields:
    - name: comment
      label: "审批意见"
      type: textarea
      required: true
```

### 4.8 条件分支

```yaml
type: condition
config:
  expression: "{{ .variables.git_branch }} == 'main'"
  cases:
    - condition: "true"
      task_ids: [deploy_prod]
    - condition: "false"
      task_ids: [deploy_dev]
  default: deploy_dev
```

### 4.9 循环任务

```yaml
type: loop
config:
  type: foreach
  items: ["service1", "service2", "service3"]
  tasks:
    - id: deploy
      type: quic
      config:
        command: "service.restart"
        payload:
          service: "{{ .item }}"
```

### 4.10 延迟任务

```yaml
type: delay
config:
  duration: 5m
  # 或
  until: "2025-01-10T14:00:00Z"
```

---

## 五、流水线 DSL (YAML)

### 5.1 运维场景示例

```yaml
name: "批量更新服务配置"
description: "向所有生产环境服务器推送配置更新"
type: ops
namespace: production

variables:
  - name: config_version
    type: string
    required: true

config:
  max_concurrent_tasks: 100
  timeout: 30m
  failure_strategy: continue

triggers:
  - type: webhook
    webhook:
      token: "${{WEBHOOK_TOKEN}}"
  - type: manual

tasks:
  - id: backup
    name: "备份当前配置"
    type: shell
    config:
      run_type: remote
      target_selector:
        tags: [prod, web]
      script: |
        #!/bin/bash
        BACKUP_DIR="/backup/config_$(date +%Y%m%d_%H%M%S)"
        mkdir -p "$BACKUP_DIR"
        cp -r /etc/myapp/* "$BACKUP_DIR/"

  - id: push_config
    name: "推送新配置"
    type: quic
    config:
      command: "config.push"
      payload:
        version: "{{ .variables.config_version }}"
      target_selector:
        tags: [prod, web]
    depends_on: [backup]

  - id: validate
    name: "验证配置"
    type: shell
    config:
      run_type: remote
      script: |
        yamllint /etc/myapp/app.yaml
        nginx -t
    depends_on: [push_config]
    when: "push_config.success_rate > 0.9"

  - id: restart
    name: "重启服务"
    type: quic
    config:
      command: "system.restart"
      payload:
        services: [myapp, nginx]
    depends_on: [validate]

  - id: health_check
    name: "健康检查"
    type: http
    config:
      url: "http://{{ .target_host }}:8080/health"
      expect_status: 200
    depends_on: [restart]
    retry_policy:
      enabled: true
      max_retries: 5

edges:
  - from: backup
    to: push_config
    condition: success
  - from: push_config
    to: validate
    condition: success
  - from: validate
    to: restart
    condition: success
  - from: restart
    to: health_check
    condition: success
```

### 5.2 CI/CD 场景示例

```yaml
name: "CI/CD 流水线"
description: "代码构建、测试、部署完整流程"
type: cicd
namespace: development

config:
  max_concurrent_tasks: 5
  timeout: 60m
  cache:
    enabled: true
    paths: [node_modules, .cache]

triggers:
  - type: git_event
    git_event:
      event_type: [push, pull_request]
      branch: [main, develop]

tasks:
  - id: checkout
    name: "代码检出"
    type: git
    config:
      url: "https://github.com/myorg/myrepo.git"
      branch: "{{ .variables.git_branch }}"

  - id: install
    name: "安装依赖"
    type: container
    config:
      image: node:18-alpine
      command: [npm]
      args: [ci]
    depends_on: [checkout]

  - id: lint
    name: "代码检查"
    type: container
    config:
      image: node:18-alpine
      command: [npm]
      args: [run, lint]
    depends_on: [install]

  - id: test
    name: "单元测试"
    type: container
    config:
      image: node:18-alpine
      command: [npm]
      args: [test, --, --coverage]
    depends_on: [install]

  - id: build
    name: "构建镜像"
    type: container
    config:
      image: docker:latest
      command: [docker]
      args: [build, -t, myapp:latest, .]
    depends_on: [lint, test]

  - id: push
    name: "推送镜像"
    type: container
    config:
      image: docker:latest
      command: [docker]
      args: [push, myapp:latest]
    depends_on: [build]
    when: ".variables.git_branch == 'main'"

  - id: approval
    name: "部署审批"
    type: approval
    config:
      approvers: ["team-lead"]
      timeout: 24h
    depends_on: [push]
    when: ".variables.git_branch == 'main'"

  - id: deploy_prod
    name: "部署到生产环境"
    type: quic
    config:
      command: "release.deploy"
      payload:
        project: myapp
        version: "latest"
        environment: prod
        strategy: canary
    depends_on: [approval]

  - id: notify
    name: "发送通知"
    type: notify
    config:
      channels: [slack]
    depends_on: [deploy_prod]
    when: always
```

---

## 六、HTTP API 设计

### 6.1 流水线管理

```yaml
# 创建流水线
POST /api/pipelines
Request:
  name: string
  description?: string
  type: string
  namespace: string
  definition: string
  enabled?: boolean

# 获取流水线详情
GET /api/pipelines/:id

# 列出流水线
GET /api/pipelines?type=ops&enabled=true

# 更新流水线
PUT /api/pipelines/:id

# 删除流水线
DELETE /api/pipelines/:id

# 启用/禁用流水线
PUT /api/pipelines/:id/toggle
```

### 6.2 流水线执行

```yaml
# 手动触发流水线
POST /api/pipelines/:id/execute
Request:
  inputs?: object
Response:
  execution_id: string

# 获取执行详情
GET /api/pipelines/executions/:execution_id

# 列出执行历史
GET /api/pipelines/:id/executions?status=success

# 取消执行
POST /api/pipelines/executions/:execution_id/cancel

# 重试执行
POST /api/pipelines/executions/:execution_id/retry

# 获取执行日志
GET /api/pipelines/executions/:execution_id/logs
```

### 6.3 SSE 实时推送

```yaml
# 订阅执行事件 (SSE)
GET /api/pipelines/executions/:execution_id/events
Response (text/event-stream):
  event: task_started
  data: {"task_id": "build", "name": "构建镜像", "status": "running"}

  event: task_completed
  data: {"task_id": "build", "status": "success", "duration": "2m30s"}

  event: execution_completed
  data: {"status": "success", "duration": "10m15s"}
```

### 6.4 模板管理

```yaml
# 创建模板
POST /api/pipeline/templates
Request:
  name: string
  category: string
  definition: string
  parameters: object[]

# 从模板创建流水线
POST /api/pipeline/templates/:template_id/instantiate
Request:
  name: string
  parameters: object
```

---

## 七、Web UI 界面设计

### 7.1 流水线列表

```
┌─────────────────────────────────────────────────────────────────────────┐
│ QUIC Flow - 流水线                                        [新建流水线]  │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│ [类型: 全部 ▼] [命名空间: ▼全部] [搜索...]                               │
│                                                                         │
│ ┌─────────────────────────────────────────────────────────────────────┐ │
│ │ 批量更新服务配置                                    ops  prod  [编辑] │ │
│ │ 向所有生产环境服务器推送配置更新                                       │ │
│ │ 📊 最近执行: 3天前 ✅ 成功 (15min)                                    │ │
│ ├─────────────────────────────────────────────────────────────────────┤ │
│ │ CI/CD 流水线                                       cicd  dev   [编辑] │ │
│ │ 代码构建、测试、部署完整流程                                          │ │
│ │ 📊 最近执行: 1小时前 ❌ 失败 (构建失败)                                 │ │
│ ├─────────────────────────────────────────────────────────────────────┤ │
│ │ 定时备份数据库                                     ops  prod  [编辑] │ │
│ │ 每天凌晨2点自动备份生产数据库                                          │ │
│ │ 📊 最近执行: 今天 02:00 ✅ 成功                                         │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.2 流水线编辑器

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 编辑流水线: 批量更新服务配置                               [保存] [取消]  │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│ 基本信息                                                                │
│ 名称: [批量更新服务配置                                    ]              │
│ 描述: [向所有生产环境服务器推送配置更新                    ]              │
│ 类型: [○ 运维场景  ● CI/CD 场景]                                          │
│                                                                         │
│ ┌───────────────────┐           ┌─────────────────────────────────────┐ │
│ │    任务组件库      │           │         画布区域 (DAG 编排)          │ │
│ │                   │           │                                     │ │
│ │ 基础任务           │           │  ┌──────┐                            │ │
│ │ 📋 Shell          │           │  │备份  │                            │ │
│ │ 🐳 Container      │           │  └──┬───┘                            │ │
│ │ 🔀 Git            │           │     │                                │ │
│ │ 🌐 HTTP           │           │     ▼                                │ │
│ │ 🚀 QUIC 批量命令   │           │  ┌──────┐                            │ │
│ │ 📢 通知           │           │  │推送  │                            │ │
│ │                   │           │  └──┬───┘                            │ │
│ │ 控制任务           │           │     │                                │ │
│ │ ✅ 人工审批        │           │     ▼                                │ │
│ │ 🔀 条件分支        │           │  ┌──────┐    ┌──────┐                │ │
│ │ 🔁 循环           │           │  │验证  │───→│重启  │                │ │
│ │ ⏱️ 延迟            │           │  └──────┘    └──────┘                │ │
│ │                   │           │     │                                │ │
│ │                   │           │     ▼                                │ │
│ │                   │           │  ┌──────┐                            │ │
│ │                   │           │  │健康  │                            │ │
│ │                   │           │  │检查  │                            │ │
│ │                   │           │  └──────┘                            │ │
│ │                   │           │                                     │ │
│ │                   │           │ 拖拽组件到画布                        │ │
│ │                   │           │ 连线配置依赖关系                       │ │
│ └───────────────────┘           └─────────────────────────────────────┘ │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.3 执行详情页面

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 执行详情 #1234                                              [返回] [重试] │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│ 状态: ✅ 成功   耗时: 15min   触发方式: 手动触发                               │
│ 触发者: admin   开始时间: 2025-01-10 10:30:00                               │
│                                                                         │
│ ┌─────────────────────────────────────────────────────────────────────┐ │
│ │ 任务执行进度                                                           │ │
│ │ ┌──────────────┐     ┌──────────────┐     ┌──────────────┐          │ │
│ │ │ ✅ 备份      │ ──→ │ ✅ 推送      │ ──→ │ ✅ 验证      │          │ │
│ │ │ 30s         │     │ 5min         │     │ 2min         │          │ │
│ │ └──────────────┘     └──────────────┘     └──────────────┘          │ │
│ │                           │                                        │   │
│ │                           ▼                                        │   │
│ │                   ┌──────────────┐     ┌──────────────┐              │   │
│ │                   │ ✅ 重启      │ ──→ │ ✅ 健康检查  │              │   │
│ │                   │ 3min         │     │ 5min         │              │   │
│ │                   └──────────────┘     └──────────────┘              │   │
│ └─────────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│ ┌─────────────────────────────────────────────────────────────────────┐ │
│ │ 日志输出                                        [实时更新: 开] [清空]  │ │
│ │ ┌───────────────────────────────────────────────────────────────────┐│ │
│ │ │ [10:30:00] 开始执行任务: 备份                                     ││ │
│ │ │ [10:30:01] 在 server1 上执行备份...                              ││ │
│ │ │ [10:30:02] 备份完成: /backup/config_20250110_103001              ││ │
│ │ │ [10:30:02] ✅ 任务完成                                          ││ │
│ │ │ [10:30:03] 开始执行任务: 推送                                    ││ │
│ │ │ [10:30:04] 向 10 个客户端推送配置...                             ││ │
│ │ │ [10:30:08] ✓ client-001 推送成功                                ││ │
│ │ │ [10:30:09] ✓ client-002 推送成功                                ││ │
│ │ │ ...                                                            ││ │
│ │ └───────────────────────────────────────────────────────────────────┘│ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 八、实施计划

### 8.1 阶段划分

| 阶段 | 内容 | 预计工时 |
|-----|------|---------|
| **Phase 1** | 数据模型 + API 服务 | 4 天 |
| **Phase 2** | 编排引擎 + DAG 解析器 | 5 天 |
| **Phase 3** | 任务执行器 (Shell/QUIC) | 4 天 |
| **Phase 4** | 任务执行器 (Container/Git) | 3 天 |
| **Phase 5** | 其他任务类型 | 3 天 |
| **Phase 6** | Web UI (编辑器) | 5 天 |
| **Phase 7** | Web UI (执行看板) | 3 天 |
| **Phase 8** | 测试与文档 | 3 天 |

**总预计工时**: ~30 天

### 8.2 优先级

| 功能 | 优先级 | 说明 |
|-----|-------|------|
| 数据模型 | 🔴 P0 | 基础 |
| API 服务 | 🔴 P0 | 接口 |
| DAG 编排 | 🔴 P0 | 核心 |
| Shell 任务 | 🔴 P0 | 运维核心 |
| QUIC 任务 | 🔴 P0 | 运维核心 |
| 基础 UI | 🟡 P1 | 用户体验 |
| Git 任务 | 🟡 P1 | CI/CD 核心 |
| 容器任务 | 🟡 P1 | CI/CD 核心 |
| HTTP 任务 | 🟡 P1 | 通用 |
| 通知任务 | 🟡 P1 | 必备 |
| 审批任务 | 🟢 P2 | 企业需求 |
| 条件分支 | 🟢 P2 | 高级功能 |
| 可视化编辑器 | 🟢 P2 | 增强 |

---

## 九、附录

### 9.1 预置模板列表

**运维模板:**
- 批量执行命令
- 配置文件分发
- 服务滚动更新
- 定时数据备份
- 健康检查巡检
- 日志收集分析

**CI/CD 模板:**
- Go 语言 CI/CD
- Node.js CI/CD
- Python CI/CD
- Java (Maven) CI/CD
- Docker 镜像构建
- Kubernetes 部署

### 9.2 任务执行器配置参考

```yaml
# Shell 执行器配置
shell_executor:
  work_dir: /tmp/pipeline
  shell: /bin/bash
  timeout: 30m
  env:
    PATH: /usr/local/bin:/usr/bin:/bin

# Container 执行器配置
container_executor:
  runtime: docker
  default_network: bridge
  volume_driver: local
  registry: docker.io

# QUIC 执行器配置
quic_executor:
  max_concurrency: 100
  timeout: 10m
  retry_policy:
    max_retries: 3
    interval: 30s
```

---

**文档结束**
