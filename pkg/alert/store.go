package alert

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ========== Store 接口 ==========

// Store 告警存储接口
type Store interface {
	// ========== 数据库迁移 ==========
	AutoMigrate(ctx context.Context) error

	// ========== 规则管理 ==========
	CreateRule(ctx context.Context, rule *AlertRule) error
	GetRule(ctx context.Context, id uint) (*AlertRule, error)
	GetRuleByName(ctx context.Context, name string) (*AlertRule, error)
	ListRules(ctx context.Context, filter *RuleFilter) ([]*AlertRule, int64, error)
	UpdateRule(ctx context.Context, rule *AlertRule) error
	DeleteRule(ctx context.Context, id uint) error

	// ========== 告警实例 ==========
	CreateAlert(ctx context.Context, alert *AlertInstance) error
	GetAlert(ctx context.Context, id uint) (*AlertInstance, error)
	GetAlertByFingerprint(ctx context.Context, fingerprint string) (*AlertInstance, error)
	ListAlerts(ctx context.Context, filter *AlertFilter) ([]*AlertInstance, int64, error)
	UpdateAlert(ctx context.Context, alert *AlertInstance) error
	ResolveAlert(ctx context.Context, id uint) error
	BatchResolveAlerts(ctx context.Context, fingerprints []string) error
	DeleteAlert(ctx context.Context, id uint) error

	// ========== 抑制规则 ==========
	CreateSilence(ctx context.Context, silence *SilenceRule) error
	GetSilence(ctx context.Context, id uint) (*SilenceRule, error)
	ListSilences(ctx context.Context, filter *SilenceFilter) ([]*SilenceRule, error)
	DeleteSilence(ctx context.Context, id uint) error

	// ========== 通知渠道 ==========
	CreateChannel(ctx context.Context, channel *NotifyChannel) error
	GetChannel(ctx context.Context, id uint) (*NotifyChannel, error)
	ListChannels(ctx context.Context, enabled *bool) ([]*NotifyChannel, error)
	UpdateChannel(ctx context.Context, channel *NotifyChannel) error
	DeleteChannel(ctx context.Context, id uint) error

	// ========== 通知历史 ==========
	CreateNotifyHistory(ctx context.Context, history *NotifyHistory) error
	ListNotifyHistory(ctx context.Context, filter *NotifyHistoryFilter) ([]*NotifyHistory, int64, error)

	// ========== 值班管理 ==========
	CreateOnCallSchedule(ctx context.Context, schedule *OnCallSchedule) error
	GetOnCallSchedule(ctx context.Context, id uint) (*OnCallSchedule, error)
	ListOnCallSchedules(ctx context.Context) ([]*OnCallSchedule, error)
	UpdateOnCallSchedule(ctx context.Context, schedule *OnCallSchedule) error
	DeleteOnCallSchedule(ctx context.Context, id uint) error

	CreateOnCallUser(ctx context.Context, user *OnCallUser) error
	GetOnCallUser(ctx context.Context, id uint) (*OnCallUser, error)
	ListOnCallUsers(ctx context.Context) ([]*OnCallUser, error)
	UpdateOnCallUser(ctx context.Context, user *OnCallUser) error
	DeleteOnCallUser(ctx context.Context, id uint) error

	// ========== 统计查询 ==========
	GetAlertStats(ctx context.Context, startTime, endTime *time.Time) (*AlertStats, error)

	// ========== 清理 ==========
	CleanupOldAlerts(ctx context.Context, olderThan time.Time) (int64, error)
	CleanupOldNotifyHistory(ctx context.Context, olderThan time.Time) (int64, error)

	// ========== 关闭 ==========
	Close() error
}

// ========== 过滤器类型 ==========

// RuleFilter 规则查询过滤器
type RuleFilter struct {
	Enabled  *bool
	Severity *AlertSeverity
	Page     int
	PageSize int
}

// AlertFilter 告警查询过滤器
type AlertFilter struct {
	Status      *AlertStatus
	Severity    *AlertSeverity
	GroupKey    string
	RuleID      *uint
	RuleName    string
	StartTime   *time.Time
	EndTime     *time.Time
	Page        int
	PageSize    int
}

// SilenceFilter 抑制规则查询过滤器
type SilenceFilter struct {
	RuleID   *uint
	Active   *bool // 是否在有效期内
	Page     int
	PageSize int
}

