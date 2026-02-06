package api

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/voilet/quic-flow/pkg/audit"
	"github.com/voilet/quic-flow/pkg/common"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/recording"
)

// WebSocket 升级器
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源（生产环境需要更严格的检查）
	},
}

// TerminalManagerAPI 终端管理器需要实现的接口
type TerminalManagerAPI interface {
	// StartPTYSession 启动 PTY 会话
	StartPTYSession(clientID string, cols, rows int) (PTYSessionInfo, error)
	// ResizePTY 调整 PTY 大小
	ResizePTY(sessionID string, cols, rows int) error
	// ClosePTYSession 关闭 PTY 会话
	ClosePTYSession(sessionID string) error
	// GetPTYSessionReader 获取 PTY 会话的输出 Reader
	GetPTYSessionReader(sessionID string) (io.Reader, error)
	// GetPTYSessionWriter 获取 PTY 会话的输入 Writer
	GetPTYSessionWriter(sessionID string) (io.WriteCloser, error)
	// GetPTYSessionDone 获取 PTY 会话完成通道
	GetPTYSessionDone(sessionID string) (<-chan struct{}, error)
	// ListPTYSessions 列出所有 PTY 会话
	ListPTYSessions() []PTYSessionInfo
}

// PTYSessionInfo PTY 会话信息
type PTYSessionInfo struct {
	ID        string    `json:"id"`
	ClientID  string    `json:"client_id"`
	Cols      int       `json:"cols"`
	Rows      int       `json:"rows"`
	CreatedAt time.Time `json:"created_at"`
}

// TerminalManager 终端管理器
type TerminalManager struct {
	sshManager       TerminalManagerAPI
	logger           *monitoring.Logger
	sessions         map[string]*TerminalSession
	mu               sync.RWMutex
	auditStore       audit.Store
	recordingConfig  *recording.Config
	recordingDBStore *recording.DBStore
}

// TerminalSession WebSocket 终端会话
type TerminalSession struct {
	ID           string
	ClientID     string
	PTYSessionID string
	WebSocket    *websocket.Conn
	CreatedAt    time.Time
	done         chan struct{}
	auditor      *audit.SessionAuditor
	recorder     *recording.Recorder
}

// WebSocket 消息类型
type WSMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// NewTerminalManager 创建终端管理器
func NewTerminalManager(sshManager TerminalManagerAPI, logger *monitoring.Logger) *TerminalManager {
	return &TerminalManager{
		sshManager: sshManager,
		logger:     logger,
		sessions:   make(map[string]*TerminalSession),
	}
}

// NewTerminalManagerWithRecording 创建带录制功能的终端管理器
func NewTerminalManagerWithRecording(sshManager TerminalManagerAPI, logger *monitoring.Logger, auditStore audit.Store, recordingConfig *recording.Config) *TerminalManager {
	return &TerminalManager{
		sshManager:       sshManager,
		logger:           logger,
		sessions:         make(map[string]*TerminalSession),
		auditStore:       auditStore,
		recordingConfig:  recordingConfig,
		recordingDBStore: nil,
	}
}

// SetRecordingDBStore 设置录制数据库存储（当数据库可用时调用）
func (tm *TerminalManager) SetRecordingDBStore(store *recording.DBStore) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.recordingDBStore = store
}

// SetAuditStore updates the audit store (used when database becomes available)
func (tm *TerminalManager) SetAuditStore(store audit.Store) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.auditStore = store
}

