package router

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/voilet/quic-flow/pkg/monitoring"
)

// Handler 命令处理函数
// ctx: 上下文（包含超时控制等）
// payload: 命令载荷（JSON格式）
// 返回: 响应数据和错误
type Handler func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error)

// Middleware 中间件函数
// 接收下一个Handler，返回包装后的Handler
type Middleware func(Handler) Handler

// Router 命令路由器
// 参考zinx框架设计，提供简洁的命令注册和分发机制
type Router struct {
	handlers    map[string]Handler // commandType -> Handler
	middlewares []Middleware       // 全局中间件链
	mu          sync.RWMutex
	logger      *monitoring.Logger
}

// NewRouter 创建新的路由器
func NewRouter(logger *monitoring.Logger) *Router {
	if logger == nil {
		logger = monitoring.NewLogger(monitoring.LogLevelInfo, "text")
	}
	return &Router{
		handlers: make(map[string]Handler),
		logger:   logger,
	}
}

// Register 注册命令处理器
// commandType: 命令类型（如 "exec_shell", "get_status" 等）
// handler: 处理函数
func (r *Router) Register(commandType string, handler Handler) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[commandType] = handler
	r.logger.Debug("Command handler registered", "command_type", commandType)
	return r
}

// Use 添加全局中间件
// 中间件按添加顺序执行（洋葱模型）
func (r *Router) Use(middleware Middleware) *Router {
	r.middlewares = append(r.middlewares, middleware)
	return r
}

// Execute 执行命令（实现 command.CommandExecutor 接口）
// commandType: 命令类型
// payload: 命令载荷
func (r *Router) Execute(commandType string, payload []byte) ([]byte, error) {
	return r.ExecuteWithContext(context.Background(), commandType, payload)
}

// ExecuteWithContext 带上下文执行命令
func (r *Router) ExecuteWithContext(ctx context.Context, commandType string, payload []byte) ([]byte, error) {
	r.mu.RLock()
	handler, exists := r.handlers[commandType]
	r.mu.RUnlock()

	if !exists {
		r.logger.Warn("Unknown command type", "command_type", commandType)
		return nil, fmt.Errorf("unknown command type: %s", commandType)
	}

	// 构建中间件链（从后向前包装）
	finalHandler := handler
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		finalHandler = r.middlewares[i](finalHandler)
	}

	// 执行处理链
	result, err := finalHandler(ctx, json.RawMessage(payload))
	if err != nil {
		return nil, err
	}

	return result, nil
}

// HasHandler 检查是否已注册某个命令类型
func (r *Router) HasHandler(commandType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.handlers[commandType]
	return exists
}

// ListCommands 列出所有已注册的命令类型
func (r *Router) ListCommands() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	commands := make([]string, 0, len(r.handlers))
	for cmd := range r.handlers {
		commands = append(commands, cmd)
	}
	return commands
}

// Unregister 注销命令处理器
func (r *Router) Unregister(commandType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, commandType)
	r.logger.Debug("Command handler unregistered", "command_type", commandType)
}

// Group 创建命令分组（共享中间件）
func (r *Router) Group(prefix string, middlewares ...Middleware) *RouterGroup {
	return &RouterGroup{
		router:      r,
		prefix:      prefix,
		middlewares: middlewares,
	}
}

// RouterGroup 路由分组
// 支持为一组命令添加共享前缀和中间件
type RouterGroup struct {
	router      *Router
	prefix      string
	middlewares []Middleware
}

// Register 在分组内注册命令
func (g *RouterGroup) Register(commandType string, handler Handler) *RouterGroup {
	// 构建分组中间件链
	finalHandler := handler
	for i := len(g.middlewares) - 1; i >= 0; i-- {
		finalHandler = g.middlewares[i](finalHandler)
	}

	// 添加前缀
	fullType := commandType
	if g.prefix != "" {
		fullType = g.prefix + "." + commandType
	}

	g.router.Register(fullType, finalHandler)
	return g
}

// Use 添加分组中间件
func (g *RouterGroup) Use(middleware Middleware) *RouterGroup {
	g.middlewares = append(g.middlewares, middleware)
	return g
}
