package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Daneel-Li/gps-back/internal/config"
	mxm "github.com/Daneel-Li/gps-back/internal/models"
	"github.com/Daneel-Li/gps-back/internal/services"
	"github.com/Daneel-Li/gps-back/pkg/utils"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// SimpleHandler 简化的处理器
type SimpleHandler struct {
	services      *services.SimpleServiceContainer
	wsManager     *services.WSManager
	jwtGenerator  services.JWTService
	wechatService services.WechatService
}

// NewSimpleHandler 创建简化的处理器
func NewSimpleHandler(services *services.SimpleServiceContainer, wsManager *services.WSManager,
	jwtGenerator services.JWTService, wechatService services.WechatService) *SimpleHandler {
	return &SimpleHandler{
		services:      services,
		wsManager:     wsManager,
		jwtGenerator:  jwtGenerator,
		wechatService: wechatService,
	}
}

// GetDevice 获取设备信息
func (h *SimpleHandler) GetDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := mux.Vars(r)["device_id"]
	userID := h.getUserIDFromContext(ctx)

	if deviceID != "" {
		// 获取单个设备
		device, err := h.services.GetDeviceByID(ctx, deviceID, userID)
		if err != nil {
			h.handleError(w, err)
			return
		}
		utils.WriteHttpResponse(w, http.StatusOK, device)
		return
	}

	// 获取用户所有设备
	devices, err := h.services.GetDevicesByUser(ctx, userID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, devices)
}