// HandleWebSocket 处理 WebSocket 终端连接
func (tm *TerminalManager) HandleWebSocket(c *gin.Context) {
	clientID := c.Param("client_id")
	if clientID == "" {
		common.ErrorResp(c, common.CodeInvalidParams, "client_id is required")
		return
	}

	// 升级到 WebSocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		tm.logger.Error("Failed to upgrade WebSocket", "error", err)
		return
	}
	defer ws.Close()

	tm.logger.Info("WebSocket terminal connected", "client_id", clientID)

	// 等待客户端发送初始终端大小（最多等待 2 秒）
	cols, rows := 120, 40 // 较大的默认值
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	var initMsg WSMessage
	if err := ws.ReadJSON(&initMsg); err == nil && initMsg.Type == "init" {
		if initMsg.Cols > 0 {
			cols = initMsg.Cols
		}
		if initMsg.Rows > 0 {
			rows = initMsg.Rows
		}
		tm.logger.Info("Received initial terminal size", "cols", cols, "rows", rows)
	}
	ws.SetReadDeadline(time.Time{}) // 清除超时

	// 启动 PTY 会话
	ptyInfo, err := tm.sshManager.StartPTYSession(clientID, cols, rows)
	if err != nil {
		tm.logger.Error("Failed to start PTY session", "client_id", clientID, "error", err)
		ws.WriteJSON(WSMessage{Type: "error", Data: err.Error()})
		return
	}

	// 获取会话资源
	stdout, err := tm.sshManager.GetPTYSessionReader(ptyInfo.ID)
	if err != nil {
		tm.logger.Error("Failed to get PTY reader", "session_id", ptyInfo.ID, "error", err)
		ws.WriteJSON(WSMessage{Type: "error", Data: err.Error()})
		tm.sshManager.ClosePTYSession(ptyInfo.ID)
		return
	}

	stdin, err := tm.sshManager.GetPTYSessionWriter(ptyInfo.ID)
	if err != nil {
		tm.logger.Error("Failed to get PTY writer", "session_id", ptyInfo.ID, "error", err)
		ws.WriteJSON(WSMessage{Type: "error", Data: err.Error()})
		tm.sshManager.ClosePTYSession(ptyInfo.ID)
		return
	}

	done, err := tm.sshManager.GetPTYSessionDone(ptyInfo.ID)
	if err != nil {
		tm.logger.Error("Failed to get PTY done channel", "session_id", ptyInfo.ID, "error", err)
		ws.WriteJSON(WSMessage{Type: "error", Data: err.Error()})
		tm.sshManager.ClosePTYSession(ptyInfo.ID)
		return
	}

	// 发送连接成功消息
	ws.WriteJSON(WSMessage{Type: "connected", Data: ptyInfo.ID})

	// 创建审计器和录制器
	var auditor *audit.SessionAuditor
	var recorder *recording.Recorder

	if tm.auditStore != nil {
		auditor = audit.NewSessionAuditor(tm.auditStore, ptyInfo.ID, clientID, "admin", c.ClientIP())
		tm.logger.Info("Audit enabled for session", "session_id", ptyInfo.ID)
	}

	if tm.recordingConfig != nil && tm.recordingConfig.Enabled {
		var err error
		recorder, err = recording.NewRecorderWithDB(tm.recordingConfig, ptyInfo.ID, clientID, "admin", cols, rows, tm.recordingDBStore)
		if err != nil {
			tm.logger.Error("Failed to create recorder", "error", err)
		} else {
			tm.logger.Info("Recording enabled for session", "session_id", ptyInfo.ID, "recording_id", recorder.GetID())
		}
	}

	// 创建终端会话
	termSession := &TerminalSession{
		ID:           ptyInfo.ID,
		ClientID:     clientID,
		PTYSessionID: ptyInfo.ID,
		WebSocket:    ws,
		CreatedAt:    time.Now(),
		done:         make(chan struct{}),
		auditor:      auditor,
		recorder:     recorder,
	}

	tm.mu.Lock()
	tm.sessions[ptyInfo.ID] = termSession
	tm.mu.Unlock()

	defer func() {
		tm.mu.Lock()
		delete(tm.sessions, ptyInfo.ID)
		tm.mu.Unlock()
		close(termSession.done)

		// 关闭审计器和录制器
		if termSession.auditor != nil {
			termSession.auditor.Close()
			tm.logger.Info("Audit session closed", "session_id", ptyInfo.ID)
		}
		if termSession.recorder != nil {
			termSession.recorder.Close()
			tm.logger.Info("Recording saved", "session_id", ptyInfo.ID, "recording_id", termSession.recorder.GetID(), "file", termSession.recorder.GetFilePath())
		}

		tm.sshManager.ClosePTYSession(ptyInfo.ID)
		tm.logger.Info("WebSocket terminal disconnected", "client_id", clientID, "session_id", ptyInfo.ID)
	}()

	// 启动输出读取协程
	outputDone := make(chan struct{})
	go func() {
		defer close(outputDone)
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				if err != io.EOF {
					tm.logger.Debug("PTY read error", "error", err)
				}
				return
			}
			if n > 0 {
				// 录制输出
				if termSession.recorder != nil {
					termSession.recorder.RecordOutput(buf[:n])
				}

				msg := WSMessage{Type: "output", Data: string(buf[:n])}
				if err := ws.WriteJSON(msg); err != nil {
					tm.logger.Debug("WebSocket write error", "error", err)
					return
				}
			}
		}
	}()

	// 启动 WebSocket 读取协程
	inputChan := make(chan WSMessage, 10)
	inputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		for {
			var msg WSMessage
			err := ws.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					tm.logger.Debug("WebSocket read error", "error", err)
				}
				return
			}
			select {
			case inputChan <- msg:
			case <-done:
				return
			}
		}
	}()

	// 主循环：处理消息
	for {
		select {
		case <-done:
			// PTY 会话结束
			ws.WriteJSON(WSMessage{Type: "disconnected", Data: "Session ended"})
			return
		case <-outputDone:
			// 输出结束
			ws.WriteJSON(WSMessage{Type: "disconnected", Data: "Connection closed"})
			return
		case <-inputDone:
			// WebSocket 输入结束
			return
		case msg := <-inputChan:
			switch msg.Type {
			case "input":
				inputData := []byte(msg.Data)

				// 录制输入
				if termSession.recorder != nil {
					termSession.recorder.RecordInput(inputData)
				}

				// 审计输入（用于命令检测）
				if termSession.auditor != nil {
					termSession.auditor.RecordInput(inputData)
				}

				// 写入终端输入
				if _, err := stdin.Write(inputData); err != nil {
					tm.logger.Debug("PTY write error", "error", err)
					return
				}
			case "resize":
				// 调整终端大小
				if msg.Cols > 0 && msg.Rows > 0 {
					// 录制窗口变化
					if termSession.recorder != nil {
						termSession.recorder.RecordResize(msg.Cols, msg.Rows)
					}

					if err := tm.sshManager.ResizePTY(ptyInfo.ID, msg.Cols, msg.Rows); err != nil {
						tm.logger.Debug("PTY resize error", "error", err)
					}
				}
			case "ping":
				ws.WriteJSON(WSMessage{Type: "pong"})
			}
		}
	}
}

