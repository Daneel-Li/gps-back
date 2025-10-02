package services

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/Daneel-Li/gps-back/internal/types"
	"github.com/Daneel-Li/gps-back/internal/vendors"
)

// DriverManager 管理所有厂商驱动的生命周期
type DriverManager struct {
	drivers map[types.DeviceType]vendors.VendorDriver
	mu      sync.RWMutex
}

// NewDriverManager 创建新的驱动管理器
func NewDriverManager() *DriverManager {
	return &DriverManager{
		drivers: make(map[types.DeviceType]vendors.VendorDriver),
	}
}

// RegisterDriver 注册厂商驱动
func (dm *DriverManager) RegisterDriver(name string, driver vendors.VendorDriver) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	deviceType := types.DeviceType(name)
	if _, exists := dm.drivers[deviceType]; exists {
		return fmt.Errorf("driver %s already registered", name)
	}
	dm.drivers[deviceType] = driver
	slog.Info("Driver registered", "name", name, "type", deviceType)
	return nil
}

// StartDriver 启动指定的厂商驱动
func (dm *DriverManager) StartDriver(name string) error {
	dm.mu.RLock()
	driver, exists := dm.drivers[types.DeviceType(name)]
	dm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("driver %s not registered", name)
	}

	if err := driver.Start(); err != nil {
		return fmt.Errorf("failed to start driver %s: %w", name, err)
	}

	slog.Info("Driver started", "name", name)
	return nil
}

// StartAllDrivers 启动所有已注册的驱动
func (dm *DriverManager) StartAllDrivers() error {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	for name, driver := range dm.drivers {
		if err := driver.Start(); err != nil {
			return fmt.Errorf("failed to start driver %s: %w", name, err)
		}
		slog.Info("Driver started", "name", name)
	}

	return nil
}

// GetDriver 获取指定类型的厂商驱动
func (dm *DriverManager) GetDriver(deviceType types.DeviceType) (vendors.VendorDriver, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	driver, exists := dm.drivers[deviceType]
	if !exists {
		return nil, fmt.Errorf("driver for type %s not found", deviceType)
	}

	return driver, nil
}

// SetMessageHandler 为所有驱动设置消息处理器
func (dm *DriverManager) SetMessageHandler(handler vendors.MessageHandler) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	for name, driver := range dm.drivers {
		driver.SetMessageHandler(handler)
		slog.Debug("Message handler set for driver", "name", name)
	}
}

// ActivateDevice 激活设备
func (dm *DriverManager) ActivateDevice(deviceType types.DeviceType, originSN string) error {
	driver, err := dm.GetDriver(deviceType)
	if err != nil {
		return err
	}

	return driver.Activate(originSN)
}

// DeactivateDevice 停用设备
func (dm *DriverManager) DeactivateDevice(deviceType types.DeviceType, originSN string) error {
	driver, err := dm.GetDriver(deviceType)
	if err != nil {
		return err
	}

	return driver.Deactivate(originSN)
}

// ListDrivers 列出所有已注册的驱动
func (dm *DriverManager) ListDrivers() []string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	var names []string
	for name := range dm.drivers {
		names = append(names, string(name))
	}

	return names
}
