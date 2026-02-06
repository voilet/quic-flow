package alert

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/voilet/quic-flow/pkg/monitoring"
)

// Processor 告警处理器
// 负责处理告警的生命周期：分组、去重、静默、路由
type Processor struct {
	store Store

	// 告警实例缓存
	alerts sync.Map // map[fingerprint]*AlertInstance

	// 静默规则缓存
	silences sync.Map // map[uint]*SilenceRule

	// 通知通道管理
	notifier *Notifier

	// 监控
	logger *monitoring.Logger

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	EvalInterval    time.Duration
	ResolveTimeout  time.Duration
	Logger          *monitoring.Logger
}

// NewProcessor 创建告警处理器
func NewProcessor(store Store, config *ProcessorConfig) *Processor {
	if config == nil {
		config = &ProcessorConfig{}
	}
	if config.EvalInterval == 0 {
		config.EvalInterval = 15 * time.Second
	}
	if config.ResolveTimeout == 0 {
		config.ResolveTimeout = 5 * time.Minute
	}
	if config.Logger == nil {
		config.Logger = monitoring.NewDefaultLogger()
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &Processor{
		store:  store,
		logger: config.Logger,
		ctx:    ctx,
		cancel: cancel,
		notifier: NewNotifier(store, &NotifierConfig{
			Logger: config.Logger,
		}),
	}

	return p
}

// Start 启动处理器
func (p *Processor) Start() error {
	// 加载静默规则
	if err := p.loadSilences(); err != nil {
		p.logger.Error("Failed to load silences", "error", err)
	}

	// 启动通知器
	if err := p.notifier.Start(); err != nil {
		return fmt.Errorf("failed to start notifier: %w", err)
	}

	// 启动处理循环
	p.wg.Add(1)
	go p.processLoop()

	p.logger.Info("Alert processor started")
	return nil
}

// Stop 停止处理器
func (p *Processor) Stop() {
	p.cancel()
	p.wg.Wait()
	p.notifier.Stop()
	p.logger.Info("Alert processor stopped")
}

// loadSilences 加载静默规则
func (p *Processor) loadSilences() error {
	ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer cancel()

	// 获取活跃的静默规则
	active := true
	silences, err := p.store.ListSilences(ctx, &SilenceFilter{
		Active: &active,
	})
	if err != nil {
		return err
	}

	for _, silence := range silences {
		p.silences.Store(silence.ID, silence)
	}

	p.logger.Info("Loaded silence rules", "count", len(silences))
	return nil
}

// processLoop 处理循环
func (p *Processor) processLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.checkExpiredAlerts()
			p.checkExpiredSilences()
		}
	}
}

// ProcessAlert 处理告警
func (p *Processor) ProcessAlert(alert *AlertInstance) error {
	// 检查静默规则
	if p.isSilenced(alert) {
		p.logger.Debug("Alert silenced",
			"fingerprint", alert.Fingerprint,
			"labels", alert.Labels)
		return nil
	}

	// 生成指纹
	fingerprint := alert.Fingerprint

	// 检查是否已存在
	existing, ok := p.alerts.Load(fingerprint)
	if ok {
		// 更新现有告警
		existingAlert := existing.(*AlertInstance)
		existingAlert.NotifyCount++

		// 如果状态从非 firing 变为 firing
		if existingAlert.Status != AlertStatusFiring && alert.Status == AlertStatusFiring {
			existingAlert.Status = AlertStatusFiring
			existingAlert.FiredAt = time.Now()

			// 发送告警通知
			p.notifyAlert(existingAlert)
		}

		// 保存到数据库
		ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
		defer cancel()
		_ = p.store.UpdateAlert(ctx, existingAlert)
	} else {
		// 新告警
		now := time.Now()
		alert.StartedAt = now
		alert.FiredAt = now

		// 保存到数据库
		ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
		defer cancel()
		if err := p.store.CreateAlert(ctx, alert); err != nil {
			p.logger.Error("Failed to create alert instance",
				"fingerprint", fingerprint,
				"error", err)
			return err
		}

		p.alerts.Store(fingerprint, alert)

		// 如果是 firing 状态，立即发送通知
		if alert.Status == AlertStatusFiring {
			p.notifyAlert(alert)
		}

		p.logger.Info("New alert created",
			"fingerprint", fingerprint,
			"rule", alert.RuleName,
			"status", alert.Status)
	}

	return nil
}

