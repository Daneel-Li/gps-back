package services

import (
	"errors"
	"log/slog"

	"github.com/Daneel-Li/gps-back/internal/dao"
)

// 话题提供者，实现mqtthandler中的TopicProvider接口
type BttTopicProvider struct {
	// 话题列表
	dao dao.Repository
}

func NewBttTopicProvider(dao dao.Repository) *BttTopicProvider {
	return &BttTopicProvider{dao: dao}
}

const _PREFIX_DEV = "dwq/device/hy/"
const _PREFIX_APP = "dwq/app/hy/"

// 查找需要监听的设备列表
func (p *BttTopicProvider) GetSubscriptionList() ([]string, error) {
	if dvcs, err := p.dao.GetDevicesByType("btt"); err == nil {
		lst := []string{}
		for _, d := range dvcs {
			if d.UserID == nil || *d.UserID == 0 { //设备未分配，不用监听
				continue
			}
			if d.OriginSN == nil { //脏数据，需要处理
				slog.Error("device has no originSN, dirty data", "deviceID", d.ID)
				continue
			}
			lst = append(lst, *d.OriginSN)
		}

		return lst, nil
	} else {
		return nil, errors.New("GetDevicesByType error: " + err.Error())
	}
}
