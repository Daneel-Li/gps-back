package mxm

import "time"

type Steps struct {
	ID        int       `gorm:"column:id;primary_key" json:"id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	Steps     int       `gorm:"column:steps" json:"steps"`
}
