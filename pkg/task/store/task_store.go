package store

import (
	"context"
	"fmt"
	"time"

	"github.com/voilet/quic-flow/pkg/task/models"
	"gorm.io/gorm"
)

// TaskStore 任务存储接口
type TaskStore interface {
	Create(ctx context.Context, task *models.Task) error
	Update(ctx context.Context, task *models.Task) error
	Delete(ctx context.Context, taskID int64) error
	GetByID(ctx context.Context, taskID int64) (*models.Task, error)
	List(ctx context.Context, params *ListParams) ([]*models.Task, int64, error)
	ListEnabled(ctx context.Context) ([]*models.Task, error)
	BindGroup(ctx context.Context, taskID int64, groupID int64) error
	UnbindGroup(ctx context.Context, taskID int64, groupID int64) error
	GetGroupIDs(ctx context.Context, taskID int64) ([]int64, error)
	
	// 增强方法 - 执行统计
	UpdateNextRunTime(ctx context.Context, taskID int64, nextRunTime *time.Time) error
	UpdateExecutionStats(ctx context.Context, taskID int64, success bool) error
	IncrementExecutionCount(ctx context.Context, taskID int64) error
	GetTasksNeedingExecution(ctx context.Context, beforeTime time.Time) ([]*models.Task, error)
	GetTaskStats(ctx context.Context, taskID int64) (*TaskStats, error)
}

// TaskStats 任务统计信息
type TaskStats struct {
	TotalExecutions   int64   `json:"total_executions"`
	SuccessCount      int64   `json:"success_count"`
	FailedCount       int64   `json:"failed_count"`
	SuccessRate       float64 `json:"success_rate"`
	LastExecutionTime string  `json:"last_execution_time"`
	NextExecutionTime string  `json:"next_execution_time"`
}

// ListParams 列表查询参数
type ListParams struct {
	Page     int    // 页码（从1开始）
	PageSize int    // 每页数量
	Status   *int   // 状态筛选（可选）
	Keyword  string // 关键词搜索（名称、描述）
}

// taskStoreImpl 任务存储实现
type taskStoreImpl struct {
	db *gorm.DB
}

// NewTaskStore 创建任务存储
func NewTaskStore(db *gorm.DB) TaskStore {
	return &taskStoreImpl{db: db}
}

// Create 创建任务
func (s *taskStoreImpl) Create(ctx context.Context, task *models.Task) error {
	return s.db.WithContext(ctx).Create(task).Error
}

// Update 更新任务
func (s *taskStoreImpl) Update(ctx context.Context, task *models.Task) error {
	return s.db.WithContext(ctx).Model(task).Updates(task).Error
}

// Delete 删除任务（软删除）
func (s *taskStoreImpl) Delete(ctx context.Context, taskID int64) error {
	return s.db.WithContext(ctx).Delete(&models.Task{}, taskID).Error
}