// NotifyHistoryFilter 通知历史查询过滤器
type NotifyHistoryFilter struct {
	AlertID   *uint
	ChannelID *uint
	Status    string
	StartTime *time.Time
	EndTime   *time.Time
	Page      int
	PageSize  int
}

// AlertStats 告警统计信息
type AlertStats struct {
	TotalAlerts    int64 `json:"total_alerts"`
	FiringAlerts   int64 `json:"firing_alerts"`
	ResolvedAlerts int64 `json:"resolved_alerts"`
	SilencedAlerts int64 `json:"silenced_alerts"`

	CriticalCount int64 `json:"critical_count"`
	WarningCount  int64 `json:"warning_count"`
	InfoCount     int64 `json:"info_count"`

	TotalNotifications int64 `json:"total_notifications"`
	SuccessNotifications int64 `json:"success_notifications"`
	FailedNotifications int64 `json:"failed_notifications"`
}

// ========== Store 实现 ==========

// storeImpl Store 的 GORM 实现
type storeImpl struct {
	db *gorm.DB
}

// NewStore 创建新的告警存储实例
func NewStore(db *gorm.DB) (Store, error) {
	if db == nil {
		return nil, fmt.Errorf("数据库连接不能为空")
	}

	store := &storeImpl{
		db: db,
	}

	return store, nil
}

