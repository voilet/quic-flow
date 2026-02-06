package configcenter

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store 配置存储接口
type Store interface {
	// ========== 配置管理 ==========
	CreateConfig(ctx context.Context, config *Config) error
	GetConfig(ctx context.Context, id uint) (*Config, error)
	GetConfigByKeys(ctx context.Context, namespace, group, dataID string) (*Config, error)
	UpdateConfig(ctx context.Context, config *Config) error
	DeleteConfig(ctx context.Context, id uint) error
	ListConfigs(ctx context.Context, filter *ConfigFilter) ([]*Config, int64, error)
	IncrementVersion(ctx context.Context, id uint) error

	// ========== 发布管理 ==========
	CreateRelease(ctx context.Context, release *ConfigRelease) error
	GetRelease(ctx context.Context, id uint) (*ConfigRelease, error)
	UpdateRelease(ctx context.Context, release *ConfigRelease) error
	ListReleases(ctx context.Context, filter *ReleaseFilter) ([]*ConfigRelease, int64, error)
	GetLatestRelease(ctx context.Context, configID uint) (*ConfigRelease, error)
	UpdateReleaseStatus(ctx context.Context, id uint, status ReleaseStatus) error

	// ========== 灰度规则 ==========
	CreateGrayRule(ctx context.Context, rule *GrayRule) error
	GetGrayRule(ctx context.Context, id uint) (*GrayRule, error)
	ListGrayRules(ctx context.Context, configID uint) ([]*GrayRule, error)
	UpdateGrayRule(ctx context.Context, rule *GrayRule) error
	DeleteGrayRule(ctx context.Context, id uint) error
	GetEnabledGrayRules(ctx context.Context, configID uint) ([]*GrayRule, error)

	// ========== 订阅管理 ==========
	RegisterSubscriber(ctx context.Context, subscriber *ConfigSubscriber) error
	UnregisterSubscriber(ctx context.Context, clientID string) error
	UpdateSubscriberHeartbeat(ctx context.Context, clientID string) error
	ListSubscribers(ctx context.Context, filter *SubscriberFilter) ([]*ConfigSubscriber, int64, error)
	GetSubscriber(ctx context.Context, clientID string) (*ConfigSubscriber, error)
	UpdateSubscriber(ctx context.Context, subscriber *ConfigSubscriber) error

	// ========== 变更历史 ==========
	CreateChangeLog(ctx context.Context, log *ConfigChangeLog) error
	ListChangeLogs(ctx context.Context, filter *ChangeLogFilter) ([]*ConfigChangeLog, error)

	// ========== 推送消息 ==========
	CreatePushMessage(ctx context.Context, msg *ConfigPushMessage) error
	GetPushMessage(ctx context.Context, msgID string) (*ConfigPushMessage, error)
	UpdatePushMessageStatus(ctx context.Context, msgID string, status string) error
	UpdatePushMessageError(ctx context.Context, msgID, errorMsg string, errorTime *time.Time) error
	ListPendingPushMessages(ctx context.Context, limit int) ([]*ConfigPushMessage, error)
	ListPushMessagesByRelease(ctx context.Context, releaseID string) ([]*ConfigPushMessage, error)

	// ========== 快照管理 ==========
	CreateSnapshot(ctx context.Context, snapshot *ConfigSnapshot) error
	GetSnapshot(ctx context.Context, id uint) (*ConfigSnapshot, error)
	ListSnapshots(ctx context.Context, configID uint, limit int) ([]*ConfigSnapshot, error)
	DeleteExpiredSnapshots(ctx context.Context) (int64, error)

	// ========== 编辑锁 ==========
	AcquireEditLock(ctx context.Context, configID uint, lockedBy, sessionID string, ttl time.Duration) (*ConfigEditLock, error)
	ReleaseEditLock(ctx context.Context, configID uint, sessionID string) error
	GetEditLock(ctx context.Context, configID uint) (*ConfigEditLock, error)
	CleanupExpiredLocks(ctx context.Context) (int64, error)
}

// ConfigFilter 配置查询过滤器
type ConfigFilter struct {
	Namespace  string
	Group      string
	DataID     string
	ConfigType ConfigType
	Tags       []string
	Keyword    string // 模糊搜索关键词
	Page       int
	PageSize   int
}

// ReleaseFilter 发布记录查询过滤器
type ReleaseFilter struct {
	ConfigID    *uint
	Namespace   string
	Group       string
	DataID      string
	ReleaseType ReleaseType
	Status      ReleaseStatus
	StartTime   *time.Time
	EndTime     *time.Time
	Page        int
	PageSize    int
}

