package mxm

// 请求数据结构
type DeviceEnrollRequest struct {
	SerialNumber string `json:"serial_number"` // 设备序列号
	Model        string `json:"model"`         // 设备型号
	Buzzer       bool   `json:"buzzer"`        // 是否有蜂鸣器,可选
	Note         string `json:"note"`          // 备注,可选
}
