package mxm

import "time"

type HisData struct {
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	RawData   []byte    `gorm:"column:raw_data" json:"raw_data"`
}
