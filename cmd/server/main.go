package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/voilet/quic-flow/pkg/api"
	"github.com/voilet/quic-flow/pkg/audit"
	"github.com/voilet/quic-flow/pkg/auth"
	"github.com/voilet/quic-flow/pkg/auth/captcha"
	"github.com/voilet/quic-flow/pkg/auth/middleware"
	"github.com/voilet/quic-flow/pkg/batch"
	"github.com/voilet/quic-flow/pkg/command"
	appconfig "github.com/voilet/quic-flow/pkg/config"
	"github.com/voilet/quic-flow/pkg/alert"
	"github.com/voilet/quic-flow/pkg/configcenter"
	"github.com/voilet/quic-flow/pkg/dispatcher"
	"github.com/voilet/quic-flow/pkg/hardware"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/profiling"
	"github.com/voilet/quic-flow/pkg/protocol"
	"github.com/voilet/quic-flow/pkg/recording"
	releaseapi "github.com/voilet/quic-flow/pkg/release/api"
	releasemodels "github.com/voilet/quic-flow/pkg/release/models"
	"github.com/voilet/quic-flow/pkg/router"
	"github.com/voilet/quic-flow/pkg/task/scheduler"
	"github.com/voilet/quic-flow/pkg/task/store"
	"github.com/voilet/quic-flow/pkg/transport/server"
	"github.com/voilet/quic-flow/pkg/version"

	"gorm.io/gorm"
)

var (
	// 命令行参数
	configFile string // 配置文件路径
	genConfig  string // 生成配置文件路径
	highPerf   bool   // 高性能模式（用于生成配置）
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "quic-server",
	Short: "QUIC Backbone Server",
	Long:  "QUIC Backbone Server - 高性能 QUIC 协议服务器",
	Run:   runServer,
}

// versionCmd 版本信息子命令
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Long:  "显示服务器版本号、Git 提交、编译时间等信息",
	Run: func(cmd *cobra.Command, args []string) {
		version.Print("quic-server")
	},
}

// genConfigCmd 生成配置文件子命令
var genConfigCmd = &cobra.Command{
	Use:   "genconfig",
	Short: "生成配置文件",
	Long:  "生成默认或高性能模式的配置文件",
	Run: func(cmd *cobra.Command, args []string) {
		if genConfig == "" {
			genConfig = "config/server.yaml"
		}

		if err := appconfig.GenerateDefaultConfig(genConfig, highPerf); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate config: %v\n", err)
			os.Exit(1)
		}

		mode := "standard"
		if highPerf {
			mode = "high-performance"
		}
		fmt.Printf("Configuration file generated: %s (mode: %s)\n", genConfig, mode)
	},
}

