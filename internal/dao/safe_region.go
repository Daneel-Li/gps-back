package dao

import (
	"encoding/json"
	"errors"
	"fmt"

	mxm "github.com/Daneel-Li/gps-back/internal/models"

	"gorm.io/gorm"
)

// 原始数据结构体
type safeRegionDB struct {
	DeviceId string `gorm:"column:device_id"`
	Type     string `gorm:"column:type"`
	Name     string `gorm:"column:name"`
	AreaJSON string `gorm:"column:area"` // 存储Area的JSON描述
}

// 将mxm.Region转换为safeRegionDB
func regionToDBModel(deviceId string, region *mxm.Region) (*safeRegionDB, error) {
	// 序列化Area为JSON
	areaJSON, err := json.Marshal(region.Area)
	if err != nil {
		return nil, fmt.Errorf("marshal area failed: %v", err)
	}

	return &safeRegionDB{
		DeviceId: deviceId,
		Type:     region.Type,
		Name:     region.Name,
		AreaJSON: string(areaJSON),
	}, nil
}

// 将safeRegionDB转换为mxm.Region
func dbModelToRegion(db *safeRegionDB) (*mxm.Region, error) {
	region := &mxm.Region{
		Type: db.Type,
		Name: db.Name,
	}

	// 根据Type解析Area
	var err error
	switch db.Type {
	case "circle":
		var circle mxm.Circle
		if err = json.Unmarshal([]byte(db.AreaJSON), &circle); err == nil {
			region.Area = &circle
		}
	case "rectangle":
		var rectangle mxm.Rectangle
		if err = json.Unmarshal([]byte(db.AreaJSON), &rectangle); err == nil {
			region.Area = &rectangle
		}
	default:
		return nil, fmt.Errorf("unknown region type: %s", db.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal area: %v", err)
	}

	return region, nil
}

func (safeRegionDB) TableName() string {
	return "safe_region"
}

type safeRegion struct {
	*mxm.Region
	DeviceId string `gorm:"column:device_id"`
}

// 设置围栏
func (d *MysqlRepository) SetSafeRegion(deviceId string, region *mxm.Region) error {
	dbModel, err := regionToDBModel(deviceId, region)
	if err != nil {
		return fmt.Errorf("convert region to db model failed:%v", err)
	}
	var existing safeRegionDB
	err = d.db.Table("safe_region").
		Where("device_id = ? AND name = ?", deviceId, region.Name).
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := d.db.Debug().Table("safe_region").Create(dbModel).Error; err != nil {
			return fmt.Errorf("set safe_region error,%v", err)
		}
	} else if err == nil {
		if err := d.db.Table("safe_region").
			Where("device_id = ? AND name = ?", deviceId, region.Name).
			Updates(dbModel).Error; err != nil {
			return fmt.Errorf("set into safe_region error,%v", err)
		}
	} else {
		// 其他错误
		return fmt.Errorf("set saferegion failed: query safe_region error: %v", err)
	}

	return nil
}

// CreateSafeRegion 创建安全区域
func (d *MysqlRepository) CreateSafeRegion(region interface{}) error {
	// 这里需要根据实际的region类型进行转换
	// 暂时返回未实现错误
	return fmt.Errorf("CreateSafeRegion not implemented")
}

// DeleteSafeRegion 删除安全区域
func (d *MysqlRepository) DeleteSafeRegion(regionID uint) error {
	// 暂时返回未实现错误
	return fmt.Errorf("DeleteSafeRegion not implemented")
}

// UpdateSafeRegion 更新安全区域
func (d *MysqlRepository) UpdateSafeRegion(regionID uint, updates map[string]interface{}) error {
	// 暂时返回未实现错误
	return fmt.Errorf("UpdateSafeRegion not implemented")
}

func (d *MysqlRepository) GetSafeRegions(deviceId string) ([]*mxm.Region, error) {
	var dbModels []*safeRegionDB

	if err := d.db.Table("safe_region").
		Where("`device_id`=?", deviceId).Find(&dbModels).Error; err != nil {
		return nil, fmt.Errorf("select safe_region by device_id failed: %v", err)
	}

	var result []*mxm.Region
	for _, dbModel := range dbModels {
		region, err := dbModelToRegion(dbModel)
		if err != nil {
			return nil, err
		}
		result = append(result, region)
	}

	return result, nil
}
