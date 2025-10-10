package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mxm "github.com/Daneel-Li/gps-back/internal/models"
	"github.com/Daneel-Li/gps-back/pkg/utils"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSimpleServiceContainer mock service container
type MockSimpleServiceContainer struct {
	mock.Mock
}

func (m *MockSimpleServiceContainer) GetDeviceByID(ctx context.Context, deviceID string, userID uint) (*mxm.Device, error) {
	args := m.Called(ctx, deviceID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mxm.Device), args.Error(1)
}

func (m *MockSimpleServiceContainer) GetDevicesByUser(ctx context.Context, userID uint) ([]*mxm.Device, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mxm.Device), args.Error(1)
}

func (m *MockSimpleServiceContainer) BindDevice(ctx context.Context, userID uint, originSN, deviceType, label string) error {
	args := m.Called(ctx, userID, originSN, deviceType, label)
	return args.Error(0)
}

func (m *MockSimpleServiceContainer) ActivateDevice(ctx context.Context, originSN, deviceType string) error {
	args := m.Called(ctx, originSN, deviceType)
	return args.Error(0)
}

func (m *MockSimpleServiceContainer) UnbindDevice(ctx context.Context, userID uint, deviceID string) error {
	args := m.Called(ctx, userID, deviceID)
	return args.Error(0)
}

func (m *MockSimpleServiceContainer) GetUser(ctx context.Context, userID uint) (*mxm.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mxm.User), args.Error(1)
}

func (m *MockSimpleServiceContainer) UpdateUser(ctx context.Context, userID uint, updates map[string]interface{}) error {
	args := m.Called(ctx, userID, updates)
	return args.Error(0)
}

func (m *MockSimpleServiceContainer) GetDeviceTrack(ctx context.Context, deviceID, startTime, endTime string, types []string) (interface{}, error) {
	args := m.Called(ctx, deviceID, startTime, endTime, types)
	return args.Get(0), args.Error(1)
}

func (m *MockSimpleServiceContainer) ExecuteCommand(ctx context.Context, deviceID, action string, args []string, terminalKey string) (string, error) {
	mockArgs := m.Called(ctx, deviceID, action, args, terminalKey)
	return mockArgs.String(0), mockArgs.Error(1)
}

func (m *MockSimpleServiceContainer) GetOrCreateUserByOpenId(ctx context.Context, openID string) (*mxm.User, error) {
	args := m.Called(ctx, openID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mxm.User), args.Error(1)
}

// MockWSManager mock WebSocket manager
type MockWSManager struct {
	mock.Mock
}

func (m *MockWSManager) AuthenticateAndRegister(conn interface{}) {
	m.Called(conn)
}

// MockJWTService mock JWT service
type MockJWTService struct {
	mock.Mock
}

func (m *MockJWTService) GenerateToken(user mxm.User) (string, error) {
	args := m.Called(user)
	return args.String(0), args.Error(1)
}

func (m *MockJWTService) ValidateToken(tokenString string) (uint, error) {
	args := m.Called(tokenString)
	return uint(args.Int(0)), args.Error(1)
}

// MockWechatService mock WeChat service
type MockWechatService struct {
	mock.Mock
}

func (m *MockWechatService) GetSession(code string) (*mxm.SessionResponse, error) {
	args := m.Called(code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mxm.SessionResponse), args.Error(1)
}

// TestableSimpleHandler 用于测试的处理器结构
type TestableSimpleHandler struct {
	services      *MockSimpleServiceContainer
	wsManager     *MockWSManager
	jwtGenerator  *MockJWTService
	wechatService *MockWechatService
}

// 实现 SimpleHandler 的辅助方法
func (h *TestableSimpleHandler) getUserIDFromContext(ctx context.Context) uint {
	if userID, ok := ctx.Value("userid").(uint); ok {
		return userID
	}
	return 0
}

