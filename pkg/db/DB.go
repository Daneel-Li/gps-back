package db

import (
	"database/sql"
)

// DB 是一个封装了 *sql.DB 对象的结构体
type DB struct {
	*sql.DB
}

// Close 关闭数据库连接
func (db *DB) Close() {
	db.DB.Close()
}
