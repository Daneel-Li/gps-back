package vendors

import (
	mxm "github.com/Daneel-Li/gps-back/internal/models"
)

type MessageHandler interface {
	Process(status *mxm.DeviceStatus1) error
}

// 定义所有厂商必须实现的强制接口
type VendorDriver interface {
	// 必须调用统一处理器（通过依赖注入）
	SetMessageHandler(handler MessageHandler)

	// 厂商自己的启动逻辑
	Start() error
	Activate(originSN string) error
	Deactivate(originSN string) error

	SetReportInterval(CommandID int64, originSN string, interval int) error //设置间隔
	Locate(CommandID int64, originSN string) error                          //立即定位
	Reboot(CommandID int64, originSN string) error                          // 远程重启
	PowerOff(CommandID int64, originSN string) error
	Find(CommandID int64, originSN string) error // 查找设备（寻宠）
}

type AdvancedDriver interface {
	AutoStart(CommandID int64, originSN string, tm string, enable bool) error //定时开机
	AutoShut(CommandID int64, originSN string, tm string, enable bool) error  //定时关机
}
