package mxm

import (
	"encoding/json"
	"time"
)

const (
	LOW_BATERY = 1
	POWER_OFF  = 2
	OUT_AREA   = 3
)

type Alarm struct {
	ID       int       `json:"id" gorm:"column:id"`
	Time     time.Time `json:"time" gorm:"column:time"`
	DeviceID string    `json:"deviceId" gorm:"column:device_id"`
	Type     int       `json:"type" gorm:"column:type"`
	Msg      string    `json:"msg" gorm:"column:msg"`
}

func (Alarm) TableName() string {
	return "alarms"
}

// 为 MyStruct 类型自定义 MarshalJSON 方法
func (m Alarm) MarshalJSON() ([]byte, error) {
	// 将时间转换为本地时区的字符串

	localTime := m.Time.Local().Format("2006-01-02 15:04:05")

	// 创建一个临时的结构体，用于序列化
	tempStruct := struct {
		ID       int    `json:"id" gorm:"column:id"`
		Time     string `json:"time" gorm:"column:time"`
		DeviceID string `json:"deviceId" gorm:"column:device_id"`
		Type     int    `json:"type" gorm:"column:type"`
		Msg      string `json:"msg" gorm:"column:msg"`
	}{
		ID:       m.ID,
		Time:     localTime,
		DeviceID: m.DeviceID,
		Type:     m.Type,
		Msg:      m.Msg,
	}

	// 序列化临时结构体
	return json.Marshal(tempStruct)
}
