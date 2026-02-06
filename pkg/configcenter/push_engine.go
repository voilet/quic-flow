package configcenter

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// PushEngine 配置推送引擎
type PushEngine struct {
	store Store

	// 订阅者管理
	subscriptions sync.Map // map[string]*SubscriptionInfo // key: "namespace:group:data_id"

	// 会话管理器接口（用于发送 QUIC 消息）
	sessionManager SessionManager

	// 监控
	logger *monitoring.Logger

	// 配置
	config *PushEngineConfig

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// PushEngineConfig 推送引擎配置
type PushEngineConfig struct {
	PushTimeout       time.Duration     // 推送超时时间
	RetryCount        int               // 重试次数
	RetryInterval     time.Duration     // 重试间隔
	BatchPushSize     int               // 批量推送大小
	BatchPushInterval time.Duration     // 批量推送间隔
	Logger            *monitoring.Logger
}

// SubscriptionInfo 订阅信息
type SubscriptionInfo struct {
	Namespace string
	Group     string
	DataID    string
	// 订阅者列表
	Subscribers map[string]*SubscriberInfo // clientID -> SubscriberInfo
	mu          sync.RWMutex
}

// SubscriberInfo 订阅者信息
type SubscriberInfo struct {
	ClientID      string
	SubscribeTime time.Time
	LastPushTime  time.Time
	LastVersion   int64
	// 客户端标签（用于灰度匹配）
	Tags map[string]string
	// 客户端 IP
	IPAddress string
}

// SessionManager 会话管理器接口（抽象，避免直接依赖 session 包）
type SessionManager interface {
	GetSession(clientID string) (Session, bool)
	GetAllSessions() []Session
}

// Session 会话接口
type Session interface {
	GetClientID() string
	GetRemoteAddr() string
	GetMetadata(key string) (interface{}, bool)
	IsConnected() bool
	SendMessage(ctx context.Context, msg *protocol.DataMessage) error
}

// NewPushEngine 创建推送引擎
func NewPushEngine(store Store, sessionManager SessionManager, config *PushEngineConfig) *PushEngine {
	if config == nil {
		config = &PushEngineConfig{}
	}

	// 设置默认值
	if config.PushTimeout == 0 {
		config.PushTimeout = 30 * time.Second
	}
	if config.RetryCount == 0 {
		config.RetryCount = 3
	}
	if config.RetryInterval == 0 {
		config.RetryInterval = time.Second
	}
	if config.BatchPushSize == 0 {
		config.BatchPushSize = 100
	}
	if config.BatchPushInterval == 0 {
		config.BatchPushInterval = 100 * time.Millisecond
	}
	if config.Logger == nil {
		config.Logger = monitoring.NewDefaultLogger()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &PushEngine{
		store:          store,
		sessionManager: sessionManager,
		subscriptions:  sync.Map{},
		logger:         config.Logger,
		config:         config,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start 启动推送引擎
func (e *PushEngine) Start() {
	e.logger.Info("Config push engine started")
}

// Stop 停止推送引擎
func (e *PushEngine) Stop() {
	e.cancel()
	e.wg.Wait()

	e.logger.Info("Config push engine stopped")
}

// Subscribe 订阅配置
func (e *PushEngine) Subscribe(clientID, namespace, group, dataID string, tags map[string]string) error {
	key := e.subscriptionKey(namespace, group, dataID)

	// 获取或创建订阅信息
	value, _ := e.subscriptions.LoadOrStore(key, &SubscriptionInfo{
		Namespace:   namespace,
		Group:       group,
		DataID:      dataID,
		Subscribers: make(map[string]*SubscriberInfo),
	})
	subInfo := value.(*SubscriptionInfo)

	// 获取客户端信息
	session, ok := e.sessionManager.GetSession(clientID)
	if !ok {
		return fmt.Errorf("session not found: %s", clientID)
	}

	// 添加订阅者
	subInfo.mu.Lock()
	defer subInfo.mu.Unlock()

	subInfo.Subscribers[clientID] = &SubscriberInfo{
		ClientID:      clientID,
		SubscribeTime: time.Now(),
		LastPushTime:  time.Time{},
		LastVersion:   0,
		Tags:          tags,
		IPAddress:     session.GetRemoteAddr(),
	}

	// 保存到数据库
	ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
	defer cancel()

	// 获取现有订阅者或创建新的
	existingSub, err := e.store.GetSubscriber(ctx, clientID)
	if err == nil && existingSub != nil {
		// 更新现有订阅者的订阅列表
		subscriptionKey := fmt.Sprintf("%s:%s", group, dataID)
		found := false
		for _, sub := range existingSub.Subscriptions {
			if sub == subscriptionKey {
				found = true
				break
			}
		}
		if !found {
			existingSub.Subscriptions = append(existingSub.Subscriptions, subscriptionKey)
		}
		existingSub.Namespace = namespace
		existingSub.LastActive = time.Now()
		existingSub.Status = SubscriberStatusOnline
		existingSub.ClientTags = StringMapToStringArray(tags)
		_ = e.store.UpdateSubscriber(ctx, existingSub)
	} else {
		// 创建新订阅者
		newSub := &ConfigSubscriber{
			ClientID:     clientID,
			Namespace:    namespace,
			Subscriptions: StringArray{fmt.Sprintf("%s:%s", group, dataID)},
			ClientIP:     session.GetRemoteAddr(),
			ClientTags:   StringMapToStringArray(tags),
			LastActive:   time.Now(),
			Status:       SubscriberStatusOnline,
		}
		_ = e.store.RegisterSubscriber(ctx, newSub)
	}

	e.logger.Info("Client subscribed to config",
		"client_id", clientID,
		"namespace", namespace,
		"group", group,
		"data_id", dataID)

	return nil
}

// Unsubscribe 取消订阅
func (e *PushEngine) Unsubscribe(clientID, namespace, group, dataID string) error {
	key := e.subscriptionKey(namespace, group, dataID)

	value, ok := e.subscriptions.Load(key)
	if !ok {
		return nil // 没有订阅记录，直接返回
	}

	subInfo := value.(*SubscriptionInfo)

	subInfo.mu.Lock()
	delete(subInfo.Subscribers, clientID)
	subInfo.mu.Unlock()

	// 从数据库更新订阅列表
	ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
	defer cancel()

	existingSub, err := e.store.GetSubscriber(ctx, clientID)
	if err == nil && existingSub != nil {
		subscriptionKey := fmt.Sprintf("%s:%s", group, dataID)
		newSubscriptions := make(StringArray, 0, len(existingSub.Subscriptions))
		for _, sub := range existingSub.Subscriptions {
			if sub != subscriptionKey {
				newSubscriptions = append(newSubscriptions, sub)
			}
		}

		if len(newSubscriptions) == 0 {
			// 没有其他订阅，删除记录
			_ = e.store.UnregisterSubscriber(ctx, clientID)
		} else {
			// 更新订阅列表
			existingSub.Subscriptions = newSubscriptions
			_ = e.store.UpdateSubscriber(ctx, existingSub)
		}
	}

	e.logger.Info("Client unsubscribed from config",
		"client_id", clientID,
		"namespace", namespace,
		"group", group,
		"data_id", dataID)

	return nil
}

// PushConfig 推送配置到订阅者
func (e *PushEngine) PushConfig(ctx context.Context, config *Config, release *ConfigRelease) error {
	key := e.subscriptionKey(config.Namespace, config.Group, config.DataID)

	value, ok := e.subscriptions.Load(key)
	if !ok {
		e.logger.Debug("No subscribers for config",
			"namespace", config.Namespace,
			"group", config.Group,
			"data_id", config.DataID)
		return nil
	}

	subInfo := value.(*SubscriptionInfo)

	// 构建推送消息
	pushMsg := &protocol.ConfigPushMessage{
		Namespace:  config.Namespace,
		Group:      config.Group,
		DataId:     config.DataID,
		Content:    config.Content,
		Format:     string(config.Format),
		Version:    int64(config.Version),
		ReleaseId:  fmt.Sprintf("%d", release.ID),
		ReleasedAt: release.ReleasedAt.UnixMilli(),
		IsGray:     release.ReleaseType == ReleaseTypeGray,
	}

	payload, err := proto.Marshal(pushMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal config push message: %w", err)
	}

	// 获取灰度规则
	grayRules, _ := e.store.GetEnabledGrayRules(ctx, config.ID)

	// 发送给所有订阅者
	subInfo.mu.RLock()
	defer subInfo.mu.RUnlock()

	var successCount, failCount int

	for clientID, subscriber := range subInfo.Subscribers {
		// 检查灰度规则
		if release.ReleaseType == ReleaseTypeGray && len(grayRules) > 0 {
			if !e.matchGrayRule(subscriber, grayRules) {
				e.logger.Debug("Subscriber not matched for gray release",
					"client_id", clientID,
					"release_id", release.ID)
				continue
			}
		}

		// 发送消息
		if err := e.sendToSubscriber(ctx, clientID, payload, release.ID); err != nil {
			e.logger.Error("Failed to push config to subscriber",
				"client_id", clientID,
				"error", err)
			failCount++
		} else {
			successCount++
			// 更新订阅者状态
			subscriber.LastPushTime = time.Now()
			subscriber.LastVersion = int64(config.Version)
		}
	}

	e.logger.Info("Config push completed",
		"namespace", config.Namespace,
		"group", config.Group,
		"data_id", config.DataID,
		"version", config.Version,
		"success", successCount,
		"failed", failCount)

	return nil
}

// sendToSubscriber 发送消息到单个订阅者
func (e *PushEngine) sendToSubscriber(ctx context.Context, clientID string, payload []byte, releaseID uint) error {
	// 获取会话
	session, ok := e.sessionManager.GetSession(clientID)
	if !ok || !session.IsConnected() {
		return fmt.Errorf("session not found or not connected: %s", clientID)
	}

	// 构建数据消息
	msg := &protocol.DataMessage{
		MsgId:      generateMessageID(),
		SenderId:   "server",
		ReceiverId: clientID,
		Type:       protocol.MessageType_MESSAGE_TYPE_CONFIG_PUSH,
		Payload:    payload,
		WaitAck:    true,
		Timestamp:  time.Now().UnixMilli(),
	}

	// 发送消息
	ctx, cancel := context.WithTimeout(ctx, e.config.PushTimeout)
	defer cancel()

	// 记录推送消息
	recordCtx, recordCancel := context.WithTimeout(e.ctx, 2*time.Second)
	defer recordCancel()

	pushRecord := &ConfigPushMessage{
		MsgID:     msg.MsgId,
		ReleaseID: releaseID,
		ClientID:  clientID,
		Status:    "sent",
	}
	_ = e.store.CreatePushMessage(recordCtx, pushRecord)

	return session.SendMessage(ctx, msg)
}

// matchGrayRule 检查订阅者是否匹配灰度规则
func (e *PushEngine) matchGrayRule(subscriber *SubscriberInfo, grayRules []*GrayRule) bool {
	if len(grayRules) == 0 {
		return true // 没有灰度规则，默认匹配
	}

	for _, rule := range grayRules {
		if !rule.Enabled {
			continue
		}

		// 解析规则值
		var ruleValue interface{}
		if err := parseRuleValue(rule.RuleValue, &ruleValue); err != nil {
			continue
		}

		switch rule.RuleType {
		case RuleTypeIP:
			// 检查 IP 白名单
			if ips, ok := ruleValue.([]string); ok {
				for _, ip := range ips {
					if ip == subscriber.IPAddress {
						return true
					}
				}
			}

		case RuleTypeTag:
			// 检查客户端标签匹配
			if tags, ok := ruleValue.(map[string]string); ok {
				matched := true
				for key, value := range tags {
					if subscriber.Tags[key] != value {
						matched = false
						break
					}
				}
				if matched && len(tags) > 0 {
					return true
				}
			}

		case RuleTypeClientID:
			// 检查客户端 ID
			if clientIDs, ok := ruleValue.([]string); ok {
				for _, cid := range clientIDs {
					if cid == subscriber.ClientID {
						return true
					}
				}
			}

		case RuleTypePercentage:
			// 检查百分比
			if percent, ok := ruleValue.(float64); ok {
				if percent > 0 && percent < 100 {
					hash := simpleHash(subscriber.ClientID)
					if hash%100 < int(percent) {
						return true
					}
				}
			}
		}
	}

	return false
}

// HandleConfigAck 处理配置确认
func (e *PushEngine) HandleConfigAck(clientID string, ack *protocol.ConfigAckMessage) error {
	// 更新推送消息状态
	ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
	defer cancel()

	// 查找对应的推送消息并更新
	pushMessages, err := e.store.ListPushMessagesByRelease(ctx, ack.ReleaseId)
	if err != nil {
		return err
	}

	for _, pm := range pushMessages {
		if pm.ClientID == clientID {
			status := "delivered"
			errorMsg := ""
			if !ack.Success {
				status = "failed"
				errorMsg = ack.ErrorMsg
			}
			_ = e.store.UpdatePushMessageStatus(ctx, pm.MsgID, status)

			// 更新错误信息
			if errorMsg != "" {
				now := time.Now()
				_ = e.store.UpdatePushMessageError(ctx, pm.MsgID, errorMsg, &now)
			}
			break
		}
	}

	e.logger.Debug("Config ack processed",
		"client_id", clientID,
		"namespace", ack.Namespace,
		"group", ack.Group,
		"data_id", ack.DataId,
		"version", ack.Version,
		"success", ack.Success)

	return nil
}

// LoadSubscribers 从数据库加载订阅者信息
func (e *PushEngine) LoadSubscribers() error {
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	defer cancel()

	// 获取所有在线订阅者
	filter := &SubscriberFilter{
		Status: SubscriberStatusOnline,
	}
	subscribers, _, err := e.store.ListSubscribers(ctx, filter)
	if err != nil {
		return err
	}

	for _, sub := range subscribers {
		// 解析订阅列表
		for _, subscription := range sub.Subscriptions {
			// 解析 "group:dataId" 格式
			group, dataID := parseSubscriptionKey(subscription)

			key := e.subscriptionKey(sub.Namespace, group, dataID)

			value, _ := e.subscriptions.LoadOrStore(key, &SubscriptionInfo{
				Namespace:   sub.Namespace,
				Group:       group,
				DataID:      dataID,
				Subscribers: make(map[string]*SubscriberInfo),
			})
			subInfo := value.(*SubscriptionInfo)

			subInfo.mu.Lock()
			subInfo.Subscribers[sub.ClientID] = &SubscriberInfo{
				ClientID:      sub.ClientID,
				SubscribeTime: sub.CreatedAt,
				LastPushTime:  sub.LastActive,
				LastVersion:   0,
				Tags:          StringArrayToStringMap(sub.ClientTags),
				IPAddress:     sub.ClientIP,
			}
			subInfo.mu.Unlock()
		}
	}

	e.logger.Info("Loaded subscribers from database", "count", len(subscribers))

	return nil
}

// subscriptionKey 生成订阅键
func (e *PushEngine) subscriptionKey(namespace, group, dataID string) string {
	return fmt.Sprintf("%s:%s:%s", namespace, group, dataID)
}

// parseSubscriptionKey 解析订阅键
func parseSubscriptionKey(key string) (group, dataID string) {
	// 解析 "group:dataId" 格式
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}

// generateMessageID 生成消息 ID
func generateMessageID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(8))
}

// simpleHash 简单哈希函数
func simpleHash(s string) int {
	hash := 0
	for _, c := range s {
		hash = (hash << 5) - hash + int(c)
		hash = hash & hash // 保持为 32 位整数
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond) // 确保随机性
	}
	return string(b)
}

// StringMapToStringArray 将 map[string]string 转换为 StringArray
func StringMapToStringArray(m map[string]string) StringArray {
	if m == nil {
		return nil
	}
	result := make(StringArray, 0, len(m)*2)
	for k, v := range m {
		result = append(result, k+":"+v)
	}
	return result
}

// StringArrayToStringMap 将 StringArray 转换为 map[string]string
func StringArrayToStringMap(arr StringArray) map[string]string {
	if arr == nil {
		return nil
	}
	result := make(map[string]string, len(arr))
	for _, item := range arr {
		for i := 0; i < len(item); i++ {
			if item[i] == ':' {
				result[item[:i]] = item[i+1:]
				break
			}
		}
	}
	return result
}

// parseRuleValue 解析灰度规则值
// 规则值是 JSON 字符串，根据规则类型解析为不同的结构：
// - tag: map[string]string
// - ip: []string
// - client_id: []string
// - percentage: float64
func parseRuleValue(ruleValue string, target interface{}) error {
	return json.Unmarshal([]byte(ruleValue), target)
}