// AutoMigrate 执行数据库迁移
func (s *storeImpl) AutoMigrate(ctx context.Context) error {
	models := []struct {
		name  string
		model interface{}
	}{
		{"alert_rules", &AlertRule{}},
		{"alert_instances", &AlertInstance{}},
		{"alert_silence_rules", &SilenceRule{}},
		{"alert_notify_channels", &NotifyChannel{}},
		{"alert_notify_history", &NotifyHistory{}},
		{"alert_oncall_schedules", &OnCallSchedule{}},
		{"alert_oncall_users", &OnCallUser{}},
	}

	var errors []string
	for _, m := range models {
		if err := s.db.WithContext(ctx).AutoMigrate(m.model); err != nil {
			// 如果表已存在，记录警告但继续
			if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "Duplicate") {
				// 表已存在，跳过
				continue
			}
			errors = append(errors, fmt.Sprintf("%s: %v", m.name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("migration errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// ========== 规则管理 ==========

// CreateRule 创建告警规则
func (s *storeImpl) CreateRule(ctx context.Context, rule *AlertRule) error {
	return s.db.WithContext(ctx).Create(rule).Error
}

// GetRule 获取告警规则
func (s *storeImpl) GetRule(ctx context.Context, id uint) (*AlertRule, error) {
	var rule AlertRule
	err := s.db.WithContext(ctx).First(&rule, id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// GetRuleByName 根据名称获取告警规则
func (s *storeImpl) GetRuleByName(ctx context.Context, name string) (*AlertRule, error) {
	var rule AlertRule
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// ListRules 列出告警规则
func (s *storeImpl) ListRules(ctx context.Context, filter *RuleFilter) ([]*AlertRule, int64, error) {
	query := s.db.WithContext(ctx).Model(&AlertRule{})

	if filter != nil {
		if filter.Enabled != nil {
			query = query.Where("enabled = ?", *filter.Enabled)
		}
		if filter.Severity != nil {
			query = query.Where("severity = ?", *filter.Severity)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rules []*AlertRule
	page, pageSize := 1, 20
	if filter != nil {
		page = filter.Page
		if page < 1 {
			page = 1
		}
		pageSize = filter.PageSize
		if pageSize <= 0 {
			pageSize = 20
		}
	}

	offset := (page - 1) * pageSize
	err := query.Order("priority DESC, created_at DESC").Offset(offset).Limit(pageSize).Find(&rules).Error
	return rules, total, err
}

// UpdateRule 更新告警规则
func (s *storeImpl) UpdateRule(ctx context.Context, rule *AlertRule) error {
	return s.db.WithContext(ctx).Save(rule).Error
}

// DeleteRule 删除告警规则（软删除）
func (s *storeImpl) DeleteRule(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&AlertRule{}, id).Error
}

// ========== 告警实例 ==========

// CreateAlert 创建告警实例
func (s *storeImpl) CreateAlert(ctx context.Context, alert *AlertInstance) error {
	return s.db.WithContext(ctx).Create(alert).Error
}

// GetAlert 获取告警实例
func (s *storeImpl) GetAlert(ctx context.Context, id uint) (*AlertInstance, error) {
	var alert AlertInstance
	err := s.db.WithContext(ctx).First(&alert, id).Error
	if err != nil {
		return nil, err
	}
	return &alert, nil
}

// GetAlertByFingerprint 根据 fingerprint 获取告警实例
func (s *storeImpl) GetAlertByFingerprint(ctx context.Context, fingerprint string) (*AlertInstance, error) {
	var alert AlertInstance
	err := s.db.WithContext(ctx).Where("fingerprint = ?", fingerprint).First(&alert).Error
	if err != nil {
		return nil, err
	}
	return &alert, nil
}

// ListAlerts 列出告警实例
func (s *storeImpl) ListAlerts(ctx context.Context, filter *AlertFilter) ([]*AlertInstance, int64, error) {
	query := s.db.WithContext(ctx).Model(&AlertInstance{})

	if filter != nil {
		if filter.Status != nil {
			query = query.Where("status = ?", *filter.Status)
		}
		if filter.Severity != nil {
			query = query.Where("severity = ?", *filter.Severity)
		}
		if filter.GroupKey != "" {
			query = query.Where("group_key = ?", filter.GroupKey)
		}
		if filter.RuleID != nil {
			query = query.Where("rule_id = ?", *filter.RuleID)
		}
		if filter.RuleName != "" {
			query = query.Where("rule_name LIKE ?", "%"+filter.RuleName+"%")
		}
		if filter.StartTime != nil {
			query = query.Where("fired_at >= ?", *filter.StartTime)
		}
		if filter.EndTime != nil {
			query = query.Where("fired_at <= ?", *filter.EndTime)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var alerts []*AlertInstance
	page, pageSize := 1, 20
	if filter != nil {
		page = filter.Page
		if page < 1 {
			page = 1
		}
		pageSize = filter.PageSize
		if pageSize <= 0 {
			pageSize = 20
		}
	}

	offset := (page - 1) * pageSize
	err := query.Order("fired_at DESC").Offset(offset).Limit(pageSize).Find(&alerts).Error
	return alerts, total, err
}

// UpdateAlert 更新告警实例
func (s *storeImpl) UpdateAlert(ctx context.Context, alert *AlertInstance) error {
	return s.db.WithContext(ctx).Save(alert).Error
}

// ResolveAlert 解决告警
func (s *storeImpl) ResolveAlert(ctx context.Context, id uint) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&AlertInstance{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      AlertStatusResolved,
			"resolved_at": &now,
		}).Error
}

// BatchResolveAlerts 批量解决告警
func (s *storeImpl) BatchResolveAlerts(ctx context.Context, fingerprints []string) error {
	if len(fingerprints) == 0 {
		return nil
	}
	now := time.Now()
	return s.db.WithContext(ctx).Model(&AlertInstance{}).
		Where("fingerprint IN ?", fingerprints).
		Where("status = ?", AlertStatusFiring).
		Updates(map[string]interface{}{
			"status":      AlertStatusResolved,
			"resolved_at": &now,
		}).Error
}

// DeleteAlert 删除告警实例（软删除）
func (s *storeImpl) DeleteAlert(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&AlertInstance{}, id).Error
}

// ========== 抑制规则 ==========

// CreateSilence 创建抑制规则
func (s *storeImpl) CreateSilence(ctx context.Context, silence *SilenceRule) error {
	return s.db.WithContext(ctx).Create(silence).Error
}

// GetSilence 获取抑制规则
func (s *storeImpl) GetSilence(ctx context.Context, id uint) (*SilenceRule, error) {
	var silence SilenceRule
	err := s.db.WithContext(ctx).First(&silence, id).Error
	if err != nil {
		return nil, err
	}
	return &silence, nil
}

// ListSilences 列出抑制规则
func (s *storeImpl) ListSilences(ctx context.Context, filter *SilenceFilter) ([]*SilenceRule, error) {
	query := s.db.WithContext(ctx).Model(&SilenceRule{})

	if filter != nil {
		if filter.RuleID != nil {
			query = query.Where("rule_id = ?", *filter.RuleID)
		}
		if filter.Active != nil {
			now := time.Now()
			if *filter.Active {
				query = query.Where("enabled = ? AND start_at <= ? AND end_at >= ?", true, now, now)
			} else {
				query = query.Where("enabled = ? OR end_at < ?", false, now)
			}
		}
	}

	var silences []*SilenceRule
	err := query.Order("created_at DESC").Find(&silences).Error
	return silences, err
}

// DeleteSilence 删除抑制规则（软删除）
func (s *storeImpl) DeleteSilence(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&SilenceRule{}, id).Error
}

// ========== 通知渠道 ==========

// CreateChannel 创建通知渠道
func (s *storeImpl) CreateChannel(ctx context.Context, channel *NotifyChannel) error {
	return s.db.WithContext(ctx).Create(channel).Error
}

// GetChannel 获取通知渠道
func (s *storeImpl) GetChannel(ctx context.Context, id uint) (*NotifyChannel, error) {
	var channel NotifyChannel
	err := s.db.WithContext(ctx).First(&channel, id).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

// ListChannels 列出通知渠道
func (s *storeImpl) ListChannels(ctx context.Context, enabled *bool) ([]*NotifyChannel, error) {
	query := s.db.WithContext(ctx).Model(&NotifyChannel{})
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}

	var channels []*NotifyChannel
	err := query.Order("created_at DESC").Find(&channels).Error
	return channels, err
}

// UpdateChannel 更新通知渠道
func (s *storeImpl) UpdateChannel(ctx context.Context, channel *NotifyChannel) error {
	return s.db.WithContext(ctx).Save(channel).Error
}

// DeleteChannel 删除通知渠道（软删除）
func (s *storeImpl) DeleteChannel(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&NotifyChannel{}, id).Error
}

// ========== 通知历史 ==========

// CreateNotifyHistory 创建通知历史记录
func (s *storeImpl) CreateNotifyHistory(ctx context.Context, history *NotifyHistory) error {
	return s.db.WithContext(ctx).Create(history).Error
}

// ListNotifyHistory 列出通知历史
func (s *storeImpl) ListNotifyHistory(ctx context.Context, filter *NotifyHistoryFilter) ([]*NotifyHistory, int64, error) {
	query := s.db.WithContext(ctx).Model(&NotifyHistory{})

	if filter != nil {
		if filter.AlertID != nil {
			query = query.Where("alert_id = ?", *filter.AlertID)
		}
		if filter.ChannelID != nil {
			query = query.Where("channel_id = ?", *filter.ChannelID)
		}
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
		if filter.StartTime != nil {
			query = query.Where("sent_at >= ?", *filter.StartTime)
		}
		if filter.EndTime != nil {
			query = query.Where("sent_at <= ?", *filter.EndTime)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var history []*NotifyHistory
	page, pageSize := 1, 20
	if filter != nil {
		page = filter.Page
		if page < 1 {
			page = 1
		}
		pageSize = filter.PageSize
		if pageSize <= 0 {
			pageSize = 20
		}
	}

	offset := (page - 1) * pageSize
	err := query.Order("sent_at DESC").Offset(offset).Limit(pageSize).Find(&history).Error
	return history, total, err
}

// ========== 值班管理 ==========

// CreateOnCallSchedule 创建值班表
func (s *storeImpl) CreateOnCallSchedule(ctx context.Context, schedule *OnCallSchedule) error {
	return s.db.WithContext(ctx).Create(schedule).Error
}

// GetOnCallSchedule 获取值班表
func (s *storeImpl) GetOnCallSchedule(ctx context.Context, id uint) (*OnCallSchedule, error) {
	var schedule OnCallSchedule
	err := s.db.WithContext(ctx).First(&schedule, id).Error
	if err != nil {
		return nil, err
	}
	return &schedule, nil
}

// ListOnCallSchedules 列出值班表
func (s *storeImpl) ListOnCallSchedules(ctx context.Context) ([]*OnCallSchedule, error) {
	var schedules []*OnCallSchedule
	err := s.db.WithContext(ctx).Order("created_at DESC").Find(&schedules).Error
	return schedules, err
}

// UpdateOnCallSchedule 更新值班表
func (s *storeImpl) UpdateOnCallSchedule(ctx context.Context, schedule *OnCallSchedule) error {
	return s.db.WithContext(ctx).Save(schedule).Error
}

// DeleteOnCallSchedule 删除值班表（软删除）
func (s *storeImpl) DeleteOnCallSchedule(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&OnCallSchedule{}, id).Error
}

// CreateOnCallUser 创建值班用户
func (s *storeImpl) CreateOnCallUser(ctx context.Context, user *OnCallUser) error {
	return s.db.WithContext(ctx).Create(user).Error
}

// GetOnCallUser 获取值班用户
func (s *storeImpl) GetOnCallUser(ctx context.Context, id uint) (*OnCallUser, error) {
	var user OnCallUser
	err := s.db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ListOnCallUsers 列出值班用户
func (s *storeImpl) ListOnCallUsers(ctx context.Context) ([]*OnCallUser, error) {
	var users []*OnCallUser
	err := s.db.WithContext(ctx).Order("created_at DESC").Find(&users).Error
	return users, err
}

// UpdateOnCallUser 更新值班用户
func (s *storeImpl) UpdateOnCallUser(ctx context.Context, user *OnCallUser) error {
	return s.db.WithContext(ctx).Save(user).Error
}

// DeleteOnCallUser 删除值班用户（软删除）
func (s *storeImpl) DeleteOnCallUser(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&OnCallUser{}, id).Error
}

// ========== 统计查询 ==========

// GetAlertStats 获取告警统计信息
func (s *storeImpl) GetAlertStats(ctx context.Context, startTime, endTime *time.Time) (*AlertStats, error) {
	stats := &AlertStats{}

	// 构建时间范围查询
	alertQuery := s.db.WithContext(ctx).Model(&AlertInstance{})
	if startTime != nil {
		alertQuery = alertQuery.Where("fired_at >= ?", *startTime)
	}
	if endTime != nil {
		alertQuery = alertQuery.Where("fired_at <= ?", *endTime)
	}

	// 按状态统计
	var statusStats []struct {
		Status AlertStatus
		Count  int64
	}
	if err := alertQuery.Select("status, count(*) as count").Group("status").Scan(&statusStats).Error; err != nil {
		return nil, err
	}

	for _, s := range statusStats {
		stats.TotalAlerts += s.Count
		switch s.Status {
		case AlertStatusFiring:
			stats.FiringAlerts = s.Count
		case AlertStatusResolved:
			stats.ResolvedAlerts = s.Count
		case AlertStatusSilenced:
			stats.SilencedAlerts = s.Count
		}
	}

	// 按严重程度统计
	var severityStats []struct {
		Severity AlertSeverity
		Count    int64
	}
	if err := alertQuery.Select("severity, count(*) as count").Group("severity").Scan(&severityStats).Error; err != nil {
		return nil, err
	}

	for _, s := range severityStats {
		switch s.Severity {
		case AlertSeverityCritical:
			stats.CriticalCount = s.Count
		case AlertSeverityWarning:
			stats.WarningCount = s.Count
		case AlertSeverityInfo:
			stats.InfoCount = s.Count
		}
	}

	// 通知统计（如果表不存在，返回空统计）
	historyQuery := s.db.WithContext(ctx).Model(&NotifyHistory{})
	if startTime != nil {
		historyQuery = historyQuery.Where("sent_at >= ?", *startTime)
	}
	if endTime != nil {
		historyQuery = historyQuery.Where("sent_at <= ?", *endTime)
	}

	var historyStats []struct {
		Status string
		Count  int64
	}
	// 检查表是否存在，如果不存在则跳过通知统计
	if err := historyQuery.Select("status, count(*) as count").Group("status").Scan(&historyStats).Error; err != nil {
		// 如果表不存在，返回空统计而不是错误
		errStr := err.Error()
		if strings.Contains(errStr, "does not exist") || strings.Contains(errStr, "relation") {
			// 表不存在，返回空统计
			historyStats = []struct {
				Status string
				Count  int64
			}{}
		} else {
			return nil, err
		}
	}

	for _, s := range historyStats {
		stats.TotalNotifications += s.Count
		if s.Status == "success" {
			stats.SuccessNotifications = s.Count
		} else if s.Status == "failed" {
			stats.FailedNotifications = s.Count
		}
	}

	return stats, nil
}

// ========== 清理 ==========

// CleanupOldAlerts 清理旧告警
func (s *storeImpl) CleanupOldAlerts(ctx context.Context, olderThan time.Time) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("resolved_at < ? OR (resolved_at IS NULL AND fired_at < ?)", olderThan, olderThan).
		Delete(&AlertInstance{})
	return result.RowsAffected, result.Error
}

// CleanupOldNotifyHistory 清理旧通知历史
func (s *storeImpl) CleanupOldNotifyHistory(ctx context.Context, olderThan time.Time) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("sent_at < ?", olderThan).
		Delete(&NotifyHistory{})
	return result.RowsAffected, result.Error
}

// ========== 关闭 ==========

// Close 关闭存储（GORM 连接由外部管理，这里为空操作）
func (s *storeImpl) Close() error {
	return nil
}
