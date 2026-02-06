# 告警系统前端页面

## 概述

告警系统前端包含 4 个主要页面和 1 个 API 客户端模块，用于管理和监控告警规则、通知渠道和抑制规则。

## 文件结构

```
web/src/
├── api/
│   └── alert.js           # 告警系统 API 客户端
└── views/
    ├── AlertList.vue      # 告警列表页面
    ├── AlertRules.vue     # 告警规则管理页面
    ├── AlertChannels.vue  # 通知渠道配置页面
    └── SilenceRules.vue   # 抑制规则管理页面
```

## 页面功能

### 1. 告警列表 (AlertList.vue)

**路由**: `/alerts`

**功能特性**:
- 活跃告警列表展示
- 按状态（活跃/已解决/已抑制）、严重程度（严重/警告/信息）筛选
- 告警详情查看（标签、注解、指标值）
- 解决告警操作（支持批量）
- 抑制告警操作（支持批量）
- 实时状态更新（SSE）
- 告警统计卡片（严重/警告/信息/总数）

**实时监控**:
- 点击"开启实时监控"按钮启用 SSE 推送
- 新告警自动添加到列表顶部
- 告警状态变化实时更新
- 解决/抑制告警实时反馈

### 2. 告警规则管理 (AlertRules.vue)

**路由**: `/alert-rules`

**功能特性**:
- 规则列表展示（分页）
- 创建/编辑/删除规则
- 规则表达式编辑器（Monaco Editor，支持 CEL 语法）
- 规则测试功能（输入测试数据验证表达式）
- 启用/禁用规则（开关）
- 规则详情查看

**规则配置项**:
- 规则名称
- 严重程度（critical/warning/info）
- CEL 表达式（例如: `metric.value > 100`）
- 评估间隔（秒）
- 持续时间（秒，0 表示立即触发）
- 告警名称、摘要、描述模板
- 标签和注解
- 关联的通知渠道

**表达式示例**:
```javascript
// CPU 使用率超过 80%
metric.value > 80 && metric.name == 'cpu_usage'

// 磁盘空间不足
metric.value < 10 && metric.name == 'disk_free_percent'

// 特定主机的告警
metric.value > 100 && metric.labels.host == 'server1'
```

### 3. 通知渠道配置 (AlertChannels.vue)

**路由**: `/alert-channels`

**功能特性**:
- 通知渠道列表展示
- 创建/编辑/删除渠道
- 测试发送功能
- 启用/禁用渠道

**支持的渠道类型**:

#### 钉钉
- Webhook URL
- 签名密钥（可选）
- 消息类型（text/markdown/actionCard）

#### 企微
- Webhook URL
- 消息类型（text/markdown）

#### 飞书
- Webhook URL
- 签名密钥（可选）

#### Slack
- Webhook URL
- 频道（如 #alerts）
- 用户名
- 头像图标 URL

#### 邮件
- SMTP 服务器和端口
- 用户名和密码
- 发件人
- TLS 支持
- 收件人列表

### 4. 抑制规则管理 (SilenceRules.vue)

**路由**: `/silence-rules`

**功能特性**:
- 抑制规则列表展示
- 创建/编辑/删除规则
- 启用/禁用规则
- 快捷模板（抑制所有严重告警、按主机抑制、维护窗口等）

**抑制规则配置**:
- 备注（抑制原因）
- 创建人
- 时间范围（开始时间、结束时间）
- 匹配条件（标签匹配，支持正则）

**快捷模板**:
- 抑制所有严重告警
- 按主机抑制
- 按告警名称抑制
- 维护窗口

## API 客户端 (alert.js)

### 规则管理
```javascript
import {
  createAlertRule,
  updateAlertRule,
  deleteAlertRule,
  getAlertRule,
  listAlertRules,
  toggleAlertRule,
  testAlertRule
} from '@/api/alert'

// 创建规则
await createAlertRule({
  name: 'HighCPU',
  expression: 'metric.value > 80',
  severity: 'warning',
  interval: 60,
  for: 300,
  labels: { type: 'cpu' },
  annotations: { description: 'CPU usage too high' }
})
```

### 告警实例
```javascript
import {
  listAlerts,
  getAlert,
  resolveAlert,
  silenceAlert,
  batchResolveAlerts,
  batchSilenceAlerts,
  getAlertStats
} from '@/api/alert'

// 获取告警列表
const alerts = await listAlerts({
  status: 'firing',
  severity: 'critical',
  page: 1,
  page_size: 20
})

// 解决告警
await resolveAlert(alertId, {
  reason: 'Issue fixed',
  resolved_by: 'admin'
})
```

