package btt

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	mxm "github.com/Daneel-Li/gps-back/internal/models"
	"github.com/Daneel-Li/gps-back/internal/vendors"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const _TBL_BTT_PREFIX = "tb_btt_" //tb_前缀
const _TBL_SG_PREFIX = "tb_sg_"   //tb_前缀
const _STB_LOCATION = "stb_location"
const _CMD_TOPIC_PREFIX = "dwq/device/hy" //device消费的消息，即为指令
const _REPORT_TOPIC_PREFIX = "dwq/app/hy" //app消费的消息，自然就是数据上报

var shanghai *time.Location = nil

func init() {
	shanghai, _ = time.LoadLocation("Asia/Shanghai")
}

// topic提供者，获取当前mqtt要监听的设备列表
// 解耦数据库操作
type TopicProvider interface {
	GetSubscriptionList() ([]string, error)
}

// 实现VendorDriver接口
type MqttHandler struct {
	unifiedFunc vendors.MessageHandler
	config      MqttConfig
	provider    TopicProvider
	mqClient    mqtt.Client
	factory     DeviceStatusFactory
}

func (h *MqttHandler) SetMessageHandler(handler vendors.MessageHandler) {
	h.unifiedFunc = handler
}

// It initializes the handler with an empty message channel map if not already initialized.
func NewMqttHandler(cfg MqttConfig, provider TopicProvider) *MqttHandler {
	return &MqttHandler{config: cfg, provider: provider, factory: NewDeviceStatusFactory()}
}