// BindDevice 绑定设备
func (h *SimpleHandler) BindDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := h.getUserIDFromContext(ctx)

	var req struct {
		OriginSN   string `json:"origin_sn"`
		DeviceType string `json:"device_type"`
		Label      string `json:"label"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.services.BindDevice(ctx, userID, req.OriginSN, req.DeviceType, req.Label); err != nil {
		h.handleError(w, err)
		return
	}

	// 激活设备
	if err := h.services.ActivateDevice(ctx, req.OriginSN, req.DeviceType); err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, map[string]string{
		"message": "Device bound successfully",
	})
}

// UnbindDevice 解绑设备
func (h *SimpleHandler) UnbindDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := mux.Vars(r)["device_id"]
	userID := h.getUserIDFromContext(ctx)

	if err := h.services.UnbindDevice(ctx, userID, deviceID); err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, map[string]string{
		"message": "Device unbound successfully",
	})
}

// GetUser 获取用户信息
func (h *SimpleHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := h.getUserIDFromContext(ctx)

	user, err := h.services.GetUser(ctx, userID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, user)
}

// UpdateUser 更新用户信息
func (h *SimpleHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := h.getUserIDFromContext(ctx)

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.services.UpdateUser(ctx, userID, updates); err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, map[string]string{
		"message": "User updated successfully",
	})
}

// handleError 统一错误处理
func (h *SimpleHandler) handleError(w http.ResponseWriter, err error) {
	slog.Error("Handler error", "error", err)

	switch {
	case h.isNotFoundError(err):
		http.Error(w, "Resource not found", http.StatusNotFound)
	case h.isPermissionError(err):
		http.Error(w, "Permission denied", http.StatusForbidden)
	case h.isValidationError(err):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// getUserIDFromContext 从上下文获取用户ID
func (h *SimpleHandler) getUserIDFromContext(ctx context.Context) uint {
	if userID, ok := ctx.Value("userid").(uint); ok {
		return userID
	}
	return 0
}

// 错误类型判断函数
func (h *SimpleHandler) isNotFoundError(err error) bool {
	return err != nil && (err.Error() == "device not found" || err.Error() == "user not found")
}

func (h *SimpleHandler) isPermissionError(err error) bool {
	return err != nil && err.Error() == "permission denied"
}

func (h *SimpleHandler) isValidationError(err error) bool {
	return err != nil && (err.Error() == "device is invalid or already bound" ||
		err.Error() == "Invalid request body")
}

// GetTrack 获取轨迹信息
func (h *SimpleHandler) GetTrack(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	deviceId := mux.Vars(r)["device_id"]
	startTime := query.Get("startTime")
	endTime := query.Get("endTime")
	typeList := query.Get("typeList")

	if deviceId == "" || startTime == "" || endTime == "" {
		http.Error(w, "needs args: device_id,startTime,endTime", http.StatusBadRequest)
		return
	}

	if typeList == "" {
		typeList = "GPS,WIFI,LBS"
	}
	types := strings.Split(typeList, ",")

	track, err := h.services.GetDeviceTrack(r.Context(), deviceId, startTime, endTime, types)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, map[string]interface{}{"code": 0, "data": track})
}

// GetReportInterval 获取设备上报间隔
func (h *SimpleHandler) GetReportInterval(w http.ResponseWriter, r *http.Request) {
	deviceId := mux.Vars(r)["device_id"]

	interval, err := h.services.GetReportInterval(r.Context(), deviceId)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, map[string]interface{}{
		"report_interval": interval,
	})
}

// GetAutoPower 获取定时开关机参数
func (h *SimpleHandler) GetAutoPower(w http.ResponseWriter, r *http.Request) {
	deviceId := mux.Vars(r)["device_id"]

	params, err := h.services.GetAutoPowerParams(r.Context(), deviceId)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, params)
}

// GetSafeRegions 获取安全区域
func (h *SimpleHandler) GetSafeRegions(w http.ResponseWriter, r *http.Request) {
	deviceId := mux.Vars(r)["device_id"]

	// 获取用户ID
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	regions, err := h.services.GetSafeRegions(r.Context(), deviceId, userID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, regions)
}

// PutSafeRegion 设置安全区域
func (h *SimpleHandler) PutSafeRegion(w http.ResponseWriter, r *http.Request) {
	deviceId := mux.Vars(r)["device_id"]

	var region mxm.Region
	if err := json.NewDecoder(r.Body).Decode(&region); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.services.SetSafeRegion(r.Context(), deviceId, &region); err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, "success")
}

// GetAlarms 获取告警列表
func (h *SimpleHandler) GetAlarms(w http.ResponseWriter, r *http.Request) {
	deviceId := mux.Vars(r)["device_id"]

	alarms, err := h.services.GetAlarmsByDeviceID(r.Context(), deviceId)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, alarms)
}

// Command 设备命令处理
func (h *SimpleHandler) Command(w http.ResponseWriter, r *http.Request) {
	deviceId := mux.Vars(r)["device_id"]

	var req struct {
		Action      string `json:"action"`
		Args        string `json:"args"`
		TerminalKey string `json:"terminal_key,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	action := strings.ToUpper(req.Action)
	args := strings.Split(req.Args, " ")

	if action == "" {
		http.Error(w, "action and args(optional) are required", http.StatusBadRequest)
		return
	}

	commandID, err := h.services.ExecuteCommand(r.Context(), deviceId, action, args, req.TerminalKey)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, map[string]interface{}{
		"command_id": commandID,
	})
}

// GetSteps 获取步数数据
func (h *SimpleHandler) GetSteps(w http.ResponseWriter, r *http.Request) {
	deviceId := mux.Vars(r)["device_id"]
	date := r.URL.Query().Get("date")

	if date == "" {
		http.Error(w, "date parameter is required", http.StatusBadRequest)
		return
	}

	steps, err := h.services.GetSteps(r.Context(), deviceId, date)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, steps)
}

// GetProfile 获取设备档案
func (h *SimpleHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["device_id"]

	profile, err := h.services.GetDeviceProfile(r.Context(), deviceID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, profile)
}

// UpdateProfile 更新设备档案
func (h *SimpleHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["device_id"]

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.services.UpdateDeviceProfile(r.Context(), deviceID, updates); err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, "success")
}

