package mxm

import (
	"time"
)

type Action string

type RecoveryCmd struct {
	ID       int           `gorm:"column:id"`
	DeviceID string        `json:"deviceId"`
	Device   Device        `json:"device"`
	Tm       time.Time     `json:"tm"`
	Action   Action 		`json:"action"`
	Args     string        `json:"args"`
}

func (RecoveryCmd) Table() string {
	return "recovery_cmds"
}
