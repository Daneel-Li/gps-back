package services

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/Daneel-Li/gps-back/internal/config"
	"github.com/Daneel-Li/gps-back/internal/dao"
	mxm "github.com/Daneel-Li/gps-back/internal/models"
	"github.com/Daneel-Li/gps-back/internal/types"
	"github.com/Daneel-Li/gps-back/internal/vendors"
	"github.com/Daneel-Li/gps-back/pkg/utils"
)

// SimpleServiceContainer 简化的服务容器
type SimpleServiceContainer struct {
	repo          dao.Repository
	driverManager *DriverManager
	cmdManager    CommandManager
	idGen         *utils.IDGenerator
}

// NewSimpleServiceContainer 创建简化的服务容器
func NewSimpleServiceContainer(repo dao.Repository, cmdManager CommandManager) *SimpleServiceContainer {
	return &SimpleServiceContainer{
		repo:          repo,
		driverManager: NewDriverManager(),
		cmdManager:    cmdManager,
		idGen:         &utils.IDGenerator{},
	}
}

// RegisterDriver 注册厂商驱动
func (c *SimpleServiceContainer) RegisterDriver(name string, driver vendors.VendorDriver) error {
	return c.driverManager.RegisterDriver(name, driver)
}

// SetMessageHandler 为所有驱动设置消息处理器
func (c *SimpleServiceContainer) SetMessageHandler(handler vendors.MessageHandler) {
	c.driverManager.SetMessageHandler(handler)
}

func (c *SimpleServiceContainer) StartAllDrivers() error {
	return c.driverManager.StartAllDrivers()
}

// ========== 设备相关方法 ==========

// GetDeviceByID 获取单个设备信息
func (c *SimpleServiceContainer) GetDeviceByID(ctx context.Context, deviceID string, userID uint) (*mxm.Device, error) {
	device, err := c.repo.GetDeviceByID(deviceID)
	if err != nil {
		slog.Error("get device failed", "deviceID", deviceID, "error", err)
		return nil, fmt.Errorf("get device failed: %w", err)
	}
	return device, nil
}

// GetDevicesByUser 获取用户的设备列表
func (c *SimpleServiceContainer) GetDevicesByUser(ctx context.Context, userID uint) (map[string][]*mxm.Device, error) {
	// 获取用户拥有的设备
	ownedDevices, err := c.repo.GetDevicesByUserID(int(userID))
	if err != nil {
		slog.Error("get owned devices failed", "userID", userID, "error", err)
		return nil, fmt.Errorf("get owned devices failed: %w", err)
	}

	// 获取分享给用户的设备
	sharedMappings, err := c.repo.GetShareMappingByUserID(int(userID))
	if err != nil {
		slog.Error("get shared devices failed", "userID", userID, "error", err)
		return nil, fmt.Errorf("get shared devices failed: %w", err)
	}

	sharedDevices := make([]*mxm.Device, len(sharedMappings))
	for i, mapping := range sharedMappings {
		sharedDevices[i] = &mapping.Device
	}

	// 试用设备列表（如果用户没有任何设备）
	trialDevices := []*mxm.Device{}
	if len(ownedDevices) == 0 && len(sharedDevices) == 0 {
		if trialDevice, err := c.repo.GetDeviceByID(config.GetConfig().TrialDeviceID); err == nil {
			trialDevices = append(trialDevices, trialDevice)
		}
	}

	result := map[string][]*mxm.Device{
		"owned":  ownedDevices,
		"shared": sharedDevices,
		"trial":  trialDevices,
	}

	return result, nil
}

// BindDevice 绑定设备到用户
func (c *SimpleServiceContainer) BindDevice(ctx context.Context, userID uint, originSN, deviceType, label string) error {
	deviceID, err := c.repo.GetDeviceIDByOriginSN(originSN, deviceType)
	if err != nil {
		slog.Error("get device by origin SN failed", "originSN", originSN, "deviceType", deviceType, "error", err)
		return fmt.Errorf("device not found: %w", err)
	}

	if err := c.repo.BindDevice(deviceID, label, userID); err != nil {
		slog.Error("bind device failed", "deviceID", deviceID, "userID", userID, "error", err)
		return fmt.Errorf("bind device failed: %w", err)
	}

	return nil
}

