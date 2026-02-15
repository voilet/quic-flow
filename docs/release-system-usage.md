# 发布系统使用指南

## 快速开始

### 1. 导入测试数据

```bash
# 进入项目目录
cd /Users/voilet/project/go_prod/src/quic-flow

# 执行测试数据脚本（需要 PostgreSQL 客户端）
psql -h localhost -U postgres -d quic_flow -f scripts/seed_release_data.sql

# 或者使用环境变量
PGHOST=localhost PGUSER=postgres PGDATABASE=quic_flow psql -f scripts/seed_release_data.sql
```

### 2. 测试数据说明

脚本创建的测试数据包括：

| 类型 | 项目数 | 版本数 | 说明 |
|------|--------|--------|------|
| 脚本部署 | 2 | 3 | 传统 Shell 脚本部署 |
| 容器部署 | 3 | 5 | Docker 容器部署 |
| K8s 部署 | 2 | 3 | Kubernetes 集群部署 |
| Git 拉取 | 2 | 4 | Git 仓库拉取部署 |

---

## 系统架构

### 核心模型关系

```
┌─────────────────────────────────────────────────────────────────┐
│                        Project (项目)                            │
│  type: script | container | kubernetes | gitpull                │
├─────────────────────────────────────────────────────────────────┤
│  ├── script_config      (脚本部署配置)                            │
│  ├── container_config   (容器部署配置)                            │
│  ├── kubernetes_config  (K8s 部署配置)                            │
│  ├── gitpull_config     (Git 拉取配置)                            │
│  │                                                               │
│  └── Version[] (版本列表)                                        │
│       ├── version: "v1.0.0"                                      │
│       ├── install_script / update_script / ...                  │
│       ├── container_image / container_env                        │
│       └── deploy_config (新版统一配置)                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     DeployTask (部署任务)                        │
├─────────────────────────────────────────────────────────────────┤
│  ├── project_id, version_id                                      │
│  ├── client_ids: ["client-001", ...]                            │
│  ├── operation: deploy | install | update | rollback | uninstall│
│  ├── canary_enabled, canary_percent (金丝雀配置)                 │
│  └── status: pending | running | canary | completed | failed    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      DeployLog (部署日志)                        │
├─────────────────────────────────────────────────────────────────┤
│  ├── task_id, client_id, version                                 │
│  ├── operation, is_canary                                        │
│  ├── status, exit_code, output, error                            │
│  └── started_at, finished_at, duration                           │
└─────────────────────────────────────────────────────────────────┘
```

---

## 部署类型详解

### 1. 脚本部署 (script)

适用于传统应用部署，支持四种操作类型：

```yaml
# 项目配置
script_config:
  work_dir: "/opt/apps/myapp"
  interpreter: "/bin/bash"
  environment:
    APP_NAME: "myapp"
    NODE_ENV: "production"

  # 四种操作脚本
  install_script: |
    #!/bin/bash
    echo "Installing ${APP_NAME}..."
    mkdir -p /opt/apps/myapp
    # ... 安装逻辑

  update_script: |
    #!/bin/bash
    echo "Updating ${APP_NAME}..."
    # ... 更新逻辑

  rollback_script: |
    #!/bin/bash
    echo "Rolling back ${APP_NAME}..."
    # ... 回滚逻辑

  uninstall_script: |
    #!/bin/bash
    echo "Uninstalling ${APP_NAME}..."
    # ... 卸载逻辑

  timeouts:
    install: 600    # 10 分钟
    update: 300     # 5 分钟
    rollback: 180   # 3 分钟
    uninstall: 120  # 2 分钟
```

### 2. 容器部署 (container)

适用于 Docker 容器化应用：

```yaml
# 项目配置
container_config:
  # 镜像配置
  image: "nginx:latest"
  container_name: "nginx-web"
  registry: "registry.example.com"  # 可选
  registry_user: "user"              # 可选
  registry_pass: "password"          # 可选

  # 端口映射
  ports:
    - host_port: 80
      container_port: 80
      protocol: tcp

  # 存储卷
  volumes:
    - host_path: "/data/nginx/html"
      container_path: "/usr/share/nginx/html"
      read_only: false

  # 资源限制
  memory_limit: "512m"
  cpu_limit: "0.5"

  # 环境变量
  environment:
    TZ: "Asia/Shanghai"

  # 健康检查
  health_check:
    command: ["CMD", "curl", "-f", "http://localhost/health"]
    interval: 30
    timeout: 10
    retries: 3
    start_period: 60

  # 部署脚本（可选）
  pre_script: "#!/bin/bash\necho 'Pre-deploy'"
  post_script: "#!/bin/bash\necho 'Post-deploy'"
```