// isSilenced 检查告警是否被静默
func (p *Processor) isSilenced(alert *AlertInstance) bool {
	var matchedSilences []*SilenceRule

	p.silences.Range(func(_, value interface{}) bool {
		silence := value.(*SilenceRule)

		// 检查时间范围
		now := time.Now()
		if now.Before(silence.StartAt) || now.After(silence.EndAt) {
			return true
		}

		// 检查匹配器
		if p.matchesSilence(alert, silence) {
			matchedSilences = append(matchedSilences, silence)
		}

		return true
	})

	return len(matchedSilences) > 0
}

// matchesSilence 检查告警是否匹配静默规则
func (p *Processor) matchesSilence(alert *AlertInstance, silence *SilenceRule) bool {
	// TODO: 实现匹配器逻辑
	// 需要根据 silence.MatchLabels 检查 alert.Labels
	return false
}

// notifyAlert 发送告警通知
func (p *Processor) notifyAlert(alert *AlertInstance) {
	// 构建通知消息
	msg := &NotificationMessage{
		AlertID:     alert.ID,
		RuleName:    alert.RuleName,
		Status:      alert.Status,
		Labels:      alert.Labels,
		Annotations: alert.Annotations,
		Timestamp:   time.Now(),
	}

	// 发送通知
	if err := p.notifier.SendNotification(context.Background(), msg); err != nil {
		p.logger.Error("Failed to send notification",
			"alert_id", alert.ID,
			"error", err)
	}
}

// ResolveAlert 解决告警
func (p *Processor) ResolveAlert(fingerprint string) error {
	value, ok := p.alerts.Load(fingerprint)
	if !ok {
		return fmt.Errorf("alert not found: %s", fingerprint)
	}

	alert := value.(*AlertInstance)
	alert.Status = AlertStatusResolved
	now := time.Now()
	alert.ResolvedAt = &now

	// 保存到数据库
	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()
	if err := p.store.UpdateAlert(ctx, alert); err != nil {
		return err
	}

	// 发送解决通知
	p.notifyAlert(alert)

	// 从内存中移除
	p.alerts.Delete(fingerprint)

	p.logger.Info("Alert resolved",
		"fingerprint", fingerprint,
		"rule", alert.RuleName)

	return nil
}

// checkExpiredAlerts 检查过期的告警
func (p *Processor) checkExpiredAlerts() {
	now := time.Now()

	p.alerts.Range(func(_, value interface{}) bool {
		alert := value.(*AlertInstance)

		// 检查是否长时间未触发
		if alert.Status == AlertStatusFiring {
			if now.Sub(alert.FiredAt) > 5*time.Minute {
				// 超过5分钟未更新，解决告警
				_ = p.ResolveAlert(alert.Fingerprint)
			}
		}

		return true
	})
}

// checkExpiredSilences 检查过期的静默规则
func (p *Processor) checkExpiredSilences() {
	now := time.Now()

	p.silences.Range(func(key, value interface{}) bool {
		silence := value.(*SilenceRule)

		if now.After(silence.EndAt) {
			// 静默规则已过期，移除
			p.silences.Delete(key)
			p.logger.Debug("Silence expired",
				"id", silence.ID,
				"name", silence.Name)
		}

		return true
	})
}

// GetActiveAlerts 获取活跃的告警
func (p *Processor) GetActiveAlerts() []*AlertInstance {
	alerts := make([]*AlertInstance, 0)

	p.alerts.Range(func(_, value interface{}) bool {
		alerts = append(alerts, value.(*AlertInstance))
		return true
	})

	return alerts
}

// AddSilence 添加静默规则
func (p *Processor) AddSilence(silence *SilenceRule) error {
	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()

	if err := p.store.CreateSilence(ctx, silence); err != nil {
		return err
	}

	p.silences.Store(silence.ID, silence)

	p.logger.Info("Silence added",
		"id", silence.ID,
		"name", silence.Name)

	return nil
}

// RemoveSilence 移除静默规则
func (p *Processor) RemoveSilence(id uint) error {
	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()

	if err := p.store.DeleteSilence(ctx, id); err != nil {
		return err
	}

	p.silences.Delete(id)

	p.logger.Info("Silence removed", "id", id)

	return nil
}

// GetAlert 获取告警详情
func (p *Processor) GetAlert(fingerprint string) (*AlertInstance, error) {
	value, ok := p.alerts.Load(fingerprint)
	if !ok {
		return nil, fmt.Errorf("alert not found: %s", fingerprint)
	}

	return value.(*AlertInstance), nil
}
