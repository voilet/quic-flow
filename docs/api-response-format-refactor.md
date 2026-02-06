# API 响应格式统一改造总结

## 已完成的工作

### 1. 创建统一响应格式基础文件

**pkg/common/response.go**
- 定义 `Response` 结构体：`{ code, data, msg }`
- 提供便捷的响应构建函数：
  - `Success()`, `SuccessWithData()`, `SuccessWithMessage()`, `SuccessEmpty()`
  - `Error()`, `ErrorWithData()`
  - `JSON()`, `SuccessResp()`, `ErrorResp()`
  - `BindAndValidate()`, `BindQueryAndValidate()`

**pkg/common/errors.go**
- 定义全局错误码常量（按功能模块分段）
- 提供错误码到消息的映射 `codeMessages`
- 提供 `GetDefaultMessage()` 函数

### 2. 后端 API 改造

**已完成的文件（8个）：**
1. ✅ `pkg/api/http_server.go` - 核心 HTTP API
2. ✅ `pkg/api/batch_api.go` - 批量执行 API
3. ✅ `pkg/api/stream_api.go` - SSE 流式 API（错误响应）
4. ✅ `pkg/api/audit_api.go` - 审计 API
5. ✅ `pkg/api/setup_api.go` - 数据库初始化 API
6. ✅ `pkg/api/health_api.go` - 健康检查 API
7. ✅ `pkg/api/terminal_api.go` - 终端管理 API（HTTP 响应部分）
8. ✅ `pkg/api/recording_api.go` - 录像 API

**响应格式示例：**

```go
// 成功响应
common.SuccessResp(c, data)

// 错误响应
common.ErrorResp(c, common.CodeClientNotFound, "客户端不存在")

// 自定义消息的成功响应
common.SuccessRespWithMsg(c, data, "操作成功")
```

### 3. 前端响应拦截器改造

**web/src/api/index.js**

```javascript
// 自动解包成功响应
if (code === 0) {
  return data  // 直接返回 data 字段
} else {
  // 业务错误，不自动弹窗
  return Promise.reject({ code, msg, data })
}
```

### 4. 统一响应格式

**成功响应：**
```json
{
  "code": 0,
  "data": {...},
  "msg": "操作成功"
}
```

**错误响应：**
```json
{
  "code": 1001,
  "data": {},
  "msg": "客户端不存在"
}
```

## 错误码分段

| 模块范围    | 说明             |
|-------------|------------------|
| 0           | 成功             |
| 1-999       | 通用错误         |
| 1000-1999   | 客户端管理       |
| 2000-2999   | 命令/任务        |
| 3000-3999   | 发布/部署        |
| 4000-4999   | 配置中心         |
| 5000-5999   | 审计/日志        |
| 6000-6999   | 性能分析         |
| 7000-7999   | 文件传输         |
| 8000-8999   | SSH/终端         |
| 9000-9999   | Alert/告警       |

## 待完成的工作

**需要继续改造的 API 文件：**
- `pkg/api/file_api.go` - 文件传输 API（部分完成）
- `pkg/api/task_api.go` - 任务 API（已添加导入）
- `pkg/api/ssh_api.go` - SSH API（已添加导入）
- `pkg/api/execution_api.go` - 执行 API（已添加导入）
- `pkg/api/group_api.go` - 分组 API（已添加导入）

## 使用示例

### 后端使用

```go
import "github.com/voilet/quic-flow/pkg/common"

// 成功响应 - 返回数据
common.SuccessResp(c, userData)

// 成功响应 - 空数据
common.SuccessEmpty(c)

// 错误响应
common.ErrorResp(c, common.CodeInvalidParams, "参数错误")
```

### 前端使用

```javascript
// 自动解包，直接获取 data
const clients = await api.getClients()

// 错误处理（不自动弹窗）
try {
  const result = await api.getClient(id)
  console.log(result)  // 已经是 data 字段的内容
} catch (err) {
  // err = { code: 1001, msg: "客户端不存在", data: {} }
  if (err.code === 1001) {
    // 处理特定错误
  }
}
```

## 注意事项

1. **SSE 流式接口**：只有错误响应使用统一格式，SSE 事件流保持原有格式
2. **文件下载接口**：直接返回文件流，不使用统一响应格式
3. **前端兼容性**：响应拦截器已修改，所有 API 调用会自动解包 `data` 字段