func init() {
	// 主命令参数（默认使用 config/server.yaml）
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "config/server.yaml", "配置文件路径")

	// 生成配置命令参数
	genConfigCmd.Flags().StringVarP(&genConfig, "output", "o", "config/server.yaml", "输出配置文件路径")
	genConfigCmd.Flags().BoolVar(&highPerf, "high-perf", false, "生成高性能模式配置")

	// 添加子命令
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(genConfigCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServer(cmd *cobra.Command, args []string) {
	// 加载配置
	cfg, err := appconfig.Load(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 创建日志器
	logLevel := monitoring.LogLevelInfo
	switch cfg.Log.Level {
	case "debug":
		logLevel = monitoring.LogLevelDebug
	case "warn":
		logLevel = monitoring.LogLevelWarn
	case "error":
		logLevel = monitoring.LogLevelError
	}
	logger := monitoring.NewLogger(logLevel, cfg.Log.Format)

	logger.Info("=== QUIC Backbone Server ===")
	logger.Info("Version", "version", version.String())
	if configFile != "" {
		logger.Info("Config file loaded", "path", configFile)
	}

	// 初始化数据库（如果启用）
	var releaseDB *gorm.DB
	if cfg.Database.Enabled {
		releaseDB, err = initDatabase(cfg, logger)
		if err != nil {
			logger.Error("Failed to initialize database", "error", err)
			logger.Warn("Release system will run without database")
		}
	} else {
		logger.Info("Database disabled, release system will be limited")
	}

	logger.Info("Starting server",
		"addr", cfg.Server.Addr,
		"high_perf", cfg.Server.HighPerf,
		"max_clients", cfg.Server.MaxClients)

	// 创建服务器配置
	serverConfig := buildServerConfig(cfg, logger)

	// 创建服务器
	srv, err := server.NewServer(serverConfig)
	if err != nil {
		logger.Error("Failed to create server", "error", err)
		os.Exit(1)
	}

	// 设置消息路由器
	msgRouter := SetupServerRouter(logger)

	// 创建 Dispatcher 并注册消息处理器
	disp := setupServerDispatcherWithConfig(logger, msgRouter,
		cfg.Message.WorkerCount,
		cfg.Message.TaskQueueSize,
		cfg.GetHandlerTimeout())

	logger.Info("Dispatcher created",
		"workers", cfg.Message.WorkerCount,
		"queue_size", cfg.Message.TaskQueueSize)

	// 设置 Dispatcher 到服务器
	srv.SetDispatcher(disp)
	logger.Info("Dispatcher attached to server")

	// 启动服务器
	if err := srv.Start(cfg.Server.Addr); err != nil {
		logger.Error("Failed to start server", "error", err)
		os.Exit(1)
	}

	logger.Info("Server started successfully")

	// 创建命令管理器
	commandManager := command.NewCommandManager(srv, logger)
	logger.Info("Command manager created")

	// 创建批量执行器
	var batchExecutor *batch.BatchExecutor
	if cfg.Batch.Enabled {
		batchConfig := &batch.BatchConfig{
			MaxConcurrency: cfg.Batch.MaxConcurrency,
			TaskTimeout:    cfg.GetTaskTimeout(),
			JobTimeout:     cfg.GetJobTimeout(),
			MaxRetries:     cfg.Batch.MaxRetries,
			RetryInterval:  cfg.GetRetryInterval(),
			Logger:         logger,
			OnProgress: func(job *batch.BatchJob) {
				progress := float64(job.SuccessCount+job.FailedCount) / float64(job.TotalCount) * 100
				logger.Info("Batch progress", "job_id", job.ID, "progress", fmt.Sprintf("%.1f%%", progress))
			},
		}
		batchExecutor = batch.NewBatchExecutor(srv, batchConfig)
		logger.Info("Batch executor created", "max_concurrency", batchConfig.MaxConcurrency)
	}

	// 启动 HTTP API 服务器
	httpServer := api.NewHTTPServer(cfg.Server.APIAddr, srv, commandManager, logger)

	// 添加数据库初始化引导 API
	setupAPI := api.NewSetupAPI(configFile, logger)
	httpServer.AddSetupRoutes(setupAPI)

	// 尝试自动连接数据库（如果配置了）
	if cfg.Database.Enabled {
		if err := setupAPI.TryAutoConnect(cfg); err != nil {
			logger.Warn("Auto-connect to database failed", "error", err)
			logger.Info("Visit /setup to configure database")
			// 如果 initDatabase 已经成功，releaseDB 应该已经有值
			if releaseDB == nil {
				logger.Warn("Database not available, some features will be disabled")
			}
		} else {
			logger.Info("Database auto-connected successfully")
			dbFromSetup := setupAPI.GetDB()
			if dbFromSetup != nil {
				releaseDB = dbFromSetup
				logger.Info("Using database from setup API")
			} else if releaseDB == nil {
				logger.Warn("Database connection from setup API is nil")
			}
		}
	}

	// 记录数据库状态
	if releaseDB != nil {
		logger.Info("Database is available", "releaseDB_not_nil", true)
	} else {
		logger.Info("Database is not available", "releaseDB_nil", true, "cfg.Database.Enabled", cfg.Database.Enabled)
	}

	// 添加批量执行 API
	if batchExecutor != nil {
		httpServer.AddBatchRoutes(batchExecutor)
	}

	// 添加流式 API（SSE）
	httpServer.AddStreamRoutes()

	// 创建 SSH 客户端管理器
	sshManager := NewSSHClientManager(srv, nil, logger)
	sshAPIAdapter := NewSSHClientManagerAPIAdapter(sshManager)
	httpServer.AddSSHRoutes(sshAPIAdapter)
	logger.Info("SSH client manager created")

	// 创建审计存储（使用 PostgreSQL）
	var auditStore audit.Store
	if releaseDB != nil {
		var err error
		auditStore, err = audit.NewPostgresStore(releaseDB)
		if err != nil {
			logger.Error("Failed to create PostgreSQL audit store", "error", err)
		} else {
			logger.Info("Audit store created (PostgreSQL)")
		}
	}
	if auditStore == nil {
		logger.Warn("Audit store not available, will be initialized when database is ready")
	}

	// 创建录像存储
	recordingStore, err := recording.NewStore("data/recordings")
	if err != nil {
		logger.Error("Failed to create recording store", "error", err)
		os.Exit(1)
	}
	logger.Info("Recording store created", "path", "data/recordings")

	// 创建录像数据库存储（如果数据库可用）
	var recordingDBStore *recording.DBStore
	if releaseDB != nil {
		var err error
		recordingDBStore, err = recording.NewDBStore(releaseDB)
		if err != nil {
			logger.Error("Failed to create recording database store", "error", err)
		} else {
			logger.Info("Recording database store created")
		}
	}

	// 创建录像配置
	recordingConfig := &recording.Config{
		Enabled:     true,
		StorePath:   "data/recordings",
		RecordInput: true,
	}

	// 创建终端管理器（WebSocket SSH 终端）带审计和录像
	terminalAdapter := NewSSHTerminalAdapter(sshManager)
	terminalManager := api.NewTerminalManagerWithRecording(terminalAdapter, logger, auditStore, recordingConfig)
	if recordingDBStore != nil {
		terminalManager.SetRecordingDBStore(recordingDBStore)
	}
	httpServer.AddTerminalRoutes(terminalManager)
	logger.Info("Terminal WebSocket routes added")

	// 添加审计 API 路由
	auditAPI := api.NewAuditAPI(auditStore, logger)
	httpServer.AddAuditRoutes(auditAPI)

	// 添加录像 API 路由
	recordingAPI := api.NewRecordingAPI(recordingStore, logger)
	if recordingDBStore != nil {
		recordingAPI.SetDBStore(recordingDBStore)
	}
	httpServer.AddRecordingRoutes(recordingAPI)

	// 添加发布系统 API 路由
	releaseAPI := releaseapi.NewReleaseAPIWithRemote(releaseDB, commandManager)
	httpServer.AddReleaseRoutes(releaseAPI)

	// ========== 硬件信息功能 ==========
	// 创建硬件存储和 API 处理器
	var hardwareStore *hardware.Store
	var hardwareAPI *hardware.Handler
	if releaseDB != nil {
		hardwareStore = hardware.NewStore(releaseDB)
		hardwareAPI = hardware.NewHandler(hardwareStore)

		// 设置到 HTTP Server（用于客户端列表整合设备信息）
		httpServer.SetHardwareStore(hardwareStore)

		// 注册硬件 API 路由
		hardwareRouter := httpServer.GetRouter().Group("/api")
		hardwareAPI.RegisterRoutes(hardwareRouter)

		// 注册命令结果处理器，自动保存硬件信息
		commandManager.RegisterResultHandler(hardware.NewCommandResultHandler(hardwareStore))

		// 启动硬件信息上报处理 goroutine（处理客户端自动上报的硬件信息）
		go processHardwareReports(hardwareStore, logger)

		logger.Info("Hardware info system enabled with database")
	} else {
		logger.Info("Hardware info system disabled (database not configured)")
	}

	// ========== 配置中心功能 ==========
	// 创建配置中心存储和 API 处理器
	if releaseDB != nil {
		configStore := configcenter.NewStore(releaseDB)
		configHandler := configcenter.NewHandler(configStore)

		// 注册配置中心 API 路由
		configRouter := httpServer.GetRouter().Group("/api")
		configHandler.RegisterRoutes(configRouter)

		logger.Info("Config center API routes registered")
	}

	// ========== 告警系统功能 ==========
	// 创建告警系统存储和 API 处理器
	if releaseDB != nil {
		alertStore, err := alert.NewStore(releaseDB)
		if err != nil {
			logger.Warn("Failed to create alert store", "error", err)
		} else {
			// 执行告警系统数据库迁移
			ctx := context.Background()
			logger.Info("Starting alert system database migration...")
			if err := alertStore.AutoMigrate(ctx); err != nil {
				logger.Error("Failed to migrate alert tables", "error", err)
				// 即使迁移失败也继续，但记录错误
			} else {
				logger.Info("Alert system database migration completed successfully")
			}

			alertHandler := alert.NewHandler(alertStore)

			// 注册告警系统 API 路由
			alertRouter := httpServer.GetRouter().Group("/api")
			alertHandler.RegisterRoutes(alertRouter)

			logger.Info("Alert system API routes registered")
		}
	}

	// ========== 性能分析功能 ==========
	// 创建性能分析器（需要数据库存储采集记录）
	var profilingHandler *profiling.Handler
	if releaseDB != nil {
		// 创建采集文件存储目录
		profilingStoreDir := "data/profiling"
		profiler, err := profiling.NewProfiler(releaseDB, profilingStoreDir)
		if err != nil {
			logger.Error("Failed to create profiler", "error", err)
		} else {
			// 初始化数据库表
			if err := profiler.Init(); err != nil {
				logger.Warn("Failed to initialize profiling tables", "error", err)
			} else {
				profilingHandler = profiling.NewHandler(profiler)
				httpServer.AddProfilingRoutes(profilingHandler)
				logger.Info("Profiling system enabled", "store_dir", profilingStoreDir)
			}
		}
	}

	// ========== 标准 pprof 端点（兼容 go tool pprof） ==========
	// 启用 block 和 mutex profiling
	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)
	stdProfilingHandler := profiling.NewStandardHandler()
	httpServer.AddStandardProfilingRoutes(stdProfilingHandler)
	logger.Info("Standard pprof enabled at /debug/pprof/ (use 'go tool pprof http://host:port/debug/pprof/profile?seconds=30')")

	// ========== 文件传输功能 ==========
	_, _, _, fileAPI := SetupFileTransfer(releaseDB, logger)
	if fileAPI != nil {
		AddFileTransferRoutes(httpServer, fileAPI)
		logger.Info("File transfer system enabled")
	}

	// ========== 任务管理系统变量（需要在回调中使用）==========
	var taskManager *scheduler.TaskManager
	var taskWSAPI *api.TaskWSAPI

	// 设置数据库初始化回调，当通过 setup 页面初始化数据库后更新 release API 和 audit_store
	setupAPI.SetOnDBReady(func(db *gorm.DB) {
		releaseDB = db // 更新 releaseDB 引用

		releaseAPI.SetDB(db)
		releaseAPI.SetRemoteExecutor(commandManager)
		logger.Info("Release API database updated via setup")

		// 创建 PostgreSQL audit store 并更新相关组件
		newAuditStore, err := audit.NewPostgresStore(db)
		if err != nil {
			logger.Error("Failed to create PostgreSQL audit store", "error", err)
			return
		}
		auditAPI.SetStore(newAuditStore)
		terminalManager.SetAuditStore(newAuditStore)
		logger.Info("Audit store updated to PostgreSQL via setup")

		// 创建录制数据库存储并更新终端管理器和API
		newRecordingDBStore, err := recording.NewDBStore(db)
		if err != nil {
			logger.Error("Failed to create recording database store", "error", err)
		} else {
			terminalManager.SetRecordingDBStore(newRecordingDBStore)
			recordingAPI.SetDBStore(newRecordingDBStore)
			logger.Info("Recording database store updated via setup")
		}

		// 初始化硬件信息系统
		if hardwareStore == nil {
			hardwareStore = hardware.NewStore(db)
			hardwareAPI = hardware.NewHandler(hardwareStore)

			// 设置到 HTTP Server（用于客户端列表整合设备信息）
			httpServer.SetHardwareStore(hardwareStore)

			hardwareRouter := httpServer.GetRouter().Group("/api")
			hardwareAPI.RegisterRoutes(hardwareRouter)
			commandManager.RegisterResultHandler(hardware.NewCommandResultHandler(hardwareStore))

			// 启动硬件信息上报处理 goroutine
			go processHardwareReports(hardwareStore, logger)

			logger.Info("Hardware info system enabled via setup")
		}

		// 初始化性能分析系统
		if profilingHandler == nil {
			profilingStoreDir := "data/profiling"
			profiler, err := profiling.NewProfiler(db, profilingStoreDir)
			if err != nil {
				logger.Error("Failed to create profiler via setup", "error", err)
			} else {
				if err := profiler.Init(); err != nil {
					logger.Warn("Failed to initialize profiling tables via setup", "error", err)
				} else {
					profilingHandler = profiling.NewHandler(profiler)
					httpServer.AddProfilingRoutes(profilingHandler)
					logger.Info("Profiling system enabled via setup")
				}
			}
		}

		// 初始化任务管理系统（如果尚未初始化）
		if taskManager == nil {
			var err error
			taskManager, taskWSAPI, err = SetupTaskSystem(db, srv, disp, srv.GetSessions(), logger)
			if err != nil {
				logger.Error("Failed to setup task system via setup", "error", err)
			} else if taskManager != nil {
				// 创建存储层
				taskStore := store.NewTaskStore(db)
				executionStore := store.NewExecutionStore(db)
				groupStore := store.NewGroupStore(db)

				// 添加任务管理 API 路由
				AddTaskRoutes(httpServer, taskManager, taskStore, executionStore, groupStore, taskWSAPI, logger)

				logger.Info("Task management system enabled via setup")
			}
		}
	})

	if releaseDB != nil {
		logger.Info("Release API routes added with database")
	} else {
		logger.Info("Release API routes added (database not configured, visit /setup)")
	}

	// ========== 任务管理系统 ==========
	// 在 HTTP 服务器启动前注册路由
	if releaseDB != nil {
		logger.Info("Initializing task management system...", "database_available", true, "releaseDB", releaseDB != nil)
		var err error
		// 获取 dispatcher 和 session manager
		if disp == nil {
			logger.Error("Dispatcher is nil, cannot setup task system")
		} else if srv.GetSessions() == nil {
			logger.Error("Session manager is nil, cannot setup task system")
		} else {
			taskManager, taskWSAPI, err = SetupTaskSystem(releaseDB, srv, disp, srv.GetSessions(), logger)
			if err != nil {
				logger.Error("Failed to setup task system", "error", err)
			} else if taskManager != nil {
				// 创建存储层
				taskStore := store.NewTaskStore(releaseDB)
				executionStore := store.NewExecutionStore(releaseDB)
				groupStore := store.NewGroupStore(releaseDB)

				// 添加任务管理 API 路由（在 Start 之前）
				logger.Info("Registering task management routes...")
				AddTaskRoutes(httpServer, taskManager, taskStore, executionStore, groupStore, taskWSAPI, logger)

				logger.Info("Task management system enabled and routes registered")
			} else {
				logger.Warn("Task manager is nil after setup")
			}
		}
	} else {
		logger.Info("Task management system disabled (database not configured)", "database_available", false)
		logger.Info("Please configure database via /setup page to enable task management")
		logger.Info("Current releaseDB status", "releaseDB_nil", releaseDB == nil)
	}

	if err := httpServer.Start(); err != nil {
		logger.Error("Failed to start HTTP API server", "error", err)
		os.Exit(1)
	}

	// 设置权限系统路由
	if releaseDB != nil {
		_, err := setupAuthRoutes(releaseDB, httpServer)
		if err != nil {
			logger.Warn("Failed to setup auth routes", "error", err)
		} else {
			logger.Info("Auth system routes registered with JWT protection")
		}
	}

	logger.Info("HTTP API server started", "addr", cfg.Server.APIAddr)
	logger.Info("Command system enabled")
	if batchExecutor != nil {
		logger.Info("Batch execution enabled")
	}
	logger.Info("SSH over QUIC enabled")
	logger.Info("Press Ctrl+C to stop")

	// 定期打印统计信息（已禁用）
	// go printServerStatus(srv, msgRouter)

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// 优雅关闭
	if batchExecutor != nil {
		batchExecutor.Stop()
	}
	if taskManager != nil {
		// 停止 Cron 调度器（通过 Stop 方法）
		// TaskManager 需要添加 Stop 方法，暂时先注释
		// taskManager.Stop()
		logger.Info("Task scheduler shutdown")
	}
	sshManager.Close()
	if auditStore != nil {
		auditStore.Close()
	}
	shutdownServer(logger, disp, httpServer, srv)
}

// buildServerConfig 从配置文件构建服务器配置
func buildServerConfig(cfg *appconfig.ServerConfig, logger *monitoring.Logger) *server.ServerConfig {
	serverConfig := &server.ServerConfig{
		TLSCertFile: cfg.TLS.CertFile,
		TLSKeyFile:  cfg.TLS.KeyFile,
		ListenAddr:  cfg.Server.Addr,

		// QUIC 配置
		MaxIdleTimeout:                 cfg.GetMaxIdleTimeout(),
		MaxIncomingStreams:             cfg.QUIC.MaxIncomingStreams,
		MaxIncomingUniStreams:          cfg.QUIC.MaxIncomingUniStreams,
		InitialStreamReceiveWindow:     cfg.QUIC.InitialStreamReceiveWindow,
		MaxStreamReceiveWindow:         cfg.QUIC.MaxStreamReceiveWindow,
		InitialConnectionReceiveWindow: cfg.QUIC.InitialConnectionReceiveWindow,
		MaxConnectionReceiveWindow:     cfg.QUIC.MaxConnectionReceiveWindow,

		// 会话管理配置
		MaxClients:             cfg.Server.MaxClients,
		HeartbeatInterval:      cfg.GetHeartbeatInterval(),
		HeartbeatTimeout:       cfg.GetHeartbeatTimeout(),
		HeartbeatCheckInterval: cfg.GetHeartbeatCheckInterval(),
		MaxTimeoutCount:        cfg.Session.MaxTimeoutCount,

		// Promise 配置
		MaxPromises:           cfg.Message.MaxPromises,
		PromiseWarnThreshold:  cfg.Message.PromiseWarnThreshold,
		DefaultMessageTimeout: cfg.GetDefaultMessageTimeout(),

		// 监控
		Logger: logger,
	}

	// 设置事件钩子
	serverConfig.Hooks = &monitoring.EventHooks{
		OnConnect: func(clientID string) {
			logger.Info("Client connected", "client_id", clientID)
		},
		OnDisconnect: func(clientID string, reason error) {
			logger.Info("Client disconnected", "client_id", clientID, "reason", reason)
		},
		OnHeartbeatTimeout: func(clientID string) {
			logger.Warn("Heartbeat timeout", "client_id", clientID)
		},
	}

	return serverConfig
}

// setupServerDispatcherWithConfig 使用自定义配置设置服务器消息分发器
func setupServerDispatcherWithConfig(logger *monitoring.Logger, msgRouter *router.Router, workerCount int, queueSize int, timeout time.Duration) *dispatcher.Dispatcher {
	dispatcherConfig := &dispatcher.DispatcherConfig{
		WorkerCount:    workerCount,
		TaskQueueSize:  queueSize,
		HandlerTimeout: timeout,
		Logger:         logger,
	}
	disp := dispatcher.NewDispatcher(dispatcherConfig)

	// 创建路由处理函数
	routeHandler := func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
		// 从payload中解析命令类型
		var cmdPayload struct {
			CommandType string          `json:"command_type"`
			Payload     json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(msg.Payload, &cmdPayload); err != nil {
			// 如果解析失败，尝试直接作为payload处理
			cmdPayload.CommandType = "unknown"
			cmdPayload.Payload = msg.Payload
		}

		// 使用路由器执行
		result, err := msgRouter.ExecuteWithContext(ctx, cmdPayload.CommandType, cmdPayload.Payload)
		if err != nil {
			return nil, err
		}

		// 构建响应消息
		return &protocol.DataMessage{
			MsgId:     msg.MsgId,
			SenderId:  "server",
			Type:      protocol.MessageType_MESSAGE_TYPE_RESPONSE,
			Payload:   result,
			Timestamp: time.Now().UnixMilli(),
		}, nil
	}

	// 注册消息类型处理器
	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_EVENT, dispatcher.MessageHandlerFunc(routeHandler))
	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_QUERY, dispatcher.MessageHandlerFunc(routeHandler))
	disp.RegisterHandler(protocol.MessageType_MESSAGE_TYPE_RESPONSE, dispatcher.MessageHandlerFunc(func(ctx context.Context, msg *protocol.DataMessage) (*protocol.DataMessage, error) {
		logger.Info("Received response from client", "msg_id", msg.MsgId, "sender", msg.SenderId)
		return nil, nil
	}))

	// 启动 Dispatcher
	disp.Start()
	logger.Info("Dispatcher started")

	return disp
}

