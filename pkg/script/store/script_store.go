package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/voilet/quic-flow/pkg/script/models"
	"gorm.io/gorm"
)

// ScriptStore 脚本存储接口
type ScriptStore interface {
	// 脚本管理
	Create(ctx context.Context, script *models.Script) error
	Update(ctx context.Context, script *models.Script) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*models.Script, error)
	GetByName(ctx context.Context, name string) (*models.Script, error)
	List(ctx context.Context, category string, status string) ([]*models.Script, error)
	ListWithStats(ctx context.Context, category string) ([]*models.ScriptWithStats, error)

	// 版本管理
	CreateVersion(ctx context.Context, version *models.ScriptVersion) error
	GetVersions(ctx context.Context, scriptID uint) ([]*models.ScriptVersion, error)
	GetVersion(ctx context.Context, versionID uint) (*models.ScriptVersion, error)
	GetLatestVersion(ctx context.Context, scriptID uint) (*models.ScriptVersion, error)

	// 执行记录
	CreateExecution(ctx context.Context, execution *models.ScriptExecution) error
	UpdateExecution(ctx context.Context, execution *models.ScriptExecution) error
	GetExecution(ctx context.Context, id uint) (*models.ScriptExecution, error)
	ListExecutions(ctx context.Context, scriptID uint, limit int) ([]*models.ScriptExecution, error)
}

// scriptStoreImpl 脚本存储实现
type scriptStoreImpl struct {
	db *gorm.DB
}

// NewScriptStore 创建脚本存储
func NewScriptStore(db *gorm.DB) ScriptStore {
	return &scriptStoreImpl{db: db}
}

// Create 创建脚本
func (s *scriptStoreImpl) Create(ctx context.Context, script *models.Script) error {
	return s.db.WithContext(ctx).Create(script).Error
}

// Update 更新脚本
func (s *scriptStoreImpl) Update(ctx context.Context, script *models.Script) error {
	return s.db.WithContext(ctx).Model(script).Updates(map[string]interface{}{
		"name":        script.Name,
		"description": script.Description,
		"category":    script.Category,
		"interpreter": script.Interpreter,
		"content":     script.Content,
		"status":      script.Status,
		"updated_at":  time.Now(),
	}).Error
}

// Delete 删除脚本（软删除）
func (s *scriptStoreImpl) Delete(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.Script{}, id).Error
}

// GetByID 根据ID获取脚本
func (s *scriptStoreImpl) GetByID(ctx context.Context, id uint) (*models.Script, error) {
	var script models.Script
	err := s.db.WithContext(ctx).First(&script, id).Error
	if err != nil {
		return nil, err
	}
	return &script, nil
}

// GetByName 根据名称获取脚本
func (s *scriptStoreImpl) GetByName(ctx context.Context, name string) (*models.Script, error) {
	var script models.Script
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&script).Error
	if err != nil {
		return nil, err
	}
	return &script, nil
}

// List 列表查询脚本
func (s *scriptStoreImpl) List(ctx context.Context, category string, status string) ([]*models.Script, error) {
	var scripts []*models.Script
	query := s.db.WithContext(ctx)

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("updated_at DESC").Find(&scripts).Error
	return scripts, err
}

// ListWithStats 列表查询脚本（带统计信息）
func (s *scriptStoreImpl) ListWithStats(ctx context.Context, category string) ([]*models.ScriptWithStats, error) {
	var scripts []*models.ScriptWithStats

	query := `
		SELECT
			s.id,
			s.name,
			s.description,
			s.category,
			s.interpreter,
			s.status,
			s.created_by,
			s.created_at,
			s.updated_at,
			COUNT(DISTINCT sv.id) as version_count,
			COUNT(DISTINCT se.id) as execution_count,
			MAX(se.created_at) as last_executed_at,
			(SELECT version FROM tb_script_version WHERE script_id = s.id ORDER BY created_at DESC LIMIT 1) as current_version
		FROM tb_script s
		LEFT JOIN tb_script_version sv ON s.id = sv.script_id
		LEFT JOIN tb_script_execution se ON s.id = se.script_id
		WHERE s.deleted_at IS NULL
	`
	args := []interface{}{}

	if category != "" {
		query += " AND s.category = ?"
		args = append(args, category)
	}

	query += " GROUP BY s.id ORDER BY s.updated_at DESC"

	err := s.db.WithContext(ctx).Raw(query, args...).Scan(&scripts).Error
	return scripts, err
}

// CreateVersion 创建版本
func (s *scriptStoreImpl) CreateVersion(ctx context.Context, version *models.ScriptVersion) error {
	return s.db.WithContext(ctx).Create(version).Error
}

// GetVersions 获取脚本的所有版本
func (s *scriptStoreImpl) GetVersions(ctx context.Context, scriptID uint) ([]*models.ScriptVersion, error) {
	var versions []*models.ScriptVersion
	err := s.db.WithContext(ctx).
		Where("script_id = ?", scriptID).
		Order("created_at DESC").
		Find(&versions).Error
	return versions, err
}

// GetVersion 获取单个版本
func (s *scriptStoreImpl) GetVersion(ctx context.Context, versionID uint) (*models.ScriptVersion, error) {
	var version models.ScriptVersion
	err := s.db.WithContext(ctx).First(&version, versionID).Error
	if err != nil {
		return nil, err
	}
	return &version, nil
}

// GetLatestVersion 获取最新版本
func (s *scriptStoreImpl) GetLatestVersion(ctx context.Context, scriptID uint) (*models.ScriptVersion, error) {
	var version models.ScriptVersion
	err := s.db.WithContext(ctx).
		Where("script_id = ?", scriptID).
		Order("created_at DESC").
		First(&version).Error
	if err != nil {
		return nil, err
	}
	return &version, nil
}

// CreateExecution 创建执行记录
func (s *scriptStoreImpl) CreateExecution(ctx context.Context, execution *models.ScriptExecution) error {
	return s.db.WithContext(ctx).Create(execution).Error
}

// UpdateExecution 更新执行记录
func (s *scriptStoreImpl) UpdateExecution(ctx context.Context, execution *models.ScriptExecution) error {
	return s.db.WithContext(ctx).Model(execution).Updates(map[string]interface{}{
		"status":     execution.Status,
		"output":     execution.Output,
		"error":      execution.Error,
		"started_at": execution.StartedAt,
		"finished_at": execution.FinishedAt,
	}).Error
}

// GetExecution 获取执行记录
func (s *scriptStoreImpl) GetExecution(ctx context.Context, id uint) (*models.ScriptExecution, error) {
	var execution models.ScriptExecution
	err := s.db.WithContext(ctx).
		Preload("Script").
		Preload("Version").
		First(&execution, id).Error
	if err != nil {
		return nil, err
	}

	// 解析 ClientIDs
	if execution.ClientIDs != "" {
		var clientIDs []string
		if err := json.Unmarshal([]byte(execution.ClientIDs), &clientIDs); err == nil {
			for _, clientID := range clientIDs {
				execution.Results = append(execution.Results, models.ExecutionResult{
					ClientID: clientID,
					Status:   string(execution.Status),
				})
			}
		}
	}

	return &execution, nil
}

// ListExecutions 获取执行记录列表
func (s *scriptStoreImpl) ListExecutions(ctx context.Context, scriptID uint, limit int) ([]*models.ScriptExecution, error) {
	var executions []*models.ScriptExecution
	query := s.db.WithContext(ctx).
		Preload("Script").
		Preload("Version").
		Where("script_id = ?", scriptID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&executions).Error
	return executions, err
}