type MqttConfig struct {
	Broker   string `json:"broker"`
	ClientID string `json:"clientid"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// 启动mqtt客户端
// Start initializes and establishes a connection to the MQTT broker using the provided configuration.
// It sets up client options including broker address, client ID, message handler, credentials,
// and connection settings. The function also configures automatic reconnection and handles
// connection loss events. Returns an error if the connection fails.
func (h *MqttHandler) Start() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(h.config.Broker)
	opts.SetClientID(h.config.ClientID)              // 客户端ID
	opts.SetDefaultPublishHandler(h.messageCallback) // 设置消息处理器
	opts.SetUsername(h.config.Username)
	opts.SetPassword(h.config.Password)
	opts.SetKeepAlive(10 * time.Second)
	opts.SetAutoReconnect(true) // 开启自动重连
	opts.SetResumeSubs(true)    // 恢复订阅
	opts.SetConnectRetry(true)
	opts.SetMaxReconnectInterval(5 * time.Second) //最多隔5秒重试一次
	opts.SetCleanSession(false)
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		slog.Debug("mqtt 连接成功！")
	})
	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		slog.Warn("mqtt client disconnected. trying to reconnect...")
		// 说明有，如果想要重连，不要自己实现，要用gsetAutoreconnect来做
		// if token := c.Connect(); token.Wait() && token.Error() != nil {
		// 	slog.Error("mqtt client connect failed. " + token.Error().Error())
		// }
	})
	// 创建并启动客户端
	h.mqClient = mqtt.NewClient(opts)
	if token := h.mqClient.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("mqtt client failed: %v", token.Error())
	}

	// 监听，这里从provider读取topic
	return h.reloadTopics()
}

func getDevTopicBySN(sn string) string {
	return fmt.Sprintf("%s/%s/", _CMD_TOPIC_PREFIX, sn)
}
func getAppTopicBySN(sn string) string {
	return fmt.Sprintf("%s/%s/", _REPORT_TOPIC_PREFIX, sn)
}
func getSNByTopic(topic string) string {
	if topic[:len(_CMD_TOPIC_PREFIX)] == _CMD_TOPIC_PREFIX {
		return topic[len(_CMD_TOPIC_PREFIX)+1 : len(topic)-1]
	} else if topic[:len(_REPORT_TOPIC_PREFIX)] == _REPORT_TOPIC_PREFIX {
		return topic[len(_REPORT_TOPIC_PREFIX)+1 : len(topic)-1]
	} else {
		return ""
	}
}

func (h *MqttHandler) getTopics(devs []string) []string {
	var topics []string
	for _, dev := range devs {
		topics = append(topics, getDevTopicBySN(dev))
		topics = append(topics, getAppTopicBySN(dev))
	}
	return topics
}

func (h *MqttHandler) reloadTopics() error {
	devs, err := h.provider.GetSubscriptionList()
	if err != nil {
		return fmt.Errorf("get device list failed: %v", err)
	}

	// 检查 mqClient 是否已初始化
	if h.mqClient == nil {
		return fmt.Errorf("mqtt client not initialized")
	}

	if err := h.subscribeMqttTopics(h.getTopics(devs)); err != nil {
		return fmt.Errorf("subscribe topic list failed: %v", err)
	}
	return nil
}

/*
*
自动调整算法，处理btt产品紧急模式下空包问题
算法逻辑：如果当前心跳包是gps定位且为紧急模式（1分钟以内），则切换成10s高速模式
如果当前出现空包且是10s高速紧急模式，切换成30s半高速模式
*/
func (h *MqttHandler) autoAdjust(dev *mxm.Device) {
	if dev == nil {
		return
	}
	if (dev.Latitude == nil || *dev.Latitude == 0) &&
		(dev.Interval != nil && *(dev.Interval) == 10) && dev.OriginSN != nil {
		h.SetReportInterval(0, *dev.OriginSN, 30)
	} else if (dev.Latitude != nil && *dev.LocType == "GPS" && dev.OriginSN != nil) &&
		(dev.Interval != nil && *(dev.Interval) > 10 && *(dev.Interval) <= 30) {
		h.SetReportInterval(0, *dev.OriginSN, 10)
	}
}

/*
*
充电状态下1分钟后上报一次
*/
func (h *MqttHandler) quickReportForCharging(dev *mxm.Device) {
	if dev == nil || dev.Charging == nil || !*dev.Charging {
		return
	}

	go func() {
		//TODO 要优化，高速模式下会有过多的冗余查询
		//60秒后定位一次，以此获取最新电量
		<-time.After(60 * time.Second)
		h.Locate(0, *dev.OriginSN)
	}()
}

func (h *MqttHandler) messageCallback(_ mqtt.Client, m mqtt.Message) {
	go func() { //TODO 测试
		topic := m.Topic()
		payload := m.Payload()
		slog.Debug("MessageHandler Received message", "topic", topic, "msg", payload)

		// 1. 先处理指令响应
		wrapper := &FlexibleMessage{Message: &Message{}}
		if err := json.Unmarshal(payload, wrapper); err != nil {
			slog.Error("wrong mqtt message:", "payload", string(payload), "err", err)
			return
		}
		deviceSN := getSNByTopic(topic)

		msg := wrapper.Message

		if len(msg.DeviceSN) == 0 {
			msg.DeviceSN = deviceSN
		} else if msg.DeviceSN != deviceSN {
			panic("wrong deviceSN")
		}

		f := h.factory
		status, err := f.CreateDeviceStatus(msg)
		if err != nil {
			slog.Error(fmt.Sprintf("btt vendor factory failed to create device status: %s", err.Error()))
			status = &mxm.DeviceStatus1{
				RawMsg: msg.ToBytes(),
			}
		}
		// TODO: 待观察效果
		h.autoAdjust(status.Device)
		//如果在charging状态，自动定期使用locate获取电量 //TODO 有缺陷，导致电耗增加
		//h.quickReportForCharging(status.Device)
		//2. 余下的消息全部交给用户处理
		//将消息传送给统一handler进行处理
		h.unifiedFunc.Process(status)
	}()
}

func (h *MqttHandler) handleCmd(msg Message) error {
	// cmd只要存档备查
	//TODO
	fmt.Println(msg)
	return nil
}

/*
*
服务器监控
*/
func Alert(check func() string) {
	res := check()
	if len(res) > 0 {
		// do alert, sending a mail or
		slog.Warn("Watch dog warning:", "msg", res)
	}
}

func (h *MqttHandler) subscribeMqttTopics(topics []string) error {
	if len(topics) == 0 {
		return nil
	}
	m := make(map[string]byte)
	for _, topic := range topics {
		m[topic] = 1
	}
	if h.mqClient == nil {
		return errors.New("mq client is invalid")
	}
	if token := h.mqClient.SubscribeMultiple(m, nil); token.Wait() && token.Error() != nil {
		return fmt.Errorf("subscribe failed: %v", token.Error())
	}
	slog.Debug("subscribed topics:", "topics", topics)
	return nil
}

func (h *MqttHandler) unsubscribeMqttTopics(topics []string) error {
	if len(topics) == 0 {
		return nil
	}
	if h.mqClient == nil {
		return errors.New("mq client is invalid")
	}
	if token := h.mqClient.Unsubscribe(topics...); token.Wait() && token.Error() != nil {
		return fmt.Errorf("unsubscribe failed: %v", token.Error())
	}
	slog.Debug("unsubscribed topics:", "topics", topics)
	return nil
}

func (op *MqttHandler) SetReportInterval(CommandID int64, deviceSN string, interval int) error {
	msg := Message{MessageId: CommandID, DataType: "3002"}

	msg.Data, _ = json.Marshal([]map[string]string{{"gpstime": fmt.Sprintf("%d", interval)}})

	return op.publishBttMessage(deviceSN, msg)
}

// 立即定位
func (op *MqttHandler) Locate(CommandID int64, deviceSN string) error {
	msg := Message{MessageId: CommandID, DataType: "2005"}

	return op.publishBttMessage(deviceSN, msg)
}

// 远程重启
func (op *MqttHandler) Reboot(CommandID int64, deviceSN string) error {
	msg := Message{MessageId: CommandID, DataType: "3900"}

	msg.Data, _ = json.Marshal([]map[string]string{{"cmd": "power", "action": "reboot"}})
	return op.publishBttMessage(deviceSN, msg)
}

// 远程关机
func (op *MqttHandler) PowerOff(CommandID int64, deviceSN string) error {
	msg := Message{MessageId: CommandID, DataType: "3900"}
	msg.Data, _ = json.Marshal([]map[string]string{{"cmd": "power", "action": "off"}})
	return op.publishBttMessage(deviceSN, msg)
}

func (op *MqttHandler) Find(CommandID int64, deviceSN string) error {

	msg := Message{MessageId: CommandID, DataType: "3006"}

	msg.Data, _ = json.Marshal([]map[string]string{
		{
			"num":   "10",
			"fre":   "2500",
			"pwm":   "50",
			"cishu": "10",
		},
	})
	return op.publishBttMessage(deviceSN, msg)
}

func (op *MqttHandler) Activate(deviceSN string) error {
	// 监听话题
	return op.subscribeMqttTopics(op.getTopics([]string{deviceSN}))
}

func (op *MqttHandler) Deactivate(deviceSN string) error {
	return op.unsubscribeMqttTopics(op.getTopics([]string{deviceSN}))
}

func (h *MqttHandler) publishBttMessage(deviceSN string, msg Message) error {
	topic := getDevTopicBySN(deviceSN)

	// 2. 序列化消息
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	// 5. 发布命令
	if h.mqClient == nil {
		return errors.New("mp client is invalid.")
	}
	if token := h.mqClient.Publish(topic, 1, false, payload); token.Error() != nil {
		return fmt.Errorf("publish failed: %v", token.Error())
	}
	return nil
}
