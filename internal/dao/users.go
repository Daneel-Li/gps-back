package dao

import (
	"errors"
	"fmt"

	mxm "github.com/Daneel-Li/gps-back/internal/models"

	"gorm.io/gorm"
)

func (d *MysqlRepository) CreateUser(user *mxm.User) error {
	res := d.db.Table("users").Omit("id").Create(user)
	if res.Error != nil {
		return fmt.Errorf("create user failed,user=%v, error=%v", user, res.Error)
	}
	return nil
}

func (d *MysqlRepository) DeleteUser(user mxm.User) error {
	if err := d.db.Table("users").Delete(user).Error; err != nil {
		return fmt.Errorf("delete user failed: %v", err)
	}
	return nil
}

func (d *MysqlRepository) GetUserByOpenID(openid string) (*mxm.User, error) {
	var user mxm.User
	if err := d.db.Table("users").Select("*").Where("openid=?", openid).First(&user).Error; err != nil {
		return nil, fmt.Errorf("select user failed: %v", err.Error())
	}
	return &user, nil
}

func (d *MysqlRepository) UpdateUser(id uint, updates map[string]interface{}) error {
	u := &mxm.User{}
	result := d.db.Model(u).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update user failed: %v", result.Error)
	}
	return nil
}

func (d *MysqlRepository) GetUserByID(id uint) (*mxm.User, error) {
	var user mxm.User
	if err := d.db.Table("users").Select("*").Where("id=?", id).First(&user).Error; err != nil {
		return nil, fmt.Errorf("select user failed: %v", err.Error())
	}
	return &user, nil
}

func (d *MysqlRepository) GetUserById(id uint) (*mxm.User, error) {
	return d.GetUserByID(id)
}

func (d *MysqlRepository) GetOrCreateUserByOpenId(openid string) (*mxm.User, error) {
	var user mxm.User
	if err := d.db.Table("users").Select("*").Where("openid=?", openid).
		First(&user).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("select user failed: %v", err.Error())
		}
		user = mxm.User{OpenID: openid}
		if err := d.db.Table("users").Create(&user).Error; err != nil {
			return nil, fmt.Errorf("create user failed: %v", err.Error())
		}
		return &user, nil
	}
	return &user, nil
}

func (d *MysqlRepository) GetUserIdByDeviceId(deviceId string) (uint, error) {
	var userId uint
	err := d.db.Raw("SELECT user_id FROM devices WHERE id = ? LIMIT 1", deviceId).Scan(&userId).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, fmt.Errorf("设备不存在: %s", deviceId)
	}
	return userId, err
}

func (d *MysqlRepository) GetSharedUserIdsByDeviceId(deviceId string) ([]uint, error) {
	var userIds []uint
	err := d.db.Raw("SELECT user_id FROM share_mapping WHERE device_id = ? AND deleted_at=null", deviceId).Scan(&userIds).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("设备无分享记录: %s", deviceId)
	}
	return userIds, err
}

func (d *MysqlRepository) UpdateUserAvatar(userId string, avatar string) error {
	var user mxm.User
	if err := d.db.Model(user).Where("id = ?", userId).Update("avatar_url", avatar).Error; err != nil {
		return fmt.Errorf("update users failed. error=%v ", err)
	}
	return nil
}

func (d *MysqlRepository) GetUserAvatar(id string) (string, error) {
	var url string
	if err := d.db.Model(mxm.User{}).Select("avatar_url").Where("id=?", id).
		First(&url).Error; err != nil {
		return "", fmt.Errorf("select device avatar_url failed: %v", err.Error())
	}
	return url, nil
}
