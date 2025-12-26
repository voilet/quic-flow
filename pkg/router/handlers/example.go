package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/voilet/QuicFlow/pkg/monitoring"
	"github.com/voilet/QuicFlow/pkg/router"
)

// ========================================
// 示例：如何添加自定义命令处理器
// ========================================

// 方式1：使用函数直接注册
// ----------------------------------------

// HandlePing 简单的ping命令
func HandlePing(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]interface{}{
		"message": "pong",
		"time":    time.Now().Format(time.RFC3339),
	})
}

// HandleEcho 回显命令
func HandleEcho(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	// 直接返回收到的payload
	return payload, nil
}

// 方式2：使用结构体处理器（推荐用于复杂逻辑）
// ----------------------------------------

// FileHandler 文件操作处理器
type FileHandler struct {
	logger  *monitoring.Logger
	baseDir string
}

// NewFileHandler 创建文件处理器
func NewFileHandler(logger *monitoring.Logger, baseDir string) *FileHandler {
	return &FileHandler{
		logger:  logger,
		baseDir: baseDir,
	}
}

// HandleList 列出目录文件
func (h *FileHandler) HandleList(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(payload, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// 这里添加实际的文件列表逻辑
	h.logger.Info("Listing files", "path", params.Path)

	return json.Marshal(map[string]interface{}{
		"path":  params.Path,
		"files": []string{"file1.txt", "file2.txt"},
	})
}

// HandleRead 读取文件内容
func (h *FileHandler) HandleRead(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(payload, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// 这里添加实际的文件读取逻辑
	h.logger.Info("Reading file", "path", params.Path)

	return json.Marshal(map[string]interface{}{
		"path":    params.Path,
		"content": "file content here",
	})
}

// 方式3：使用分组注册相关命令
// ----------------------------------------

// RegisterFileCommands 注册文件相关命令
func RegisterFileCommands(r *router.Router, logger *monitoring.Logger) {
	h := NewFileHandler(logger, "/tmp")

	// 使用分组注册，命令会变成 "file.list", "file.read"
	fileGroup := r.Group("file")
	fileGroup.Register("list", h.HandleList)
	fileGroup.Register("read", h.HandleRead)
}

// 方式4：HTTP代理命令
// ----------------------------------------

// HTTPProxyHandler HTTP代理处理器
type HTTPProxyHandler struct {
	logger *monitoring.Logger
	client *http.Client
}

// NewHTTPProxyHandler 创建HTTP代理处理器
func NewHTTPProxyHandler(logger *monitoring.Logger) *HTTPProxyHandler {
	return &HTTPProxyHandler{
		logger: logger,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Handle 处理HTTP请求
func (h *HTTPProxyHandler) Handle(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Method  string            `json:"method"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers,omitempty"`
		Body    string            `json:"body,omitempty"`
	}
	if err := json.Unmarshal(payload, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if params.Method == "" {
		params.Method = "GET"
	}

	h.logger.Info("HTTP proxy request", "method", params.Method, "url", params.URL)

	req, err := http.NewRequestWithContext(ctx, params.Method, params.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	for k, v := range params.Headers {
		req.Header.Set(k, v)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	return json.Marshal(map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        string(body),
	})
}

// ========================================
// 使用示例
// ========================================

// SetupExampleRouter 设置示例路由器
func SetupExampleRouter(logger *monitoring.Logger) *router.Router {
	r := router.NewRouter(logger)

	// 添加中间件
	r.Use(router.RecoveryMiddleware(logger))
	r.Use(router.LoggingMiddleware(logger))

	// 注册内置处理器
	RegisterBuiltinHandlers(r, &Config{
		Logger:  logger,
		Version: "1.0.0",
	})

	// 方式1：直接注册函数
	r.Register("ping", HandlePing)
	r.Register("echo", HandleEcho)

	// 方式2：注册结构体方法
	httpProxy := NewHTTPProxyHandler(logger)
	r.Register("http_request", httpProxy.Handle)

	// 方式3：使用分组
	RegisterFileCommands(r, logger)

	// 方式4：内联匿名函数
	r.Register("time", func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
		return json.Marshal(map[string]string{
			"time": time.Now().Format(time.RFC3339),
			"zone": time.Now().Location().String(),
		})
	})

	return r
}