// SubscriberFilter 订阅者查询过滤器
type SubscriberFilter struct {
	Namespace string
	ClientID  string
	Status    SubscriberStatus
	Page      int
	PageSize  int
}

// ChangeLogFilter 变更日志查询过滤器
type ChangeLogFilter struct {
	ConfigID   *uint
	Namespace  string
	Group      string
	DataID     string
	ChangeType string
	StartTime  *time.Time
	EndTime    *time.Time
	Offset     int
	Limit      int
}

// configStore 配置存储实现
type configStore struct {
	db *gorm.DB
}

// NewStore 创建配置存储
func NewStore(db *gorm.DB) Store {
	return &configStore{db: db}
}

// ==================== 配置管理实现 ====================

// CreateConfig 创建配置
func (s *configStore) CreateConfig(ctx context.Context, config *Config) error {
	return s.db.WithContext(ctx).Create(config).Error
}

// GetConfig 获取配置
func (s *configStore) GetConfig(ctx context.Context, id uint) (*Config, error) {
	var config Config
	err := s.db.WithContext(ctx).
		Preload("Releases").
		Preload("GrayRules").
		First(&config, id).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetConfigByKeys 根据键获取配置
func (s *configStore) GetConfigByKeys(ctx context.Context, namespace, group, dataID string) (*Config, error) {
	var config Config
	err := s.db.WithContext(ctx).
		Where("namespace = ? AND group = ? AND data_id = ?", namespace, group, dataID).
		First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// UpdateConfig 更新配置
func (s *configStore) UpdateConfig(ctx context.Context, config *Config) error {
	return s.db.WithContext(ctx).Save(config).Error
}

// DeleteConfig 删除配置
func (s *configStore) DeleteConfig(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&Config{}, id).Error
}

// ListConfigs 列出配置
func (s *configStore) ListConfigs(ctx context.Context, filter *ConfigFilter) ([]*Config, int64, error) {
	var configs []*Config
	var total int64

	query := s.db.WithContext(ctx).Model(&Config{})

	// 应用过滤条件
	if filter != nil {
		if filter.Namespace != "" {
			query = query.Where("namespace = ?", filter.Namespace)
		}
		if filter.Group != "" {
			query = query.Where("group = ?", filter.Group)
		}
		if filter.DataID != "" {
			query = query.Where("data_id = ?", filter.DataID)
		}
		if filter.ConfigType != "" {
			query = query.Where("config_type = ?", filter.ConfigType)
		}
		if filter.Keyword != "" {
			query = query.Where("data_id LIKE ? OR description LIKE ?",
				"%"+filter.Keyword+"%", "%"+filter.Keyword+"%")
		}
		// 标签过滤（JSONB 包含查询）
		if len(filter.Tags) > 0 {
			for _, tag := range filter.Tags {
				query = query.Where("tags @> ?", fmt.Sprintf(`["%s"]`, tag))
			}
		}
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	if filter != nil && filter.Page > 0 && filter.PageSize > 0 {
		offset := (filter.Page - 1) * filter.PageSize
		query = query.Offset(offset).Limit(filter.PageSize)
	}

	// 查询
	err := query.Order("created_at DESC").Find(&configs).Error
	return configs, total, err
}

// IncrementVersion 增加配置版本号
func (s *configStore) IncrementVersion(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).
		Model(&Config{}).
		Where("id = ?", id).
		UpdateColumn("version", gorm.Expr("version + 1")).
		Error
}

// ==================== 发布管理实现 ====================

// CreateRelease 创建发布记录
func (s *configStore) CreateRelease(ctx context.Context, release *ConfigRelease) error {
	return s.db.WithContext(ctx).Create(release).Error
}

// GetRelease 获取发布记录
func (s *configStore) GetRelease(ctx context.Context, id uint) (*ConfigRelease, error) {
	var release ConfigRelease
	err := s.db.WithContext(ctx).
		Preload("Config").
		Preload("GrayRule").
		First(&release, id).Error
	if err != nil {
		return nil, err
	}
	return &release, nil
}

// UpdateRelease 更新发布记录
func (s *configStore) UpdateRelease(ctx context.Context, release *ConfigRelease) error {
	return s.db.WithContext(ctx).Save(release).Error
}

// ListReleases 列出发布记录
func (s *configStore) ListReleases(ctx context.Context, filter *ReleaseFilter) ([]*ConfigRelease, int64, error) {
	var releases []*ConfigRelease
	var total int64

	query := s.db.WithContext(ctx).Model(&ConfigRelease{})

	// 应用过滤条件
	if filter != nil {
		if filter.ConfigID != nil {
			query = query.Where("config_id = ?", *filter.ConfigID)
		}
		if filter.Namespace != "" {
			query = query.Where("namespace = ?", filter.Namespace)
		}
		if filter.Group != "" {
			query = query.Where("group = ?", filter.Group)
		}
		if filter.DataID != "" {
			query = query.Where("data_id = ?", filter.DataID)
		}
		if filter.ReleaseType != "" {
			query = query.Where("release_type = ?", filter.ReleaseType)
		}
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
		if filter.StartTime != nil {
			query = query.Where("released_at >= ?", *filter.StartTime)
		}
		if filter.EndTime != nil {
			query = query.Where("released_at <= ?", *filter.EndTime)
		}
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	if filter != nil && filter.Page > 0 && filter.PageSize > 0 {
		offset := (filter.Page - 1) * filter.PageSize
		query = query.Offset(offset).Limit(filter.PageSize)
	}

	// 查询
	err := query.Order("released_at DESC").Find(&releases).Error
	return releases, total, err
}

// GetLatestRelease 获取最新发布记录
func (s *configStore) GetLatestRelease(ctx context.Context, configID uint) (*ConfigRelease, error) {
	var release ConfigRelease
	err := s.db.WithContext(ctx).
		Where("config_id = ?", configID).
		Order("released_at DESC").
		First(&release).Error
	if err != nil {
		return nil, err
	}
	return &release, nil
}

// UpdateReleaseStatus 更新发布状态
func (s *configStore) UpdateReleaseStatus(ctx context.Context, id uint, status ReleaseStatus) error {
	return s.db.WithContext(ctx).
		Model(&ConfigRelease{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// ==================== 灰度规则实现 ====================

// CreateGrayRule 创建灰度规则
func (s *configStore) CreateGrayRule(ctx context.Context, rule *GrayRule) error {
	return s.db.WithContext(ctx).Create(rule).Error
}

// GetGrayRule 获取灰度规则
func (s *configStore) GetGrayRule(ctx context.Context, id uint) (*GrayRule, error) {
	var rule GrayRule
	err := s.db.WithContext(ctx).First(&rule, id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// ListGrayRules 列出灰度规则
func (s *configStore) ListGrayRules(ctx context.Context, configID uint) ([]*GrayRule, error) {
	var rules []*GrayRule
	err := s.db.WithContext(ctx).
		Where("config_id = ?", configID).
		Order("priority DESC, created_at DESC").
		Find(&rules).Error
	return rules, err
}

// UpdateGrayRule 更新灰度规则
func (s *configStore) UpdateGrayRule(ctx context.Context, rule *GrayRule) error {
	return s.db.WithContext(ctx).Save(rule).Error
}

// DeleteGrayRule 删除灰度规则
func (s *configStore) DeleteGrayRule(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&GrayRule{}, id).Error
}

// GetEnabledGrayRules 获取启用的灰度规则
func (s *configStore) GetEnabledGrayRules(ctx context.Context, configID uint) ([]*GrayRule, error) {
	var rules []*GrayRule
	err := s.db.WithContext(ctx).
		Where("config_id = ? AND enabled = ?", configID, true).
		Order("priority DESC").
		Find(&rules).Error
	return rules, err
}

// ==================== 订阅管理实现 ====================

// RegisterSubscriber 注册订阅者
func (s *configStore) RegisterSubscriber(ctx context.Context, subscriber *ConfigSubscriber) error {
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "client_id"}, {Name: "namespace"}},
			DoUpdates: clause.AssignmentColumns([]string{"sdk_type", "subscriptions", "client_ip", "client_tags", "last_active", "status", "updated_at"}),
		}).
		Create(subscriber).Error
}

// UnregisterSubscriber 取消注册订阅者
func (s *configStore) UnregisterSubscriber(ctx context.Context, clientID string) error {
	return s.db.WithContext(ctx).
		Where("client_id = ?", clientID).
		Delete(&ConfigSubscriber{}).Error
}

// UpdateSubscriberHeartbeat 更新订阅者心跳
func (s *configStore) UpdateSubscriberHeartbeat(ctx context.Context, clientID string) error {
	now := time.Now()
	return s.db.WithContext(ctx).
		Model(&ConfigSubscriber{}).
		Where("client_id = ?", clientID).
		Updates(map[string]interface{}{
			"last_active": now,
			"status":      string(SubscriberStatusOnline),
			"updated_at":  now,
		}).Error
}

// ListSubscribers 列出订阅者
func (s *configStore) ListSubscribers(ctx context.Context, filter *SubscriberFilter) ([]*ConfigSubscriber, int64, error) {
	var subscribers []*ConfigSubscriber
	var total int64

	query := s.db.WithContext(ctx).Model(&ConfigSubscriber{})

	// 应用过滤条件
	if filter != nil {
		if filter.Namespace != "" {
			query = query.Where("namespace = ?", filter.Namespace)
		}
		if filter.ClientID != "" {
			query = query.Where("client_id LIKE ?", "%"+filter.ClientID+"%")
		}
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	if filter != nil && filter.Page > 0 && filter.PageSize > 0 {
		offset := (filter.Page - 1) * filter.PageSize
		query = query.Offset(offset).Limit(filter.PageSize)
	}

	// 查询
	err := query.Order("last_active DESC").Find(&subscribers).Error
	return subscribers, total, err
}

// GetSubscriber 获取订阅者
func (s *configStore) GetSubscriber(ctx context.Context, clientID string) (*ConfigSubscriber, error) {
	var subscriber ConfigSubscriber
	err := s.db.WithContext(ctx).
		Where("client_id = ?", clientID).
		First(&subscriber).Error
	if err != nil {
		return nil, err
	}
	return &subscriber, nil
}

// UpdateSubscriber 更新订阅者
func (s *configStore) UpdateSubscriber(ctx context.Context, subscriber *ConfigSubscriber) error {
	return s.db.WithContext(ctx).Save(subscriber).Error
}

// ==================== 变更历史实现 ====================

// CreateChangeLog 创建变更日志
func (s *configStore) CreateChangeLog(ctx context.Context, log *ConfigChangeLog) error {
	return s.db.WithContext(ctx).Create(log).Error
}

// ListChangeLogs 列出变更日志
func (s *configStore) ListChangeLogs(ctx context.Context, filter *ChangeLogFilter) ([]*ConfigChangeLog, error) {
	var logs []*ConfigChangeLog

	query := s.db.WithContext(ctx).Model(&ConfigChangeLog{})

	// 应用过滤条件
	if filter != nil {
		if filter.ConfigID != nil {
			query = query.Where("config_id = ?", *filter.ConfigID)
		}
		if filter.Namespace != "" {
			query = query.Where("namespace = ?", filter.Namespace)
		}
		if filter.Group != "" {
			query = query.Where("group = ?", filter.Group)
		}
		if filter.DataID != "" {
			query = query.Where("data_id = ?", filter.DataID)
		}
		if filter.ChangeType != "" {
			query = query.Where("change_type = ?", filter.ChangeType)
		}
		if filter.StartTime != nil {
			query = query.Where("operated_at >= ?", *filter.StartTime)
		}
		if filter.EndTime != nil {
			query = query.Where("operated_at <= ?", *filter.EndTime)
		}
		// 分页
		if filter.Offset > 0 {
			query = query.Offset(filter.Offset)
		}
		if filter.Limit > 0 {
			query = query.Limit(filter.Limit)
		}
	}

	// 查询
	err := query.Order("operated_at DESC").Find(&logs).Error
	return logs, err
}

// ==================== 推送消息实现 ====================

// CreatePushMessage 创建推送消息
func (s *configStore) CreatePushMessage(ctx context.Context, msg *ConfigPushMessage) error {
	return s.db.WithContext(ctx).Create(msg).Error
}

// GetPushMessage 获取推送消息
func (s *configStore) GetPushMessage(ctx context.Context, msgID string) (*ConfigPushMessage, error) {
	var msg ConfigPushMessage
	err := s.db.WithContext(ctx).
		Where("msg_id = ?", msgID).
		First(&msg).Error
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// UpdatePushMessageStatus 更新推送消息状态
func (s *configStore) UpdatePushMessageStatus(ctx context.Context, msgID string, status string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	// 根据状态设置时间戳
	now := time.Now()
	if status == "sent" {
		updates["sent_at"] = &now
	} else if status == "acknowledged" {
		updates["ack_at"] = &now
	}

	return s.db.WithContext(ctx).
		Model(&ConfigPushMessage{}).
		Where("msg_id = ?", msgID).
		Updates(updates).Error
}

// ListPendingPushMessages 列出待推送消息
func (s *configStore) ListPendingPushMessages(ctx context.Context, limit int) ([]*ConfigPushMessage, error) {
	var msgs []*ConfigPushMessage
	err := s.db.WithContext(ctx).
		Where("status = ?", "pending").
		Order("created_at ASC").
		Limit(limit).
		Find(&msgs).Error
	return msgs, err
}

// ListPushMessagesByRelease 根据发布 ID 列出推送消息
func (s *configStore) ListPushMessagesByRelease(ctx context.Context, releaseID string) ([]*ConfigPushMessage, error) {
	var msgs []*ConfigPushMessage
	err := s.db.WithContext(ctx).
		Where("release_id = ?", releaseID).
		Order("created_at ASC").
		Find(&msgs).Error
	return msgs, err
}

// UpdatePushMessageError 更新推送消息错误信息
func (s *configStore) UpdatePushMessageError(ctx context.Context, msgID, errorMsg string, errorTime *time.Time) error {
	updates := map[string]interface{}{
		"error_msg":   errorMsg,
		"status":      "failed",
		"updated_at":  time.Now(),
	}
	if errorTime != nil {
		updates["ack_at"] = errorTime
	}
	return s.db.WithContext(ctx).
		Model(&ConfigPushMessage{}).
		Where("msg_id = ?", msgID).
		Updates(updates).Error
}

// ==================== 快照管理实现 ====================

// CreateSnapshot 创建快照
func (s *configStore) CreateSnapshot(ctx context.Context, snapshot *ConfigSnapshot) error {
	return s.db.WithContext(ctx).Create(snapshot).Error
}

// GetSnapshot 获取快照
func (s *configStore) GetSnapshot(ctx context.Context, id uint) (*ConfigSnapshot, error) {
	var snapshot ConfigSnapshot
	err := s.db.WithContext(ctx).First(&snapshot, id).Error
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

// ListSnapshots 列出快照
func (s *configStore) ListSnapshots(ctx context.Context, configID uint, limit int) ([]*ConfigSnapshot, error) {
	var snapshots []*ConfigSnapshot
	query := s.db.WithContext(ctx).
		Where("config_id = ?", configID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&snapshots).Error
	return snapshots, err
}

// DeleteExpiredSnapshots 删除过期快照
func (s *configStore) DeleteExpiredSnapshots(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at < ?", time.Now()).
		Delete(&ConfigSnapshot{})
	return result.RowsAffected, result.Error
}

// ==================== 编辑锁实现 ====================

// AcquireEditLock 获取编辑锁
func (s *configStore) AcquireEditLock(ctx context.Context, configID uint, lockedBy, sessionID string, ttl time.Duration) (*ConfigEditLock, error) {
	now := time.Now()
	expiresAt := now.Add(ttl)

	// 首先清理过期锁
	s.CleanupExpiredLocks(ctx)

	// 检查是否已有锁
	var existingLock ConfigEditLock
	err := s.db.WithContext(ctx).
		Where("config_id = ?", configID).
		First(&existingLock).Error

	if err == nil {
		// 锁已存在，检查是否过期
		if !existingLock.IsExpired() {
			// 锁仍然有效
			return nil, fmt.Errorf("config is locked by %s until %s",
				existingLock.LockedBy, existingLock.ExpiresAt.Format(time.RFC3339))
		}
		// 锁已过期，删除旧锁
		s.db.WithContext(ctx).Delete(&existingLock)
	}

	// 创建新锁
	lock := &ConfigEditLock{
		ConfigID:  configID,
		LockedBy:  lockedBy,
		LockedAt:  now,
		ExpiresAt: expiresAt,
		SessionID: sessionID,
	}

	err = s.db.WithContext(ctx).Create(lock).Error
	if err != nil {
		return nil, err
	}

	return lock, nil
}

// ReleaseEditLock 释放编辑锁
func (s *configStore) ReleaseEditLock(ctx context.Context, configID uint, sessionID string) error {
	return s.db.WithContext(ctx).
		Where("config_id = ? AND session_id = ?", configID, sessionID).
		Delete(&ConfigEditLock{}).Error
}

// GetEditLock 获取编辑锁
func (s *configStore) GetEditLock(ctx context.Context, configID uint) (*ConfigEditLock, error) {
	var lock ConfigEditLock
	err := s.db.WithContext(ctx).
		Where("config_id = ?", configID).
		First(&lock).Error
	if err != nil {
		return nil, err
	}
	return &lock, nil
}

// CleanupExpiredLocks 清理过期锁
func (s *configStore) CleanupExpiredLocks(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&ConfigEditLock{})
	return result.RowsAffected, result.Error
}
