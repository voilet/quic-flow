package alert

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ========== 枚举类型 ==========

// AlertSeverity 告警严重程度
type AlertSeverity string

const (
	AlertSeverityCritical AlertSeverity = "critical" // 严重
	AlertSeverityWarning  AlertSeverity = "warning"  // 警告
	AlertSeverityInfo     AlertSeverity = "info"     // 信息
)

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusFiring   AlertStatus = "firing"   // 触发中
	AlertStatusResolved AlertStatus = "resolved" // 已解决
	AlertStatusSilenced AlertStatus = "silenced" // 已抑制
)

// NotifyChannelType 通知渠道类型
type NotifyChannelType string

const (
	ChannelTypeWebhook  NotifyChannelType = "webhook"  // Webhook
	ChannelTypeEmail    NotifyChannelType = "email"    // 邮件
	ChannelTypeDingTalk NotifyChannelType = "dingtalk" // 钉钉
	ChannelTypeWeChat   NotifyChannelType = "wechat"   // 企业微信
	ChannelTypeFeishu   NotifyChannelType = "feishu"   // 飞书
	ChannelTypeSlack    NotifyChannelType = "slack"    // Slack
)

// ========== JSONMap 类型 ==========

// JSONMap 用于存储 JSON 格式的键值对
type JSONMap map[string]interface{}

// Value 实现 driver.Valuer 接口
func (j JSONMap) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// ========== 告警规则 ==========

// AlertRule 告警规则
type AlertRule struct {
	ID          uint `gorm:"primaryKey"`
	Name        string `gorm:"size:128;uniqueIndex;not null"`
	Description string `gorm:"size:512"`
	Enabled     bool   `gorm:"default:true;index"`
	Priority    int    `gorm:"default:0;index"`

	// 规则定义 (CEL 表达式)
	Condition   string `gorm:"type:text;not null"`
	ForDuration time.Duration

	// 严重程度
	Severity AlertSeverity `gorm:"size:32;index"`

	// 标签和注解
	Labels      JSONMap `gorm:"type:json"`
	Annotations JSONMap `gorm:"type:json"`

	// 通知配置
	NotifyChannels []uint  `gorm:"type:json"`
	NotifyGroup    string `gorm:"size:64"`

	// 统计
	TriggeredCount int        `gorm:"default:0"`
	LastTriggered  *time.Time

	CreatedBy string    `gorm:"size:128"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName 指定表名
func (AlertRule) TableName() string {
	return "alert_rules"
}

// ========== 告警实例 ==========

// AlertInstance 告警实例
type AlertInstance struct {
	ID uint `gorm:"primaryKey"`

	// 关联规则
	RuleID   uint   `gorm:"index;not null"`
	RuleName string `gorm:"size:128;index;not null"`

	// 告警状态
	Status   AlertStatus   `gorm:"size:32;index"`
	Severity AlertSeverity `gorm:"size:32;index"`

	// 标签和注解
	Labels      JSONMap `gorm:"type:json;index"`
	Annotations JSONMap `gorm:"type:json"`

	// 告警内容
	Summary     string `gorm:"size:512"`
	Description string `gorm:"type:text"`

	// 时间信息
	StartedAt  time.Time  `gorm:"index"`
	FiredAt    time.Time  `gorm:"index"`
	ResolvedAt *time.Time `gorm:"index"`

	// 指标快照
	MetricValues JSONMap `gorm:"type:json"`

	// 通知状态
	Notified    bool `gorm:"default:false;index"`
	NotifyCount int  `gorm:"default:0"`

	// 归属
	Fingerprint string `gorm:"size:64;uniqueIndex;index;not null"`
	GroupKey    string `gorm:"size:128;index"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName 指定表名
func (AlertInstance) TableName() string {
	return "alert_instances"
}

// ========== 抑制规则 ==========

// SilenceRule 抑制规则
type SilenceRule struct {
	ID uint `gorm:"primaryKey"`

	Name    string `gorm:"size:128;not null"`
	Comment string `gorm:"size:512"`
	RuleID  *uint `gorm:"index"`

	// 匹配条件
	MatchLabels JSONMap `gorm:"type:json"`
	MatchRegex  JSONMap `gorm:"type:json"`

	// 时间范围
	StartAt time.Time `gorm:"index"`
	EndAt   time.Time `gorm:"index"`

	Enabled bool `gorm:"default:true;index"`

	CreatedBy string    `gorm:"size:128"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName 指定表名
func (SilenceRule) TableName() string {
	return "alert_silence_rules"
}

// ========== 通知渠道 ==========

// NotifyChannel 通知渠道
type NotifyChannel struct {
	ID uint `gorm:"primaryKey"`

	Name   string            `gorm:"size:128;uniqueIndex;not null"`
	Type   NotifyChannelType `gorm:"size:32;index"`
	Config JSONMap           `gorm:"type:json"`

	Enabled bool `gorm:"default:true"`

	// 重试配置
	MaxRetries      int           `gorm:"default:3"`
	RetryInterval   time.Duration `gorm:"default:0"`

	CreatedBy string    `gorm:"size:128"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName 指定表名
func (NotifyChannel) TableName() string {
	return "alert_notify_channels"
}

// ========== 通知历史 ==========

// NotifyHistory 通知历史
type NotifyHistory struct {
	ID uint `gorm:"primaryKey"`

	AlertID   uint `gorm:"index;not null"`
	ChannelID uint `gorm:"index;not null"`

	Payload  string `gorm:"type:text"`
	Status   string `gorm:"size:32"` // success | failed
	Response string `gorm:"type:text"`
	Error    string `gorm:"type:text"`

	SentAt      time.Time  `gorm:"index"`
	DeliveredAt *time.Time

	RetryCount int `gorm:"default:0"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (NotifyHistory) TableName() string {
	return "alert_notify_history"
}

// ========== 值班管理 ==========

// OnCallSchedule 值班表
type OnCallSchedule struct {
	ID uint `gorm:"primaryKey"`

	Name        string  `gorm:"size:128;not null"`
	Description string  `gorm:"size:512"`
	TimeZone    string  `gorm:"size:64;default:'Asia/Shanghai'"`
	Config      JSONMap `gorm:"type:json"`

	CurrentOnCall string `gorm:"size:128"`
	Enabled       bool   `gorm:"default:true"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName 指定表名
func (OnCallSchedule) TableName() string {
	return "alert_oncall_schedules"
}

// OnCallUser 值班用户
type OnCallUser struct {
	ID uint `gorm:"primaryKey"`

	Name  string   `gorm:"size:128;not null"`
	Email string   `gorm:"size:256"`
	Phone string   `gorm:"size:64"`

	NotifyChannels []string `gorm:"type:json"`
	Constraints    JSONMap  `gorm:"type:json"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName 指定表名
func (OnCallUser) TableName() string {
	return "alert_oncall_users"
}
