package vendors

import (
	mxm "github.com/Daneel-Li/gps-back/internal/models"
)

type MessageHandler interface {
	Process(status *mxm.DeviceStatus1) error
}

// Define mandatory interface that all vendors must implement
type VendorDriver interface {
	// Must call unified processor (via dependency injection)
	SetMessageHandler(handler MessageHandler)

	// Vendor's own startup logic
	Start() error
	Activate(originSN string) error
	Deactivate(originSN string) error

	SetReportInterval(CommandID int64, originSN string, interval int) error // Set interval
	Locate(CommandID int64, originSN string) error                          // Immediate location
	Reboot(CommandID int64, originSN string) error                          // Remote restart
	PowerOff(CommandID int64, originSN string) error
	Find(CommandID int64, originSN string) error // Find device (pet finder)
}

type AdvancedDriver interface {
	AutoStart(CommandID int64, originSN string, tm string, enable bool) error // Scheduled power on
	AutoShut(CommandID int64, originSN string, tm string, enable bool) error  // Scheduled power off
}