// ActivateDevice 激活设备
func (c *SimpleServiceContainer) ActivateDevice(ctx context.Context, originSN, deviceType string) error {
	return c.driverManager.ActivateDevice(types.DeviceType(deviceType), originSN)
}

// UnbindDevice 解绑设备
func (c *SimpleServiceContainer) UnbindDevice(ctx context.Context, userID uint, deviceID string) error {
	if err := c.repo.UnBindDevice(userID, deviceID); err != nil {
		slog.Error("unbind device failed", "deviceID", deviceID, "userID", userID, "error", err)
		return fmt.Errorf("unbind device failed: %w", err)
	}
	return nil
}

// GetDeviceTrack 获取设备轨迹
func (c *SimpleServiceContainer) GetDeviceTrack(ctx context.Context, deviceID string, startTime, endTime string, types []string) ([]*mxm.Location, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("deviceID is required")
	}
	if startTime == "" {
		return nil, fmt.Errorf("startTime is required")
	}
	if endTime == "" {
		return nil, fmt.Errorf("endTime is required")
	}

	if len(types) == 0 {
		types = []string{"GPS", "WIFI", "LBS"}
	}

	track, err := c.repo.GetPosHis(deviceID, startTime, endTime, types)
	if err != nil {
		slog.Error("get track error", "deviceID", deviceID, "startTime", startTime, "endTime", endTime, "types", types, "error", err)
		return nil, fmt.Errorf("get track error: %w", err)
	}

	slog.Info("get track success", "deviceID", deviceID, "startTime", startTime, "endTime", endTime, "count", len(track))
	return track, nil
}

// GetDeviceProfile 获取设备档案
func (c *SimpleServiceContainer) GetDeviceProfile(ctx context.Context, deviceID string) (*mxm.Profile, error) {
	profile, err := c.repo.GetDeviceProfileByID(deviceID)
	if err != nil {
		slog.Error("get device profile failed", "deviceID", deviceID, "error", err)
		return nil, fmt.Errorf("get device profile failed: %w", err)
	}

	slog.Info("get device profile success", "deviceID", deviceID)
	return profile, nil
}

// UpdateDeviceProfile 更新设备档案
func (c *SimpleServiceContainer) UpdateDeviceProfile(ctx context.Context, deviceID string, updates map[string]interface{}) error {
	_, err := c.repo.UpdateDevice(deviceID, updates)
	if err != nil {
		slog.Error("update device profile failed", "deviceID", deviceID, "updates", updates, "error", err)
		return fmt.Errorf("update device profile failed: %w", err)
	}

	slog.Info("update device profile success", "deviceID", deviceID, "updates", updates)
	return nil
}

// UpdateAvatar 更新头像（设备或用户）
func (c *SimpleServiceContainer) UpdateAvatar(ctx context.Context, target string, id string, filename string) error {
	if target == "devices" {
		return c.repo.UpdateDeviceAvatar(id, filename)
	} else if target == "users" {
		return c.repo.UpdateUserAvatar(id, filename)
	}
	return fmt.Errorf("invalid target: %s", target)
}

// GetAvatar 获取头像（设备或用户）
func (c *SimpleServiceContainer) GetAvatar(ctx context.Context, target string, id string) (string, error) {
	if target == "devices" {
		return c.repo.GetDeviceAvatar(id)
	} else if target == "users" {
		return c.repo.GetUserAvatar(id)
	}
	return "", fmt.Errorf("invalid target: %s", target)
}

