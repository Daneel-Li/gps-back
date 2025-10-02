package dao

import (
	"fmt"

	mxm "github.com/Daneel-Li/gps-back/internal/models"

	"gorm.io/gorm/clause"
)

func (d *MysqlRepository) GetDeviceSettingByDeviceID(deviceID string) (*mxm.Settings, error) {
	var device mxm.Device
	if err := d.db.First(&device, deviceID).Error; err != nil {
		return nil, err
	}
	interval := 0
	if device.Interval != nil {
		interval = *device.Interval
	}
	return &mxm.Settings{
		ReportInterval: interval,
	}, nil
}

// GetSettingsByDeviceID 根据设备ID获取设置
func (d *MysqlRepository) GetSettingsByDeviceID(deviceID string) (*mxm.Settings, error) {
	return d.GetDeviceSettingByDeviceID(deviceID)
}

// UpdateSettings 更新设备设置
func (d *MysqlRepository) UpdateSettings(deviceID string, updates map[string]interface{}) error {
	return d.db.Model(&mxm.Device{}).Where("id = ?", deviceID).Updates(updates).Error
}

// 获取特定类型的设备
func (d *MysqlRepository) GetAutoPowerParams(id string) (*mxm.AutoPowerParam, error) {
	var res mxm.AutoPowerParam
	if err := d.db.Table("device_settings").Where("`device_id`=?", id).First(&res).Error; err != nil {
		return nil, fmt.Errorf("select autopower params failed: %v", err)
	}
	return &res, nil
}

// UpsertFields 动态更新或插入指定字段（不关心其他字段）
func (d *MysqlRepository) UpsertSettingsFields(data map[string]interface{}) error {

	return d.db.Table("device_settings").
		Clauses(clause.OnConflict{
			UpdateAll: false,                    // 禁用更新所有列
			DoUpdates: clause.Assignments(data), // 只更新指定字段
		}).
		Create(&data).Error
}
