package mxm

/*
*
分享映射表，用户和设备之间多对多关系
*/
type ShareMap struct {
	UserID   int    `gorm:"foreignkey:UserId" json:"userId"` //对应user表的id字段
	User     User   `json:"user"`
	DeviceID string `gorm:"column:device_id" json:"deviceId"`
	Device   Device `json:"device"`
}

func (ShareMap) TableName() string {
	return "share_mapping"
}
