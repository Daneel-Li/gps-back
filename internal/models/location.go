package mxm

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

/**
位置信息结构体，不管什么机型，原始数据为什么格式，都要整理成统一格式
*/

type Location struct {
	Address    string    `json:"address"`                         //地址描述
	Longitude  float64   `json:"longitude"`                       //经度
	Latitude   float64   `json:"latitude"`                        // 纬度
	Altitude   float64   `json:"altitude"`                        //海拔
	Satellites int       `json:"satellites"`                      //卫星个数
	Type       string    `json:"type"`                            //类型：GPS/WIFI/LBS TODO 限定类型
	LocTime    time.Time `gorm:"column:loc_time" json:"loc_time"` //时间
	Accuracy   float64   `json:"accuracy"`                        //精度
	Speed      float64   `json:"speed"`                           //速度
	Heading    float64   `json:"heading"`                         //方向
}

// 定义WiFi信息结构体
type WiFiInfo struct {
	Mac  string `json:"mac"`
	Rssi int    `json:"rssi"`
}

type WifiList []*WiFiInfo

func (l WifiList) JoinedMacs() string {
	arr := make([]string, len(l))
	for i, wifi := range l {
		arr[i] = wifi.Mac
	}
	return strings.Join(arr, "|")
}

// 实现 GORM 的 Scanner 接口（从数据库读取时调用）
func (loc *Location) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	// 处理二进制数据（如存储为BLOB）
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("invalid location data type")
	}

	// 反序列化二进制到结构体
	return json.Unmarshal(bytes, loc) // 或使用 proto.Unmarshal 等二进制协议
}

// 实现 GORM 的 Valuer 接口（写入数据库时调用）
func (loc Location) Value() (driver.Value, error) {
	// 序列化为二进制（默认用JSON，可替换为ProtoBuf等）
	return json.Marshal(loc)
}