// EnrollDevice 设备入库
func (c *SimpleServiceContainer) EnrollDevice(ctx context.Context, req mxm.DeviceEnrollRequest) error {
	deviceID := fmt.Sprintf("%d", c.idGen.Next())

	if err := c.repo.CreateDevice(deviceID, req.SerialNumber, req.Model, false, req.Note); err != nil {
		slog.Error("enroll device failed", "req", req, "error", err)
		return fmt.Errorf("enroll device failed: %w", err)
	}

	slog.Info("enroll device success", "deviceID", deviceID, "req", req)
	return nil
}

// IsEnrollAdmin 检查用户是否有入库权限
func (c *SimpleServiceContainer) IsEnrollAdmin(ctx context.Context, userID uint) bool {
	// TODO: 实现权限检查逻辑
	return true // 暂时返回 true
}

// ========== 设备命令相关方法 ==========

// ExecuteCommand 执行设备命令（统一入口）
func (c *SimpleServiceContainer) ExecuteCommand(ctx context.Context, deviceID string, action string, args []string, terminalKey string) (int64, error) {
	device, err := c.repo.GetDeviceByID(deviceID)
	if err != nil {
		slog.Error("get device failed", "deviceID", deviceID, "error", err)
		return 0, fmt.Errorf("get device failed: %w", err)
	}

	if device.Type == nil {
		return 0, fmt.Errorf("device type is nil")
	}

	driver, err := c.driverManager.GetDriver(types.DeviceType(*device.Type))
	if err != nil {
		slog.Error("driver not found", "deviceID", deviceID, "type", *device.Type, "error", err)
		return 0, fmt.Errorf("driver not found for device type: %s", *device.Type)
	}

	if device.OriginSN == nil {
		return 0, fmt.Errorf("device origin SN is nil")
	}

	commandID := c.idGen.Next()

	if terminalKey != "" {
		t := TerminalKey{}
		c.cmdManager.AddCommand(commandID, t, &mxm.Command{Action: action, Args: args})
	}

	var execErr error
	switch action {
	case "POWER_OFF":
		execErr = driver.PowerOff(commandID, *device.OriginSN)
	case "REBOOT":
		execErr = driver.Reboot(commandID, *device.OriginSN)
	case "LOCATE":
		execErr = driver.Locate(commandID, *device.OriginSN)
	case "FIND":
		execErr = driver.Find(commandID, *device.OriginSN)
	case "AUTO_START":
		advancedDriver, ok := driver.(vendors.AdvancedDriver)
		if !ok {
			return 0, fmt.Errorf("driver does not support advanced operations")
		}
		if len(args) < 2 {
			return 0, fmt.Errorf("AUTO_START requires 2 arguments: time and enable")
		}
		tm := args[0]
		enable := args[1] == "1"
		execErr = advancedDriver.AutoStart(commandID, *device.OriginSN, tm, enable)
	case "AUTO_SHUT":
		advancedDriver, ok := driver.(vendors.AdvancedDriver)
		if !ok {
			return 0, fmt.Errorf("driver does not support advanced operations")
		}
		if len(args) < 2 {
			return 0, fmt.Errorf("AUTO_SHUT requires 2 arguments: time and enable")
		}
		tm := args[0]
		enable := args[1] == "1"
		execErr = advancedDriver.AutoShut(commandID, *device.OriginSN, tm, enable)
	case "SET_REPORTINTERVAL":
		if len(args) < 1 {
			return 0, fmt.Errorf("SET_REPORTINTERVAL requires 1 argument: interval")
		}
		interval, err := strconv.Atoi(args[0])
		if err != nil {
			return 0, fmt.Errorf("invalid interval: %s", args[0])
		}
		execErr = driver.SetReportInterval(commandID, *device.OriginSN, interval)
	default:
		execErr = fmt.Errorf("action not found: %s", action)
	}

	if execErr != nil {
		slog.Error("exec cmd to device error", "deviceID", deviceID, "action", action, "args", args, "error", execErr.Error())
		return 0, fmt.Errorf("exec cmd to device error: %w", execErr)
	}

	slog.Info("command executed successfully", "deviceID", deviceID, "action", action, "commandID", commandID)
	return commandID, nil
}

