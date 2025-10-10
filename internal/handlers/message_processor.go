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
	// This contains standardized processing flow:
	// 1. Message parsing
	// 2. Data validation
	// 3. Status conversion
	// 4. Business event triggering

	// First match with device record
	devID, err := mp.repo.GetDeviceIDByOriginSN(status.OriginSN, status.Type)
	if err != nil {
		slog.Error("handle status failed", "error", err, "status", status)
		return fmt.Errorf("get device failed: %v", err)
	}

	// Update history data table, as raw interaction data, whether position update, heartbeat or command, all stored for backup
	if err := mp.repo.AddHisData(devID, status.RawMsg); err != nil {
		slog.Error("Save history data failed", "error", err, "status", status)
	}

	// 2. TODO: Business logic processing (supplement the following implementation)
	// --------------------------------------------
	// Example 1: Check if device online status changes
	if status.Device != nil {
		d := status.Device
		d.ID = &devID
		if d.Electricity != nil && *d.Electricity < 20 {
			// TODO Send reminder task
			mp.repo.AddAlarm(mxm.Alarm{
				DeviceID: devID,
				Msg:      fmt.Sprintf("%v%%", *d.Electricity),
				Time:     *d.LastOnline,
				Type:     mxm.LOW_BATERY,
			})
		}

		// Step count record
		if d.Steps != nil {
			if err := mp.repo.AddSteps(devID, *(d.Steps)); err != nil {
				slog.Error("addSteps failed", "error", err.Error(), "status", status)
			}
		}

		// If positioning fails, coordinates and loctime, only update communication time
		locFailed := false
		if d.Latitude == nil || d.Longitude == nil ||
			(*d.Latitude == 0.0 && *d.Longitude == 0.0) {
			locFailed = true
		}

		if locFailed {
			// Only update partial fields of device status table, no fence check
			d.LocTime, d.Accuracy, d.Speed, d.Heading, d.Latitude, d.Longitude,
				d.Address, d.LocType, d.Satellites = nil, nil, nil, nil, nil, nil, nil, nil, nil

			update := utils.StructToUpdateMap(*d)
			utils.RemoveGormModelFields(update)
			if dev, err := mp.repo.UpdateDevice(*d.ID, update); err != nil {
				return fmt.Errorf("update device failed: %v", err)
			} else {
				// Find users associated with this device, send push
				slog.Debug("update device success", "device", dev)
			}
		} else {
			//TODO Fence check
			// if !locFailed && checkFence(d.Location) {
			// 	slog.Error("Device out of fence:", "deviceID", devID)
			// }
			update := utils.StructToUpdateMap(*d)
			utils.RemoveGormModelFields(update)

			if dev, err := mp.repo.UpdateDevice(*d.ID, update); err != nil {
				return fmt.Errorf("update device failed: %v", err)
			} else {
				//TODO Broadcast update
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

			// Update track table (historical track must come from device status to maintain consistency)
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

		// 1. Push notification to client
		m := &services.WSMessage{
			Type: "command_result",
			Data: res,
		}
		if err := mp.wsManager.PushMsg(*pCmd.Terminal, m); err != nil {
			slog.Error("push command result failed", "error", err, "result", res)
		}
		slog.Debug(fmt.Sprintf("command result: %v", res))

		// 2. Some parameters need to be updated based on execution results, such as scheduled power on/off (no query method found yet)
		if res.Succeed { // Only update database when execution succeeds
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
