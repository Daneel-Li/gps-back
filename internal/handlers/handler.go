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

// SimpleHandler simplified handler
type SimpleHandler struct {
	services      *services.SimpleServiceContainer
	wsManager     *services.WSManager
	jwtGenerator  services.JWTService
	wechatService services.WechatService
}

// NewSimpleHandler creates simplified handler
func NewSimpleHandler(services *services.SimpleServiceContainer, wsManager *services.WSManager,
	jwtGenerator services.JWTService, wechatService services.WechatService) *SimpleHandler {
	return &SimpleHandler{
		services:      services,
		wsManager:     wsManager,
		jwtGenerator:  jwtGenerator,
		wechatService: wechatService,
	}
}

// GetDevice gets device information
func (h *SimpleHandler) GetDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := mux.Vars(r)["device_id"]
	userID := h.getUserIDFromContext(ctx)

	if deviceID != "" {
		// Get single device
		device, err := h.services.GetDeviceByID(ctx, deviceID, userID)
		if err != nil {
			h.handleError(w, err)
			return
		}
		utils.WriteHttpResponse(w, http.StatusOK, device)
		return
	}

	// Get all devices of user
	devices, err := h.services.GetDevicesByUser(ctx, userID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, devices)
}

// BindDevice binds device
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

	// Activate device
	if err := h.services.ActivateDevice(ctx, req.OriginSN, req.DeviceType); err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, map[string]string{
		"message": "Device bound successfully",
	})
}

// UnbindDevice unbinds device
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

// GetUser gets user information
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

// UpdateUser updates user information
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

// handleError unified error handling
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

// getUserIDFromContext gets user ID from context
func (h *SimpleHandler) getUserIDFromContext(ctx context.Context) uint {
	if userID, ok := ctx.Value("userid").(uint); ok {
		return userID
	}
	return 0
}

// Error type judgment functions
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

// GetTrack gets track information
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

// GetReportInterval gets device reporting interval
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

// GetAutoPower gets scheduled power on/off parameters
func (h *SimpleHandler) GetAutoPower(w http.ResponseWriter, r *http.Request) {
	deviceId := mux.Vars(r)["device_id"]

	params, err := h.services.GetAutoPowerParams(r.Context(), deviceId)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, params)
}

// GetSafeRegions gets safe regions
func (h *SimpleHandler) GetSafeRegions(w http.ResponseWriter, r *http.Request) {
	deviceId := mux.Vars(r)["device_id"]

	// Get user ID
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

// PutSafeRegion sets safe region
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

// GetAlarms gets alarm list
func (h *SimpleHandler) GetAlarms(w http.ResponseWriter, r *http.Request) {
	deviceId := mux.Vars(r)["device_id"]

	alarms, err := h.services.GetAlarmsByDeviceID(r.Context(), deviceId)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, alarms)
}

// Command handles device commands
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

// GetSteps gets step count data
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

// GetProfile gets device profile
func (h *SimpleHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["device_id"]

	profile, err := h.services.GetDeviceProfile(r.Context(), deviceID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, profile)
}

// UpdateProfile updates device profile
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

