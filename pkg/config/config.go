package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// ServerConfig 服务器完整配置
type ServerConfig struct {
	// 服务器基础配置
	Server ServerSettings `mapstructure:"server"`

	// TLS 配置
	TLS TLSSettings `mapstructure:"tls"`

	// QUIC 协议配置
	QUIC QUICSettings `mapstructure:"quic"`

	// 会话管理配置
	Session SessionSettings `mapstructure:"session"`

	// 消息处理配置
	Message MessageSettings `mapstructure:"message"`

	// 批量执行配置
	Batch BatchSettings `mapstructure:"batch"`

	// 数据库配置
	Database DatabaseSettings `mapstructure:"database"`

	// 日志配置
	Log LogSettings `mapstructure:"log"`
}

// ServerSettings 服务器基础设置
type ServerSettings struct {
	// 监听地址
	Addr string `mapstructure:"addr"`
	// HTTP API 地址
	APIAddr string `mapstructure:"api_addr"`
	// 高性能模式
	HighPerf bool `mapstructure:"high_perf"`
	// 最大客户端数
	MaxClients int64 `mapstructure:"max_clients"`
}

// TLSSettings TLS 设置
type TLSSettings struct {
	// 证书文件路径
	CertFile string `mapstructure:"cert_file"`
	// 私钥文件路径
	KeyFile string `mapstructure:"key_file"`
	// Session Ticket 密钥（逗号分隔）
	SessionTicketKeys string `mapstructure:"session_ticket_keys"`
	// 密钥轮换间隔（小时）
	KeyRotationInterval int `mapstructure:"key_rotation_interval"`
}

// QUICSettings QUIC 协议设置
type QUICSettings struct {
	// 空闲超时（秒）
	MaxIdleTimeout int `mapstructure:"max_idle_timeout"`
	// 每连接最大并发流数
	MaxIncomingStreams int64 `mapstructure:"max_incoming_streams"`
	// 单向流数量
	MaxIncomingUniStreams int64 `mapstructure:"max_incoming_uni_streams"`
	// 初始流接收窗口（字节）
	InitialStreamReceiveWindow uint64 `mapstructure:"initial_stream_receive_window"`
	// 最大流接收窗口（字节）
	MaxStreamReceiveWindow uint64 `mapstructure:"max_stream_receive_window"`
	// 初始连接接收窗口（字节）
	InitialConnectionReceiveWindow uint64 `mapstructure:"initial_connection_receive_window"`
	// 最大连接接收窗口（字节）
	MaxConnectionReceiveWindow uint64 `mapstructure:"max_connection_receive_window"`
	// 启用 0-RTT（减少连接延迟）
	Allow0RTT bool `mapstructure:"allow_0rtt"`
}

// SessionSettings 会话管理设置
type SessionSettings struct {
	// 心跳间隔（秒）
	HeartbeatInterval int `mapstructure:"heartbeat_interval"`
	// 心跳超时（秒）
	HeartbeatTimeout int `mapstructure:"heartbeat_timeout"`
	// 心跳检查间隔（秒）
	HeartbeatCheckInterval int `mapstructure:"heartbeat_check_interval"`
	// 最大超时次数
	MaxTimeoutCount int32 `mapstructure:"max_timeout_count"`
}

// MessageSettings 消息处理设置
type MessageSettings struct {
	// Dispatcher Worker 数量
	WorkerCount int `mapstructure:"worker_count"`
	// 任务队列大小
	TaskQueueSize int `mapstructure:"task_queue_size"`
	// 处理超时（秒）
	HandlerTimeout int `mapstructure:"handler_timeout"`
	// 最大 Promise 数量
	MaxPromises int64 `mapstructure:"max_promises"`
	// Promise 警告阈值
	PromiseWarnThreshold int64 `mapstructure:"promise_warn_threshold"`
	// 默认消息超时（秒）
	DefaultMessageTimeout int `mapstructure:"default_message_timeout"`
	// 多队列模式
	EnableMultiQueue bool `mapstructure:"enable_multi_queue"`
	// 队列数量（多队列模式）
	QueueCount int `mapstructure:"queue_count"`
}

// BatchSettings 批量执行设置
type BatchSettings struct {
	// 是否启用
	Enabled bool `mapstructure:"enabled"`
	// 最大并发数
	MaxConcurrency int `mapstructure:"max_concurrency"`
	// 单任务超时（秒）
	TaskTimeout int `mapstructure:"task_timeout"`
	// 整体任务超时（秒）
	JobTimeout int `mapstructure:"job_timeout"`
	// 最大重试次数
	MaxRetries int `mapstructure:"max_retries"`
	// 重试间隔（秒）
	RetryInterval int `mapstructure:"retry_interval"`
}