// GetReportInterval 获取设备上报间隔
func (c *SimpleServiceContainer) GetReportInterval(ctx context.Context, deviceID string) (int, error) {
	device, err := c.repo.GetDeviceByID(deviceID)
	if err != nil {
		slog.Error("get device failed", "deviceID", deviceID, "error", err)
		return 0, fmt.Errorf("get device failed: %w", err)
	}

	if device.Interval == nil {
		return 0, fmt.Errorf("device report interval is not set")
	}

	slog.Info("get report interval success", "deviceID", deviceID, "interval", *device.Interval)
	return *device.Interval, nil
}

// GetAutoPowerParams 获取定时开关机参数
func (c *SimpleServiceContainer) GetAutoPowerParams(ctx context.Context, deviceID string) (*mxm.AutoPowerParam, error) {
	params, err := c.repo.GetAutoPowerParams(deviceID)
	if err != nil {
		slog.Error("get auto power params failed", "deviceID", deviceID, "error", err)
		return nil, fmt.Errorf("get auto power params failed: %w", err)
	}

	slog.Info("get auto power params success", "deviceID", deviceID)
	return params, nil
}

// ========== 安全区域相关方法 ==========

// GetSafeRegions 获取安全区域
func (c *SimpleServiceContainer) GetSafeRegions(ctx context.Context, deviceID string, userID uint) ([]*mxm.Region, error) {
	regions, err := c.repo.GetSafeRegions(deviceID)
	if err != nil {
		slog.Error("get safe regions failed", "deviceID", deviceID, "userID", userID, "error", err)
		return nil, fmt.Errorf("get safe regions failed: %w", err)
	}

	slog.Info("get safe regions success", "deviceID", deviceID, "userID", userID, "count", len(regions))
	return regions, nil
}

// SetSafeRegion 设置安全区域
func (c *SimpleServiceContainer) SetSafeRegion(ctx context.Context, deviceID string, region *mxm.Region) error {
	// 首先检查是否已存在同名的安全区域
	existingRegions, err := c.repo.GetSafeRegions(deviceID)
	if err != nil {
		slog.Error("get existing safe regions failed", "deviceID", deviceID, "error", err)
		return fmt.Errorf("get existing safe regions failed: %w", err)
	}

	// 检查是否存在同名的区域
	var existingRegion *mxm.Region
	for _, r := range existingRegions {
		if r.Name == region.Name {
			existingRegion = r
			break
		}
	}

	if existingRegion != nil {
		// 区域已存在，执行更新操作
		slog.Info("updating existing safe region", "deviceID", deviceID, "regionName", region.Name)
		if err := c.repo.SetSafeRegion(deviceID, region); err != nil {
			slog.Error("update safe region failed", "deviceID", deviceID, "region", region, "error", err)
			return fmt.Errorf("update safe region failed: %w", err)
		}
		slog.Info("update safe region success", "deviceID", deviceID, "region", region)
	} else {
		// 区域不存在，执行创建操作
		slog.Info("creating new safe region", "deviceID", deviceID, "regionName", region.Name)
		if err := c.repo.SetSafeRegion(deviceID, region); err != nil {
			slog.Error("create safe region failed", "deviceID", deviceID, "region", region, "error", err)
			return fmt.Errorf("create safe region failed: %w", err)
		}
		slog.Info("create safe region success", "deviceID", deviceID, "region", region)
	}

	return nil
}

// ========== 告警相关方法 ==========

// GetAlarmsByDeviceID 获取设备告警列表
func (c *SimpleServiceContainer) GetAlarmsByDeviceID(ctx context.Context, deviceID string) ([]*mxm.Alarm, error) {
	alarms, err := c.repo.GetAlarmsByDeviceID(deviceID, 100, 0)
	if err != nil {
		slog.Error("get alarms failed", "deviceID", deviceID, "error", err)
		return nil, fmt.Errorf("get alarms failed: %w", err)
	}

	slog.Info("get alarms success", "deviceID", deviceID, "count", len(alarms))
	return alarms, nil
}

