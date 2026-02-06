package configcenter

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ==================== 枚举类型 ====================

// ConfigType 配置类型
type ConfigType string

const (
	ConfigTypeApplication ConfigType = "application" // 应用配置
	ConfigTypeSystem      ConfigType = "system"      // 系统参数
)

// ConfigFormat 配置格式
type ConfigFormat string

const (
	ConfigFormatJSON       ConfigFormat = "json"
	ConfigFormatYAML       ConfigFormat = "yaml"
	ConfigFormatProperties ConfigFormat = "properties"
	ConfigFormatTEXT       ConfigFormat = "text"
	ConfigFormatXML        ConfigFormat = "xml"
)

// ReleaseType 发布类型
type ReleaseType string

const (
	ReleaseTypeFull     ReleaseType = "full"     // 全量发布
	ReleaseTypeRollback ReleaseType = "rollback" // 回滚
	ReleaseTypeGray     ReleaseType = "gray"     // 灰度发布
)

// ReleaseStatus 发布状态
type ReleaseStatus string

const (
	ReleaseStatusPending    ReleaseStatus = "pending"
	ReleaseStatusPublishing ReleaseStatus = "publishing"
	ReleaseStatusSuccess    ReleaseStatus = "success"
	ReleaseStatusFailed     ReleaseStatus = "failed"
	ReleaseStatusPartial    ReleaseStatus = "partial" // 部分成功
)

// RuleType 灰度规则类型
type RuleType string

const (
	RuleTypeTag        RuleType = "tag"         // 按标签匹配
	RuleTypeIP         RuleType = "ip"          // 按 IP 匹配
	RuleTypeClientID   RuleType = "client_id"   // 按客户端 ID
	RuleTypePercentage RuleType = "percentage"  // 按百分比
)

// SubscriberStatus 订阅者状态
type SubscriberStatus string

const (
	SubscriberStatusOnline  SubscriberStatus = "online"
	SubscriberStatusOffline SubscriberStatus = "offline"
)

// ==================== 自定义类型 ====================

// StringArray 用于存储字符串数组（PostgreSQL TEXT[] 或 JSONB）
type StringArray []string

// Value 实现 driver.Valuer 接口
func (s StringArray) Value() (driver.Value, error) {
	if len(s) == 0 {
		return nil, nil
	}
	return json.Marshal(s)
}

// Scan 实现 sql.Scanner 接口
func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		// 尝试处理字符串类型
		str, strOk := value.(string)
		if !strOk {
			return nil
		}
		bytes = []byte(str)
	}
	return json.Unmarshal(bytes, s)
}

// JSONMap 用于存储 JSON 对象
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
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		// 尝试处理字符串类型
		str, strOk := value.(string)
		if !strOk {
			return nil
		}
		bytes = []byte(str)
	}
	return json.Unmarshal(bytes, j)
}

// ==================== 配置管理 ====================

// Config 配置项
type Config struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	// 业务标识（唯一键）
	Namespace  string       `gorm:"size:64;not null;index:idx_config_unique,priority:1;uniqueIndex:idx_config_unique,priority:1" json:"namespace"`
	Group      string       `gorm:"size:64;not null;index:idx_config_unique,priority:2;uniqueIndex:idx_config_unique,priority:2" json:"group"`
	DataID     string       `gorm:"size:128;not null;index:idx_config_unique,priority:3;uniqueIndex:idx_config_unique,priority:3" json:"data_id"`
	ConfigType ConfigType   `gorm:"size:32;not null;index" json:"config_type"`
	Content    string       `gorm:"type:text;not null" json:"content"`
	Format     ConfigFormat `gorm:"size:32;not null" json:"format"`
	Encrypted  bool         `gorm:"default:false" json:"encrypted"`
	Tags       StringArray  `gorm:"type:jsonb" json:"tags,omitempty"`
	Description string       `gorm:"size:512" json:"description,omitempty"`

	// 版本控制
	Version int `gorm:"default:1;not null;index" json:"version"`

	// 时间戳
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 关联
	Releases  []ConfigRelease `gorm:"foreignKey:ConfigID;constraint:OnDelete:CASCADE" json:"releases,omitempty"`
	GrayRules []GrayRule      `gorm:"foreignKey:ConfigID;constraint:OnDelete:CASCADE" json:"gray_rules,omitempty"`
}

// TableName 指定表名
func (Config) TableName() string {
	return "configs"
}

// BeforeCreate GORM hook
func (c *Config) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	if c.Version == 0 {
		c.Version = 1
	}
	return nil
}

// BeforeUpdate GORM hook
func (c *Config) BeforeUpdate(tx *gorm.DB) error {
	c.UpdatedAt = time.Now()
	return nil
}