// LogSettings 日志设置
type LogSettings struct {
	// 日志级别: debug, info, warn, error
	Level string `mapstructure:"level"`
	// 日志格式: text, json
	Format string `mapstructure:"format"`
	// 日志文件路径（空表示输出到 stdout）
	File string `mapstructure:"file"`
}

// DatabaseSettings 数据库设置
type DatabaseSettings struct {
	// 是否启用数据库
	Enabled bool `mapstructure:"enabled"`
	// 数据库类型: postgres, mysql
	Type string `mapstructure:"type"`
	// 数据库主机
	Host string `mapstructure:"host"`
	// 数据库端口
	Port int `mapstructure:"port"`
	// 数据库用户名
	User string `mapstructure:"user"`
	// 数据库密码
	Password string `mapstructure:"password"`
	// 数据库名称
	DBName string `mapstructure:"dbname"`
	// SSL 模式: disable, require, verify-ca, verify-full (PostgreSQL 专用)
	SSLMode string `mapstructure:"sslmode"`
	// 字符集 (MySQL 专用)
	Charset string `mapstructure:"charset"`
	// 是否自动迁移表结构
	AutoMigrate bool `mapstructure:"auto_migrate"`
	// GORM 日志级别: silent, error, warn, info
	// silent: 禁用所有日志（生产环境推荐）
	// error: 仅错误日志
	// warn: 警告和错误日志
	// info: 详细 SQL 日志（开发调试用）
	LogLevel string `mapstructure:"log_level"`
	// ========== 连接池配置 ==========
	// 最大空闲连接数
	MaxIdleConns int `mapstructure:"max_idle_conns"`
	// 最大打开连接数
	MaxOpenConns int `mapstructure:"max_open_conns"`
	// 连接最大存活时间（秒）
	ConnMaxLifetime int `mapstructure:"conn_max_lifetime"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		Server: ServerSettings{
			Addr:       ":8474",
			APIAddr:    ":8475",
			HighPerf:   false,
			MaxClients: 10000,
		},
		TLS: TLSSettings{
			CertFile:            "certs/server-cert.pem",
			KeyFile:             "certs/server-key.pem",
			SessionTicketKeys:    "",
			KeyRotationInterval: 24,
		},
		QUIC: QUICSettings{
			MaxIdleTimeout:                 60,
			MaxIncomingStreams:             1000,
			MaxIncomingUniStreams:          100,
			InitialStreamReceiveWindow:     512 * 1024,      // 512KB
			MaxStreamReceiveWindow:         6 * 1024 * 1024, // 6MB
			InitialConnectionReceiveWindow: 1024 * 1024,     // 1MB
			MaxConnectionReceiveWindow:     15 * 1024 * 1024, // 15MB
			Allow0RTT:                      true,
		},
		Session: SessionSettings{
			HeartbeatInterval:      15,
			HeartbeatTimeout:       45,
			HeartbeatCheckInterval: 5,
			MaxTimeoutCount:        3,
		},
		Message: MessageSettings{
			WorkerCount:           20,
			TaskQueueSize:         2000,
			HandlerTimeout:        30,
			MaxPromises:           50000,
			PromiseWarnThreshold:  40000,
			DefaultMessageTimeout: 30,
		},
		Batch: BatchSettings{
			Enabled:        false,
			MaxConcurrency: 5000,
			TaskTimeout:    60,
			JobTimeout:     600, // 10 minutes
			MaxRetries:     2,
			RetryInterval:  1,
		},
		Log: LogSettings{
			Level:  "info",
			Format: "text",
			File:   "",
		},
		Database: DatabaseSettings{
			Enabled:        true,
			Type:           "postgres",
			Host:           "localhost",
			Port:           5432,
			User:           "postgres",
			Password:       "postgres",
			DBName:         "quic_release",
			SSLMode:        "disable",
			Charset:        "utf8mb4",
			AutoMigrate:    true,
			LogLevel:       "silent", // 默认关闭 GORM 日志，避免高并发下的性能问题
			MaxIdleConns:   10,
			MaxOpenConns:   100,
			ConnMaxLifetime: 3600, // 1 小时
		},
	}
}

// HighPerfConfig 返回高性能配置
func HighPerfConfig() *ServerConfig {
	cfg := DefaultConfig()

	// 服务器设置
	cfg.Server.HighPerf = true
	cfg.Server.MaxClients = 150000

	// QUIC 设置
	cfg.QUIC.MaxIdleTimeout = 120
	cfg.QUIC.MaxIncomingStreams = 10000
	cfg.QUIC.MaxIncomingUniStreams = 1000
	cfg.QUIC.InitialStreamReceiveWindow = 1 * 1024 * 1024      // 1MB
	cfg.QUIC.MaxStreamReceiveWindow = 16 * 1024 * 1024         // 16MB
	cfg.QUIC.InitialConnectionReceiveWindow = 2 * 1024 * 1024  // 2MB
	cfg.QUIC.MaxConnectionReceiveWindow = 32 * 1024 * 1024     // 32MB

	// 会话设置
	cfg.Session.HeartbeatInterval = 30
	cfg.Session.HeartbeatTimeout = 90
	cfg.Session.HeartbeatCheckInterval = 10

	// 消息处理设置
	cfg.Message.WorkerCount = 200
	cfg.Message.TaskQueueSize = 100000
	cfg.Message.HandlerTimeout = 60
	cfg.Message.MaxPromises = 150000
	cfg.Message.PromiseWarnThreshold = 120000
	cfg.Message.DefaultMessageTimeout = 60

	// 批量执行设置
	cfg.Batch.Enabled = true
	cfg.Batch.MaxConcurrency = 5000
	cfg.Batch.TaskTimeout = 60
	cfg.Batch.JobTimeout = 1800 // 30 minutes

	return cfg
}

// Load 加载配置
// configPath: 配置文件路径（可选）
func Load(configPath string) (*ServerConfig, error) {
	v := viper.New()

	// 设置默认值
	setDefaults(v)

	// 配置文件设置
	v.SetConfigName("server")
	v.SetConfigType("yaml")

	// 搜索路径
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("/etc/quic-flow")
	v.AddConfigPath("$HOME/.quic-flow")

	// 如果指定了配置文件路径
	if configPath != "" {
		v.SetConfigFile(configPath)
	}

	// 环境变量支持
	v.SetEnvPrefix("QUIC")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// 如果指定了配置文件路径，给出更详细的错误信息
			if configPath != "" {
				return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
			}
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// 配置文件不存在，使用默认值
		if configPath != "" {
			// 如果明确指定了配置文件但不存在，返回错误
			return nil, fmt.Errorf("config file not found: %s", configPath)
		}
	}

	// 解析配置
	cfg := &ServerConfig{}
	if err := v.Unmarshal(cfg); err != nil {
		configPathUsed := configPath
		if configPathUsed == "" {
			configPathUsed = v.ConfigFileUsed()
		}
		return nil, fmt.Errorf("failed to unmarshal config from %s: %w", configPathUsed, err)
	}

	return cfg, nil
}

// setDefaults 设置默认值
func setDefaults(v *viper.Viper) {
	defaults := DefaultConfig()

	// Server
	v.SetDefault("server.addr", defaults.Server.Addr)
	v.SetDefault("server.api_addr", defaults.Server.APIAddr)
	v.SetDefault("server.high_perf", defaults.Server.HighPerf)
	v.SetDefault("server.max_clients", defaults.Server.MaxClients)

	// TLS
	v.SetDefault("tls.cert_file", defaults.TLS.CertFile)
	v.SetDefault("tls.key_file", defaults.TLS.KeyFile)
	v.SetDefault("tls.session_ticket_keys", defaults.TLS.SessionTicketKeys)
	v.SetDefault("tls.key_rotation_interval", defaults.TLS.KeyRotationInterval)

	// QUIC
	v.SetDefault("quic.max_idle_timeout", defaults.QUIC.MaxIdleTimeout)
	v.SetDefault("quic.max_incoming_streams", defaults.QUIC.MaxIncomingStreams)
	v.SetDefault("quic.max_incoming_uni_streams", defaults.QUIC.MaxIncomingUniStreams)
	v.SetDefault("quic.initial_stream_receive_window", defaults.QUIC.InitialStreamReceiveWindow)
	v.SetDefault("quic.max_stream_receive_window", defaults.QUIC.MaxStreamReceiveWindow)
	v.SetDefault("quic.initial_connection_receive_window", defaults.QUIC.InitialConnectionReceiveWindow)
	v.SetDefault("quic.max_connection_receive_window", defaults.QUIC.MaxConnectionReceiveWindow)
	v.SetDefault("quic.allow_0rtt", defaults.QUIC.Allow0RTT)

	// Session
	v.SetDefault("session.heartbeat_interval", defaults.Session.HeartbeatInterval)
	v.SetDefault("session.heartbeat_timeout", defaults.Session.HeartbeatTimeout)
	v.SetDefault("session.heartbeat_check_interval", defaults.Session.HeartbeatCheckInterval)
	v.SetDefault("session.max_timeout_count", defaults.Session.MaxTimeoutCount)

	// Message
	v.SetDefault("message.worker_count", defaults.Message.WorkerCount)
	v.SetDefault("message.task_queue_size", defaults.Message.TaskQueueSize)
	v.SetDefault("message.handler_timeout", defaults.Message.HandlerTimeout)
	v.SetDefault("message.max_promises", defaults.Message.MaxPromises)
	v.SetDefault("message.promise_warn_threshold", defaults.Message.PromiseWarnThreshold)
	v.SetDefault("message.default_message_timeout", defaults.Message.DefaultMessageTimeout)

	// Batch
	v.SetDefault("batch.enabled", defaults.Batch.Enabled)
	v.SetDefault("batch.max_concurrency", defaults.Batch.MaxConcurrency)
	v.SetDefault("batch.task_timeout", defaults.Batch.TaskTimeout)
	v.SetDefault("batch.job_timeout", defaults.Batch.JobTimeout)
	v.SetDefault("batch.max_retries", defaults.Batch.MaxRetries)
	v.SetDefault("batch.retry_interval", defaults.Batch.RetryInterval)

	// Log
	v.SetDefault("log.level", defaults.Log.Level)
	v.SetDefault("log.format", defaults.Log.Format)
	v.SetDefault("log.file", defaults.Log.File)

	// Database
	v.SetDefault("database.enabled", defaults.Database.Enabled)
	v.SetDefault("database.type", defaults.Database.Type)
	v.SetDefault("database.host", defaults.Database.Host)
	v.SetDefault("database.port", defaults.Database.Port)
	v.SetDefault("database.user", defaults.Database.User)
	v.SetDefault("database.password", defaults.Database.Password)
	v.SetDefault("database.dbname", defaults.Database.DBName)
	v.SetDefault("database.sslmode", defaults.Database.SSLMode)
	v.SetDefault("database.charset", defaults.Database.Charset)
	v.SetDefault("database.auto_migrate", defaults.Database.AutoMigrate)
	v.SetDefault("database.log_level", defaults.Database.LogLevel)
	v.SetDefault("database.max_idle_conns", defaults.Database.MaxIdleConns)
	v.SetDefault("database.max_open_conns", defaults.Database.MaxOpenConns)
	v.SetDefault("database.conn_max_lifetime", defaults.Database.ConnMaxLifetime)
}

// GenerateDefaultConfig 生成默认配置文件
func GenerateDefaultConfig(path string, highPerf bool) error {
	var cfg *ServerConfig
	if highPerf {
		cfg = HighPerfConfig()
	} else {
		cfg = DefaultConfig()
	}

	return GenerateConfig(path, cfg)
}

// GenerateConfig 生成配置文件
func GenerateConfig(path string, cfg *ServerConfig) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	v := viper.New()
	v.SetConfigType("yaml")

	// 设置配置值
	v.Set("server", cfg.Server)
	v.Set("tls", cfg.TLS)
	v.Set("quic", cfg.QUIC)
	v.Set("session", cfg.Session)
	v.Set("message", cfg.Message)
	v.Set("batch", cfg.Batch)
	v.Set("database", cfg.Database)
	v.Set("log", cfg.Log)

	// 写入文件
	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ToServerConfig 转换为 transport/server.ServerConfig
func (c *ServerConfig) GetHeartbeatInterval() time.Duration {
	return time.Duration(c.Session.HeartbeatInterval) * time.Second
}

func (c *ServerConfig) GetHeartbeatTimeout() time.Duration {
	return time.Duration(c.Session.HeartbeatTimeout) * time.Second
}

func (c *ServerConfig) GetHeartbeatCheckInterval() time.Duration {
	return time.Duration(c.Session.HeartbeatCheckInterval) * time.Second
}

func (c *ServerConfig) GetMaxIdleTimeout() time.Duration {
	return time.Duration(c.QUIC.MaxIdleTimeout) * time.Second
}

func (c *ServerConfig) GetHandlerTimeout() time.Duration {
	return time.Duration(c.Message.HandlerTimeout) * time.Second
}

func (c *ServerConfig) GetDefaultMessageTimeout() time.Duration {
	return time.Duration(c.Message.DefaultMessageTimeout) * time.Second
}

func (c *ServerConfig) GetTaskTimeout() time.Duration {
	return time.Duration(c.Batch.TaskTimeout) * time.Second
}

func (c *ServerConfig) GetJobTimeout() time.Duration {
	return time.Duration(c.Batch.JobTimeout) * time.Second
}

func (c *ServerConfig) GetRetryInterval() time.Duration {
	return time.Duration(c.Batch.RetryInterval) * time.Second
}
