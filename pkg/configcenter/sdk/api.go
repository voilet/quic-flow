// Package sdk 提供配置中心 Go SDK
//
// 基本使用:
//
//	// 创建客户端
//	client := sdk.NewClient(&sdk.Config{
//		ServerAddr:  "localhost:8443",
//		ClientID:    "my-app",
//		Namespace:   "production",
//		InsecureTLS: true, // 仅开发环境
//	})
//
//	// 监听配置变更
//	client.Watch("app", "database.yaml", func(config *sdk.Config) error {
//	    fmt.Printf("配置已更新: version=%d\n", config.Version)
//	    return nil
//	})
//
//	// 获取当前配置
//	config, err := client.Get("app", "database.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// 解析为结构体
//	var dbConfig DatabaseConfig
//	if err := config.Scan(&dbConfig); err != nil {
//	    log.Fatal(err)
//	}
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Config 客户端配置
type Config struct {
	ServerAddr  string            // 服务器地址 host:port
	ClientID    string            // 客户端唯一标识
	Namespace   string            // 命名空间
	InsecureTLS bool              // 跳过 TLS 证书验证（仅开发环境）
	Tags        map[string]string // 客户端标签（用于灰度）
}

// ConfigItem 配置对象
type ConfigItem struct {
	Namespace string
	Group     string
	DataID    string
	Content   string
	Format    string // json, yaml, properties, text
	Version   int64
	Metadata  map[string]string
}

// Scan 将配置内容解析到目标对象
func (c *ConfigItem) Scan(target interface{}) error {
	switch c.Format {
	case "json":
		return json.Unmarshal([]byte(c.Content), target)
	default:
		return fmt.Errorf("unsupported format: %s", c.Format)
	}
}

// String 返回配置内容
func (c *ConfigItem) String() string {
	return c.Content
}

// Client 配置中心客户端
type Client struct {
	config *Config

	// 连接状态
	connected bool
	conn      interface{} // QUIC 连接（实际类型未导出）

	// 配置缓存
	cache sync.Map // map[string]*ConfigItem

	// 监听器
	watchers sync.Map // map[string]*watcherInfo

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
}

// watcherInfo 监听器信息
type watcherInfo struct {
	group    string
	dataID   string
	callback ConfigChangeCallback
	version  int64
}

// ConfigChangeCallback 配置变更回调函数
// 当配置发生变更时被调用
// 如果返回错误，配置不会更新到本地缓存
type ConfigChangeCallback func(*ConfigItem) error

// NewClient 创建新的配置中心客户端
func NewClient(config *Config) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Connect 连接到配置中心服务器
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	// TODO: 实际的 QUIC 连接逻辑
	// 这里只是占位符，实际需要集成 QUIC 客户端
	c.connected = true

	return nil
}

// Disconnect 断开连接
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.cancel()
	c.connected = false

	return nil
}

// Get 获取配置
// 从本地缓存获取，如果缓存不存在则从服务器拉取
func (c *Client) Get(group, dataID string) (*ConfigItem, error) {
	return c.GetWithContext(context.Background(), group, dataID)
}

// GetWithContext 使用上下文获取配置
func (c *Client) GetWithContext(ctx context.Context, group, dataID string) (*ConfigItem, error) {
	key := c.cacheKey(group, dataID)

	// 先检查缓存
	if cached, ok := c.cache.Load(key); ok {
		return cached.(*ConfigItem), nil
	}

	// 从服务器拉取
	return c.fetchConfig(ctx, group, dataID)
}

// Watch 监听配置变更
// 当配置发生变更时，callback 会被调用
func (c *Client) Watch(group, dataID string, callback ConfigChangeCallback) error {
	key := c.cacheKey(group, dataID)

	// 保存监听器
	c.watchers.Store(key, &watcherInfo{
		group:    group,
		dataID:   dataID,
		callback: callback,
		version:  0,
	})

	// 获取初始配置
	config, err := c.Get(group, dataID)
	if err == nil {
		// 触发初始回调
		_ = callback(config)
	}

	return nil
}

// Unwatch 取消监听配置
func (c *Client) Unwatch(group, dataID string) {
	key := c.cacheKey(group, dataID)
	c.watchers.Delete(key)
}

// Subscribe 订阅配置（以便接收推送更新）
func (c *Client) Subscribe(group, dataID string) error {
	// TODO: 发送订阅请求到服务器
	return nil
}

// Unsubscribe 取消订阅
func (c *Client) Unsubscribe(group, dataID string) error {
	// TODO: 发送取消订阅请求到服务器
	return nil
}

// fetchConfig 从服务器获取配置
func (c *Client) fetchConfig(ctx context.Context, group, dataID string) (*ConfigItem, error) {
	// TODO: 实际的网络请求逻辑
	// 这里只是占位符

	config := &ConfigItem{
		Namespace: c.config.Namespace,
		Group:     group,
		DataID:    dataID,
		Content:   "",
		Format:    "json",
		Version:   1,
		Metadata:  make(map[string]string),
	}

	// 更新缓存
	key := c.cacheKey(group, dataID)
	c.cache.Store(key, config)

	return config, nil
}

// handlePush 处理服务器推送的配置更新
func (c *Client) handlePush(config *ConfigItem) error {
	key := c.cacheKey(config.Group, config.DataID)

	// 检查是否有监听器
	if watcher, ok := c.watchers.Load(key); ok {
		info := watcher.(*watcherInfo)

		// 检查版本是否更新
		if config.Version <= info.version {
			return nil // 版本未更新，忽略
		}

		// 调用回调函数
		if err := info.callback(config); err != nil {
			return fmt.Errorf("callback error: %w", err)
		}

		// 更新版本号
		info.version = config.Version
	}

	// 更新缓存
	c.cache.Store(key, config)

	return nil
}

// cacheKey 生成缓存键
func (c *Client) cacheKey(group, dataID string) string {
	return fmt.Sprintf("%s:%s:%s", c.config.Namespace, group, dataID)
}

// ==================== 便捷方法 ====================

// GetString 获取配置内容（字符串格式）
func (c *Client) GetString(group, dataID string) (string, error) {
	config, err := c.Get(group, dataID)
	if err != nil {
		return "", err
	}
	return config.String(), nil
}

// GetJSON 获取 JSON 配置并解析到目标对象
func (c *Client) GetJSON(group, dataID string, target interface{}) error {
	config, err := c.Get(group, dataID)
	if err != nil {
		return err
	}
	return config.Scan(target)
}

// WatchString 监听配置变更（字符串格式）
func (c *Client) WatchString(group, dataID string, callback func(content string) error) error {
	return c.Watch(group, dataID, func(config *ConfigItem) error {
		return callback(config.String())
	})
}

// WatchJSON 监听 JSON 配置变更并解析
func (c *Client) WatchJSON(group, dataID string, target interface{}, callback func() error) error {
	return c.Watch(group, dataID, func(config *ConfigItem) error {
		if err := config.Scan(target); err != nil {
			return err
		}
		return callback()
	})
}

// ==================== 心跳和重连 ====================

// startHeartbeat 启动心跳
func (c *Client) startHeartbeat() {
	ticker := time.NewTicker(15 * time.Second)
	go func() {
		for {
			select {
			case <-c.ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				// TODO: 发送心跳
			}
		}
	}()
}

// startReconnect 启动自动重连
func (c *Client) startReconnect() {
	go func() {
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(5 * time.Second):
				if !c.connected {
					_ = c.Connect()
				}
			}
		}
	}()
}
