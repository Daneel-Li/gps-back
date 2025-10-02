package dao

import (
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	mxm "github.com/Daneel-Li/gps-back/internal/models"

	"gorm.io/gorm"
)

// 获取特定类型的设备
func (d *MysqlRepository) GetDevicesByType(tp string) ([]*mxm.Device, error) {
	var lst []*mxm.Device
	if err := d.db.Where("`type`=?", tp).Find(&lst).Error; err != nil {
		return nil, fmt.Errorf("select devices by type failed: %v", err)
	}
	return lst, nil
}

func (d *MysqlRepository) GetDeviceIDByOriginSN(originSN string, tp string) (string, error) {
	var deviceIDs []string
	// 使用GORM First 方法查找记录
	if err := d.db.Model(&mxm.Device{}).
		Where("originSN = ? AND type = ?", originSN, tp).
		Pluck("id", &deviceIDs).Error; err != nil {
		return "", fmt.Errorf("select device id by originSN failed: %v", err)
	}
	if len(deviceIDs) == 0 {
		return "", gorm.ErrRecordNotFound
	}
	return deviceIDs[0], nil
}

func (d *MysqlRepository) GetDeviceByOriginSN(originSN string, tp string) (*mxm.Device, error) {
	var device mxm.Device
	// 使用GORM First 方法查找记录
	if err := d.db.Where("originSN=? and type=?", originSN, tp).First(&device).Error; err != nil {
		return nil, err
	}
	// 返回找到的记录
	return &device, nil
}

func (d *MysqlRepository) GetDeviceByID(id string) (*mxm.Device, error) {
	var device mxm.Device
	// 使用GORM First 方法查找记录
	if err := d.db.Where("id=?", id).First(&device).Error; err != nil {
		return nil, err
	}
	// 返回找到的记录
	return &device, nil
}

func (d *MysqlRepository) GetDevicesByUserID(userID int) ([]*mxm.Device, error) {
	var devices []*mxm.Device
	res := d.db.Table("devices").Select("devices.*").
		Joins("right join users on users.id=devices.user_id").
		Where("user_id=?", userID).Find(&devices)
	if res.Error != nil {
		return nil, fmt.Errorf("dao GetDevicesByUserID error %v", res.Error)
	}
	return devices, nil
}

/*
*
绑定设备
*/

const NO_ROWS_AFFECTED = "affected 0 rows"

// 建立用户和设备的关系
func (d *MysqlRepository) BindDevice(deviceId string, label string, userid uint) error {
	if len(label) > 25 {
		label = label[:25]
	}

	// 1. 开始事务
	tx := d.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("begin transaction failed: %v", tx.Error)
	}

	// 定义回滚函数
	rollback := func(err error) error {
		tx.Rollback()
		return fmt.Errorf("bind device failed: %v", err)
	}

	// 2. 更新设备绑定状态
	if err := tx.Table("devices").
		Where("id=? and user_id is NULL", deviceId).
		Updates(map[string]interface{}{
			"user_id": userid,
			"label":   label,
		}).Error; err != nil {
		return rollback(fmt.Errorf("invalid device id or has been bound user: %v", err))
	}

	// 3. 需要创建的相关表列表
	// 假设与设备相关的表有：device_data, device_logs, safe_region 等
	relatedTables := map[string]string{
		fmt.Sprintf("his_pos_%s", deviceId):  "device_his_pos_template",
		fmt.Sprintf("his_data_%s", deviceId): "device_his_data_template",
		fmt.Sprintf("steps_%s", deviceId):    "device_steps_template",
	}

	// 4. 对每个表执行重命名操作
	for tbl, tmpl := range relatedTables {
		if err := tx.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s LIKE %s", tbl, tmpl)).Error; err != nil {

			return rollback(fmt.Errorf("create table %s failed: %v", tbl, err))
		}
	}

	// 5. 提交事务
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit transaction failed: %v", err)
	}

	return nil
}

