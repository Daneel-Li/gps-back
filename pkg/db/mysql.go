package db

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

type MysqlConfig struct {
	Username string
	Password string
	Host     string
	Port     string
	DBName   string
}

// NewDB 返回一个新的 *DB 实例
// NewDB 返回一个新的 *DB 实例
func NewDB(config MysqlConfig) (*DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=UTC", config.Username, config.Password, config.Host, config.Port, config.DBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	return &DB{db}, nil
}
