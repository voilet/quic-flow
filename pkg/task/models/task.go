package models

import (
	"time"

	"gorm.io/gorm"
)

// ExecutorType 执行器类型
type ExecutorType int

const (
	ExecutorTypeShell ExecutorType = 1 // Shell 脚本
	ExecutorTypeHTTP  ExecutorType = 2 // HTTP 请求
	ExecutorTypePlugin ExecutorType = 3 // 插件
)

// TaskStatus 任务状态
type TaskStatus int

const (
	TaskStatusDisabled TaskStatus = 0 // 禁用
	TaskStatusEnabled  TaskStatus = 1 // 启用
)

// Task 定时任务表
type Task struct {
	ID             int64       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name           string      `gorm:"size:128;uniqueIndex;not null;comment:任务名称" json:"name"`
	Description    string      `gorm:"size:512;comment:任务描述" json:"description"`
	ExecutorType   ExecutorType `gorm:"not null;comment:执行器类型:1=Shell,2=HTTP,3=Plugin" json:"executor_type"`
	ExecutorConfig string      `gorm:"type:text;not null;comment:执行器配置(JSON)" json:"executor_config"`
	CronExpr       string      `gorm:"size:64;not null;comment:Cron表达式" json:"cron_expr"`
	
	// 增强调度字段
	Timezone       string      `gorm:"size:64;default:'Local';comment:时区" json:"timezone"`
	StartTime      *time.Time  `gorm:"comment:调度开始时间" json:"start_time,omitempty"`
	EndTime        *time.Time  `gorm:"comment:调度结束时间" json:"end_time,omitempty"`
	
	// 执行统计
	NextRunTime    *time.Time  `gorm:"index;comment:下次执行时间" json:"next_run_time,omitempty"`
	LastRunTime    *time.Time  `gorm:"comment:上次执行时间" json:"last_run_time,omitempty"`
	ExecutionCount int64       `gorm:"not null;default:0;comment:执行次数" json:"execution_count"`
	SuccessCount   int64       `gorm:"not null;default:0;comment:成功次数" json:"success_count"`
	FailedCount    int64       `gorm:"not null;default:0;comment:失败次数" json:"failed_count"`
	
	// 执行限制
	MaxExecutions  int64       `gorm:"not null;default:0;comment:最大执行次数(0=无限制)" json:"max_executions"`
	
	// 基础配置
	Timeout        int         `gorm:"not null;default:300;comment:超时时间(秒)" json:"timeout"`
	RetryCount     int         `gorm:"not null;default:0;comment:重试次数" json:"retry_count"`
	RetryInterval  int         `gorm:"not null;default:60;comment:重试间隔(秒)" json:"retry_interval"`
	Concurrency    int         `gorm:"not null;default:1;comment:最大并发数" json:"concurrency"`
	Status         TaskStatus  `gorm:"not null;default:1;index;comment:状态:0=禁用,1=启用" json:"status"`
	CreatedBy      string      `gorm:"size:64;comment:创建人" json:"created_by"`
	CreatedAt      time.Time   `gorm:"not null;default:CURRENT_TIMESTAMP;index" json:"created_at"`
	UpdatedAt      time.Time   `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联关系
	Groups []TaskGroup `gorm:"many2many:tb_task_group_relation;" json:"groups,omitempty"`
}

// TableName 指定表名
func (Task) TableName() string {
	return "tb_task"
}

// IsEnabled 检查任务是否启用
func (t *Task) IsEnabled() bool {
	return t.Status == TaskStatusEnabled
}

// CanExecute 检查任务是否可以执行
func (t *Task) CanExecute(now time.Time) bool {
	// 检查是否启用
	if !t.IsEnabled() {
		return false
	}

	// 检查调度时间窗口
	if t.StartTime != nil && now.Before(*t.StartTime) {
		return false
	}
	if t.EndTime != nil && now.After(*t.EndTime) {
		return false
	}

	// 检查最大执行次数限制
	if t.MaxExecutions > 0 && t.ExecutionCount >= t.MaxExecutions {
		return false
	}

	return true
}

// HasReachedMaxExecutions 检查是否已达到最大执行次数
func (t *Task) HasReachedMaxExecutions() bool {
	return t.MaxExecutions > 0 && t.ExecutionCount >= t.MaxExecutions
}

// IsInScheduleWindow 检查当前时间是否在调度时间窗口内
func (t *Task) IsInScheduleWindow(now time.Time) bool {
	if t.StartTime != nil && now.Before(*t.StartTime) {
		return false
	}
	if t.EndTime != nil && now.After(*t.EndTime) {
		return false
	}
	return true
}

// GetSuccessRate 获取成功率
func (t *Task) GetSuccessRate() float64 {
	if t.ExecutionCount == 0 {
		return 0
	}
	return float64(t.SuccessCount) / float64(t.ExecutionCount) * 100
}