// GetByID 根据ID获取任务
func (s *taskStoreImpl) GetByID(ctx context.Context, taskID int64) (*models.Task, error) {
	var task models.Task
	err := s.db.WithContext(ctx).Preload("Groups").First(&task, taskID).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// List 列表查询任务
func (s *taskStoreImpl) List(ctx context.Context, params *ListParams) ([]*models.Task, int64, error) {
	if params == nil {
		params = &ListParams{
			Page:     1,
			PageSize: 20,
		}
	}

	query := s.db.WithContext(ctx).Model(&models.Task{})

	// 状态筛选
	if params.Status != nil {
		query = query.Where("status = ?", *params.Status)
	}

	// 关键词搜索
	if params.Keyword != "" {
		keyword := "%" + params.Keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", keyword, keyword)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	var tasks []*models.Task
	offset := (params.Page - 1) * params.PageSize
	err := query.Preload("Groups").
		Offset(offset).
		Limit(params.PageSize).
		Order("created_at DESC").
		Find(&tasks).Error

	return tasks, total, err
}

// ListEnabled 获取所有启用的任务
func (s *taskStoreImpl) ListEnabled(ctx context.Context) ([]*models.Task, error) {
	var tasks []*models.Task
	err := s.db.WithContext(ctx).
		Where("status = ?", int(models.TaskStatusEnabled)).
		Preload("Groups").
		Find(&tasks).Error
	return tasks, err
}

// BindGroup 绑定任务到分组
func (s *taskStoreImpl) BindGroup(ctx context.Context, taskID int64, groupID int64) error {
	var task models.Task
	if err := s.db.WithContext(ctx).First(&task, taskID).Error; err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	var group models.TaskGroup
	if err := s.db.WithContext(ctx).First(&group, groupID).Error; err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	return s.db.WithContext(ctx).Model(&task).Association("Groups").Append(&group)
}

// UnbindGroup 解绑任务和分组
func (s *taskStoreImpl) UnbindGroup(ctx context.Context, taskID int64, groupID int64) error {
	var task models.Task
	if err := s.db.WithContext(ctx).First(&task, taskID).Error; err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	var group models.TaskGroup
	if err := s.db.WithContext(ctx).First(&group, groupID).Error; err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	return s.db.WithContext(ctx).Model(&task).Association("Groups").Delete(&group)
}

// GetGroupIDs 获取任务关联的分组ID列表
func (s *taskStoreImpl) GetGroupIDs(ctx context.Context, taskID int64) ([]int64, error) {
	var task models.Task
	if err := s.db.WithContext(ctx).Preload("Groups").First(&task, taskID).Error; err != nil {
		return nil, err
	}

	groupIDs := make([]int64, len(task.Groups))
	for i, group := range task.Groups {
		groupIDs[i] = group.ID
	}

	return groupIDs, nil
}

// UpdateNextRunTime 更新下次执行时间
func (s *taskStoreImpl) UpdateNextRunTime(ctx context.Context, taskID int64, nextRunTime *time.Time) error {
	return s.db.WithContext(ctx).
		Model(&models.Task{}).
		Where("id = ?", taskID).
		Update("next_run_time", nextRunTime).Error
}

// UpdateExecutionStats 更新执行统计
func (s *taskStoreImpl) UpdateExecutionStats(ctx context.Context, taskID int64, success bool) error {
	now := time.Now()
	updates := map[string]interface{}{
		"last_run_time":   now,
		"execution_count": gorm.Expr("execution_count + 1"),
	}

	if success {
		updates["success_count"] = gorm.Expr("success_count + 1")
	} else {
		updates["failed_count"] = gorm.Expr("failed_count + 1")
	}

	return s.db.WithContext(ctx).
		Model(&models.Task{}).
		Where("id = ?", taskID).
		Updates(updates).Error
}

// IncrementExecutionCount 增加执行次数
func (s *taskStoreImpl) IncrementExecutionCount(ctx context.Context, taskID int64) error {
	now := time.Now()
	return s.db.WithContext(ctx).
		Model(&models.Task{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"execution_count": gorm.Expr("execution_count + 1"),
			"last_run_time":   now,
		}).Error
}

// GetTasksNeedingExecution 获取需要执行的任务（下次执行时间已过）
func (s *taskStoreImpl) GetTasksNeedingExecution(ctx context.Context, beforeTime time.Time) ([]*models.Task, error) {
	var tasks []*models.Task
	err := s.db.WithContext(ctx).
		Where("status = ?", int(models.TaskStatusEnabled)).
		Where("next_run_time IS NOT NULL").
		Where("next_run_time <= ?", beforeTime).
		Where("(max_executions = 0 OR execution_count < max_executions)").
		Where("(end_time IS NULL OR end_time > ?)", time.Now()).
		Preload("Groups").
		Find(&tasks).Error
	return tasks, err
}

// GetTaskStats 获取任务统计信息
func (s *taskStoreImpl) GetTaskStats(ctx context.Context, taskID int64) (*TaskStats, error) {
	var task models.Task
	if err := s.db.WithContext(ctx).First(&task, taskID).Error; err != nil {
		return nil, err
	}

	stats := &TaskStats{
		TotalExecutions: task.ExecutionCount,
		SuccessCount:    task.SuccessCount,
		FailedCount:     task.FailedCount,
		SuccessRate:     task.GetSuccessRate(),
	}

	if task.LastRunTime != nil {
		stats.LastExecutionTime = task.LastRunTime.Format(time.RFC3339)
	}
	if task.NextRunTime != nil {
		stats.NextExecutionTime = task.NextRunTime.Format(time.RFC3339)
	}

	return stats, nil
}