### 通知渠道
```javascript
import {
  createAlertChannel,
  updateAlertChannel,
  deleteAlertChannel,
  listAlertChannels,
  testAlertChannel
} from '@/api/alert'

// 创建钉钉渠道
await createAlertChannel({
  name: '钉钉告警',
  type: 'dingtalk',
  config: {
    webhook: 'https://oapi.dingtalk.com/robot/send?access_token=xxx',
    secret: 'SEC***',
    msg_type: 'markdown'
  }
})
```

### 抑制规则
```javascript
import {
  createSilenceRule,
  updateSilenceRule,
  deleteSilenceRule,
  listSilenceRules,
  toggleSilenceRule
} from '@/api/alert'

// 创建抑制规则
await createSilenceRule({
  comment: '服务器维护',
  created_by: 'admin',
  starts_at: '2024-01-01T00:00:00Z',
  ends_at: '2024-01-01T06:00:00Z',
  matchers: [
    { name: 'host', is_regex: false, value: 'server1' }
  ]
})
```

### 实时事件 (SSE)
```javascript
import { subscribeAlertEvents } from '@/api/alert'

// 订阅告警事件
const connection = subscribeAlertEvents(
  // 新告警
  (alert) => {
    console.log('新告警:', alert)
  },
  // 告警更新
  (alert) => {
    console.log('告警更新:', alert)
  },
  // 告警解决
  (alert) => {
    console.log('告警解决:', alert)
  },
  // 错误
  (error) => {
    console.error('SSE 错误:', error)
  }
)

// 关闭连接
connection.close()
```

## 路由配置

路由已在 `/web/src/router/index.js` 中配置:

```javascript
{
  path: '/alerts',
  name: 'AlertList',
  component: () => import('@/views/AlertList.vue'),
  meta: { title: '告警列表' }
},
{
  path: '/alert-rules',
  name: 'AlertRules',
  component: () => import('@/views/AlertRules.vue'),
  meta: { title: '告警规则' }
},
{
  path: '/alert-channels',
  name: 'AlertChannels',
  component: () => import('@/views/AlertChannels.vue'),
  meta: { title: '通知渠道' }
},
{
  path: '/silence-rules',
  name: 'SilenceRules',
  component: () => import('@/views/SilenceRules.vue'),
  meta: { title: '抑制规则' }
}
```

## 组件依赖

- **Element Plus**: UI 组件库
- **Monaco Editor**: 代码编辑器（已在项目中配置）
- **dayjs**: 日期格式化
- **Vue Router**: 路由管理

## 使用示例

### 1. 创建告警规则

1. 访问 `/alert-rules`
2. 点击"新建规则"
3. 填写规则信息：
   - 规则名称: `DiskSpaceLow`
   - 表达式: `metric.value < 10 && metric.name == 'disk_free_percent'`
   - 严重程度: `严重`
   - 评估间隔: `60` 秒
   - 持续时间: `300` 秒
4. 添加标签和注解
5. 选择通知渠道
6. 点击"测试"验证表达式
7. 点击"保存"

### 2. 配置通知渠道

1. 访问 `/alert-channels`
2. 点击"新建渠道"
3. 选择渠道类型（如钉钉）
4. 填写配置信息（Webhook URL、密钥等）
5. 点击"测试"验证配置
6. 点击"保存"

### 3. 创建抑制规则

1. 访问 `/silence-rules`
2. 点击"新建规则"
3. 填写抑制信息：
   - 备注: `维护窗口`
   - 创建人: `admin`
   - 时间范围: 选择开始和结束时间
4. 添加匹配条件（如按主机抑制）
5. 点击"保存"

### 4. 监控告警

1. 访问 `/alerts`
2. 点击"开启实时监控"启用 SSE 推送
3. 使用筛选器查找特定告警
4. 点击"详情"查看完整信息
5. 对告警进行解决或抑制操作

## 注意事项

1. **SSE 连接**: 实时监控功能需要后端支持 SSE，确保后端 API `/api/alert/events` 可用
2. **Monaco Editor**: 确保 Monaco Editor 组件正确加载，项目已配置
3. **时间格式**: 所有时间使用 ISO 8601 格式（UTC）
4. **表达式语法**: 使用 CEL 表达式语法，参考官方文档
5. **权限控制**: 根据后端实现，可能需要相应的权限才能访问某些功能

## 后续扩展

可以添加的功能：
- 告警趋势图表
- 告警聚合和分组
- 告警拓扑图
- 告警分析报告
- 告警自动修复建议
- 通知模板管理
- 告警规则导入/导出