// UploadFileHandler handles file upload
func (h *SimpleHandler) UploadFileHandler(w http.ResponseWriter, r *http.Request) {
	target := mux.Vars(r)["target"] // target can be devices/users
	id := mux.Vars(r)["id"]         // corresponds to device_id/user_id

	// Limit request body size to 10MB
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

	// Get uploaded file
	file, handler, err := r.FormFile("avatar")
	if err != nil {
		http.Error(w, "Error Retrieving the File", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	slog.Debug(fmt.Sprintf("Uploaded File: %s", handler.Filename))
	slog.Debug(fmt.Sprintf("File Size: %d", handler.Size))

	// Define file save path
	avatarDir := filepath.Join(config.GetConfig().AvatarPath, target)
	if err := os.MkdirAll(avatarDir, 0755); err != nil {
		slog.Error("无法创建存储目录", "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}

	// Generate unique filename
	ext := filepath.Ext(handler.Filename)
	if ext == "" {
		ext = ".jpg" // Default extension
	}
	filename := fmt.Sprintf("%s_%d%s", id, time.Now().Unix(), ext)
	filePath := filepath.Join(avatarDir, filename)

	if err := saveFile(handler, filePath); err != nil {
		http.Error(w, "Unable to save file", http.StatusInternalServerError)
		return
	}

	// Update database record
	if err := h.services.UpdateAvatar(r.Context(), target, id, filename); err != nil {
		slog.Error("update avatar failed", "err", err)
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	// Return response
	utils.WriteHttpResponse(w, http.StatusOK, map[string]interface{}{
		"avatar_url": filename,
	})
}

// Save uploaded file
func saveFile(fileHeader *multipart.FileHeader, targetPath string) error {
	// Open file
	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	// Create target file
	dst, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// Copy file content
	_, err = io.Copy(dst, src)
	return err
}

// GetAvatar gets avatar
func (h *SimpleHandler) GetAvatar(w http.ResponseWriter, r *http.Request) {
	target := mux.Vars(r)["target"] // target can be devices/users
	id := mux.Vars(r)["id"]         // corresponds to device_id/user_id

	filename, err := h.services.GetAvatar(r.Context(), target, id)
	if err != nil {
		h.handleError(w, err)
		return
	}

	http.ServeFile(w, r, filepath.Join(config.GetConfig().AvatarPath, target, filename))
}

// AddFeedback adds feedback
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

// GetFeedbacks gets feedback list
func (h *SimpleHandler) GetFeedbacks(w http.ResponseWriter, r *http.Request) {
	userid := h.getUserIDFromContext(r.Context())

	feedbacks, err := h.services.GetFeedbacksByUserId(r.Context(), userid)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, feedbacks)
}

// UpgradeWS handles WebSocket upgrade
func (h *SimpleHandler) UpgradeWS(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Adjust according to security requirements
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	slog.Debug("新建连接", "connptr", fmt.Sprintf("%p", conn))
	if err != nil {
		return
	}

	// Authenticate and register on first frame
	h.wsManager.AuthenticateAndRegister(conn)
}

// PaySuccNotify payment success callback notification
func (h *SimpleHandler) PaySuccNotify(w http.ResponseWriter, r *http.Request) {
	slog.Debug("收到微信支付回调通知", "body", r.Body)
	// TODO: Implement payment callback processing logic
	utils.WriteHttpResponse(w, http.StatusOK, "success")
}

// Renew subscription recharge
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

// CreateShareMapping creates device sharing
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

// MoveShareMapping moves device sharing
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

// GetShareMappings gets sharing mapping list
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

// EnrollDeviceHandler handles device enrollment
func (h *SimpleHandler) EnrollDeviceHandler(w http.ResponseWriter, r *http.Request) {
	userid := h.getUserIDFromContext(r.Context())

	// Check if user has enrollment permission
	if !h.services.IsEnrollAdmin(r.Context(), userid) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var req mxm.DeviceEnrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request: invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.SerialNumber == "" || req.Model == "" {
		http.Error(w, "Bad request: serial_number and model are required", http.StatusBadRequest)
		return
	}

	// Process device enrollment logic
	if err := h.services.EnrollDevice(r.Context(), req); err != nil {
		h.handleError(w, err)
		return
	}

	// Return success response
	utils.WriteHttpResponse(w, http.StatusOK, "success")
}

// LoginHandler handles mini program login requests
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

	// Call WeChat interface to get session_key and openid
	session, err := h.wechatService.GetSession(req.Code)
	if err != nil {
		http.Error(w, "获取 session 失败", http.StatusInternalServerError)
		return
	}

	// If WeChat interface returns error
	if session.Errcode != 0 {
		http.Error(w, fmt.Sprintf("微信接口错误: %d, %s", session.Errcode, session.Errmsg), http.StatusInternalServerError)
		return
	}

	// Query or create user
	user, err := h.services.GetOrCreateUserByOpenId(r.Context(), session.OpenID)
	if err != nil {
		slog.Error("GetOrCreateUserByOpenId failed", "openid", session.OpenID, "err", err.Error())
		http.Error(w, fmt.Sprintf("获取用户信息失败: %v", err.Error()), http.StatusInternalServerError)
		return
	}

	// Generate JWT token
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

// getUserIDFromContext gets user ID from request context
func getUserIDFromContext(ctx context.Context) (uint, error) {
	userID, ok := ctx.Value("userid").(uint)
	if !ok {
		return 0, fmt.Errorf("user ID not found in context")
	}
	return userID, nil
}
