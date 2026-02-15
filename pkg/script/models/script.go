package models

import (
	"time"

	"gorm.io/gorm"
)

// ScriptCategory 脚本分类
type ScriptCategory string

const (
	CategoryDeploy    ScriptCategory = "deploy"    // 部署脚本
	CategoryMonitor   ScriptCategory = "monitor"   // 监控脚本
	CategoryOperation ScriptCategory = "operation" // 运维脚本
	CategoryOther     ScriptCategory = "other"     // 其他
)

// ScriptStatus 脚本状态
type ScriptStatus string

const (
	ScriptStatusDraft     ScriptStatus = "draft"     // 草稿
	ScriptStatusPublished ScriptStatus = "published" // 已发布
	ScriptStatusArchived  ScriptStatus = "archived"  // 已归档
)

// ExecutionStatus 执行状态
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"   // 等待执行
	ExecutionStatusRunning   ExecutionStatus = "running"   // 执行中
	ExecutionStatusSuccess   ExecutionStatus = "success"   // 执行成功
	ExecutionStatusFailed    ExecutionStatus = "failed"    // 执行失败
	ExecutionStatusCancelled ExecutionStatus = "cancelled" // 已取消
	ExecutionStatusTimeout   ExecutionStatus = "timeout"   // 执行超时
)

// Script 脚本表
type Script struct {
	ID          uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string         `gorm:"size:128;uniqueIndex;not null;comment:脚本名称" json:"name"`
	Description string         `gorm:"size:512;comment:脚本描述" json:"description"`
	Category    ScriptCategory `gorm:"size:32;default:'other';comment:分类:deploy/monitor/operation/other" json:"category"`
	Interpreter string         `gorm:"size:32;default:'bash';comment:解释器:bash/python/powershell" json:"interpreter"`
	Content     string         `gorm:"type:text;not null;comment:脚本内容" json:"content"`
	Status      ScriptStatus   `gorm:"size:32;default:'draft';comment:状态:draft/published/archived" json:"status"`
	CreatedBy   string         `gorm:"size:64;comment:创建人" json:"created_by"`
	UpdatedAt   time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
	CreatedAt   time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联关系
	Versions []ScriptVersion `gorm:"foreignKey:ScriptID" json:"versions,omitempty"`
}

// TableName 指定表名
func (Script) TableName() string {
	return "tb_script"
}

// ScriptVersion 脚本版本表
type ScriptVersion struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ScriptID  uint      `gorm:"not null;index;comment:脚本ID" json:"script_id"`
	Version   string    `gorm:"size:32;not null;comment:版本号" json:"version"`
	Content   string    `gorm:"type:text;not null;comment:脚本内容" json:"content"`
	ChangeLog string    `gorm:"size:512;comment:变更说明" json:"change_log"`
	CreatedBy string    `gorm:"size:64;comment:创建人" json:"created_by"`
	CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`

	// 关联关系
	Script Script `gorm:"foreignKey:ScriptID" json:"script,omitempty"`
}

// TableName 指定表名
func (ScriptVersion) TableName() string {
	return "tb_script_version"
}

// ScriptExecution 脚本执行记录
type ScriptExecution struct {
	ID          uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	ScriptID    uint            `gorm:"not null;index;comment:脚本ID" json:"script_id"`
	VersionID   uint            `gorm:"not null;comment:版本ID" json:"version_id"`
	TriggerType string          `gorm:"size:32;default:'manual';comment:触发类型:manual/scheduled" json:"trigger_type"`
	ClientIDs   string          `gorm:"type:text;comment:目标客户端ID列表(JSON)" json:"client_ids"`
	Status      ExecutionStatus `gorm:"size:32;default:'pending';comment:执行状态" json:"status"`
	Output      string          `gorm:"type:longtext;comment:执行输出" json:"output"`
	Error       string          `gorm:"type:text;comment:错误信息" json:"error"`
	Timeout     int             `gorm:"default:300;comment:超时时间(秒)" json:"timeout"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty"`
	CreatedAt   time.Time       `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`

	// 关联关系
	Script  Script       `gorm:"foreignKey:ScriptID" json:"script,omitempty"`
	Version ScriptVersion `gorm:"foreignKey:VersionID" json:"version,omitempty"`

	// 执行结果详情（不存储，运行时填充）
	Results []ExecutionResult `gorm:"-" json:"results,omitempty"`
}

// TableName 指定表名
func (ScriptExecution) TableName() string {
	return "tb_script_execution"
}

// ExecutionResult 单个客户端的执行结果
type ExecutionResult struct {
	ClientID  string    `json:"client_id"`
	Status    string    `json:"status"`
	Output    string    `json:"output"`
	Error     string    `json:"error"`
	StartedAt time.Time `json:"started_at,omitempty"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
}

// ScriptWithStats 带统计信息的脚本
type ScriptWithStats struct {
	ID              uint           `json:"id"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Category        ScriptCategory `json:"category"`
	Interpreter     string         `json:"interpreter"`
	Status          ScriptStatus   `json:"status"`
	CreatedBy       string         `json:"created_by"`
	VersionCount    int64          `json:"version_count"`
	ExecutionCount  int64          `json:"execution_count"`
	LastExecutedAt  *time.Time     `json:"last_executed_at,omitempty"`
	CurrentVersion  string         `json:"current_version,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// AllModels 所有需要迁移的模型
var AllModels = []interface{}{
	&Script{},
	&ScriptVersion{},
	&ScriptExecution{},
}

// Migrate 执行数据库迁移
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(AllModels...)
}
