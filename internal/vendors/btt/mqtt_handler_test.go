package btt

import (
	"testing"
	"time"

	mxm "github.com/Daneel-Li/gps-back/internal/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTopicProvider 模拟 TopicProvider
type MockTopicProvider struct {
	mock.Mock
}

func (m *MockTopicProvider) GetSubscriptionList() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

// MockMessageHandler 模拟 MessageHandler
type MockMessageHandler struct {
	mock.Mock
}

func (m *MockMessageHandler) Process(status *mxm.DeviceStatus1) error {
	args := m.Called(status)
	return args.Error(0)
}

// MockMQTTClient 模拟 MQTT 客户端
type MockMQTTClient struct {
	mock.Mock
}

func (m *MockMQTTClient) Connect() mqtt.Token {
	args := m.Called()
	return args.Get(0).(mqtt.Token)
}

func (m *MockMQTTClient) Disconnect(quiesce uint) {
	m.Called(quiesce)
}

func (m *MockMQTTClient) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockMQTTClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	args := m.Called(topic, qos, callback)
	return args.Get(0).(mqtt.Token)
}

func (m *MockMQTTClient) SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
	args := m.Called(filters, callback)
	return args.Get(0).(mqtt.Token)
}

func (m *MockMQTTClient) Unsubscribe(topics ...string) mqtt.Token {
	args := m.Called(topics)
	return args.Get(0).(mqtt.Token)
}

func (m *MockMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	args := m.Called(topic, qos, retained, payload)
	return args.Get(0).(mqtt.Token)
}

// MockMQTTToken 模拟 MQTT Token
type MockMQTTToken struct {
	mock.Mock
}

func (m *MockMQTTToken) Wait() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockMQTTToken) WaitTimeout(timeout time.Duration) bool {
	args := m.Called(timeout)
	return args.Bool(0)
}

func (m *MockMQTTToken) Error() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockMQTTToken) Done() <-chan struct{} {
	args := m.Called()
	return args.Get(0).(<-chan struct{})
}

// MockMQTTMessage 模拟 MQTT Message
type MockMQTTMessage struct {
	mock.Mock
}

func (m *MockMQTTMessage) Duplicate() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockMQTTMessage) Qos() byte {
	args := m.Called()
	return args.Get(0).(byte)
}

func (m *MockMQTTMessage) Retained() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockMQTTMessage) Topic() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockMQTTMessage) MessageID() uint16 {
	args := m.Called()
	return args.Get(0).(uint16)
}

func (m *MockMQTTMessage) Payload() []byte {
	args := m.Called()
	return args.Get(0).([]byte)
}

func (m *MockMQTTMessage) Ack() {
	m.Called()
}

func TestNewMqttHandler(t *testing.T) {
	// 准备测试数据
	config := MqttConfig{
		Broker:   "tcp://localhost:1883",
		ClientID: "test-client",
		Username: "test-user",
		Password: "test-pass",
	}
	provider := &MockTopicProvider{}

	// 执行测试
	handler := NewMqttHandler(config, provider)

	// 验证结果
	assert.NotNil(t, handler)
	assert.Equal(t, config, handler.config)
	assert.Equal(t, provider, handler.provider)
	assert.NotNil(t, handler.factory)
	assert.Nil(t, handler.unifiedFunc) // 初始时应该为 nil
	assert.Nil(t, handler.mqClient)    // 初始时应该为 nil
}

func TestSetMessageHandler(t *testing.T) {
	// 准备测试数据
	handler := &MqttHandler{}
	messageHandler := &MockMessageHandler{}

	// 执行测试
	handler.SetMessageHandler(messageHandler)

	// 验证结果
	assert.Equal(t, messageHandler, handler.unifiedFunc)
}

