package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/task/models"
)

// WebSocket 升级器
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源（生产环境需要更严格的检查）
	},
}

// TaskWSMessage WebSocket 任务消息
type TaskWSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// TaskWSAPI WebSocket 推送服务
type TaskWSAPI struct {
	hub    *WSHub
	logger *monitoring.Logger
}

// WSHub WebSocket 连接中心
type WSHub struct {
	clients    map[*WSClient]bool
	register   chan *WSClient
	unregister chan *WSClient
	broadcast  chan []byte
	mu         sync.RWMutex
}

// WSClient WebSocket 客户端
type WSClient struct {
	hub    *WSHub
	conn   *websocket.Conn
	send   chan []byte
	logger *monitoring.Logger
}

// NewTaskWSAPI 创建 WebSocket API
func NewTaskWSAPI(logger *monitoring.Logger) *TaskWSAPI {
	hub := &WSHub{
		clients:    make(map[*WSClient]bool),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		broadcast:  make(chan []byte, 256),
	}

	go hub.run()

	return &TaskWSAPI{
		hub:    hub,
		logger: logger,
	}
}

// run Hub 主循环
func (h *WSHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// readPump 读取客户端消息
func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Warn("WebSocket error", "error", err)
			}
			break
		}
	}
}

// writePump 向客户端发送消息
func (c *WSClient) writePump() {
	defer c.conn.Close()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.logger.Warn("Failed to write WebSocket message", "error", err)
				return
			}
		}
	}
}

// RegisterRoutes 注册路由
func (api *TaskWSAPI) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/ws/tasks", api.HandleWebSocket)
}

// HandleWebSocket 处理 WebSocket 连接
func (api *TaskWSAPI) HandleWebSocket(c *gin.Context) {
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		api.logger.Error("Failed to upgrade WebSocket", "error", err)
		return
	}

	client := &WSClient{
		hub:    api.hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		logger: api.logger,
	}

	client.hub.register <- client

	// 启动读写 goroutines
	go client.writePump()
	go client.readPump()

	api.logger.Info("WebSocket client connected", "remote_addr", c.Request.RemoteAddr)
}

// BroadcastTaskStatus 广播任务状态更新
func (api *TaskWSAPI) BroadcastTaskStatus(taskID string, status int) {
	message := TaskWSMessage{
		Type: "task_status",
		Data: gin.H{
			"task_id": taskID,
			"status":  status,
		},
	}

	data, err := json.Marshal(message)
	if err != nil {
		api.logger.Error("Failed to marshal task status message", "error", err)
		return
	}

	api.hub.broadcast <- data
}

// BroadcastExecutionUpdate 广播执行记录更新
func (api *TaskWSAPI) BroadcastExecutionUpdate(execution *models.Execution) {
	message := TaskWSMessage{
		Type: "execution_update",
		Data: execution,
	}

	data, err := json.Marshal(message)
	if err != nil {
		api.logger.Error("Failed to marshal execution update message", "error", err)
		return
	}

	api.hub.broadcast <- data
}

// BroadcastTaskCreated 广播任务创建
func (api *TaskWSAPI) BroadcastTaskCreated(task *models.Task) {
	message := TaskWSMessage{
		Type: "task_created",
		Data: task,
	}

	data, err := json.Marshal(message)
	if err != nil {
		api.logger.Error("Failed to marshal task created message", "error", err)
		return
	}

	api.hub.broadcast <- data
}

// BroadcastTaskDeleted 广播任务删除
func (api *TaskWSAPI) BroadcastTaskDeleted(taskID int64) {
	message := TaskWSMessage{
		Type: "task_deleted",
		Data: gin.H{
			"task_id": taskID,
		},
	}

	data, err := json.Marshal(message)
	if err != nil {
		api.logger.Error("Failed to marshal task deleted message", "error", err)
		return
	}

	api.hub.broadcast <- data
}

// GetClientCount 获取当前连接的客户端数量
func (api *TaskWSAPI) GetClientCount() int {
	api.hub.mu.RLock()
	defer api.hub.mu.RUnlock()
	return len(api.hub.clients)
}