func (d *MysqlRepository) UnBindDevice(userId uint, deviceId string) error {

	var device mxm.Device
	// 使用GORM First 方法查找记录
	if err := d.db.Where("id=? AND user_id=?", deviceId, userId).First(&device).Error; err != nil {
		// 如果是其他类型的错误，返回错误
		return err
	}

	// 1. 开始事务
	tx := d.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("begin transaction failed: %v", tx.Error)
	}

	// 定义回滚函数
	rollback := func(err error) error {
		tx.Rollback()
		return fmt.Errorf("unbind device failed: %v", err)
	}

	// 2. 获取当前时间戳
	timestamp := time.Now().Unix()

	// 3. 构建表名后缀
	suffix := fmt.Sprintf("_%d_%d", userId, timestamp)

	// 4. 需要重命名的相关表列表
	// 假设与设备相关的表有：device_data, device_logs, safe_region 等
	relatedTables := []string{
		fmt.Sprintf("his_pos_%s", deviceId),
		fmt.Sprintf("his_data_%s", deviceId),
		fmt.Sprintf("steps_%s", deviceId),
	}

	// 5. 对每个表执行重命名操作
	for _, tableName := range relatedTables {
		// 检查表是否存在
		var tableExists int
		if err := tx.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", tableName).Scan(&tableExists).Error; err != nil {
			return rollback(fmt.Errorf("check table existence failed: %v", err))
		}

		// 如果表存在，则直接重命名
		if tableExists > 0 {
			newTableName := tableName + suffix

			// 检查新表名是否已存在
			var newTableExists int
			if err := tx.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", newTableName).Scan(&newTableExists).Error; err != nil {
				return rollback(fmt.Errorf("check new table existence failed: %v", err))
			}

			if newTableExists > 0 {
				// 如果新表已存在，添加随机字符串避免冲突
				randomStr := fmt.Sprintf("%06d", rand.Intn(1000000))
				newTableName = newTableName + "_" + randomStr
			}

			// 直接重命名表
			if err := tx.Exec(fmt.Sprintf("RENAME TABLE %s TO %s", tableName, newTableName)).Error; err != nil {
				return rollback(fmt.Errorf("rename table %s to %s failed: %v", tableName, newTableName, err))
			}
			slog.Info(fmt.Sprintf("table %s renamed to %s", tableName, newTableName))
		}
		slog.Info(fmt.Sprintf("table %s not exists, skip renaming", tableName))
	}

	// 6. 更新设备绑定状态
	if err := tx.Unscoped().Where("id=? AND user_id=?", deviceId, userId).Delete(&mxm.Device{}).Error; err != nil {
		return rollback(fmt.Errorf("unbind device failed: %v", err))
	}
	if err := tx.Create(&mxm.Device{
		ID:        &deviceId,
		OriginSN:  device.OriginSN,
		Type:      device.Type,
		CreatedAt: device.CreatedAt,
	}).Error; err != nil {
		return rollback(fmt.Errorf("create device failed: %v", err))
	}

	// 7. 更新share_mapping表
	if err := tx.Where("device_id=?", deviceId).Delete(&mxm.ShareMap{}).Error; err != nil {
		return rollback(fmt.Errorf("update share_mapping failed: %v", err))
	}
	// 8. 更新safe_region表
	if err := tx.Table("safe_region").Where("device_id=?", deviceId).Delete(&struct{}{}).Error; err != nil {
		return rollback(fmt.Errorf("update safe_region failed: %v", err))
	}

	// 9. 提交事务
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit transaction failed: %v", err)
	}
	return nil
}

/*
*
出厂初始化添加到可用列表
*/
func (d *MysqlRepository) AddDevice(device mxm.Device) error {
	if err := d.db.Table("devices").Create(device).Error; err != nil {
		return fmt.Errorf("insert devices failed. obj=%v, error=%v", device, err)
	}
	return nil
}

func (d *MysqlRepository) GetDeviceProfileByID(deviceID string) (*mxm.Profile, error) {
	p := &mxm.Profile{}
	if err := d.db.Table("devices").
		Select("species,age,label,avatar_url,description,phone_number,sex,weight").
		Where("id=?", deviceID).Scan(p).Error; err != nil {
		return nil, fmt.Errorf("select device profile failed: %v", err)
	}
	return p, nil
}

func (d *MysqlRepository) UpdateDevice(deiviceID string, updates map[string]interface{}) (*mxm.Device, error) {
	dev := &mxm.Device{}
	result := d.db.Model(dev).
		Where("id = ?", deiviceID).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	return dev, nil
}

const (
	hisDataPrefix = "his_data_"
	posDataPrefix = "his_pos_"
	stepPrefix    = "steps_"
)

// 创建双表（事务保证原子性）
func (d *MysqlRepository) CreateDeviceTables(deviceID string) error {

	// 2. 获取表名
	hisTbl := hisDataPrefix + deviceID
	posTbl := posDataPrefix + deviceID
	stepTbl := stepPrefix + deviceID

	// 3. 在事务中执行
	return d.db.Transaction(func(tx *gorm.DB) error {
		// 检查历史表
		if err := checkAndCreateTable(tx, hisTbl, "device_his_data_template"); err != nil {
			return fmt.Errorf("create history table failed: %w", err)
		}

		// 检查轨迹表
		if err := checkAndCreateTable(tx, posTbl, "device_his_pos_template"); err != nil {
			return fmt.Errorf("create realtime table failed: %w", err)
		}

		// 运动统计表
		if err := checkAndCreateTable(tx, stepTbl, "device_steps_template"); err != nil {
			return fmt.Errorf("create steps table failed: %w", err)
		}

		return nil
	})
}

