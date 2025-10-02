package services

import (
	"context"
	"sync"
	"time"

	mxm "github.com/Daneel-Li/gps-back/internal/models"
)

/*
*
缓存未响应指令
*/

type pendingCommand struct {
	Terminal *TerminalKey
	Command  *mxm.Command
	ExpireAt time.Time
}

// 2. 全局变量
type commandsWaiting struct {
	sync.Mutex
	data map[int64]*pendingCommand
}

type CommandManager interface {
	AddCommand(requestID int64, terminal TerminalKey, cmd *mxm.Command)
	GetAndRemoveCommand(requestID int64) (*pendingCommand, bool)
}

func NewCommandManager() CommandManager {
	m := commandsWaiting{
		data: make(map[int64]*pendingCommand),
	}
	m.start(context.Background())
	return &m
}

// 3. 添加命令
func (m *commandsWaiting) AddCommand(requestID int64, terminal TerminalKey, cmd *mxm.Command) {
	m.Lock()
	defer m.Unlock()
	m.data[requestID] = &pendingCommand{
		Terminal: &terminal,
		Command:  cmd,
		ExpireAt: time.Now().Add(30 * time.Second),
	}
}

// 4. 获取并删除命令（如果未超时）
func (m *commandsWaiting) GetAndRemoveCommand(requestID int64) (*pendingCommand, bool) {
	m.Lock()
	defer m.Unlock()
	pCmd, loaded := m.data[requestID]

	if !loaded {
		return nil, false
	}
	delete(m.data, requestID)
	if time.Now().After(pCmd.ExpireAt) {
		return nil, false
	}
	return pCmd, true
}

func (m *commandsWaiting) start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.cleanExpiredCommands()
			}
		}
	}()
}

func (m *commandsWaiting) cleanExpiredCommands() {
	now := time.Now()
	toDel := []int64{}
	for k, cmd := range m.data {
		if now.After(cmd.ExpireAt) {
			toDel = append(toDel, k)
		}
		if len(toDel) > 100 { //避免长时间占用锁，小量多次处理
			break
		}
	}
	for _, id := range toDel {
		delete(m.data, id)
	}
}
