package dao

import (
	"fmt"

	mxm "github.com/Daneel-Li/gps-back/internal/models"
)

func (d *MysqlRepository) AddAlarm(alarm mxm.Alarm) error {

	if err := d.db.Table("alarms").Create(&alarm).Error; err != nil {
		return fmt.Errorf("insert into alarms error, %v", err)
	}
	return nil
}

func (d *MysqlRepository) GetAlarmsByDeviceID(deviceID string, limit, offset int) ([]*mxm.Alarm, error) {
	var lst []*mxm.Alarm
	query := d.db.Where("device_id=?", deviceID)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&lst).Error; err != nil {
		return nil, fmt.Errorf("query alarms by device_id(%s) error, %v", deviceID, err)
	}
	return lst, nil
}

// UpdateAlarmStatus 更新报警状态
func (d *MysqlRepository) UpdateAlarmStatus(alarmID uint, status string) error {
	return d.db.Model(&mxm.Alarm{}).Where("id = ?", alarmID).Update("status", status).Error
}
