package mxm

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// 设备总表，存储统一设备id，型号，原始id等信息
type Device struct {
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at"`
	ID          *string        `gorm:"primaryKey;type:char(16);column:id" json:"id"`
	OriginSN    *string        `gorm:"type:varchar(16);not null;column:originSN" json:"originSN"`
	Type        *string        `gorm:"type:varchar(12);not null;column:type" json:"type"`
	Enable      *bool          `gorm:"not null;default:false" json:"enable"`
	UserID      *uint          `gorm:"type:int;column:user_id" json:"user_id"` // 使用 sql.NullInt64 处理可能为 NULL 的情况
	BindTime    *time.Time     `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;column:bind_time" json:"bind_time"`
	Label       *string        `gorm:"type:varchar(26)" json:"label"`
	LastOnline  *time.Time     `gorm:"type:timestamp;column:last_online" json:"last_online"` // 使用 sql.NullTime 处理可能为 NULL 的情况
	Status      *string        `gorm:"type:varchar(16);not null;default:'0'" json:"status"`
	Profile     datatypes.JSON `gorm:"column:profile;type:json;not null" json:"profile"`
	Electricity *int           `gorm:"not null;default:0" json:"electricity"`
	Charging    *bool          `gorm:"type:BOOLEAN; column:charging" json:"charging"`
	//Location      *Location      `gorm:"type:json;not null" json:"location"`
	Interval      *int       `gorm:"not null;default:10;column:interval" json:"interval"`
	SimCardSignal *int       `gorm:"not null;default:0;column:sim_card_signal" json:"sim_card_signal"`
	Steps         *int       `gorm:"not null;default:0;column:steps" json:"steps"`
	Address       *string    `json:"address"`                         //地址描述
	Longitude     *float64   `json:"longitude"`                       //经度
	Latitude      *float64   `json:"latitude"`                        // 纬度
	Altitude      *float64   `json:"altitude"`                        //海拔
	Satellites    *int       `json:"satellites"`                      //卫星个数
	LocType       *string    `json:"loc_type"`                        //类型：GPS/WIFI/LBS TODO 限定类型
	LocTime       *time.Time `gorm:"column:loc_time" json:"loc_time"` //时间
	Accuracy      *float64   `json:"accuracy"`                        //精度
	Speed         *float64   `json:"speed"`                           //速度
	Heading       *float64   `json:"heading"`                         //方向
	Age           *int       `json:"age"`
	AvatarURL     *string    `json:"avatar_url"`
	Description   *string    `json:"description"`
	PhoneNumber   *string    `json:"phone_number"`
	Sex           *string    `json:"sex"`
	Weight        *int       `json:"weight"`
	Buzzer        *bool      `json:"buzzer"`
	// 关联关系（可选）
	User *User `gorm:"foreignKey:UserID;references:ID" json:"user"`
}

type Profile struct {
	Species     int    `json:"species"` //种类 0-other,1-猫，2-狗，3-老人/孩子
	Age         int    `json:"age"`
	AvatarURL   string `json:"avatar_url"`
	Description string `json:"description"`
	Label       string `json:"label"` //nick name
	PhoneNumber string `json:"phone_number"`
	Sex         string `json:"sex"`
	Weight      int    `json:"weight"`
}

// 设置表名（可选）
func (Device) TableName() string {
	return "devices"
}

type CommandResult struct {
	CommandID int64 `json:"command_id"`
	Succeed   bool  `json:"succeed"`
	Msg       string
	Extra     []byte `json:"extra"`
}

type Command struct { //指令结构体
	Action string         `json:"action"`
	Args   []string       `json:"args"`
	Result *CommandResult `json:"result"`
}

// 所有与设备相关的msg都看作状态，包括设备命令、设备状态、设备报警等
type DeviceStatus1 struct {
	OriginSN string   //这是接近原始数据的状态封装，所以使用原厂序列号和类型定位记录
	Type     string   // 机型（厂商）
	Device   *Device  `gorm:"foreignKey:ID;references:ID" json:"device"`
	Command  *Command `gorm:"foreignKey:ID;references:ID" json:"command"`
	RawMsg   []byte   `gorm:"foreignKey:ID;references:ID" json:"raw_msg"`
}