// ========== 步数相关方法 ==========

// GetSteps 获取步数数据（包含平滑处理）
func (c *SimpleServiceContainer) GetSteps(ctx context.Context, deviceID string, date string) (map[string]interface{}, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("deviceID is required")
	}
	if date == "" {
		return nil, fmt.Errorf("date is required")
	}

	st, err1 := time.ParseInLocation("2006-1-2 15:4:5", fmt.Sprintf("%s 00:00:00", date), time.Local)
	ed, err2 := time.ParseInLocation("2006-1-2 15:4:5", fmt.Sprintf("%s 23:59:59", date), time.Local)
	if err1 != nil || err2 != nil {
		slog.Error("invalid date format", "date", date)
		return nil, fmt.Errorf("invalid date format: %s", date)
	}

	steps, err := c.repo.GetStepsByDeviceID(deviceID, st, ed)
	if err != nil {
		slog.Error("get steps failed", "deviceID", deviceID, "date", date, "error", err)
		return nil, fmt.Errorf("get steps failed: %w", err)
	}

	// 平滑处理steps数据,均匀分布在24小时内
	slot := make([]int, 24)
	preTm := st
	slotEd := st.Add(time.Hour)
	slotIdx := 0
	total := 0
	for _, step := range steps {
		total += step.Steps
		sec := int(step.CreatedAt.Sub(preTm).Seconds())
		if sec == 0 {
			continue
		}
		base := (step.Steps * 3600) / sec

		for {
			delivered := 0
			if step.CreatedAt.Before(slotEd) {
				slot[slotIdx] += (step.Steps - delivered)
				preTm = step.CreatedAt
				break
			} else {
				deliver := int(slotEd.Sub(preTm).Seconds()) * base / 3600
				slot[slotIdx] += deliver
				delivered += deliver
				preTm = slotEd
				slotEd = slotEd.Add(time.Hour)
				slotIdx++
			}
		}
	}

	result := map[string]interface{}{
		"total": total,
		"slots": slot,
	}

	slog.Info("get steps success", "deviceID", deviceID, "date", date, "total", total)
	return result, nil
}

// ========== 用户相关方法 ==========

// GetUser 获取用户信息
func (c *SimpleServiceContainer) GetUser(ctx context.Context, userID uint) (*mxm.User, error) {
	user, err := c.repo.GetUserByID(userID)
	if err != nil {
		slog.Error("get user failed", "userID", userID, "error", err)
		return nil, fmt.Errorf("get user failed: %w", err)
	}
	return user, nil
}

// UpdateUser 更新用户信息
func (c *SimpleServiceContainer) UpdateUser(ctx context.Context, userID uint, updates map[string]interface{}) error {
	if err := c.repo.UpdateUser(userID, updates); err != nil {
		slog.Error("update user failed", "userID", userID, "error", err)
		return fmt.Errorf("update user failed: %w", err)
	}
	return nil
}

// GetOrCreateUserByOpenId 根据OpenID获取或创建用户
func (c *SimpleServiceContainer) GetOrCreateUserByOpenId(ctx context.Context, openid string) (*mxm.User, error) {
	user, err := c.repo.GetOrCreateUserByOpenId(openid)
	if err != nil {
		slog.Error("get or create user by openid failed", "openid", openid, "error", err)
		return nil, fmt.Errorf("get or create user by openid failed: %w", err)
	}
	return user, nil
}

// ========== 分享相关方法 ==========

// CreateShareMapping 创建分享映射
func (c *SimpleServiceContainer) CreateShareMapping(ctx context.Context, ownerUserID uint, targetUserID int, deviceID string) error {
	shareMap := &mxm.ShareMap{
		UserID:   targetUserID,
		DeviceID: deviceID,
	}

	if err := c.repo.CreateShareMapping(shareMap); err != nil {
		slog.Error("create share mapping failed", "ownerUserID", ownerUserID, "targetUserID", targetUserID, "deviceID", deviceID, "error", err)
		return fmt.Errorf("create share mapping failed: %w", err)
	}

	slog.Info("create share mapping success", "ownerUserID", ownerUserID, "targetUserID", targetUserID, "deviceID", deviceID)
	return nil
}

