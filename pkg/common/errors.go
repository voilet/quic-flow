package common

// ========== 错误码定义 ==========

const (
	// 成功
	CodeSuccess = 0

	// ========== 通用错误 (1-999) ==========
	CodeInvalidParams      = 1    // 参数错误
	CodeUnauthorized       = 2    // 未授权
	CodeForbidden          = 3    // 禁止访问
	CodeNotFound           = 4    // 资源不存在
	CodeInternalError      = 5    // 内部错误
	CodeServiceUnavailable = 6    // 服务不可用
	CodeRateLimitExceeded  = 7    // 请求过于频繁
	CodeRequestTimeout     = 8    // 请求超时
	CodeMethodNotAllowed   = 9    // 方法不允许
	CodeConflict           = 10   // 资源冲突
	CodeDatabaseError      = 11   // 数据库错误

	// ========== 客户端管理 (1000-1999) ==========
	CodeClientNotFound    = 1000 // 客户端不存在
	CodeClientOffline     = 1001 // 客户端离线
	CodeClientDuplicate   = 1002 // 客户端重复
	CodeSessionExpired    = 1003 // 会话过期
	CodeHeartbeatTimeout  = 1004 // 心跳超时
	CodeSessionNotFound   = 1005 // 会话不存在
	CodeInvalidClientID   = 1006 // 无效客户端ID

	// ========== 命令/任务 (2000-2999) ==========
	CodeCommandNotFound    = 2000 // 命令不存在
	CodeCommandTimeout     = 2001 // 命令执行超时
	CodeCommandFailed      = 2002 // 命令执行失败
	CodeTaskNotFound       = 2003 // 任务不存在
	CodeTaskCancelled      = 2004 // 任务已取消
	CodeInvalidCommandType = 2005 // 无效命令类型
	CodeTaskAlreadyRunning = 2006 // 任务已在运行

	// ========== 发布/部署 (3000-3999) ==========
	CodeProjectNotFound        = 3000 // 项目不存在
	CodeVersionNotFound        = 3001 // 版本不存在
	CodeDeployTaskNotFound     = 3002 // 部署任务不存在
	CodeDeployFailed           = 3003 // 部署失败
	CodeRollbackFailed         = 3004 // 回滚失败
	CodeInvalidScript          = 3005 // 脚本无效
	CodeEnvironmentNotFound    = 3006 // 环境不存在
	CodeTargetNotFound         = 3007 // 目标不存在
	CodePipelineNotFound       = 3008 // 流水线不存在
	CodeCredentialNotFound     = 3009 // 凭证不存在
	CodeCallbackConfigNotFound = 3010 // 回调配置不存在

	// ========== 配置中心 (4000-4999) ==========
	CodeConfigNotFound      = 4000 // 配置不存在
	CodeConfigDuplicate     = 4001 // 配置重复
	CodePublishFailed       = 4002 // 发布失败
	CodeInvalidConfigFormat = 4003 // 配置格式无效
	CodeNamespaceNotFound   = 4004 // 命名空间不存在
	CodeGroupNotFound       = 4005 // 分组不存在
	CodeGrayRuleNotFound    = 4006 // 灰度规则不存在

	// ========== 审计/日志 (5000-5999) ==========
	CodeAuditLogNotFound  = 5000 // 审计日志不存在
	CodeRecordingNotFound = 5001 // 录像不存在

	// ========== 性能分析 (6000-6999) ==========
	CodeProfileNotFound        = 6000 // 性能采集不存在
	CodeProfileGenerationFailed = 6001 // 生成失败

	// ========== 文件传输 (7000-7999) ==========
	CodeFileNotFound      = 7000 // 文件不存在
	CodeFileUploadFailed  = 7001 // 文件上传失败
	CodeFileDownloadFailed = 7002 // 文件下载失败
	CodeInvalidFileType   = 7003 // 无效文件类型

	// ========== SSH/终端 (8000-8999) ==========
	CodeSSHConnectionFailed = 8000 // SSH 连接失败
	CodeSSHTerminalNotFound = 8001 // SSH 终端不存在

	// ========== Alert/告警 (9000-9999) ==========
	CodeAlertNotFound     = 9000 // 告警不存在
	CodeAlertRuleNotFound = 9001 // 告警规则不存在
)

// ========== 错误码消息映射 ==========

var codeMessages = map[int]string{
	CodeSuccess:           "操作成功",
	CodeInvalidParams:     "参数错误",
	CodeUnauthorized:      "未授权，请先登录",
	CodeForbidden:         "禁止访问",
	CodeNotFound:          "资源不存在",
	CodeInternalError:     "内部错误",
	CodeServiceUnavailable: "服务暂不可用",
	CodeRateLimitExceeded: "请求过于频繁",
	CodeRequestTimeout:    "请求超时",
	CodeMethodNotAllowed:  "方法不允许",
	CodeConflict:          "资源冲突",
	CodeDatabaseError:     "数据库错误",

	// 客户端管理
	CodeClientNotFound:   "客户端不存在",
	CodeClientOffline:    "客户端离线",
	CodeClientDuplicate:  "客户端重复",
	CodeSessionExpired:   "会话过期",
	CodeHeartbeatTimeout: "心跳超时",
	CodeSessionNotFound:  "会话不存在",
	CodeInvalidClientID:  "无效客户端ID",

	// 命令/任务
	CodeCommandNotFound:    "命令不存在",
	CodeCommandTimeout:     "命令执行超时",
	CodeCommandFailed:      "命令执行失败",
	CodeTaskNotFound:       "任务不存在",
	CodeTaskCancelled:      "任务已取消",
	CodeInvalidCommandType: "无效命令类型",
	CodeTaskAlreadyRunning: "任务已在运行",

	// 发布/部署
	CodeProjectNotFound:        "项目不存在",
	CodeVersionNotFound:        "版本不存在",
	CodeDeployTaskNotFound:     "部署任务不存在",
	CodeDeployFailed:           "部署失败",
	CodeRollbackFailed:         "回滚失败",
	CodeInvalidScript:          "脚本无效",
	CodeEnvironmentNotFound:    "环境不存在",
	CodeTargetNotFound:         "目标不存在",
	CodePipelineNotFound:       "流水线不存在",
	CodeCredentialNotFound:     "凭证不存在",
	CodeCallbackConfigNotFound: "回调配置不存在",

	// 配置中心
	CodeConfigNotFound:      "配置不存在",
	CodeConfigDuplicate:     "配置重复",
	CodePublishFailed:       "发布失败",
	CodeInvalidConfigFormat: "配置格式无效",
	CodeNamespaceNotFound:   "命名空间不存在",
	CodeGroupNotFound:       "分组不存在",
	CodeGrayRuleNotFound:    "灰度规则不存在",

	// 审计/日志
	CodeAuditLogNotFound:  "审计日志不存在",
	CodeRecordingNotFound: "录像不存在",

	// 性能分析
	CodeProfileNotFound:        "性能采集不存在",
	CodeProfileGenerationFailed: "生成失败",

	// 文件传输
	CodeFileNotFound:       "文件不存在",
	CodeFileUploadFailed:   "文件上传失败",
	CodeFileDownloadFailed: "文件下载失败",
	CodeInvalidFileType:    "无效文件类型",

	// SSH/终端
	CodeSSHConnectionFailed: "SSH 连接失败",
	CodeSSHTerminalNotFound: "SSH 终端不存在",

	// Alert/告警
	CodeAlertNotFound:     "告警不存在",
	CodeAlertRuleNotFound: "告警规则不存在",
}

// GetDefaultMessage 获取错误码的默认消息
func GetDefaultMessage(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "未知错误"
}