### 3. Kubernetes 部署 (kubernetes)

适用于 K8s 集群应用：

```yaml
# 项目配置
kubernetes_config:
  namespace: "production"
  resource_type: "deployment"  # deployment | statefulset | daemonset
  resource_name: "user-service"
  container_name: "user-service"

  # 镜像配置
  image: "registry.example.com/user-service:latest"
  image_pull_policy: "IfNotPresent"
  image_pull_secret: "registry-secret"

  # 副本配置
  replicas: 3
  update_strategy: "RollingUpdate"
  max_unavailable: "25%"
  max_surge: "25%"

  # 资源限制
  cpu_request: "100m"
  cpu_limit: "500m"
  memory_request: "128Mi"
  memory_limit: "512Mi"

  # 环境变量
  environment:
    TZ: "Asia/Shanghai"
    LOG_LEVEL: "info"

  # Service 配置
  service_type: "ClusterIP"
  service_ports:
    - name: "http"
      port: 8080
      target_port: 8080

  # 部署超时
  rollout_timeout: 300
```

### 4. Git 拉取部署 (gitpull)

适用于从 Git 仓库直接拉取代码部署：

```yaml
# 项目配置
gitpull_config:
  repo_url: "https://github.com/example/app.git"
  branch: "main"
  depth: 1  # 浅克隆

  # 认证配置
  auth_type: "token"  # none | ssh | token | basic
  token: "ghp_xxxx"   # 或 SSH key / 用户名密码

  # 部署配置
  work_dir: "/opt/apps/frontend"
  clean_before: true
  backup_before: true
  backup_dir: "/data/backup"
  backup_keep: 5

  # 部署脚本
  pre_script: |
    #!/bin/bash
    npm install

  post_script: |
    #!/bin/bash
    npm run build
    systemctl reload nginx

  # 超时配置
  clone_timeout: 300
  script_timeout: 600
```

---

## 前端交互逻辑

### 页面结构

```
ProjectWorkspace.vue (项目列表)
    │
    ├── 项目卡片/列表
    │   └── 点击"进入项目" → Release.vue
    │
    └── 新建项目对话框
        └── 根据部署类型显示不同配置表单

Release.vue (发布管理)
    │
    ├── 项目选择器 (左侧/顶部)
    │
    ├── Tab 页签
    │   ├── 版本管理
    │   │   ├── 版本列表
    │   │   ├── 新建版本对话框
    │   │   └── 编辑/删除版本
    │   │
    │   ├── 部署任务
    │   │   ├── 任务列表
    │   │   ├── 创建部署对话框
    │   │   └── 任务控制（开始/暂停/取消/全量）
    │   │
    │   ├── 部署日志
    │   │   └── 历史执行记录
    │   │
    │   ├── Webhook 配置
    │   │   └── 跳转到 Webhook 管理页面
    │   │
    │   └── 成员管理
    │       └── 跳转到成员管理页面
    │
    └── 对话框组件
        ├── 项目配置对话框
        ├── 版本配置对话框
        ├── 部署任务对话框
        ├── 实时日志对话框
        └── Docker 高级配置对话框
```

### 关键交互流程

#### 1. 创建项目流程

```
用户点击"新建项目"
    │
    ▼
显示项目表单对话框
    │
    ├── 填写基本信息（名称、描述）
    │
    ├── 选择部署类型
    │   ├── script → 显示脚本配置
    │   ├── container → 显示容器配置
    │   ├── kubernetes → 显示 K8s 配置
    │   └── gitpull → 显示 Git 配置
    │
    ▼
调用 api.createProject(data)
    │
    ▼
刷新项目列表
```

#### 2. 创建版本流程

