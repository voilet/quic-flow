package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/voilet/quic-flow/pkg/monitoring"
)

// NotifierConfig 通知器配置
type NotifierConfig struct {
	QueueSize       int
	MaxRetries      int
	RetryInterval   time.Duration
	RequestTimeout  time.Duration
	Logger          *monitoring.Logger
}

// Notifier 通知发送器
type Notifier struct {
	store Store

	// HTTP 客户端
	httpClient *http.Client

	// 发送队列
	queue chan *NotificationTask

	// 重试队列
	retryQueue chan *NotificationTask

	// 监控
	logger *monitoring.Logger

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NotificationTask 通知任务
type NotificationTask struct {
	Message  *NotificationMessage
	Channel  *NotifyChannel
	Retries  int
	LastTime time.Time
}

// NotificationMessage 通知消息
type NotificationMessage struct {
	AlertID     uint
	RuleName    string
	Status      AlertStatus
	Labels      JSONMap
	Annotations JSONMap
	Timestamp   time.Time
}

// NewNotifier 创建通知发送器
func NewNotifier(store Store, config *NotifierConfig) *Notifier {
	if config == nil {
		config = &NotifierConfig{}
	}
	if config.QueueSize == 0 {
		config.QueueSize = 1000
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryInterval == 0 {
		config.RetryInterval = 5 * time.Second
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 10 * time.Second
	}
	if config.Logger == nil {
		config.Logger = monitoring.NewDefaultLogger()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Notifier{
		store:      store,
		httpClient: &http.Client{Timeout: config.RequestTimeout},
		queue:      make(chan *NotificationTask, config.QueueSize),
		retryQueue: make(chan *NotificationTask, config.QueueSize),
		logger:     config.Logger,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start 启动通知发送器
func (n *Notifier) Start() error {
	// 启动工作协程
	for i := 0; i < 10; i++ {
		n.wg.Add(1)
		go n.worker()
	}

	// 启动重试协程
	n.wg.Add(1)
	go n.retryWorker()

	n.logger.Info("Notifier started")
	return nil
}

// Stop 停止通知发送器
func (n *Notifier) Stop() {
	n.cancel()
	n.wg.Wait()
	n.logger.Info("Notifier stopped")
}

// SendNotification 发送通知
func (n *Notifier) SendNotification(ctx context.Context, msg *NotificationMessage) error {
	// 获取启用的通知渠道
	enabled := true
	channels, err := n.store.ListChannels(ctx, &enabled)
	if err != nil {
		return fmt.Errorf("failed to get channels: %w", err)
	}

	// 为每个渠道创建发送任务
	for _, channel := range channels {
		task := &NotificationTask{
			Message: msg,
			Channel: channel,
			Retries: 0,
		}

		select {
		case n.queue <- task:
		default:
			n.logger.Error("Notification queue full",
				"channel", channel.Name,
				"alert_id", msg.AlertID)
		}
	}

	return nil
}

// worker 工作协程
func (n *Notifier) worker() {
	defer n.wg.Done()

	for {
		select {
		case <-n.ctx.Done():
			return
		case task := <-n.queue:
			n.send(task)
		}
	}
}

// retryWorker 重试协程
func (n *Notifier) retryWorker() {
	defer n.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			// 处理重试队列
			select {
			case task := <-n.retryQueue:
				n.send(task)
			default:
			}
		}
	}
}

// send 发送通知
func (n *Notifier) send(task *NotificationTask) {
	var err error

	switch task.Channel.Type {
	case ChannelTypeDingTalk:
		err = n.sendDingTalk(task)
	case ChannelTypeFeishu:
		err = n.sendFeishu(task)
	case ChannelTypeWeChat:
		err = n.sendWeChat(task)
	case ChannelTypeSlack:
		err = n.sendSlack(task)
	case ChannelTypeEmail:
		err = n.sendEmail(task)
	default:
		err = fmt.Errorf("unsupported channel type: %s", task.Channel.Type)
	}

	if err != nil {
		n.logger.Error("Failed to send notification",
			"channel", task.Channel.Name,
			"type", task.Channel.Type,
			"error", err,
			"retries", task.Retries)

		// 加入重试队列
		task.Retries++
		task.LastTime = time.Now()

		if task.Retries < 3 {
			select {
			case n.retryQueue <- task:
			default:
			}
		}
	} else {
		n.logger.Info("Notification sent successfully",
			"channel", task.Channel.Name,
			"alert_id", task.Message.AlertID)

		// 记录发送历史
		n.recordHistory(task, true)
	}
}

// sendDingTalk 发送钉钉通知
func (n *Notifier) sendDingTalk(task *NotificationTask) error {
	// 将 JSONMap 转换为 JSON 字符串
	configBytes, err := json.Marshal(task.Channel.Config)
	if err != nil {
		return fmt.Errorf("invalid DingTalk config: %w", err)
	}

	var config DingTalkConfig
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return fmt.Errorf("failed to parse DingTalk config: %w", err)
	}

	// 构建消息
	msg := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"title": fmt.Sprintf("告警通知: %s", task.Message.RuleName),
			"text":  n.buildDingTalkMessage(task),
		},
	}

	// 如果有 @ 人，添加 at
	if len(config.AtMobiles) > 0 || config.AtAll {
		msg["at"] = map[string]interface{}{
			"atMobiles": config.AtMobiles,
			"isAtAll":   config.AtAll,
		}
	}

	return n.postJSON(config.Webhook, msg)
}

