package db

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/taosdata/driver-go/v3/common"
	_ "github.com/taosdata/driver-go/v3/taosWS"
	"github.com/taosdata/driver-go/v3/ws/stmt"
)

type TaosConfig struct {
	Username string
	Password string
	Protocol string
	Address  string
	DBName   string
	Param    string
}

// NewDB 返回一个新的 *Taos 实例
// NewDB 返回一个新的 *Taos 实例
// 封装taos的db连接器和stmt生成器(没办法，tdengine的驱动设计不遵循sql.DB)
func NewTaos(config TaosConfig) (*TaosDB, error) {

	var taosUri = fmt.Sprintf("%s:%s@%s(%s)/%s", config.Username, config.Password, config.Protocol, config.Address, config.DBName)
	if len(config.Param) > 0 {
		taosUri = fmt.Sprintf("%s?params=%s", taosUri, config.Param)
	}
	slog.Debug(taosUri)
	db, err := sql.Open("taosWS", taosUri)
	if err != nil {
		return nil, fmt.Errorf("failed to connect TDengine: %v", err)
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}

	return &TaosDB{config, db}, nil
}

type TaosDB struct {
	Cfg TaosConfig
	*sql.DB
}

func (t *TaosDB) NewStmt() (*stmt.Stmt, error) {
	config := stmt.NewConfig(fmt.Sprintf("ws://%s", t.Cfg.Address), 0)
	config.SetConnectUser(t.Cfg.Username)
	config.SetConnectPass(t.Cfg.Password)
	config.SetConnectDB(t.Cfg.DBName)
	config.SetMessageTimeout(common.DefaultMessageTimeout)
	config.SetWriteWait(common.DefaultWriteWait)
	conn, err := stmt.NewConnector(config)
	if err != nil {
		return nil, fmt.Errorf("NewConnector failed:%v", err.Error())
	}
	stmt, err := conn.Init()
	if err != nil {
		return nil, fmt.Errorf("conn init failed:%v", err.Error())
	}
	return stmt, nil
}