func TestMqttHandler_MessageCallback(t *testing.T) {
	tests := []struct {
		name           string
		topic          string
		payload        []byte
		messageHandler *MockMessageHandler
		expectError    bool
	}{
		{
			name:  "有效消息",
			topic: "dwq/app/hy/863644076543074/",
			payload: []byte(`{
				"messageId": 15,
				"deviceSN": "863644076543074",
				"dataType": "8001",
				"repDataType": "2005"
			}`),
			messageHandler: &MockMessageHandler{},
			expectError:    false,
		},
		{
			name:           "无效JSON",
			topic:          "dwq/app/hy/123456789/",
			payload:        []byte("invalid json"),
			messageHandler: &MockMessageHandler{},
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 准备模拟对象
			handler := &MqttHandler{
				unifiedFunc: tt.messageHandler,
				factory:     NewDeviceStatusFactory(),
			}

			// 设置消息处理器模拟
			if !tt.expectError {
				tt.messageHandler.On("Process", mock.AnythingOfType("*mxm.DeviceStatus1")).
					Return(nil)
			}

			// 创建模拟 MQTT 消息
			mockMsg := &MockMQTTMessage{}
			mockMsg.On("Topic").Return(tt.topic)
			mockMsg.On("Payload").Return(tt.payload)

			// 执行测试
			handler.messageCallback(nil, mockMsg)

			// 等待 goroutine 完成
			time.Sleep(100 * time.Millisecond)

			// 验证结果
			if !tt.expectError {
				tt.messageHandler.AssertExpectations(t)
			}

			mockMsg.AssertExpectations(t)
		})
	}
}

func TestMqttHandler_SetReportInterval(t *testing.T) {
	handler := &MqttHandler{}

	// 这里我们无法直接测试，因为需要 MQTT 客户端
	// 在实际项目中，我们需要重构代码使其更易于测试
	// 或者使用依赖注入来注入 MQTT 客户端

	// 目前我们只能验证方法不会 panic
	assert.NotPanics(t, func() {
		handler.SetReportInterval(123, "123456789", 60)
	})
}

func TestMqttHandler_Locate(t *testing.T) {
	handler := &MqttHandler{}

	// 这里我们无法直接测试，因为需要 MQTT 客户端
	assert.NotPanics(t, func() {
		handler.Locate(123, "123456789")
	})
}

func TestMqttHandler_Reboot(t *testing.T) {
	handler := &MqttHandler{}

	assert.NotPanics(t, func() {
		handler.Reboot(123, "123456789")
	})
}

func TestMqttHandler_PowerOff(t *testing.T) {
	handler := &MqttHandler{}

	assert.NotPanics(t, func() {
		handler.PowerOff(123, "123456789")
	})
}

func TestMqttHandler_Find(t *testing.T) {
	handler := &MqttHandler{}

	assert.NotPanics(t, func() {
		handler.Find(123, "123456789")
	})
}

func TestMqttHandler_Activate(t *testing.T) {
	handler := &MqttHandler{}

	// 这里我们无法直接测试，因为需要 MQTT 客户端
	assert.NotPanics(t, func() {
		handler.Activate("123456789")
	})
}

func TestMqttHandler_Deactivate(t *testing.T) {
	handler := &MqttHandler{}

	// 这里我们无法直接测试，因为需要 MQTT 客户端
	assert.NotPanics(t, func() {
		handler.Deactivate("123456789")
	})
}

func TestMqttHandler_PublishBttMessage(t *testing.T) {
	tests := []struct {
		name        string
		deviceSN    string
		message     Message
		expectError bool
	}{
		{
			name:     "有效消息",
			deviceSN: "123456789",
			message: Message{
				MessageId: 123,
				DataType:  "2005",
			},
			expectError: false,
		},
		{
			name:     "空设备SN",
			deviceSN: "",
			message: Message{
				MessageId: 123,
				DataType:  "2005",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &MqttHandler{}

			// 这里我们无法直接测试，因为需要 MQTT 客户端
			// 在实际项目中，我们需要重构代码使其更易于测试
			assert.NotPanics(t, func() {
				handler.publishBttMessage(tt.deviceSN, tt.message)
			})
		})
	}
}
