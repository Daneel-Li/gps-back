package btt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Daneel-Li/gps-back/internal/config"
	mxm "github.com/Daneel-Li/gps-back/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMain(m *testing.M) {
	// Setup test environment
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)

	// 假设 config.json 在项目根目录下
	configPath := filepath.Join(dir, "../../../config.json")

	config.LoadConfig(configPath)

	os.Exit(m.Run())
}

// MockLocalCache 模拟本地缓存
type MockLocalCache struct {
	mock.Mock
}

func (m *MockLocalCache) GetCache(class string, key string) (interface{}, bool) {
	args := m.Called(class, key)
	return args.Get(0), args.Bool(1)
}

func (m *MockLocalCache) SetCache(class string, key string, value interface{}) {
	m.Called(class, key, value)
}

func (m *MockLocalCache) DeleteCache(key string, subKey interface{}) {
	m.Called(key, subKey)
}

func (m *MockLocalCache) ClearCache() {
	m.Called()
}

func (m *MockLocalCache) Get(key string) (interface{}, bool) {
	args := m.Called(key)
	return args.Get(0), args.Bool(1)
}

func (m *MockLocalCache) Set(key string, value interface{}) {
	m.Called(key, value)
}

// MockDeviceStatusFactory 模拟设备状态工厂
type MockDeviceStatusFactory struct {
	mock.Mock
}

func (m *MockDeviceStatusFactory) CreateDeviceStatus(msg interface{}) (*mxm.DeviceStatus1, error) {
	args := m.Called(msg)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mxm.DeviceStatus1), args.Error(1)
}

func TestNewDeviceStatusFactory(t *testing.T) {
	factory := NewDeviceStatusFactory()
	assert.NotNil(t, factory)
	assert.Implements(t, (*DeviceStatusFactory)(nil), factory)
}

func TestBttDeviceStatusFactory_CreateDeviceStatus(t *testing.T) {
	tests := []struct {
		name        string
		message     *Message
		expectError bool
		expectType  string
	}{
		{
			name: "命令响应消息",
			message: &Message{
				MessageId:   108,
				DeviceSN:    "863644076543074",
				DataType:    CMD_REPLY,
				Code:        "1",
				RepDataType: "2005",
			},
			expectError: false,
			expectType:  TYPE_BTT,
		},
		{
			name: "无效消息类型",
			message: &Message{
				MessageId: 999,
				DeviceSN:  "863644076543074",
				DataType:  "9999",
			},
			expectError: false,
			expectType:  TYPE_BTT,
		},
		{
			name:        "无效参数",
			message:     nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewDeviceStatusFactory()

			result, err := factory.CreateDeviceStatus(tt.message)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectType, result.Type)
				assert.Equal(t, tt.message.DeviceSN, result.OriginSN)
				assert.NotNil(t, result.RawMsg)
			}
		})
	}
}

func TestBttDeviceStatusFactory_HandleCommand(t *testing.T) {
	tests := []struct {
		name         string
		message      *Message
		expectError  bool
		expectAction string
	}{
		{
			name: "重启命令",
			message: &Message{
				MessageId: 101,
				DeviceSN:  "863644076543074",
				DataType:  POWRER,
				Data:      json.RawMessage(`[{"cmd":"power","action":"reboot"}]`),
			},
			expectError:  false,
			expectAction: "REBOOT",
		},
		{
			name: "关机命令",
			message: &Message{
				MessageId: 102,
				DeviceSN:  "863644076543074",
				DataType:  POWRER,
				Data:      json.RawMessage(`[{"cmd":"power","action":"off"}]`),
			},
			expectError:  false,
			expectAction: "POWER_OFF",
		},
		{
			name: "设置上报间隔命令",
			message: &Message{
				MessageId: 103,
				DeviceSN:  "863644076543074",
				DataType:  SET_REPORTINTERVAL,
				Data:      json.RawMessage(`[{"gpstime":"300"}]`),
			},
			expectError:  false,
			expectAction: "SET_REPORTINTERVAL",
		},
		{
			name: "查找命令",
			message: &Message{
				MessageId: 104,
				DeviceSN:  "863644076543074",
				DataType:  FIND,
				Data:      json.RawMessage(`[{"num":"10","fre":"2500","pwm":"50","cishu":"10"}]`),
			},
			expectError:  false,
			expectAction: "FIND",
		},
		{
			name: "命令响应",
			message: &Message{
				MessageId:   105,
				DeviceSN:    "863644076543074",
				DataType:    CMD_REPLY,
				Code:        "1",
				RepDataType: "2005",
			},
			expectError:  false,
			expectAction: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := &bttDeviceStatusFactory{}

			result, err := factory.handleCommand(tt.message)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.expectAction != "" {
					assert.Equal(t, tt.expectAction, result.Action)
				}
			}
		})
	}
}