// UploadFileHandler 文件上传处理
func (h *SimpleHandler) UploadFileHandler(w http.ResponseWriter, r *http.Request) {
	target := mux.Vars(r)["target"] // target可以是devices/users
	id := mux.Vars(r)["id"]         // 对应device_id/user_id

	// 限制请求体的大小为10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		http.Error(w, "只支持multipart/form-data格式", http.StatusBadRequest)
		return
	}

	// 获取上传的文件
	file, handler, err := r.FormFile("avatar")
	if err != nil {
		http.Error(w, "Error Retrieving the File", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	slog.Debug(fmt.Sprintf("Uploaded File: %s", handler.Filename))
	slog.Debug(fmt.Sprintf("File Size: %d", handler.Size))

	// 定义文件保存路径
	avatarDir := filepath.Join(config.GetConfig().AvatarPath, target)
	if err := os.MkdirAll(avatarDir, 0755); err != nil {
		slog.Error("无法创建存储目录", "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}

	// 生成唯一文件名
	ext := filepath.Ext(handler.Filename)
	if ext == "" {
		ext = ".jpg" // 默认扩展名
	}
	filename := fmt.Sprintf("%s_%d%s", id, time.Now().Unix(), ext)
	filePath := filepath.Join(avatarDir, filename)

	if err := saveFile(handler, filePath); err != nil {
		http.Error(w, "Unable to save file", http.StatusInternalServerError)
		return
	}

	// 更新数据库记录
	if err := h.services.UpdateAvatar(r.Context(), target, id, filename); err != nil {
		slog.Error("update avatar failed", "err", err)
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	// 返回响应
	utils.WriteHttpResponse(w, http.StatusOK, map[string]interface{}{
		"avatar_url": filename,
	})
}

// 保存上传的文件
func saveFile(fileHeader *multipart.FileHeader, targetPath string) error {
	// 打开文件
	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	// 创建目标文件
	dst, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// 复制文件内容
	_, err = io.Copy(dst, src)
	return err
}

// GetAvatar 获取头像
func (h *SimpleHandler) GetAvatar(w http.ResponseWriter, r *http.Request) {
	target := mux.Vars(r)["target"] // target可以是devices/users
	id := mux.Vars(r)["id"]         // 对应device_id/user_id

	filename, err := h.services.GetAvatar(r.Context(), target, id)
	if err != nil {
		h.handleError(w, err)
		return
	}

	http.ServeFile(w, r, filepath.Join(config.GetConfig().AvatarPath, target, filename))
}

// AddFeedback 添加反馈
func (h *SimpleHandler) AddFeedback(w http.ResponseWriter, r *http.Request) {
	userid := h.getUserIDFromContext(r.Context())

	var feedback mxm.Feedback
	if err := json.NewDecoder(r.Body).Decode(&feedback); err != nil {
		slog.Warn("error unmarshal body", "err", err)
		http.Error(w, "Error unmarshal body", http.StatusBadRequest)
		return
	}

	feedback.UserID = userid
	if err := h.services.AddFeedback(r.Context(), &feedback); err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, "success")
}

// GetFeedbacks 获取反馈列表
func (h *SimpleHandler) GetFeedbacks(w http.ResponseWriter, r *http.Request) {
	userid := h.getUserIDFromContext(r.Context())

	feedbacks, err := h.services.GetFeedbacksByUserId(r.Context(), userid)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, feedbacks)
}

// UpgradeWS WebSocket升级处理
func (h *SimpleHandler) UpgradeWS(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // 根据安全需求调整
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	slog.Debug("新建连接", "connptr", fmt.Sprintf("%p", conn))
	if err != nil {
		return
	}

	// 首帧鉴权并注册
	h.wsManager.AuthenticateAndRegister(conn)
}

// PaySuccNotify 支付成功回调通知
func (h *SimpleHandler) PaySuccNotify(w http.ResponseWriter, r *http.Request) {
	slog.Debug("收到微信支付回调通知", "body", r.Body)
	// TODO: 实现支付回调处理逻辑
	utils.WriteHttpResponse(w, http.StatusOK, "success")
}

// Renew 续费充值
func (h *SimpleHandler) Renew(w http.ResponseWriter, r *http.Request) {
	userid := h.getUserIDFromContext(r.Context())
	deviceId := mux.Vars(r)["device_id"]

	paymentReq, err := h.services.RenewDevice(r.Context(), userid, deviceId, 100)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, paymentReq)
}

// CreateShareMapping 创建设备分享
func (h *SimpleHandler) CreateShareMapping(w http.ResponseWriter, r *http.Request) {
	userid := h.getUserIDFromContext(r.Context())

	var req struct {
		UserID   float64 `json:"userId"`
		DeviceID string  `json:"deviceId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.services.CreateShareMapping(r.Context(), userid, int(req.UserID), req.DeviceID); err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, "success")
}

// MoveShareMapping 移动设备分享
func (h *SimpleHandler) MoveShareMapping(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   float64 `json:"userId"`
		DeviceID string  `json:"deviceId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.services.MoveShareMapping(r.Context(), int(req.UserID), req.DeviceID); err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, "success")
}

// GetShareMappings 获取分享映射列表
func (h *SimpleHandler) GetShareMappings(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	sUserId := query.Get("userId")
	deviceId := query.Get("deviceId")

	var mappings []*mxm.ShareMap
	var err error

	if sUserId != "" {
		userId, err := strconv.Atoi(sUserId)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}
		mappings, err = h.services.GetShareMappingsByUserID(r.Context(), userId)
	} else if deviceId != "" {
		mappings, err = h.services.GetShareMappingsByDeviceID(r.Context(), deviceId)
	} else {
		http.Error(w, "Need userId or deviceId parameter", http.StatusBadRequest)
		return
	}

	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, mappings)
}