// buildDingTalkMessage 构建钉钉消息
func (n *Notifier) buildDingTalkMessage(task *NotificationTask) string {
	var stateText string
	switch task.Message.Status {
	case AlertStatusFiring:
		stateText = "🔴 告警触发"
	case AlertStatusResolved:
		stateText = "🟢 告警恢复"
	case AlertStatusSilenced:
		stateText = "⚫ 告警抑制"
	default:
		stateText = "🟡 告警"
	}

	text := fmt.Sprintf("### %s\n\n", stateText)
	text += fmt.Sprintf("**规则**: %s\n\n", task.Message.RuleName)
	text += fmt.Sprintf("**状态**: %s\n\n", task.Message.Status)
	text += fmt.Sprintf("**时间**: %s\n\n", task.Message.Timestamp.Format("2006-01-02 15:04:05"))

	// 添加标签
	if len(task.Message.Labels) > 0 {
		text += "**标签**:\n"
		for k, v := range task.Message.Labels {
			text += fmt.Sprintf("- %s: %v\n", k, v)
		}
		text += "\n"
	}

	// 添加注释
	if len(task.Message.Annotations) > 0 {
		text += "**详情**:\n"
		for k, v := range task.Message.Annotations {
			text += fmt.Sprintf("- %s: %v\n", k, v)
		}
	}

	return text
}

// sendFeishu 发送飞书通知
func (n *Notifier) sendFeishu(task *NotificationTask) error {
	// TODO: 实现飞书通知
	return fmt.Errorf("Feishu notification not implemented")
}

// sendWeChat 发送企业微信通知
func (n *Notifier) sendWeChat(task *NotificationTask) error {
	// TODO: 实现企业微信通知
	return fmt.Errorf("WeChat notification not implemented")
}

// sendSlack 发送 Slack 通知
func (n *Notifier) sendSlack(task *NotificationTask) error {
	// TODO: 实现 Slack 通知
	return fmt.Errorf("Slack notification not implemented")
}

// sendEmail 发送邮件通知
func (n *Notifier) sendEmail(task *NotificationTask) error {
	// TODO: 实现邮件通知
	return fmt.Errorf("Email notification not implemented")
}

// postJSON 发送 JSON POST 请求
func (n *Notifier) postJSON(url string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(n.ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// recordHistory 记录发送历史
func (n *Notifier) recordHistory(task *NotificationTask, success bool) {
	ctx, cancel := context.WithTimeout(n.ctx, 5*time.Second)
	defer cancel()

	status := "success"
	if !success {
		status = "failed"
	}

	history := &NotifyHistory{
		AlertID:    task.Message.AlertID,
		ChannelID:  task.Channel.ID,
		Status:     status,
		SentAt:     time.Now(),
		RetryCount: task.Retries,
	}

	_ = n.store.CreateNotifyHistory(ctx, history)
}

// DingTalkConfig 钉钉配置
type DingTalkConfig struct {
	Webhook   string   `json:"webhook"`
	Secret    string   `json:"secret"`
	AtMobiles []string `json:"at_mobiles"`
	AtAll     bool     `json:"at_all"`
}