```
选中项目 → 点击"新建版本"
    │
    ▼
显示版本表单对话框
    │
    ├── 输入版本号（如 v1.0.0）
    │
    ├── 根据项目类型显示配置
    │   ├── script → 安装/更新/回滚/卸载脚本
    │   ├── container → 镜像地址、环境变量、资源限制
    │   ├── kubernetes → 镜像版本、副本数、YAML 配置
    │   └── gitpull → Git 版本选择（Tag/Branch/Commit）
    │
    ▼
调用 api.createVersion(projectId, data)
    │
    ▼
刷新版本列表
```

#### 3. 创建部署任务流程

```
选中项目 + 版本 → 点击"部署"
    │
    ▼
显示部署配置对话框
    │
    ├── 选择目标客户端
    │   ├── 手动选择
    │   └── 按已安装版本自动选择
    │
    ├── 配置执行计划
    │   ├── 立即执行
    │   └── 定时执行
    │
    ├── 配置金丝雀发布（可选）
    │   ├── 启用金丝雀
    │   ├── 灰度比例（%）
    │   ├── 观察时间
    │   └── 自动全量
    │
    ├── 配置失败处理
    │   ├── 继续执行
    │   ├── 暂停等待
    │   └── 终止任务
    │
    ▼
调用 api.createDeployTask(data)
    │
    ▼
调用 api.startDeployTask(taskId)
    │
    ▼
显示实时日志对话框
```

---

## API 接口参考

### 项目管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/release/projects | 获取项目列表 |
| GET | /api/release/projects/:id | 获取项目详情 |
| POST | /api/release/projects | 创建项目 |
| PUT | /api/release/projects/:id | 更新项目 |
| DELETE | /api/release/projects/:id | 删除项目 |

### 版本管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/release/projects/:id/versions | 获取版本列表 |
| GET | /api/release/versions/:id | 获取版本详情 |
| POST | /api/release/projects/:id/versions | 创建版本 |
| PUT | /api/release/versions/:id | 更新版本 |
| DELETE | /api/release/versions/:id | 删除版本 |

### 部署任务

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/release/projects/:id/tasks | 获取任务列表 |
| GET | /api/release/tasks/:id | 获取任务详情 |
| POST | /api/release/tasks | 创建部署任务 |
| POST | /api/release/tasks/:id/start | 开始任务 |
| POST | /api/release/tasks/:id/cancel | 取消任务 |
| POST | /api/release/tasks/:id/pause | 暂停任务 |
| POST | /api/release/tasks/:id/promote | 金丝雀全量 |
| POST | /api/release/tasks/:id/rollback | 回滚任务 |

---

## 常见问题

### Q: 为什么无法配置发布项？

检查以下事项：

1. **项目是否创建成功**
   - 确保项目在数据库中存在
   - 检查项目的 `type` 字段是否正确

2. **版本是否创建**
   - 发布任务必须基于某个版本
   - 确保至少创建了一个版本

3. **客户端是否在线**
   - 部署任务需要目标客户端
   - 确保客户端已连接到服务端

4. **API 是否正常**
   - 打开浏览器开发者工具
   - 检查 Network 请求是否返回错误

### Q: 如何调试部署脚本？

1. 在"脚本管理"页面测试脚本
2. 使用"命令发送"功能在目标机器上执行
3. 查看部署日志中的输出和错误信息

### Q: 金丝雀发布如何工作？

```
1. 创建任务时启用金丝雀，设置灰度比例（如 20%）
2. 开始任务后，随机选择 20% 的客户端进行部署
3. 任务状态变为 "canary"，等待观察期
4. 观察期结束后，可手动或自动执行全量发布
5. 全量发布将部署到剩余 80% 的客户端
```

---

## 配置分层设计

系统采用三层配置覆盖机制：

```
1. 项目配置（默认）
   └── 定义基础配置，所有版本共享

2. 版本配置（覆盖）
   └── 继承项目配置，可覆盖特定字段

3. 任务配置（临时）
   └── 单次部署临时调整，不影响其他配置
```

示例：

```yaml
# 项目配置
container_config:
  memory_limit: "512m"
  cpu_limit: "0.5"

# 版本配置（覆盖内存限制）
deploy_config:
  resources:
    memory_limit: "1g"  # 覆盖项目配置

# 任务配置（临时扩容）
override_config:
  replicas: 5  # 紧急扩容
```

---

## 下一步

1. 执行测试数据脚本
2. 启动前端开发服务器：`cd web && npm run dev`
3. 访问发布管理页面测试功能
4. 根据实际需求调整配置