// ConfigRelease 配置发布记录
type ConfigRelease struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// 关联配置
	ConfigID uint `gorm:"index:idx_config_release_config;not null" json:"config_id"`

	// 配置快照（冗余字段，便于查询）
	Namespace string `gorm:"size:64;not null;index" json:"namespace"`
	Group     string `gorm:"size:64;not null;index" json:"group"`
	DataID    string `gorm:"size:128;not null;index" json:"data_id"`

	// 配置快照
	Content string `gorm:"type:text;not null" json:"content"`
	Version int    `gorm:"not null;index" json:"version"`

	// 发布信息
	ReleaseType ReleaseType   `gorm:"size:32;not null;index" json:"release_type"`
	Status      ReleaseStatus `gorm:"size:32;not null;index" json:"status"`
	ReleasedBy  string        `gorm:"size:128;not null" json:"released_by"`
	ReleasedAt  time.Time     `gorm:"index" json:"released_at"`

	// 统计
	TotalTargets int `gorm:"default:0;not null" json:"total_targets"`
	SuccessCount int `gorm:"default:0;not null" json:"success_count"`
	FailedCount  int `gorm:"default:0;not null" json:"failed_count"`

	// 灰度信息
	IsGray     bool  `gorm:"default:false;index" json:"is_gray"`
	GrayRuleID *uint `gorm:"index" json:"gray_rule_id,omitempty"`

	// 回滚信息
	RollbackFromVersion *int `gorm:"index" json:"rollback_from_version,omitempty"`

	// 时间戳
	CreatedAt time.Time `json:"created_at"`

	// 关联
	Config   *Config   `gorm:"foreignKey:ConfigID" json:"config,omitempty"`
	GrayRule *GrayRule `gorm:"foreignKey:GrayRuleID" json:"gray_rule,omitempty"`
}

// TableName 指定表名
func (ConfigRelease) TableName() string {
	return "config_releases"
}

// GrayRule 灰度发布规则
type GrayRule struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// 关联配置
	ConfigID uint `gorm:"index:idx_gray_rule_config;not null;constraint:OnDelete:CASCADE" json:"config_id"`

	RuleName    string  `gorm:"size:128;not null;index" json:"rule_name"`
	RuleType    RuleType `gorm:"size:32;not null;index" json:"rule_type"`
	RuleValue   string  `gorm:"type:text;not null" json:"rule_value"` // JSON: ["tag1","tag2"] | ["1.1.1.1","2.2.2.2"] | 50

	Enabled     bool   `gorm:"default:true;index" json:"enabled"`
	Priority    int    `gorm:"default:0;index" json:"priority"` // 数字越大优先级越高
	Description string `gorm:"size:512" json:"description,omitempty"`

	CreatedBy string    `gorm:"size:128;not null" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 关联
	Config *Config `gorm:"foreignKey:ConfigID" json:"config,omitempty"`
}

// TableName 指定表名
func (GrayRule) TableName() string {
	return "gray_rules"
}

// BeforeCreate GORM hook
func (g *GrayRule) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	g.CreatedAt = now
	g.UpdatedAt = now
	return nil
}

// BeforeUpdate GORM hook
func (g *GrayRule) BeforeUpdate(tx *gorm.DB) error {
	g.UpdatedAt = time.Now()
	return nil
}

// ==================== 客户端订阅 ====================

// ConfigSubscriber 配置订阅者
type ConfigSubscriber struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// 客户端标识
	ClientID string `gorm:"size:128;not null;uniqueIndex:idx_subscriber_client_ns,priority:1;index:idx_subscriber_client_ns" json:"client_id"`
	SDKType  string `gorm:"size:32" json:"sdk_type"` // go | python | java | javascript

	// 订阅的配置
	Namespace      string      `gorm:"size:64;not null;uniqueIndex:idx_subscriber_client_ns,priority:2;index" json:"namespace"`
	Subscriptions StringArray `gorm:"type:jsonb" json:"subscriptions"` // ["group:dataId", ...]

	// 客户端信息
	ClientIP   string      `gorm:"size:64" json:"client_ip,omitempty"`
	ClientTags StringArray `gorm:"type:jsonb" json:"client_tags,omitempty"` // 客户端标签 (用于灰度匹配)

	// 状态
	LastActive time.Time        `gorm:"index" json:"last_active"`
	Status     SubscriberStatus `gorm:"size:32;not null;index" json:"status"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (ConfigSubscriber) TableName() string {
	return "config_subscribers"
}

// BeforeCreate GORM hook
func (s *ConfigSubscriber) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now
	if s.LastActive.IsZero() {
		s.LastActive = now
	}
	if s.Status == "" {
		s.Status = SubscriberStatusOnline
	}
	return nil
}

// BeforeUpdate GORM hook
func (s *ConfigSubscriber) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = time.Now()
	return nil
}

// ==================== 配置变更历史 ====================

