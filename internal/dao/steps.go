package dao

import (
	"fmt"
	"time"

	mxm "github.com/Daneel-Li/gps-back/internal/models"
)

func (d *MysqlRepository) AddSteps(deviceId string, steps int) error {
	if err := d.db.Table(fmt.Sprintf("steps_%s", deviceId)).Create(&mxm.Steps{
		Steps: steps,
	}).Error; err != nil {
		return fmt.Errorf("insert steps failed: %v", err)
	}
	return nil
}

func (d *MysqlRepository) GetSteps(deviceId string, st time.Time, ed time.Time) ([]*mxm.Steps, error) {
	var steps []*mxm.Steps
	if err := d.db.Table(fmt.Sprintf("steps_%s", deviceId)).
		Where("created_at BETWEEN ? AND ?", st, ed).Order("created_at ASC").
		Find(&steps).Error; err != nil {
		return nil, fmt.Errorf("select steps failed: %v", err)
	}
	return steps, nil
}

// GetStepsByDeviceID 根据设备ID获取步数
func (d *MysqlRepository) GetStepsByDeviceID(deviceID string, startTime, endTime time.Time) ([]*mxm.Steps, error) {
	return d.GetSteps(deviceID, startTime, endTime)
}
