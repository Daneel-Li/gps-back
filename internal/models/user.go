package mxm

import (
	"time"

	"gorm.io/gorm"
)

// User 结构体代表用户信息
type User struct {
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at"`
	ID          uint           `gorm:"primaryKey" json:"id"`
	OpenID      string         `gorm:"column:openid" json:"openID"` //对应微信openID
	Nickname    string         `gorm:"column:nick_name" json:"nick_name"`
	EnrollAdmin bool           `gorm:"column:enroll_admin" json:"enroll_admin"` //是否为入库管理员
}