func TestBttDeviceStatusFactory_HandleGnss(t *testing.T) {
	tests := []struct {
		name        string
		gnss        *Gnss
		expectError bool
		expectType  string
	}{
		{
			name: "GPS定位",
			gnss: &Gnss{
				Lng:     "116.3974",
				Lat:     "39.9042",
				Alt:     "102",
				Speed:   "0",
				Direc:   "0",
				Bssid:   "",
				Rssi:    "",
				Sates:   "8",
				Snr:     "",
				TimeStr: "2025-01-01 12:00:00",
				Type:    "3",
			},
			expectError: false,
			expectType:  "GPS",
		},
		{
			name: "WIFI定位",
			gnss: &Gnss{
				Lng:     "",
				Lat:     "",
				Alt:     "102",
				Speed:   "",
				Direc:   "",
				Bssid:   "5C:02:14:FD:89:74|EC:CF:70:6A:E4:46",
				Rssi:    "-46|-47",
				Sates:   "0",
				Snr:     "",
				TimeStr: "2025-01-01 12:00:00",
				Type:    "1",
			},
			expectError: false,
			expectType:  "WIFI",
		},
		{
			name: "无效坐标",
			gnss: &Gnss{
				Lng:     "",
				Lat:     "",
				Alt:     "102",
				Speed:   "",
				Direc:   "",
				Bssid:   "",
				Rssi:    "",
				Sates:   "0",
				Snr:     "",
				TimeStr: "2025-01-01 12:00:00",
				Type:    "3",
			},
			expectError: true,
		},
		{
			name: "无效时间格式",
			gnss: &Gnss{
				Lng:     "116.3974",
				Lat:     "39.9042",
				Alt:     "102",
				Speed:   "0",
				Direc:   "0",
				Bssid:   "",
				Rssi:    "",
				Sates:   "8",
				Snr:     "",
				TimeStr: "invalid-time",
				Type:    "3",
			},
			expectError: false,
			expectType:  "GPS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := &bttDeviceStatusFactory{}

			result, err := factory.handleGnss(tt.gnss)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectType, result.Type)
			}
		})
	}
}

func TestBttDeviceStatusFactory_ChargingStatus(t *testing.T) {
	tests := []struct {
		name           string
		originSN       string
		elect          int
		preElect       *ElectWithTm
		expectCharging bool
		expectElect    int
	}{
		{
			name:           "首次记录",
			originSN:       "863644076543074",
			elect:          80,
			preElect:       nil,
			expectCharging: false,
			expectElect:    80,
		},
		{
			name:     "电量稳定（时间差小）",
			originSN: "863644076543074",
			elect:    80,
			preElect: &ElectWithTm{
				Elect:      80,
				Tm:         time.Now().Add(-30 * time.Second), // 30秒前
				Status:     0b1000,
				LazyStatus: true,
			},
			expectCharging: true, // 保持之前的充电状态
			expectElect:    80,   // 电量不变
		},
		{
			name:     "电量稳定（时间差大）",
			originSN: "863644076543074",
			elect:    80,
			preElect: &ElectWithTm{
				Elect:      80,
				Tm:         time.Now().Add(-3 * time.Hour), // 3小时前
				Status:     0b1000,
				LazyStatus: true,
			},
			expectCharging: true, // 保持之前的充电状态
			expectElect:    80,   // 电量更新
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建模拟缓存
			mockCache := &MockLocalCache{}
			factory := &bttDeviceStatusFactory{cache: mockCache}

			// 设置缓存模拟
			if tt.preElect != nil {
				mockCache.On("GetCache", "btt_elect", tt.originSN).Return(*tt.preElect, true)
			} else {
				mockCache.On("GetCache", "btt_elect", tt.originSN).Return(nil, false)
			}
			mockCache.On("SetCache", "btt_elect", tt.originSN, mock.AnythingOfType("ElectWithTm")).Return()

			charging, elect := factory.chargingStatus(tt.originSN, tt.elect)

			assert.Equal(t, tt.expectCharging, charging)
			assert.Equal(t, tt.expectElect, elect)

			mockCache.AssertExpectations(t)
		})
	}
}