// printServerStatus 定期打印服务器状态
func printServerStatus(srv *server.Server, msgRouter *router.Router) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		metrics := srv.GetMetrics()
		clients := srv.ListClients()

		fmt.Printf("\n=== Server Status ===\n")
		fmt.Printf("Connected Clients: %d\n", len(clients))
		fmt.Printf("Total Connections: %d\n", metrics.ConnectedClients)
		fmt.Printf("Messages Sent: %d\n", metrics.MessageThroughput)
		fmt.Printf("Registered Routes: %v\n", msgRouter.ListCommands())

		if len(clients) > 0 {
			fmt.Printf("Active Clients:\n")
			for _, clientID := range clients {
				info, err := srv.GetClientInfo(clientID)
				if err == nil {
					uptime := time.Since(time.UnixMilli(info.ConnectedAt))
					fmt.Printf("  - %s (uptime: %v)\n", clientID, uptime.Round(time.Second))
				}
			}
		}
		fmt.Println()
	}
}

// shutdownServer 优雅关闭服务器
func shutdownServer(logger *monitoring.Logger, disp *dispatcher.Dispatcher, httpServer *api.HTTPServer, srv *server.Server) {
	logger.Info("Shutting down server...")

	// 停止 Dispatcher
	disp.Stop()
	logger.Info("Dispatcher stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 停止 HTTP API 服务器
	if err := httpServer.Stop(ctx); err != nil {
		logger.Error("Error stopping HTTP API server", "error", err)
	}

	if err := srv.Stop(ctx); err != nil {
		logger.Error("Error during shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Server stopped gracefully")
}

// initDatabase 初始化数据库连接
func initDatabase(cfg *appconfig.ServerConfig, logger *monitoring.Logger) (*gorm.DB, error) {
	// 确定数据库类型
	dbType := releasemodels.DBType(cfg.Database.Type)
	if dbType == "" {
		dbType = releasemodels.DBTypePostgres
	}

	dbConfig := &releasemodels.DatabaseConfig{
		Type:            dbType,
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		DBName:          cfg.Database.DBName,
		SSLMode:         cfg.Database.SSLMode,
		Charset:         cfg.Database.Charset,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	}

	// 获取配置的 GORM 日志级别
	logLevel := cfg.Database.LogLevel
	if logLevel == "" {
		logLevel = "silent" // 默认静默，避免高并发性能问题
	}

	logger.Info("Connecting to database",
		"host", dbConfig.Host,
		"port", dbConfig.Port,
		"dbname", dbConfig.DBName,
		"log_level", logLevel)

	db, err := releasemodels.InitDBWithLogLevel(dbConfig, logLevel)
	if err != nil {
		return nil, err
	}

	logger.Info("Database connected successfully")

	// 自动迁移表结构
	if cfg.Database.AutoMigrate {
		logger.Info("Running database migrations...")
		if err := releasemodels.Migrate(db); err != nil {
			return nil, fmt.Errorf("database migration failed: %w", err)
		}

		// 修复硬件表的列名问题
		if err := hardware.AutoMigrateFixes(db); err != nil {
			logger.Warn("Failed to apply hardware migration fixes", "error", err)
		}

		logger.Info("Database migrations completed")

		// 初始化权限系统数据库表和种子数据
		if err := initAuthSystem(db); err != nil {
			logger.Warn("Failed to initialize auth system", "error", err)
		}
	}

	return db, nil
}

// initAuthSystem 初始化权限系统
func initAuthSystem(db *gorm.DB) error {
	fmt.Println("=== 初始化权限系统 ===")

	// 检查是否已初始化
	hasData, err := auth.CheckInit(db)
	if err != nil {
		return fmt.Errorf("检查权限系统状态失败: %w", err)
	}

	if !hasData {
		// 首次初始化，创建表结构和种子数据
		if err := auth.InitDB(db); err != nil {
			return fmt.Errorf("权限系统初始化失败: %w", err)
		}
	}

	return nil
}

// setupAuthRoutes 设置权限系统路由，返回authManager供后续使用
func setupAuthRoutes(db *gorm.DB, httpServer *api.HTTPServer) (*auth.Manager, error) {
	// 创建 JWT 配置
	jwtConfig := &middleware.JWTConfig{
		SigningKey:  "quic-flow-jwt-secret-key-change-in-production",
		ExpiresTime: 7 * 24 * time.Hour, // 7天
		BufferTime:  1 * time.Hour,      // 1小时缓冲
		Issuer:      "quic-flow",
	}

	// 创建权限管理器
	authManager, err := auth.NewManager(db, &auth.Config{
		JWTSigningKey: jwtConfig.SigningKey,
		JWTExpires:    jwtConfig.ExpiresTime.String(),
		BufferTime:    jwtConfig.BufferTime.String(),
		RouterPrefix:  "/api",
	})
	if err != nil {
		return nil, err
	}

	// 初始化权限系统
	if err := authManager.Initialize(); err != nil {
		return nil, err
	}

	// 设置白名单路径（不需要认证的路由）
	authManager.SetWhitelist([]string{
		"/api/base/login",
		"/api/base/captcha",
		"/api/setup",
		"/health",
	})

	// 注册验证码路由（公开，不需要JWT）
	captch := captcha.NewCaptcha(nil)
	captch.RegisterRoutes(httpServer.GetRouter().Group("/api/base"))

	// 设置验证码验证函数
	auth.SetCaptchaVerify(func(id, code string) bool {
		return captcha.GetCodeStore().Verify(id, code)
	})

	// 注册公开路由（登录、登出等）
	publicRouter := httpServer.GetRouter().Group("/api")
	authManager.RegisterPublicRoutes(publicRouter)

	// 为业务API添加JWT中间件保护
	jwtMiddleware := authManager.GetJWTMiddleware()

	// 保护需要认证的业务API路由
	businessAPI := httpServer.GetRouter().Group("/api")
	businessAPI.Use(jwtMiddleware.Handler())

	return authManager, nil
}

// processHardwareReports 处理客户端硬件信息自动上报
func processHardwareReports(store *hardware.Store, logger *monitoring.Logger) {
	reportChan := GetHardwareReportChan()

	for report := range reportChan {
		// 解析硬件信息
		var hwInfo command.HardwareInfoResult
		if err := json.Unmarshal(report.HardwareInfo, &hwInfo); err != nil {
			logger.Warn("Failed to parse hardware report", "client_id", report.ClientID, "error", err)
			continue
		}

		// 保存到数据库
		if _, err := store.SaveHardwareInfo(report.ClientID, &hwInfo); err != nil {
			logger.Warn("Failed to save hardware report", "client_id", report.ClientID, "error", err)
		} else {
			logger.Info("Hardware report saved", "client_id", report.ClientID, "report_type", report.ReportType)
		}
	}
}