func (h *TestableSimpleHandler) handleError(w http.ResponseWriter, err error) {
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

func (h *TestableSimpleHandler) isNotFoundError(err error) bool {
	return err != nil && (err.Error() == "device not found" || err.Error() == "user not found")
}

func (h *TestableSimpleHandler) isPermissionError(err error) bool {
	return err != nil && err.Error() == "permission denied"
}

func (h *TestableSimpleHandler) isValidationError(err error) bool {
	return err != nil && (err.Error() == "device is invalid or already bound" ||
		err.Error() == "Invalid request body")
}

// 实现所有需要测试的处理器方法
func (h *TestableSimpleHandler) GetDevice(w http.ResponseWriter, r *http.Request) {
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

func (h *TestableSimpleHandler) BindDevice(w http.ResponseWriter, r *http.Request) {
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

func (h *TestableSimpleHandler) UnbindDevice(w http.ResponseWriter, r *http.Request) {
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

func (h *TestableSimpleHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := h.getUserIDFromContext(ctx)

	user, err := h.services.GetUser(ctx, userID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.WriteHttpResponse(w, http.StatusOK, user)
}

func (h *TestableSimpleHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
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

func (h *TestableSimpleHandler) GetTrack(w http.ResponseWriter, r *http.Request) {
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

func (h *TestableSimpleHandler) Command(w http.ResponseWriter, r *http.Request) {
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

	if action == "" || req.Args == "" {
		http.Error(w, "action and args are required", http.StatusBadRequest)
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

func (h *TestableSimpleHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
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

// 创建测试处理器的辅助函数
func createTestSimpleHandler() (*TestableSimpleHandler, *MockSimpleServiceContainer, *MockWSManager, *MockJWTService, *MockWechatService) {
	mockServices := &MockSimpleServiceContainer{}
	mockWSManager := &MockWSManager{}
	mockJWTService := &MockJWTService{}
	mockWechatService := &MockWechatService{}

	handler := &TestableSimpleHandler{
		services:      mockServices,
		wsManager:     mockWSManager,
		jwtGenerator:  mockJWTService,
		wechatService: mockWechatService,
	}

	return handler, mockServices, mockWSManager, mockJWTService, mockWechatService
}

func createRequestWithContext(method, url string, body interface{}, userID uint) *http.Request {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBody)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, url, reqBody)
	req.Header.Set("Content-Type", "application/json")

	// 添加用户ID到上下文
	ctx := context.WithValue(req.Context(), "userid", userID)
	return req.WithContext(ctx)
}

// TestGetDevice 测试获取设备信息
func TestGetDevice(t *testing.T) {
	handler, mockServices, _, _, _ := createTestSimpleHandler()

	t.Run("GetSingleDevice", func(t *testing.T) {
		// Prepare test data
		deviceID := "test-device-id"
		userID := uint(1)
		expectedDevice := &mxm.Device{
			ID:       &deviceID,
			OriginSN: stringPtr("123456"),
			Type:     stringPtr("GPS"),
			UserID:   &userID,
		}

		// Set mock expectations
		mockServices.On("GetDeviceByID", mock.Anything, deviceID, userID).Return(expectedDevice, nil)

		// Create request
		req := createRequestWithContext("GET", "/device/"+deviceID, nil, userID)
		req = mux.SetURLVars(req, map[string]string{"device_id": deviceID})
		w := httptest.NewRecorder()

		// Execute test
		handler.GetDevice(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		mockServices.AssertExpectations(t)
	})

	t.Run("GetAllDevices", func(t *testing.T) {
		// Prepare test data
		userID := uint(1)
		expectedDevices := []*mxm.Device{
			{ID: stringPtr("device1"), UserID: &userID},
			{ID: stringPtr("device2"), UserID: &userID},
		}

		// Set mock expectations
		mockServices.On("GetDevicesByUser", mock.Anything, userID).Return(expectedDevices, nil)

		// Create request
		req := createRequestWithContext("GET", "/device", nil, userID)
		w := httptest.NewRecorder()

		// Execute test
		handler.GetDevice(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		mockServices.AssertExpectations(t)
	})

	t.Run("DeviceNotFound", func(t *testing.T) {
		deviceID := "non-existent-device"
		userID := uint(1)

		// Set mock expectations返回错误
		mockServices.On("GetDeviceByID", mock.Anything, deviceID, userID).Return(nil, errors.New("device not found"))

		// Create request
		req := createRequestWithContext("GET", "/device/"+deviceID, nil, userID)
		req = mux.SetURLVars(req, map[string]string{"device_id": deviceID})
		w := httptest.NewRecorder()

		// Execute test
		handler.GetDevice(w, req)

		// Verify result
		assert.Equal(t, http.StatusNotFound, w.Code)
		mockServices.AssertExpectations(t)
	})
}

// TestBindDevice 测试绑定设备
func TestBindDevice(t *testing.T) {
	handler, mockServices, _, _, _ := createTestSimpleHandler()

	t.Run("SuccessfulBind", func(t *testing.T) {
		userID := uint(1)
		bindReq := map[string]string{
			"origin_sn":   "123456",
			"device_type": "GPS",
			"label":       "My Device",
		}

		// Set mock expectations
		mockServices.On("BindDevice", mock.Anything, userID, "123456", "GPS", "My Device").Return(nil)
		mockServices.On("ActivateDevice", mock.Anything, "123456", "GPS").Return(nil)

		// Create request
		req := createRequestWithContext("POST", "/device/bind", bindReq, userID)
		w := httptest.NewRecorder()

		// Execute test
		handler.BindDevice(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		mockServices.AssertExpectations(t)
	})

	t.Run("InvalidRequestBody", func(t *testing.T) {
		userID := uint(1)

		// 创建无效的请求体
		req := httptest.NewRequest("POST", "/device/bind", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "userid", userID)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		// Execute test
		handler.BindDevice(w, req)

		// Verify result
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("BindDeviceError", func(t *testing.T) {
		// 创建新的 mock 服务以避免冲突
		handler, mockServices, _, _, _ := createTestSimpleHandler()
		userID := uint(1)
		bindReq := map[string]string{
			"origin_sn":   "123456",
			"device_type": "GPS",
			"label":       "My Device",
		}

		// Set mock expectations返回错误
		mockServices.On("BindDevice", mock.Anything, userID, "123456", "GPS", "My Device").Return(errors.New("device is invalid or already bound"))

		// Create request
		req := createRequestWithContext("POST", "/device/bind", bindReq, userID)
		w := httptest.NewRecorder()

		// Execute test
		handler.BindDevice(w, req)

		// Verify result
		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockServices.AssertExpectations(t)
	})
}

// TestUnbindDevice 测试解绑设备
func TestUnbindDevice(t *testing.T) {
	handler, mockServices, _, _, _ := createTestSimpleHandler()

	t.Run("SuccessfulUnbind", func(t *testing.T) {
		deviceID := "test-device-id"
		userID := uint(1)

		// Set mock expectations
		mockServices.On("UnbindDevice", mock.Anything, userID, deviceID).Return(nil)

		// Create request
		req := createRequestWithContext("DELETE", "/device/"+deviceID, nil, userID)
		req = mux.SetURLVars(req, map[string]string{"device_id": deviceID})
		w := httptest.NewRecorder()

		// Execute test
		handler.UnbindDevice(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		mockServices.AssertExpectations(t)
	})

	t.Run("UnbindError", func(t *testing.T) {
		// 创建新的 mock 服务以避免冲突
		handler, mockServices, _, _, _ := createTestSimpleHandler()
		deviceID := "test-device-id"
		userID := uint(1)

		// Set mock expectations返回错误
		mockServices.On("UnbindDevice", mock.Anything, userID, deviceID).Return(errors.New("permission denied"))

		// Create request
		req := createRequestWithContext("DELETE", "/device/"+deviceID, nil, userID)
		req = mux.SetURLVars(req, map[string]string{"device_id": deviceID})
		w := httptest.NewRecorder()

		// Execute test
		handler.UnbindDevice(w, req)

		// Verify result
		assert.Equal(t, http.StatusForbidden, w.Code)
		mockServices.AssertExpectations(t)
	})
}

// TestGetUser 测试获取用户信息
func TestGetUser(t *testing.T) {
	handler, mockServices, _, _, _ := createTestSimpleHandler()

	t.Run("SuccessfulGetUser", func(t *testing.T) {
		userID := uint(1)
		expectedUser := &mxm.User{
			ID:       userID,
			OpenID:   "test-openid",
			Nickname: "Test User",
		}

		// Set mock expectations
		mockServices.On("GetUser", mock.Anything, userID).Return(expectedUser, nil)

		// Create request
		req := createRequestWithContext("GET", "/user", nil, userID)
		w := httptest.NewRecorder()

		// Execute test
		handler.GetUser(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		mockServices.AssertExpectations(t)
	})

	t.Run("UserNotFound", func(t *testing.T) {
		userID := uint(999)

		// Set mock expectations返回错误
		mockServices.On("GetUser", mock.Anything, userID).Return(nil, errors.New("user not found"))

		// Create request
		req := createRequestWithContext("GET", "/user", nil, userID)
		w := httptest.NewRecorder()

		// Execute test
		handler.GetUser(w, req)

		// Verify result
		assert.Equal(t, http.StatusNotFound, w.Code)
		mockServices.AssertExpectations(t)
	})
}

// TestUpdateUser 测试更新用户信息
func TestUpdateUser(t *testing.T) {
	handler, mockServices, _, _, _ := createTestSimpleHandler()

	t.Run("SuccessfulUpdate", func(t *testing.T) {
		userID := uint(1)
		updates := map[string]interface{}{
			"nickname": "New Nickname",
		}

		// Set mock expectations
		mockServices.On("UpdateUser", mock.Anything, userID, updates).Return(nil)

		// Create request
		req := createRequestWithContext("PUT", "/user", updates, userID)
		w := httptest.NewRecorder()

		// Execute test
		handler.UpdateUser(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		mockServices.AssertExpectations(t)
	})

	t.Run("InvalidRequestBody", func(t *testing.T) {
		userID := uint(1)

		// 创建无效的请求体
		req := httptest.NewRequest("PUT", "/user", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "userid", userID)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		// Execute test
		handler.UpdateUser(w, req)

		// Verify result
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestGetTrack 测试获取轨迹信息
func TestGetTrack(t *testing.T) {
	handler, mockServices, _, _, _ := createTestSimpleHandler()

	t.Run("SuccessfulGetTrack", func(t *testing.T) {
		deviceID := "test-device-id"
		startTime := "2023-01-01 00:00:00"
		endTime := "2023-01-01 23:59:59"
		expectedTrack := []interface{}{
			map[string]interface{}{"lat": 39.9, "lng": 116.4},
		}

		// Set mock expectations - 使用实际会被解析的时间格式
		mockServices.On("GetDeviceTrack", mock.Anything, deviceID, startTime, endTime, []string{"GPS", "WIFI", "LBS"}).Return(expectedTrack, nil)

		// Create request - 使用 URL 编码的时间格式
		url := fmt.Sprintf("/device/%s/track?startTime=%s&endTime=%s", deviceID, "2023-01-01+00:00:00", "2023-01-01+23:59:59")
		req := httptest.NewRequest("GET", url, nil)
		req = mux.SetURLVars(req, map[string]string{"device_id": deviceID})
		w := httptest.NewRecorder()

		// Execute test
		handler.GetTrack(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		mockServices.AssertExpectations(t)
	})

	t.Run("MissingParameters", func(t *testing.T) {
		deviceID := "test-device-id"

		// 创建缺少参数的请求
		req := httptest.NewRequest("GET", "/device/"+deviceID+"/track", nil)
		req = mux.SetURLVars(req, map[string]string{"device_id": deviceID})
		w := httptest.NewRecorder()

		// Execute test
		handler.GetTrack(w, req)

		// Verify result
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestCommand 测试设备命令处理
func TestCommand(t *testing.T) {
	handler, mockServices, _, _, _ := createTestSimpleHandler()

	t.Run("SuccessfulCommand", func(t *testing.T) {
		deviceID := "test-device-id"
		commandReq := map[string]string{
			"action": "LOCATE",
			"args":   "now",
		}
		expectedCommandID := "cmd-123"

		// Set mock expectations
		mockServices.On("ExecuteCommand", mock.Anything, deviceID, "LOCATE", []string{"now"}, "").Return(expectedCommandID, nil)

		// Create request
		req := createRequestWithContext("POST", "/device/"+deviceID+"/command", commandReq, uint(1))
		req = mux.SetURLVars(req, map[string]string{"device_id": deviceID})
		w := httptest.NewRecorder()

		// Execute test
		handler.Command(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		mockServices.AssertExpectations(t)
	})

	t.Run("MissingAction", func(t *testing.T) {
		deviceID := "test-device-id"
		commandReq := map[string]string{
			"args": "now",
		}

		// Create request
		req := createRequestWithContext("POST", "/device/"+deviceID+"/command", commandReq, uint(1))
		req = mux.SetURLVars(req, map[string]string{"device_id": deviceID})
		w := httptest.NewRecorder()

		// Execute test
		handler.Command(w, req)

		// Verify result
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestLoginHandler 测试登录处理
func TestLoginHandler(t *testing.T) {
	handler, mockServices, _, mockJWTService, mockWechatService := createTestSimpleHandler()

	t.Run("SuccessfulLogin", func(t *testing.T) {
		loginReq := map[string]string{
			"code": "test-code",
		}

		wechatSession := &mxm.SessionResponse{
			OpenID:     "test-openid",
			SessionKey: "test-session-key",
			Errcode:    0,
		}

		user := &mxm.User{
			ID:       1,
			OpenID:   "test-openid",
			Nickname: "Test User",
		}

		token := "jwt-token"

		// Set mock expectations
		mockWechatService.On("GetSession", "test-code").Return(wechatSession, nil)
		mockServices.On("GetOrCreateUserByOpenId", mock.Anything, "test-openid").Return(user, nil)
		mockJWTService.On("GenerateToken", *user).Return(token, nil)

		// Create request
		req := createRequestWithContext("POST", "/login", loginReq, uint(0))
		w := httptest.NewRecorder()

		// Execute test
		handler.LoginHandler(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		mockWechatService.AssertExpectations(t)
		mockServices.AssertExpectations(t)
		mockJWTService.AssertExpectations(t)
	})

	t.Run("MissingCode", func(t *testing.T) {
		loginReq := map[string]string{}

		// Create request
		req := createRequestWithContext("POST", "/login", loginReq, uint(0))
		w := httptest.NewRecorder()

		// Execute test
		handler.LoginHandler(w, req)

		// Verify result
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("WechatError", func(t *testing.T) {
		loginReq := map[string]string{
			"code": "test-code3",
		}

		wechatSession := &mxm.SessionResponse{
			Errcode: 40013,
			Errmsg:  "invalid code",
		}

		// Set mock expectations
		mockWechatService.On("GetSession", "test-code3").Return(wechatSession, nil)
		// Create request
		req := createRequestWithContext("POST", "/login", loginReq, uint(0))
		w := httptest.NewRecorder()

		// Execute test
		handler.LoginHandler(w, req)

		// Verify result
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockWechatService.AssertExpectations(t)
	})
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	handler, _, _, _, _ := createTestSimpleHandler()

	tests := []struct {
		name           string
		error          error
		expectedStatus int
	}{
		{
			name:           "NotFoundError",
			error:          errors.New("device not found"),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "PermissionError",
			error:          errors.New("permission denied"),
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "ValidationError",
			error:          errors.New("device is invalid or already bound"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "InternalError",
			error:          errors.New("database connection failed"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.handleError(w, tt.error)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestGetUserIDFromContext 测试从上下文获取用户ID
func TestGetUserIDFromContext(t *testing.T) {
	handler, _, _, _, _ := createTestSimpleHandler()

	t.Run("ValidUserID", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "userid", uint(123))
		userID := handler.getUserIDFromContext(ctx)
		assert.Equal(t, uint(123), userID)
	})

	t.Run("NoUserID", func(t *testing.T) {
		ctx := context.Background()
		userID := handler.getUserIDFromContext(ctx)
		assert.Equal(t, uint(0), userID)
	})

	t.Run("InvalidUserIDType", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "userid", "invalid")
		userID := handler.getUserIDFromContext(ctx)
		assert.Equal(t, uint(0), userID)
	})
}

// TestErrorTypeChecking 测试错误类型检查
func TestErrorTypeChecking(t *testing.T) {
	handler, _, _, _, _ := createTestSimpleHandler()

	t.Run("IsNotFoundError", func(t *testing.T) {
		assert.True(t, handler.isNotFoundError(errors.New("device not found")))
		assert.True(t, handler.isNotFoundError(errors.New("user not found")))
		assert.False(t, handler.isNotFoundError(errors.New("other error")))
		assert.False(t, handler.isNotFoundError(nil))
	})

	t.Run("IsPermissionError", func(t *testing.T) {
		assert.True(t, handler.isPermissionError(errors.New("permission denied")))
		assert.False(t, handler.isPermissionError(errors.New("other error")))
		assert.False(t, handler.isPermissionError(nil))
	})

	t.Run("IsValidationError", func(t *testing.T) {
		assert.True(t, handler.isValidationError(errors.New("device is invalid or already bound")))
		assert.True(t, handler.isValidationError(errors.New("Invalid request body")))
		assert.False(t, handler.isValidationError(errors.New("other error")))
		assert.False(t, handler.isValidationError(nil))
	})
}

// 辅助函数
func stringPtr(s string) *string {
	return &s
}
