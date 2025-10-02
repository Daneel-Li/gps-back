package handlers

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Daneel-Li/gps-back/internal/dao"
	mxm "github.com/Daneel-Li/gps-back/internal/models"
	"github.com/Daneel-Li/gps-back/internal/services"
	"github.com/Daneel-Li/gps-back/pkg/utils"
)

type MessageProcessor struct {
	repo        dao.Repository
	wsManager   *services.WSManager
	cmdsManager services.CommandManager
}

func NewMessageProcessor(repo dao.Repository, wsManager *services.WSManager, cmdManager services.CommandManager) *MessageProcessor {
	return &MessageProcessor{repo, wsManager, cmdManager}
}

func (mp *MessageProcessor) Process(status *mxm.DeviceStatus1) error {
	if status == nil {
		return fmt.Errorf("status is nil")
	}
	// 这里包含标准化处理流程:
	// 1. 消息解析
	// 2. 数据校验
	// 3. 状态转换
	// 4. 业务事件触发

	// 首先对标上device记录
	devID, err := mp.repo.GetDeviceIDByOriginSN(status.OriginSN, status.Type)
	if err != nil {
		slog.Error("handle status failed", "error", err, "status", status)
		return fmt.Errorf("get device failed: %v", err)
	}

	// 更新历史数据表，作为原始交互数据，不管是位置更新，心跳还是指令，都存下来备用
	if err := mp.repo.AddHisData(devID, status.RawMsg); err != nil {
		slog.Error("Save history data failed", "error", err, "status", status)
	}

	// 2. TODO: 业务逻辑处理（补充以下实现）
	// --------------------------------------------
	// 示例1：检查设备是否在线状态变化
	if status.Device != nil {
		d := status.Device
		d.ID = &devID
		if d.Electricity != nil && *d.Electricity < 20 {
			// TODO 下发提醒任务
			mp.repo.AddAlarm(mxm.Alarm{
				DeviceID: devID,
				Msg:      fmt.Sprintf("%v%%", *d.Electricity),
				Time:     *d.LastOnline,
				Type:     mxm.LOW_BATERY,
			})
		}

		// 步数记录
		if d.Steps != nil {
			if err := mp.repo.AddSteps(devID, *(d.Steps)); err != nil {
				slog.Error("addSteps failed", "error", err.Error(), "status", status)
			}
		}

		//如果定位失败，坐标和loctime，只更新通信时间
		locFailed := false
		if d.Latitude == nil || d.Longitude == nil ||
			(*d.Latitude == 0.0 && *d.Longitude == 0.0) {
			locFailed = true
		}

		if locFailed {
			// 只更新设备状态表部分字段,也不做围栏检查
			d.LocTime, d.Accuracy, d.Speed, d.Heading, d.Latitude, d.Longitude,
				d.Address, d.LocType, d.Satellites = nil, nil, nil, nil, nil, nil, nil, nil, nil

			update := utils.StructToUpdateMap(*d)
			utils.RemoveGormModelFields(update)
			if dev, err := mp.repo.UpdateDevice(*d.ID, update); err != nil {
				return fmt.Errorf("update device failed: %v", err)
			} else {
				// 找到该设备关联的用户，下发推送
				slog.Debug("update device success", "device", dev)
			}
		} else {
			//TODO 围栏检查
			// if !locFailed && checkFence(d.Location) {
			// 	slog.Error("设备超出围栏:", "deviceID", devID)
			// }
			update := utils.StructToUpdateMap(*d)
			utils.RemoveGormModelFields(update)

			if dev, err := mp.repo.UpdateDevice(*d.ID, update); err != nil {
				return fmt.Errorf("update device failed: %v", err)
			} else {
				//TODO 广播更新
				toNotify := make([]uint, 0)
				if id, err := mp.repo.GetUserIdByDeviceId(*d.ID); err == nil {
					toNotify = append(toNotify, id)
				}
				if ids, err := mp.repo.GetSharedUserIdsByDeviceId(*d.ID); err == nil {
					toNotify = append(toNotify, ids...)
				}
				mp.wsManager.BroadcastToUsers(toNotify, services.WSMessage{
					Type: "device_update",
					Data: dev,
				})
				slog.Debug("update device success", "device", dev)
			}

			// 更新轨迹表(历史轨迹必须来自于device状态，以保持统一)
			loc := mxm.Location{
				Address:    utils.Deref(d.Address, ""),
				Longitude:  utils.Deref(d.Longitude, 0),
				Latitude:   utils.Deref(d.Latitude, 0),
				Altitude:   utils.Deref(d.Altitude, 0),
				Satellites: utils.Deref(d.Satellites, 0),
				Type:       utils.Deref(d.LocType, "LBS"),
				LocTime:    utils.Deref(d.LocTime, time.Now()),
				Accuracy:   utils.Deref(d.Accuracy, 1000),
				Speed:      utils.Deref(d.Speed, 0),
				Heading:    utils.Deref(d.Heading, 0),
			}
			if err := mp.repo.AddPosHis(*d.ID, &loc); err != nil {
				slog.Error("Save pos data failed", "error", err, "status", status)
			}
		}
	}
	if status.Command != nil && status.Command.Result != nil {
		res := status.Command.Result

		pCmd, ok := mp.cmdsManager.GetAndRemoveCommand(res.CommandID)
		if !ok {
			return nil // do nothing
		}

		// 1. 推送通知客户端
		m := &services.WSMessage{
			Type: "command_result",
			Data: res,
		}
		if err := mp.wsManager.PushMsg(*pCmd.Terminal, m); err != nil {
			slog.Error("push command result failed", "error", err, "result", res)
		}
		slog.Debug(fmt.Sprintf("command result: %v", res))

		// 2. 有些参数需要根据执行结果来更新数据，比如定时开关机（暂时没找到查询方法）
		if res.Succeed { //执行成功才更新数据库
			cmd := pCmd.Command
			if cmd.Action == "AUTO_START" {
				mp.repo.UpsertSettingsFields(map[string]interface{}{
					"device_id":         devID,
					"auto_start_at":     cmd.Args[0],
					"auto_start_enable": cmd.Args[1] == "1",
				})
			} else if cmd.Action == "AUTO_SHUT" {
				mp.repo.UpsertSettingsFields(map[string]interface{}{
					"device_id":        devID,
					"auto_shut_at":     cmd.Args[0],
					"auto_shut_enable": cmd.Args[1] == "1",
				})
			}
		}
	}

	return nil
}
