package dao

import (
	"fmt"
	"time"

	mxm "github.com/Daneel-Li/gps-back/internal/models"
)

func (d *MysqlRepository) AddRecoveryCmd(o *mxm.RecoveryCmd) error {

	if err := d.db.Create(o).Error; err != nil {
		return fmt.Errorf("add recovery cmd error, %v", err)
	}
	return nil
}

// 返回截至某个时间已经生效等待执行的recoverycmd, before 为nil则返回所有记录
func (d *MysqlRepository) GetRecoveryCmds(before *time.Time) ([]*mxm.RecoveryCmd, error) {
	if before == nil {
		b := time.Now().Add(-10 * time.Hour)
		before = &b
	}
	var lst []*mxm.RecoveryCmd
	if err := d.db.Preload("Device").Where("tm < ?", before.Local().Format("2006-01-02 15:04:05")).Find(&lst).Error; err != nil {
		return nil, fmt.Errorf("select recovery cmds failed, %v", err)
	}
	return lst, nil
}

func (d *MysqlRepository) DelRecoveryCmds(lst []*mxm.RecoveryCmd) error {

	if len(lst) == 0 {
		return nil
	}
	if err := d.db.Delete(lst).Error; err != nil {
		return fmt.Errorf("delete recovery cmds failed, %v", err)
	}
	return nil
}
