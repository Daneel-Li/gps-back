package mxm

import (
	"encoding/json"
	"fmt"
	"math"
)

type Settings struct {
	DeviceId       string    `json:"deviceId"`
	ReportInterval int       `json:"reportInterval"`
	SafeRegions    []*Region `json:"safeRegion"`
	Share          *Share    `json:"share"` //分享/删除分享操作
}

// 自动开关机参数
type AutoPowerParam struct {
	AutoStartAt     string `json:"auto_start_at"`
	AutoShutAt      string `json:"auto_shut_at"`
	AutoStartEnable bool   `json:"auto_start_enable"`
	AutoShutEnable  bool   `json:"auto_shut_enable"`
}

type Share struct {
	Add    bool `json:"add"` //true为新增，false为删除
	UserId int  `json:"userId"`
}

type Region struct {
	Type string `json:"type"` //类型，circle，rectangle,square等
	Name string `json:"name"` //区域名
	Area Area   `json:"area"` //区域描述
}

type Area interface {
	IsOut(pt Point) bool
}

// RegionUnmarshaler 是一个辅助结构体，用于反序列化
type RegionUnmarshaler struct {
	Type string          `json:"type"`
	Name string          `json:"name"`
	Area json.RawMessage `json:"area"`
}

func (r *Region) UnmarshalJSON(data []byte) error {
	u := RegionUnmarshaler{}
	if err := json.Unmarshal(data, &u); err != nil {
		return err
	}
	r.Type = u.Type
	r.Name = u.Name
	switch u.Type {
	case "circle":
		var c Circle
		if err := json.Unmarshal(u.Area, &c); err != nil {
			return err
		}
		r.Area = &c
	case "rectangle":
		var rct Rectangle
		if err := json.Unmarshal(u.Area, &rct); err != nil {
			return err
		}
		r.Area = &rct
	// Add more cases for other types
	default:
		return fmt.Errorf("unknown type: %s", u.Type)
	}

	return nil
}

type Point struct {
	Latitude  float64
	Longitude float64
}

type Circle struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Radius    float64 `json:"radius"`
}

func (c *Circle) IsOut(pt Point) bool {
	return haversine(pt.Latitude, pt.Longitude, c.Latitude, c.Longitude) >= c.Radius
}

type Rectangle struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func (r Rectangle) IsOut(pt Point) bool {
	return true
	//return pt.X < 0 || pt.X > r.Width || pt.Y < 0 || pt.Y > r.Height
}

const (
	RadiusEarthKm = 6371 // 地球半径，单位公里
)

func haversine(lat1, lon1, lat2, lon2 float64) float64 {

	var dlat = (lat2 - lat1) * math.Pi / 180
	var dlon = (lon2 - lon1) * math.Pi / 180
	lat1 = (lat1) * math.Pi / 180
	lat2 = (lat2) * math.Pi / 180

	a := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1)*math.Cos(lat2)*
			math.Sin(dlon/2)*math.Sin(dlon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	distance := RadiusEarthKm * c
	return distance
}
