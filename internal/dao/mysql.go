package dao

import (
	"gorm.io/gorm"
)

const (
	TB_BTT_HB = "tb_btt_hb_" //下划线的常量只能包内用
)

// MysqlRepository MySQL数据库实现
type MysqlRepository struct {
	db *gorm.DB
}

// NewMysqlRepository 创建MySQL数据访问对象
func NewMysqlRepository(db *gorm.DB) Repository {
	return &MysqlRepository{db: db}
}

// 确保MysqlRepository实现了所有接口
var _ Repository = (*MysqlRepository)(nil)
