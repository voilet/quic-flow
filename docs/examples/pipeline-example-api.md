# 流水线用例测试指南

本文档说明如何使用流水线示例进行测试，包括前端、后端和 Agent 的协同执行。

## 目录

1. [前置准备](#前置准备)
2. [参数配置](#参数配置)
3. [通过 API 创建流水线](#通过-api-创建流水线)
4. [通过前端创建流水线](#通过前端创建流水线)
5. [执行流水线](#执行流水线)
6. [监控执行状态](#监控执行状态)
7. [Agent 执行流程](#agent-执行流程)

---

## 前置准备

### 1. 启动服务

确保以下服务已启动：

```bash
# 启动后端服务
cd cmd/server
go run main.go

# 启动前端服务
cd web
npm run dev

# 确保 Agent 已连接并注册
```

### 2. 准备测试环境

创建一个测试项目（如果还没有）：

```bash
curl -X POST http://localhost:8080/api/release/projects \
  -H "Content-Type: application/json" \
  -d '{
    "name": "测试项目",
    "type": "custom",
    "description": "用于测试流水线功能"
  }'
```

---

## 参数配置

在创建流水线之前，需要准备以下参数：

### 参数模板

```json
{
  "app": {
    "name": "demo-app"
  },
  "deployment": {
    "package_url": "https://example.com/releases/demo-app-1.0.0.tar.gz",
    "checksum": "sha256:abc123...",
    "version": "1.0.0",
    "dashboard_url": "https://dashboard.example.com/deploy/123"
  },
  "env": {
    "name": "staging",
    "target_hosts": ["192.168.1.100:8080", "192.168.1.101:8080"]
  },
  "db": {
    "host": "localhost",
    "port": 5432,
    "name": "demo_db"
  },
  "notification": {
    "webhook_url": "https://hooks.example.com/deploy",
    "email": "team@example.com"
  },
  "user": {
    "name": "admin"
  }
}
```

---

## 通过 API 创建流水线

### 1. 创建流水线

```bash
curl -X POST http://localhost:8080/api/release/projects/{PROJECT_ID}/pipelines \
  -H "Content-Type: application/json" \
  -d @pipeline-example-deployment.yaml
```

或者直接传递 JSON 格式的流水线定义：

```bash
curl -X POST http://localhost:8080/api/release/projects/{PROJECT_ID}/pipelines \
  -H "Content-Type: application/json" \
  -d '{
    "name": "标准应用部署流水线",
    "description": "包含健康检查、备份、部署、验证和通知的完整部署流程",
    "type": "deploy",
    "stages": [
      {
        "name": "预发布检查",
        "phase": "pre_release",
        "on_error": "stop",
        "parallel": false,
        "tasks": [
          {
            "id": "pre-health-check",
            "name": "目标环境健康检查",
            "type": "health_check",
            "timeout": 30,
            "retry": 3,
            "retry_delay": 5,
            "config": {
              "hosts": ["{{ .env.target_hosts }}"],
              "ports": [8080, 8443]
            }
          }
        ]
      }
    ]
  }'
```

### 2. 获取流水线列表

```bash
curl http://localhost:8080/api/release/projects/{PROJECT_ID}/pipelines
```

---

## 通过前端创建流水线

### 步骤 1：进入项目工作台

1. 打开浏览器访问 `http://localhost:5173`
2. 选择测试项目进入工作台

### 步骤 2：进入流水线编辑器

1. 点击左侧菜单「流水线」→「流水线编辑器」
2. 或者点击「模板管理」→「从模板创建」

### 步骤 3：导入 YAML 配置

1. 点击「导入 YAML」按钮
2. 粘贴 `pipeline-example-deployment.yaml` 的内容
3. 点击「确认导入」

### 步骤 4：配置参数

1. 点击「参数配置」标签
2. 填写所有必填参数
3. 点击「保存」

---

## 执行流水线

### 通过 API 执行

```bash
curl -X POST http://localhost:8080/api/release/pipelines/{PIPELINE_ID}/execute \
  -H "Content-Type: application/json" \
  -d '{
    "parameters": {
      "app.name": "demo-app",
      "deployment.version": "1.0.0",
      "env.name": "staging",
      "env.target_hosts": ["192.168.1.100:8080"]
    }
  }'
```

### 通过前端执行

1. 在流水线列表页面找到要执行的流水线
2. 点击「执行」按钮
3. 填写执行参数
4. 点击「确认执行」

---

## 监控执行状态

### 1. 获取执行记录

```bash
curl http://localhost:8080/api/release/pipelines/{PIPELINE_ID}/executions
```

### 2. 获取执行详情

```bash
curl http://localhost:8080/api/release/executions/{EXECUTION_ID}
```

### 3. 查看 WebSocket 实时日志

前端使用 WebSocket 连接获取实时日志：

```javascript
const ws = new WebSocket(`ws://localhost:8080/api/release/executions/${EXECUTION_ID}/logs`);

ws.onmessage = (event) => {
  const log = JSON.parse(event.data);
  console.log(`[${log.stage}] ${log.task}: ${log.message}`);
};
```

---

## Agent 执行流程

### 1. Agent 接收任务

当流水线执行时，后端会通过 QUIC 连接向 Agent 发送任务：

```json
{
  "task_id": "pre-health-check",
  "task_type": "health_check",
  "config": {
    "hosts": ["192.168.1.100:8080"],
    "ports": [8080, 8443],
    "expected_status": 200,
    "request_timeout": 10
  },
  "timeout": 30,
  "retry": 3
}
```

### 2. Agent 执行任务

Agent 根据任务类型执行相应的操作：

#### 健康检查任务

```go
func ExecuteHealthCheck(config HealthCheckConfig) error {
    for _, host := range config.Hosts {
        resp, err := http.Get(host + "/health")
        if err != nil || resp.StatusCode != config.ExpectedStatus {
            return fmt.Errorf("health check failed for %s", host)
        }
    }
    return nil
}
```

#### 脚本执行任务

```go
func ExecuteScript(script string) error {
    cmd := exec.Command("bash", "-c", script)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("script execution failed: %s", output)
    }
    return nil
}
```

### 3. Agent 返回结果

Agent 将执行结果发送回后端：

```json
{
  "task_id": "pre-health-check",
  "status": "success",
  "output": "Health check passed for all hosts",
  "started_at": "2025-01-06T10:00:00Z",
  "completed_at": "2025-01-06T10:00:05Z",
  "duration_ms": 5000
}
```

### 4. 后端处理结果

后端根据任务结果决定下一步操作：

- **成功**：继续执行下一个任务
- **失败**：根据 `on_error` 配置决定是停止、继续还是回滚
- **重试**：如果配置了重试，会重新发送任务给 Agent

---

## 完整执行流程图

```
┌─────────────┐
│  前端发起   │
│  执行请求   │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  后端创建   │
│  执行记录   │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  解析流水线 │
│  创建任务图 │
└──────┬──────┘
       │
       ▼
┌─────────────┐     ┌─────────────┐
│  发送任务   │────▶│  Agent接收  │
│  给Agent    │     │  并执行     │
└─────────────┘     └──────┬──────┘
       ▲                    │
       │                    ▼
       │              ┌─────────────┐
       │              │  返回执行   │
       │              │  结果       │
       │              └──────┬──────┘
       │                     │
       │                     ▼
       │              ┌─────────────┐
       │              │  后端处理   │
       │              │  结果       │
       │              └──────┬──────┘
       │                     │
       │                     ▼
       │              ┌─────────────┐
       │              │  推送日志   │
       │              │  到前端     │
       │              └──────┬──────┘
       │                     │
       │                     ▼
       │              ┌─────────────┐
       │              │  前端展示   │
       │              │  实时状态   │
       └──────────────┴─────────────┘
              所有任务完成
```

---

## 测试检查清单

执行流水线后，验证以下功能：

- [ ] 前端能够显示流水线列表
- [ ] 前端能够创建和编辑流水线
- [ ] 前端能够发起执行请求
- [ ] 后端能够创建执行记录
- [ ] 后端能够正确解析流水线定义
- [ ] Agent 能够接收并执行任务
- [ ] Agent 能够返回执行结果
- [ ] 后端能够处理任务依赖关系
- [ ] 后端能够处理错误和重试
- [ ] 前端能够实时显示执行状态
- [ ] 前端能够查看执行日志

---

## 常见问题

### Q1: Agent 没有收到任务？

检查 Agent 是否已连接并注册：

```bash
curl http://localhost:8080/api/clients
```

### Q2: 任务执行失败？

查看执行日志：

```bash
curl http://localhost:8080/api/release/executions/{EXECUTION_ID}/logs
```

### Q3: 如何调试流水线？

在流水线编辑器中点击「验证」按钮，检查配置是否正确。

---

## 附录：完整 API 列表

```
POST   /api/release/projects/{project_id}/pipelines          创建流水线
GET    /api/release/projects/{project_id}/pipelines          获取流水线列表
GET    /api/release/pipelines/{pipeline_id}                  获取流水线详情
PUT    /api/release/pipelines/{pipeline_id}                  更新流水线
DELETE /api/release/pipelines/{pipeline_id}                  删除流水线

POST   /api/release/pipelines/{pipeline_id}/execute          执行流水线
GET    /api/release/pipelines/{pipeline_id}/executions       获取执行记录
GET    /api/release/executions/{execution_id}                获取执行详情
GET    /api/release/executions/{execution_id}/logs           获取执行日志
DELETE /api/release/executions/{execution_id}                取消执行

WS     /api/release/executions/{execution_id}/logs/stream    WebSocket 日志流
```