// ConfigChangeLog 配置变更日志
type ConfigChangeLog struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// 关联配置
	ConfigID uint `gorm:"index;not null" json:"config_id"`

	// 配置标识（冗余字段）
	Namespace string `gorm:"size:64;not null;index" json:"namespace"`
	Group     string `gorm:"size:64;not null;index" json:"group"`
	DataID    string `gorm:"size:128;not null;index" json:"data_id"`

	ChangeType string `gorm:"size:32;not null;index" json:"change_type"` // create | update | delete | release

	// 变更内容
	OldContent string `gorm:"type:text" json:"old_content,omitempty"`
	NewContent string `gorm:"type:text" json:"new_content,omitempty"`
	Diff       string `gorm:"type:text" json:"diff,omitempty"` // JSON 格式的 diff

	// 操作信息
	OperatedBy string    `gorm:"size:128;not null" json:"operated_by"`
	OperatedAt time.Time `gorm:"index" json:"operated_at"`

	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名
func (ConfigChangeLog) TableName() string {
	return "config_change_logs"
}

// BeforeCreate GORM hook
func (l *ConfigChangeLog) BeforeCreate(tx *gorm.DB) error {
	if l.OperatedAt.IsZero() {
		l.OperatedAt = time.Now()
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}
	return nil
}

// ==================== 配置推送消息 ====================

// ConfigPushMessage 配置推送消息记录
type ConfigPushMessage struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	MsgID  string `gorm:"size:64;uniqueIndex;not null" json:"msg_id"` // 消息唯一标识

	// 关联发布
	ReleaseID uint `gorm:"index;not null" json:"release_id"`

	// 目标信息
	ClientID string `gorm:"size:128;not null;index" json:"client_id"`

	// 推送状态
	Status    string     `gorm:"size:32;not null;index" json:"status"` // pending | sent | failed | acknowledged
	SentAt    *time.Time `json:"sent_at,omitempty"`
	AckAt     *time.Time `json:"ack_at,omitempty"`
	ErrorMsg  string     `gorm:"type:text" json:"error_msg,omitempty"`
	RetryCount int       `gorm:"default:0" json:"retry_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (ConfigPushMessage) TableName() string {
	return "config_push_messages"
}

// BeforeCreate GORM hook
func (m *ConfigPushMessage) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	m.CreatedAt = now
	m.UpdatedAt = now
	return nil
}

// BeforeUpdate GORM hook
func (m *ConfigPushMessage) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = time.Now()
	return nil
}

// ==================== 配置快照 ====================

// ConfigSnapshot 配置快照（用于快速回滚）
type ConfigSnapshot struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// 关联配置
	ConfigID uint `gorm:"index;not null" json:"config_id"`

	// 配置标识（冗余字段）
	Namespace string `gorm:"size:64;not null;index" json:"namespace"`
	Group     string `gorm:"size:64;not null" json:"group"`
	DataID    string `gorm:"size:128;not null" json:"data_id"`

	// 快照内容
	Content string `gorm:"type:text;not null" json:"content"`
	Version int    `gorm:"not null" json:"version"`

	// 快照信息
	CreatedBy string    `gorm:"size:128;not null" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`

	// 过期时间
	ExpiresAt *time.Time `gorm:"index" json:"expires_at,omitempty"`
}

// TableName 指定表名
func (ConfigSnapshot) TableName() string {
	return "config_snapshots"
}

// BeforeCreate GORM hook
func (s *ConfigSnapshot) BeforeCreate(tx *gorm.DB) error {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	return nil
}

// ==================== 配置编辑锁 ====================

// ConfigEditLock 配置编辑锁（防止多人同时编辑）
type ConfigEditLock struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// 关联配置
	ConfigID uint `gorm:"uniqueIndex;not null" json:"config_id"`

	// 锁定信息
	LockedBy   string    `gorm:"size:128;not null" json:"locked_by"`
	LockedAt   time.Time `gorm:"not null" json:"locked_at"`
	ExpiresAt  time.Time `gorm:"index;not null" json:"expires_at"`

	// 会话标识（用于解锁验证）
	SessionID string `gorm:"size:128;not null" json:"session_id"`
}

// TableName 指定表名
func (ConfigEditLock) TableName() string {
	return "config_edit_locks"
}

// BeforeCreate GORM hook
func (l *ConfigEditLock) BeforeCreate(tx *gorm.DB) error {
	if l.LockedAt.IsZero() {
		l.LockedAt = time.Now()
	}
	return nil
}

// IsExpired 检查锁是否已过期
func (l *ConfigEditLock) IsExpired() bool {
	return time.Now().After(l.ExpiresAt)
}

// ==================== 数据库迁移 ====================

// AllConfigModels 所有配置中心模型列表
var AllConfigModels = []interface{}{
	&Config{},
	&ConfigRelease{},
	&GrayRule{},
	&ConfigSubscriber{},
	&ConfigChangeLog{},
	&ConfigPushMessage{},
	&ConfigSnapshot{},
	&ConfigEditLock{},
}

// AutoMigrateConfig 自动迁移配置中心表
func AutoMigrateConfig(db *gorm.DB) error {
	return db.AutoMigrate(AllConfigModels...)
}