// MoveShareMapping 移除分享映射
func (c *SimpleServiceContainer) MoveShareMapping(ctx context.Context, targetUserID int, deviceID string) error {
	if err := c.repo.DeleteShareMapping(uint(targetUserID), deviceID); err != nil {
		slog.Error("remove share mapping failed", "targetUserID", targetUserID, "deviceID", deviceID, "error", err)
		return fmt.Errorf("remove share mapping failed: %w", err)
	}

	slog.Info("remove share mapping success", "targetUserID", targetUserID, "deviceID", deviceID)
	return nil
}

// GetShareMappingsByUserID 根据用户ID获取分享映射
func (c *SimpleServiceContainer) GetShareMappingsByUserID(ctx context.Context, userID int) ([]*mxm.ShareMap, error) {
	mappings, err := c.repo.GetShareMappingByUserID(userID)
	if err != nil {
		slog.Error("get share mappings by user id failed", "userID", userID, "error", err)
		return nil, fmt.Errorf("get share mappings by user id failed: %w", err)
	}

	slog.Info("get share mappings by user id success", "userID", userID, "count", len(mappings))
	return mappings, nil
}

// GetShareMappingsByDeviceID 根据设备ID获取分享映射
func (c *SimpleServiceContainer) GetShareMappingsByDeviceID(ctx context.Context, deviceID string) ([]*mxm.ShareMap, error) {
	mappings, err := c.repo.GetShareMappingByDeviceID(deviceID)
	if err != nil {
		slog.Error("get share mappings by device id failed", "deviceID", deviceID, "error", err)
		return nil, fmt.Errorf("get share mappings by device id failed: %w", err)
	}

	slog.Info("get share mappings by device id success", "deviceID", deviceID, "count", len(mappings))
	return mappings, nil
}

// ========== 反馈相关方法 ==========

// AddFeedback 添加用户反馈
func (c *SimpleServiceContainer) AddFeedback(ctx context.Context, feedback *mxm.Feedback) error {
	if err := c.repo.CreateFeedback(feedback); err != nil {
		slog.Error("add feedback failed", "feedback", feedback, "error", err)
		return fmt.Errorf("add feedback failed: %w", err)
	}

	slog.Info("add feedback success", "feedback", feedback)
	return nil
}

// GetFeedbacksByUserId 根据用户ID获取反馈列表
func (c *SimpleServiceContainer) GetFeedbacksByUserId(ctx context.Context, userID uint) ([]*mxm.Feedback, error) {
	feedbacks, err := c.repo.GetFeedbacksByUserID(userID, 100, 0)
	if err != nil {
		slog.Error("get feedbacks by user id failed", "userID", userID, "error", err)
		return nil, fmt.Errorf("get feedbacks by user id failed: %w", err)
	}

	slog.Info("get feedbacks by user id success", "userID", userID, "count", len(feedbacks))
	return feedbacks, nil
}

// ========== 订单相关方法 ==========

// RenewDevice 设备续费
func (c *SimpleServiceContainer) RenewDevice(ctx context.Context, userID uint, deviceID string, amount int) (*mxm.Order, error) {
	order := &mxm.Order{
		OrderNo:     fmt.Sprintf("%d_%d", userID, time.Now().Unix()),
		Description: fmt.Sprintf("设备 %s 服务费用", deviceID),
		Amount:      amount,
		Currency:    "CNY",
		Status:      mxm.OrderStatusCreated,
		Attach:      deviceID,
	}

	if err := c.repo.CreateOrder(order); err != nil {
		slog.Error("create order failed", "userID", userID, "deviceID", deviceID, "amount", amount, "error", err)
		return nil, fmt.Errorf("create order failed: %w", err)
	}

	slog.Info("create order success", "order", order)
	return order, nil
}
