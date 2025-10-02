package dao

import (
	"time"

	mxm "github.com/Daneel-Li/gps-back/internal/models"
)

// DeviceRepository 设备相关数据访问接口
type DeviceRepository interface {
	// 设备基础操作
	GetDeviceByID(id string) (*mxm.Device, error)
	GetDeviceByOriginSN(originSN, deviceType string) (*mxm.Device, error)
	GetDeviceIDByOriginSN(originSN, deviceType string) (string, error)
	GetDevicesByUserID(userID int) ([]*mxm.Device, error)
	GetDevicesByType(deviceType string) ([]*mxm.Device, error)

	// 设备绑定操作
	BindDevice(deviceID, label string, userID uint) error
	UnBindDevice(userID uint, deviceID string) error
	CreateDevice(deviceID, originSN, deviceType string, buzzer bool, note string) error

	// 设备更新操作
	UpdateDevice(deviceID string, updates map[string]interface{}) (*mxm.Device, error)
	UpdateDeviceAvatar(deviceID, avatar string) error
	GetDeviceAvatar(deviceID string) (string, error)
	GetDeviceProfileByID(deviceID string) (*mxm.Profile, error)

	// 设备历史数据
	AddHisData(deviceID string, raw []byte) error
	AddPosHis(deviceID string, loc *mxm.Location) error
	GetPosHis(deviceID, startTime, endTime string, types []string) ([]*mxm.Location, error)

	// 设备相关表管理
	CreateDeviceTables(deviceID string) error
	DropDeviceTables(deviceID string) error
}

// UserRepository 用户相关数据访问接口
type UserRepository interface {
	GetUserByID(userID uint) (*mxm.User, error)
	GetUserById(userID uint) (*mxm.User, error)
	GetUserByOpenID(openID string) (*mxm.User, error)
	CreateUser(user *mxm.User) error
	GetOrCreateUserByOpenId(openid string) (*mxm.User, error)
	UpdateUser(userID uint, updates map[string]interface{}) error
	GetUserAvatar(userID string) (string, error)
	UpdateUserAvatar(userID string, avatar string) error
}

// ShareRepository 设备分享相关数据访问接口
type ShareRepository interface {
	GetShareMappingByUserID(userID int) ([]*mxm.ShareMap, error)
	GetShareMappingByDeviceID(deviceID string) ([]*mxm.ShareMap, error)
	GetSharedUserIdsByDeviceId(deviceID string) ([]uint, error)
	GetUserIdByDeviceId(deviceID string) (uint, error)
	CreateShareMapping(mapping *mxm.ShareMap) error
	AddShare(o *mxm.ShareMap) error
	MoveShare(o *mxm.ShareMap) error
	DeleteShareMapping(userID uint, deviceID string) error
}

// AlarmRepository 报警相关数据访问接口
type AlarmRepository interface {
	GetAlarmsByDeviceID(deviceID string, limit, offset int) ([]*mxm.Alarm, error)
	AddAlarm(alarm mxm.Alarm) error
	UpdateAlarmStatus(alarmID uint, status string) error
}

// SafeRegionRepository 安全区域相关数据访问接口
type SafeRegionRepository interface {
	GetSafeRegions(deviceID string) ([]*mxm.Region, error)
	SetSafeRegion(deviceID string, region *mxm.Region) error
	CreateSafeRegion(region interface{}) error
	UpdateSafeRegion(regionID uint, updates map[string]interface{}) error
	DeleteSafeRegion(regionID uint) error
}

// StepsRepository 步数统计相关数据访问接口
type StepsRepository interface {
	AddSteps(deviceID string, steps int) error
	GetStepsByDeviceID(deviceID string, startTime, endTime time.Time) ([]*mxm.Steps, error)
	GetSteps(deviceId string, st time.Time, ed time.Time) ([]*mxm.Steps, error)
}

// OrderRepository 订单相关数据访问接口
type OrderRepository interface {
	CreateOrder(order *mxm.Order) error
	GetOrderByID(orderID string) (*mxm.Order, error)
	UpdateOrderStatus(orderID string, status string) error
	GetOrdersByUserID(userID uint) ([]*mxm.Order, error)
}

// FeedbackRepository 反馈相关数据访问接口
type FeedbackRepository interface {
	CreateFeedback(feedback *mxm.Feedback) error
	AddFeedback(feedback *mxm.Feedback) error
	GetFeedbacks(limit, offset int) ([]*mxm.Feedback, error)
	GetFeedbacksByUserID(userID uint, limit, offset int) ([]*mxm.Feedback, error)
	GetFeedbacksByUserId(userId uint, limit, offset int) ([]*mxm.Feedback, error)
}

// SettingsRepository 设备设置相关数据访问接口
type SettingsRepository interface {
	GetSettingsByDeviceID(deviceID string) (*mxm.Settings, error)
	UpsertSettingsFields(fields map[string]interface{}) error
	UpdateSettings(deviceID string, updates map[string]interface{}) error

	GetAutoPowerParams(id string) (*mxm.AutoPowerParam, error)
}

// Repository 统一的数据访问接口
type Repository interface {
	DeviceRepository
	UserRepository
	ShareRepository
	AlarmRepository
	SafeRegionRepository
	StepsRepository
	OrderRepository
	FeedbackRepository
	SettingsRepository
}