// 删除双表（事务保证原子性）
func (d *MysqlRepository) DropDeviceTables(deviceID string) error {

	// 2. 获取表名
	hisTbl := hisDataPrefix + deviceID
	posTbl := posDataPrefix + deviceID

	// 3. 在事务中执行
	return d.db.Transaction(func(tx *gorm.DB) error {
		// 删除历史表
		if err := checkAndDropTable(tx, hisTbl); err != nil {
			return fmt.Errorf("drop history table failed: %w", err)
		}

		// 删除轨迹表
		if err := checkAndDropTable(tx, posTbl); err != nil {
			return fmt.Errorf("drop realtime table failed: %w", err)
		}

		return nil
	})
}

// 复用检查与创建逻辑
func checkAndCreateTable(tx *gorm.DB, tblName string, templateName string) error {
	var count int
	if err := tx.Raw(`
        SELECT COUNT(*) 
        FROM information_schema.tables 
        WHERE table_schema = DATABASE() 
        AND table_name = ?`, tblName).Scan(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return tx.Exec(fmt.Sprintf(`CREATE TABLE %s LIKE %s`, tblName, templateName)).Error
}

// 复用检查与删除逻辑
func checkAndDropTable(tx *gorm.DB, tblName string) error {
	var count int
	if err := tx.Raw(`
        SELECT COUNT(*) 
        FROM information_schema.tables 
        WHERE table_schema = DATABASE() 
        AND table_name = ?`, tblName).Scan(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	return tx.Exec(fmt.Sprintf(`DROP TABLE %s`, tblName)).Error
}

func (d *MysqlRepository) AddHisData(deviceID string, raw []byte) error {

	his := &mxm.HisData{RawData: raw}
	if err := d.db.Table(hisDataPrefix + deviceID).Create(his).Error; err != nil {
		return fmt.Errorf("insert his_data failed. obj=%v, error=%v", his, err)
	}
	return nil
}

func (d *MysqlRepository) AddPosHis(deviceID string, loc *mxm.Location) error {

	if err := d.db.Table(posDataPrefix + deviceID).Create(loc).Error; err != nil {
		return fmt.Errorf("insert pos_his failed. obj=%v, error=%v", loc, err)
	}
	return nil
}

func (d *MysqlRepository) GetPosHis(deviceID string, st string, ed string, types []string) ([]*mxm.Location, error) {
	var hispos []*mxm.Location
	parsedSt, err := time.ParseInLocation("2006-1-2 15:4:5", st, time.Local)
	if err != nil {
		return nil, fmt.Errorf("failed to parse time: %v", err)
	}
	stUTC := parsedSt.UTC()
	parsedEd, err := time.ParseInLocation("2006-1-2 15:4:5", ed, time.Local)
	if err != nil {
		return nil, fmt.Errorf("failed to parse time: %v", err)
	}
	edUTC := parsedEd.UTC()
	if err := d.db.Table(posDataPrefix+deviceID).
		Where("loc_time BETWEEN ? AND ? AND `type` is not null AND `type` in (?)", stUTC, edUTC, types).
		Find(&hispos).Error; err != nil {
		return nil, fmt.Errorf("select his_pos_%s failed. error=%v ", deviceID, err)
	}
	return hispos, nil
}

func (d *MysqlRepository) UpdateDeviceAvatar(deviceID string, avatar string) error {
	var device mxm.Device
	if err := d.db.Model(device).Where("id = ?", deviceID).Update("avatar_url", avatar).Error; err != nil {
		return fmt.Errorf("update device failed. error=%v ", err)
	}
	return nil
}

func (d *MysqlRepository) GetDeviceAvatar(deviceID string) (string, error) {
	var device mxm.Device
	var url string
	if err := d.db.Model(device).Select("avatar_url").Where("id=?", deviceID).
		First(&url).Error; err != nil {
		return "", fmt.Errorf("select device avatar_url failed: %v", err.Error())
	}
	return url, nil
}

func (d *MysqlRepository) CreateDevice(deviceID string, originSN string, tp string, buzzer bool, note string) error {
	dvc := struct {
		mxm.Device
		Note string `gorm:"column:note"`
	}{
		Device: mxm.Device{
			ID:       &deviceID,
			OriginSN: &originSN,
			Type:     &tp,
			Buzzer:   &buzzer,
		},
		Note: note,
	}

	if err := d.db.Table("devices").Create(&dvc).Error; err != nil {
		return fmt.Errorf("create device failed. error=%v ", err)
	}
	return nil
}