func TestBttDeviceStatusFactory_HandleHeartBeat(t *testing.T) {
	tests := []struct {
		name        string
		message     *Message
		expectError bool
		expectSN    string
	}{
		{
			name: "无效JSON",
			message: &Message{
				MessageId: 102,
				DeviceSN:  "863644076543074",
				DataType:  REPORT_HEARTBEAT,
				Data:      json.RawMessage(`invalid json`),
			},
			expectError: true,
		},
		{
			name: "空GNSS",
			message: &Message{
				MessageId: 102,
				DeviceSN:  "863644076543074",
				DataType:  REPORT_HEARTBEAT,
				Data:      json.RawMessage(`{"LTE":{"csq":"13"},"GNSS":[],"BAT":{"vol":"4020"},"other":{"gpstime":"1800"},"health":{"step":"100"}}`),
			},
			expectError: false,
			expectSN:    "863644076543074",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建模拟缓存
			mockCache := &MockLocalCache{}
			factory := &bttDeviceStatusFactory{cache: mockCache}

			// 设置缓存模拟
			mockCache.On("GetCache", "btt_elect", tt.message.DeviceSN).Return(nil, false)
			mockCache.On("SetCache", "btt_elect", tt.message.DeviceSN, mock.AnythingOfType("ElectWithTm")).Return()

			result, err := factory.handleHeartBeat(tt.message)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectSN, *result.OriginSN)
				assert.Equal(t, TYPE_BTT, *result.Type)
			}
		})
	}
}

func TestTypeAsString(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"WIFI", 1, "WIFI"},
		{"GPS+北斗", 2, "GPS"},
		{"GPS", 3, "GPS"},
		{"北斗", 4, "GPS"},
		{"LBS", 5, "LBS"},
		{"手机", 6, "手机"},
		{"未知", 99, "未知"},
		{"无效类型", 999, "未知"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TypeAsString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeAsInt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"WIFI", "WIFI", 1},
		{"GPS+北斗", "GPS+北斗", 2},
		{"GPS", "GPS", 3},
		{"北斗", "北斗", 4},
		{"LBS", "LBS", 5},
		{"手机", "手机", 6},
		{"未知", "UNKNOWN", 99},
		{"小写wifi", "wifi", 1},
		{"大写WIFI", "WIFI", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TypeAsInt(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCsqAsPercent(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"很强信号", 30, 100},
		{"强信号", 25, 85},
		{"中高信号", 20, 65},
		{"中信号", 17, 45},
		{"中低信号", 14, 25},
		{"低信号", 10, 15},
		{"弱信号", 5, 7},
		{"极弱信号", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CsqAsPercent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVol2Percent(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"满电", 4170, 100},
		{"高电量", 4000, 81},  // 根据实际电压表修正
		{"中等电量", 3800, 47}, // 根据实际电压表修正
		{"低电量", 3600, 23},  // 根据实际电压表修正
		{"极低电量", 3200, 0},
		{"超低电压", 3000, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Vol2Percent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMessage_ToBytes(t *testing.T) {
	msg := Message{
		MessageId:   102,
		DeviceSN:    "863644076543074",
		DataType:    REPORT_HEARTBEAT,
		Data:        json.RawMessage(`{"test":"data"}`),
		Code:        "",
		RepDataType: "",
	}

	bytes := msg.ToBytes()
	assert.NotEmpty(t, bytes)

	// 验证可以重新解析
	var parsed Message
	err := json.Unmarshal(bytes, &parsed)
	assert.NoError(t, err)
	assert.Equal(t, msg.MessageId, parsed.MessageId)
	assert.Equal(t, msg.DeviceSN, parsed.DeviceSN)
	assert.Equal(t, msg.DataType, parsed.DataType)
}

func TestFlexibleMessage_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		expectError bool
		expectedID  int64
	}{
		{
			name:        "数字ID",
			jsonData:    `{"messageId":102,"deviceSN":"863644076543074","dataType":"1002"}`,
			expectError: false,
			expectedID:  102,
		},
		{
			name:        "字符串ID",
			jsonData:    `{"messageId":"102","deviceSN":"863644076543074","dataType":"1002"}`,
			expectError: false,
			expectedID:  102,
		},
		{
			name:        "无效ID格式",
			jsonData:    `{"messageId":"invalid","deviceSN":"863644076543074","dataType":"1002"}`,
			expectError: true,
		},
		{
			name:        "缺失ID",
			jsonData:    `{"deviceSN":"863644076543074","dataType":"1002"}`,
			expectError: false,
			expectedID:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg FlexibleMessage
			err := json.Unmarshal([]byte(tt.jsonData), &msg)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, msg.MessageId)
			}
		})
	}
}
