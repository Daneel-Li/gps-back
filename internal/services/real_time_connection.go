package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type WSMessage struct {
	Type string      `json:"type"` // auth/command_result/device_update
	Data interface{} `json:"data"`
}

type WSManager struct {
	// 一个连接对应一个用户终端
	connections     map[TerminalKey]*websocket.Conn
	userIndex       map[uint][]TerminalKey
	cleanupInterval time.Duration
	sync.RWMutex
}

func NewWsManager(cleanupInterval time.Duration) *WSManager {
	m := &WSManager{
		cleanupInterval: cleanupInterval,
		connections:     make(map[TerminalKey]*websocket.Conn),
		userIndex:       make(map[uint][]TerminalKey),
	}
	m.start()
	return m
}

type TerminalKey struct {
	UserID uint
	Random string //随机短串
}

func (t *TerminalKey) FromString(s string) {
	fmt.Sscanf(s, "%d:%s", &t.UserID, &t.Random)
}

func (t TerminalKey) ToString() string {
	return fmt.Sprintf("%d:%s", t.UserID, t.Random)
}

func (m *WSManager) GetConnection(key TerminalKey) *websocket.Conn {
	m.RLock()
	defer m.RUnlock()
	return m.connections[key]
}

func (m *WSManager) SetConnection(key TerminalKey, conn *websocket.Conn) {
	m.Lock()
	defer m.Unlock()
	m.connections[key] = conn
	m.userIndex[key.UserID] = append(m.userIndex[key.UserID], key)
}

// 按UserID批量获取连接（O(1)复杂度）
func (m *WSManager) GetConnectionsByUser(userID uint) []*websocket.Conn {
	m.RLock()
	defer m.RUnlock()

	var conns []*websocket.Conn
	if keys, ok := m.userIndex[userID]; ok {
		for _, key := range keys {
			if conn, exists := m.connections[key]; exists {
				conns = append(conns, conn)
			}
		}
	}
	return conns
}

// 删除连接（同步清理索引）
func (m *WSManager) RemoveConnection(key TerminalKey) {
	m.Lock()
	defer m.Unlock()

	delete(m.connections, key)

	// 从userIndex中移除对应的key
	if keys, ok := m.userIndex[key.UserID]; ok {
		newKeys := make([]TerminalKey, 0, len(keys)-1)
		for _, k := range keys {
			if k != key {
				newKeys = append(newKeys, k)
			}
		}
		m.userIndex[key.UserID] = newKeys
	}
}

func generateTerminalKey(userID uint) TerminalKey {
	return TerminalKey{
		userID,
		uuid.New().String()[:8], // 短随机会话ID
	}
}

func (m *WSManager) AuthenticateAndRegister(conn *websocket.Conn) {
	//首帧鉴权
	defer func() {
		if err := recover(); err != nil {
			conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Server Error"),
				time.Now().Add(5*time.Second),
			)
		}
	}()

	// 首帧鉴权超时（5秒）
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// 读取首帧消息
	_, msg, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return
	}

	// 解析鉴权消息
	var auth struct {
		Token string       `json:"token"`
		WsKey *TerminalKey `json:"ws_key"` //用于ws注册绑定的key，如果已经有了就用原来的
	}

	if json.Unmarshal(msg, &auth) != nil {
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Invalid auth msg"))
		conn.Close()
		return
	}
	//TODO 鉴权有多处，需要统一
	userID, err := NewJWTService().ValidateToken(auth.Token)
	if err != nil {
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Invalid Token"))
		conn.Close()
		return
	}

	// 注册
	key := TerminalKey{}
	if auth.WsKey != nil && auth.WsKey.UserID == userID {
		key = *auth.WsKey
	} else {
		key = generateTerminalKey(userID)
	}
	m.SetConnection(key, conn)
	if err := m.PushMsg(key, WSMessage{
		Type: "auth",
		Data: map[string]interface{}{"terminal_key": key.ToString()}}); err != nil {
		slog.Error("push auth msg failed: %v", "error", err)
	}

	go m.handleConnection(key, conn)
}

func (m *WSManager) handleConnection(key TerminalKey, conn *websocket.Conn) {
	defer func() {
		m.Lock()
		defer m.Unlock()
		delete(m.connections, key)
		conn.Close()
	}()

	// 设置读写超时（重要！）
	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	count := 0 //test code
	for {
		// 消息处理循环
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				slog.Warn(fmt.Sprintf("终端 %v 异常断开: %v", key, err))
			}
			break
		}
		count += 1
		slog.Debug(fmt.Sprintf("%v 收到终端 %v key:%v 消息: %s, 类型:%v", count, fmt.Sprintf("%p", conn), key, msg, msgType))

		// TODO 业务处理（示例）
		m := WSMessage{}
		if err := json.Unmarshal(msg, &m); err != nil {
			slog.Error(fmt.Sprintf("解析终端 %v 消息失败: %v", key, err))
			continue
		}
		if m.Type == "ping" {
			if err := writeWsJson(conn, WSMessage{Type: "pong"}); err != nil {
				slog.Error(fmt.Sprintf("终端 %v 回送 pong 失败: %v", key, err))
			}
		}
	}
}

func (m *WSManager) PushMsg(key TerminalKey, v interface{}) error {
	conn := m.GetConnection(key)
	if conn == nil {
		return fmt.Errorf("no connection found for key: %s", key.ToString())
	}
	return writeWsJson(conn, v)
}

func writeWsJson(conn *websocket.Conn, v interface{}) error {
	slog.Debug("WriteJSON", "remote_addr", conn.RemoteAddr().String(), "content", v)
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(v)
}

func (m *WSManager) BroadcastToUsers(userIDs []uint, v interface{}) error {
	flag := false
	for _, u := range userIDs {
		if err := m.BroadcastToUser(u, v); err != nil {
			slog.Error("BroadcastToUser 部分发送失败", "user", u, "error", err)
			flag = true
		}
	}
	if flag {
		return fmt.Errorf("BroadcastToUsers 部分发送失败: %v", userIDs)
	}
	return nil
}

// 安全地向所有用户连接发送消息
func (m *WSManager) BroadcastToUser(userID uint, v interface{}) error {
	conns := m.GetConnectionsByUser(userID)
	var errs []error

	for _, conn := range conns {
		if err := writeWsJson(conn, v); err != nil {
			slog.Error("WriteJSON 失败", "user", userID, "error", err, "conn_ptr", fmt.Sprintf("%p", conn))
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("部分发送失败: %v", errs)
	}
	return nil
}

func (m *WSManager) start() {

	ctx := context.Background()

	go func() {
		ticker := time.NewTicker(m.cleanupInterval)
		for {
			select {
			case <-ticker.C:
				m.cleanupDeadConnections()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (m *WSManager) cleanupDeadConnections() {
	m.Lock()
	defer m.Unlock()

	for key, conn := range m.connections {
		if err := conn.WriteControl(
			websocket.PingMessage,
			nil,
			time.Now().Add(100*time.Millisecond),
		); err != nil {
			// 连接已失效
			m.RemoveConnection(key) // 内部会同步清理索引
		}
	}
}
