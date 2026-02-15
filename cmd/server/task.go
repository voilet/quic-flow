package main

import (
	"context"
	"fmt"

	"github.com/voilet/quic-flow/pkg/api"
	"github.com/voilet/quic-flow/pkg/dispatcher"
	"github.com/voilet/quic-flow/pkg/session"
	"github.com/voilet/quic-flow/pkg/task/models"
	"github.com/voilet/quic-flow/pkg/task/scheduler"
	"github.com/voilet/quic-flow/pkg/task/store"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/transport/server"

	"gorm.io/gorm"
)

// SetupTaskSystem 初始化任务管理系统
func SetupTaskSystem(
	db *gorm.DB,
	srv *server.Server,
	disp *dispatcher.Dispatcher,
	sessionMgr *session.SessionManager,
	logger *monitoring.Logger,
) (*scheduler.TaskManager, *api.TaskWSAPI, error) {
	if db == nil {
		logger.Warn("Database not available, task system will be disabled")
		return nil, nil, nil
	}

	// 执行数据库迁移
	if err := models.Migrate(db); err != nil {
		return nil, nil, fmt.Errorf("failed to migrate task models: %w", err)
	}
	logger.Info("Task models migrated")

	// 创建存储层
	taskStore := store.NewTaskStore(db)
	executionStore := store.NewExecutionStore(db)

	// 创建任务分发器
	if disp == nil {
		return nil, nil, fmt.Errorf("dispatcher not available")
	}
	if sessionMgr == nil {
		return nil, nil, fmt.Errorf("session manager not available")
	}

	taskDispatcher := scheduler.NewTaskDispatcher(disp, sessionMgr, taskStore, logger)
	logger.Info("Task dispatcher created")

	// 创建调度器（需要 dispatcher）
	cronScheduler := scheduler.NewCronScheduler(logger, taskDispatcher)
	cronScheduler.Start()
	logger.Info("Cron scheduler started")

	// 创建任务管理器
	taskManager := scheduler.NewTaskManager(
		cronScheduler,
		taskDispatcher,
		taskStore,
		executionStore,
		logger,
	)

	// 初始化任务管理器（加载所有启用的任务）
	ctx := context.Background()
	if err := taskManager.Initialize(ctx); err != nil {
		logger.Warn("Failed to initialize task manager", "error", err)
		// 不返回错误，允许系统继续运行
	} else {
		logger.Info("Task manager initialized")
	}

	// 创建 WebSocket API
	wsAPI := api.NewTaskWSAPI(logger)

	return taskManager, wsAPI, nil
}

// AddTaskRoutes 添加任务管理 API 路由
func AddTaskRoutes(
	httpServer *api.HTTPServer,
	taskManager *scheduler.TaskManager,
	taskStore store.TaskStore,
	executionStore store.ExecutionStore,
	groupStore store.GroupStore,
	wsAPI *api.TaskWSAPI,
	logger *monitoring.Logger,
) {
	if taskManager == nil {
		logger.Warn("Task manager not available, skipping task routes")
		return
	}

	// 使用 GetRouter() 获取路由器，然后创建 /api 路由组
	router := httpServer.GetRouter()
	if router == nil {
		logger.Error("Router is nil, cannot register task routes")
		return
	}

	apiGroup := router.Group("/api")
	if apiGroup == nil {
		logger.Error("API group is nil, cannot register task routes")
		return
	}

	logger.Info("Router and API group are ready", "router_exists", router != nil, "apiGroup_exists", apiGroup != nil)

	// 任务管理 API
	taskAPI := api.NewTaskAPI(taskManager, taskStore, logger)
	taskAPI.RegisterRoutes(apiGroup)
	logger.Info("Task API routes registered", "path", "/api/tasks")

	// 执行监控 API
	executionAPI := api.NewExecutionAPI(executionStore, logger)
	executionAPI.RegisterRoutes(apiGroup)
	logger.Info("Execution API routes registered", "path", "/api/executions")

	// 分组管理 API
	groupAPI := api.NewGroupAPI(groupStore, logger)
	groupAPI.RegisterRoutes(apiGroup)
	logger.Info("Group API routes registered", "path", "/api/groups")

	// WebSocket API
	if wsAPI != nil {
		wsAPI.RegisterRoutes(apiGroup)
		logger.Info("Task WebSocket routes registered", "path", "/api/ws/tasks")
	}

	logger.Info("Task management routes registered successfully")
}

// AddClientTagRoutes 添加客户端标签管理 API 路由
func AddClientTagRoutes(
	httpServer *api.HTTPServer,
	db *gorm.DB,
	logger *monitoring.Logger,
) {
	if db == nil {
		logger.Warn("Database not available, skipping client tag routes")
		return
	}

	// 使用 GetRouter() 获取路由器，然后创建 /api 路由组
	router := httpServer.GetRouter()
	if router == nil {
		logger.Error("Router is nil, cannot register client tag routes")
		return
	}

	apiGroup := router.Group("/api")
	if apiGroup == nil {
		logger.Error("API group is nil, cannot register client tag routes")
		return
	}

	// 创建客户端标签存储层
	clientTagStore := store.NewClientTagStore(db)

	// 客户端标签管理 API
	clientTagAPI := api.NewClientTagAPI(clientTagStore, logger)
	clientTagAPI.RegisterRoutes(apiGroup)
	logger.Info("Client tag API routes registered", "path", "/api/client-tags")
}

// RegisterTaskResultHandler 注册任务执行结果处理器到服务器路由
func RegisterTaskResultHandler(
	msgRouter interface{}, // *router.Router
	executionStore store.ExecutionStore,
	logger *monitoring.Logger,
) {
	// 这个函数将在 router.go 中实现，用于处理客户端上报的任务执行结果
	// 由于 router 的类型是 *router.Router，我们需要在 router.go 中实现
	logger.Info("Task result handler registration (to be implemented in router.go)")
}