// ListSessions 列出所有终端会话
func (tm *TerminalManager) ListSessions() []PTYSessionInfo {
	return tm.sshManager.ListPTYSessions()
}

// CloseSession 关闭终端会话
func (tm *TerminalManager) CloseSession(sessionID string) error {
	tm.mu.Lock()
	session, exists := tm.sessions[sessionID]
	tm.mu.Unlock()

	if exists && session.WebSocket != nil {
		session.WebSocket.WriteJSON(WSMessage{Type: "disconnected", Data: "Session closed by server"})
		session.WebSocket.Close()
	}

	return tm.sshManager.ClosePTYSession(sessionID)
}

// TerminalSessionInfo 终端会话信息（用于 API 返回）
type TerminalSessionInfo struct {
	ID        string    `json:"id"`
	ClientID  string    `json:"client_id"`
	CreatedAt time.Time `json:"created_at"`
}

// GetActiveSessions 获取活跃的 WebSocket 会话
func (tm *TerminalManager) GetActiveSessions() []TerminalSessionInfo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	sessions := make([]TerminalSessionInfo, 0, len(tm.sessions))
	for _, session := range tm.sessions {
		sessions = append(sessions, TerminalSessionInfo{
			ID:        session.ID,
			ClientID:  session.ClientID,
			CreatedAt: session.CreatedAt,
		})
	}
	return sessions
}

// HandleTerminalSessionsList 处理列出终端会话的 HTTP 请求
func (tm *TerminalManager) HandleTerminalSessionsList(c *gin.Context) {
	sessions := tm.ListSessions()
	common.SuccessResp(c, gin.H{
		"sessions": sessions,
	})
}

// HandleTerminalSessionClose 处理关闭终端会话的 HTTP 请求
func (tm *TerminalManager) HandleTerminalSessionClose(c *gin.Context) {
	sessionID := c.Param("session_id")
	if sessionID == "" {
		common.ErrorResp(c, common.CodeInvalidParams, "session_id is required")
		return
	}

	if err := tm.CloseSession(sessionID); err != nil {
		common.ErrorResp(c, common.CodeSSHTerminalNotFound, err.Error())
		return
	}

	common.SuccessResp(c, struct{}{})
}

// TerminalManagerAdapter 适配器：将 SSHClientManager 适配到 TerminalManagerAPI 接口
type TerminalManagerAdapter struct {
	manager interface {
		StartPTYSession(clientID string, cols, rows int) (interface{}, error)
		ResizePTY(sessionID string, cols, rows int) error
		ClosePTYSession(sessionID string) error
		GetPTYSession(sessionID string) (interface{}, error)
		ListPTYSessions() interface{}
	}
}

// NewTerminalManagerAdapter 创建适配器
func NewTerminalManagerAdapter(manager interface{}) *TerminalManagerAdapter {
	return &TerminalManagerAdapter{
		manager: manager.(interface {
			StartPTYSession(clientID string, cols, rows int) (interface{}, error)
			ResizePTY(sessionID string, cols, rows int) error
			ClosePTYSession(sessionID string) error
			GetPTYSession(sessionID string) (interface{}, error)
			ListPTYSessions() interface{}
		}),
	}
}

// 实现接口的辅助函数
func unmarshalPTYSession(v interface{}) (id string, clientID string, stdin io.WriteCloser, stdout io.Reader, done <-chan struct{}, cols int, rows int, createdAt time.Time) {
	// 使用反射或 JSON 序列化来提取字段
	data, _ := json.Marshal(v)
	var info struct {
		ID        string    `json:"ID"`
		ClientID  string    `json:"ClientID"`
		Cols      int       `json:"Cols"`
		Rows      int       `json:"Rows"`
		CreatedAt time.Time `json:"CreatedAt"`
	}
	json.Unmarshal(data, &info)
	return info.ID, info.ClientID, nil, nil, nil, info.Cols, info.Rows, info.CreatedAt
}
