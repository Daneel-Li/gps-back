package dao

import (
	"fmt"

	mxm "github.com/Daneel-Li/gps-back/internal/models"
)

// 查询某个用户被分享了哪些设备
func (d *MysqlRepository) GetShareMappingByUserID(userID int) ([]*mxm.ShareMap, error) {
	var lst []*mxm.ShareMap
	query := d.db.Preload("User", "ID=?", userID).Preload("Device")
	if err := query.Where("user_id=?", userID).Find(&lst).Error; err != nil {
		return nil, fmt.Errorf("mysql db query error:%v", err.Error())
	}

	return lst, nil
}

// 查询某个用户被分享了哪些设备
func (d *MysqlRepository) GetShareMappingByDeviceID(deviceID string) ([]*mxm.ShareMap, error) {
	var lst []*mxm.ShareMap
	query := d.db.Preload("User").Preload("Device", "ID=?", deviceID)
	if err := query.Where("device_id=?", deviceID).Find(&lst).Error; err != nil {
		return nil, fmt.Errorf("mysql db query error:%v", err.Error())
	}

	return lst, nil
}

func (d *MysqlRepository) AddShare(o *mxm.ShareMap) error {
	if err := d.db.Table("share_mapping").Create(o).Error; err != nil {
		return fmt.Errorf("insert into share_mapping failed, %v", err)
	}
	return nil
}

func (d *MysqlRepository) MoveShare(o *mxm.ShareMap) error {
	if err := d.db.Table("share_mapping").Where("device_id=? and user_id=?", o.DeviceID, o.UserID).Delete(o).Error; err != nil {
		return fmt.Errorf("delete from share_mapping failed, %v", err)
	}
	return nil
}

// CreateShareMapping 创建分享映射
func (d *MysqlRepository) CreateShareMapping(mapping *mxm.ShareMap) error {
	return d.AddShare(mapping)
}

// DeleteShareMapping 删除分享映射
func (d *MysqlRepository) DeleteShareMapping(userID uint, deviceID string) error {
	if err := d.db.Table("share_mapping").Where("device_id=? and user_id=?", deviceID, userID).Delete(&mxm.ShareMap{}).Error; err != nil {
		return fmt.Errorf("delete share mapping failed: %v", err)
	}
	return nil
}