// EnrollDeviceHandler 设备入库处理
func (h *SimpleHandler) EnrollDeviceHandler(w http.ResponseWriter, r *http.Request) {
	userid := h.getUserIDFromContext(r.Context())

	// 检查用户是否有入库权限
	if !h.services.IsEnrollAdmin(r.Context(), userid) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 解析请求体
	var req mxm.DeviceEnrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request: invalid JSON", http.StatusBadRequest)
		return
	}

	// 验证必填字段
	if req.SerialNumber == "" || req.Model == "" {
		http.Error(w, "Bad request: serial_number and model are required", http.StatusBadRequest)
		return
	}

	// 处理设备入库逻辑
	if err := h.services.EnrollDevice(r.Context(), req); err != nil {
		h.handleError(w, err)
		return
	}

	// 返回成功响应
	utils.WriteHttpResponse(w, http.StatusOK, "success")
}

// LoginHandler 处理小程序端的登录请求
func (h *SimpleHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("Login handler")

	var req struct {
		Code string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	if req.Code == "" {
		http.Error(w, "login needs param: code", http.StatusBadRequest)
		return
	}

	// 调用微信接口获取 session_key 和 openid
	session, err := h.wechatService.GetSession(req.Code)
	if err != nil {
		http.Error(w, "获取 session 失败", http.StatusInternalServerError)
		return
	}

	// 如果微信接口返回错误
	if session.Errcode != 0 {
		http.Error(w, fmt.Sprintf("微信接口错误: %d, %s", session.Errcode, session.Errmsg), http.StatusInternalServerError)
		return
	}

	// 查询或创建用户
	user, err := h.services.GetOrCreateUserByOpenId(r.Context(), session.OpenID)
	if err != nil {
		slog.Error("GetOrCreateUserByOpenId failed", "openid", session.OpenID, "err", err.Error())
		http.Error(w, fmt.Sprintf("获取用户信息失败: %v", err.Error()), http.StatusInternalServerError)
		return
	}

	// 生成JWT token
	token, err := h.jwtGenerator.GenerateToken(*user)
	if err != nil {
		http.Error(w, "登录失败: 无法生成令牌", http.StatusInternalServerError)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

// getUserIDFromContext 从请求上下文中获取用户ID
func getUserIDFromContext(ctx context.Context) (uint, error) {
	userID, ok := ctx.Value("userid").(uint)
	if !ok {
		return 0, fmt.Errorf("user ID not found in context")
	}
	return userID, nil
}
