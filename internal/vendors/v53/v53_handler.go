package v53

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Daneel-Li/gps-back/internal/config"
	mxm "github.com/Daneel-Li/gps-back/internal/models"
	"github.com/Daneel-Li/gps-back/internal/services"
	"github.com/Daneel-Li/gps-back/internal/vendors"
	"github.com/Daneel-Li/gps-back/pkg/utils"

	"github.com/gorilla/mux"
	"github.com/qichengzx/coordtransform"
)

//v53 驱动

// 定义所有厂商必须实现的强制接口
type v53_Handler struct {
	listenerPort   int                    //监听端口
	messageHandler vendors.MessageHandler //统一消息处理接口
	locS           []services.LocationService
}

type NotifyMsg struct {
	MsgType string      `json:"msg_type"`
	Data    interface{} `json:"data"`
}

func (h *v53_Handler) HandleNotify(w http.ResponseWriter, r *http.Request) {

	// 请帮我完善这里，服务器会收到notify消息，可能会是0200位置上报也可能是别的
	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("Failed to read request body", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	msg := NotifyMsg{}
	if err := json.Unmarshal(body, &msg); err != nil {
		slog.Error("Failed to parse message type", "error", err)
		http.Error(w, "Invalid message format", http.StatusBadRequest)
		return
	}

	tp := "v53"
	status := &mxm.DeviceStatus1{
		Type:   tp,
		RawMsg: body,
	}
	switch msg.MsgType {
	case "0200", "0201": // 位置上报消息,立即定位的回应位置上报消息
		h.handle0200(status, msg.Data)
	case "0202": // 立即定位的回应位置上报消息
	case "0002": // 心跳
		handle0002(status, msg.Data)
	case "0104": // 参数查询回应
		handle0104(status, msg.Data)
	case "8103_reply": // 终端通用应答
		handle8103_reply(status, msg.Data)
	case "8300_reply": //文本指令应答
		handle8300_reply(status, msg.Data)

	case "0300": // 假设的其他消息类型
		var otherMsg struct {
			SerialNumber string `json:"sn"`
			// 其他字段...
		}
		if err := json.Unmarshal(body, &otherMsg); err != nil {
			slog.Error("Failed to parse other message", "error", err)
			http.Error(w, "Invalid message data", http.StatusBadRequest)
			return
		}

		status = &mxm.DeviceStatus1{
			OriginSN: otherMsg.SerialNumber,
			// 设置其他状态字段...
		}

	default:
		slog.Error("Unknown message type", "type", msg.MsgType)
		http.Error(w, "Unsupported message type", http.StatusBadRequest)
		return
	}

	// 调用统一消息处理接口
	if err := h.messageHandler.Process(status); err != nil {
		slog.Error("Failed to process message", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *v53_Handler) handle0200(status *mxm.DeviceStatus1, data interface{}) {
	bytes, e := json.Marshal(data)
	if e != nil {
		slog.Error("Failed to marshal message", "error", e)
		return
	}
	geo := DeviceGeo{}
	json.Unmarshal(bytes, &geo)
	if status.Device == nil {
		tp := "v53"
		status.Device = &mxm.Device{
			Type: &tp,
		}
	}
	dev := status.Device
	dev.OriginSN = &geo.Phone
	status.OriginSN = geo.Phone
	if geo.Battery != nil {
		ele := int(geo.Battery.BatteryLevel)
		dev.Electricity = &ele
		dev.Charging = &geo.Battery.Charging
	}
	dev.Steps = &geo.Steps
	csq := int(geo.CsqLevel)
	dev.SimCardSignal = &csq
	dev.LastOnline = &geo.Time
	dev.LocTime = &geo.Time
	if geo.Location != nil {
		longi, lati := coordtransform.WGS84toGCJ02(geo.Location.Longitude, geo.Location.Latitude)
		dev.Latitude = &lati
		dev.Longitude = &longi
		alt := float64(geo.Location.Altitude)
		dev.Altitude = &alt
	}

	sate := int(geo.Sattelite)
	dev.Satellites = &sate
	locT := "GPS"
	if dev.Latitude != nil && *dev.Latitude != 0 {
		locT = "GPS"
		var geoRes *services.GeoCoderResult = nil
		var lastErr error

		// 循环尝试不同的地理编码服务
		for i, locService := range h.locS {
			var err error
			geoRes, err = locService.Geocode(*dev.Latitude, *dev.Longitude, 2*time.Second)
			if err == nil {
				break // 成功则跳出循环
			}
			lastErr = err
			if i < len(h.locS)-1 {
				slog.Warn("invoking geocoder service failed, trying next service...", "error", err.Error())
			}
		}

		if geoRes == nil && lastErr != nil {
			slog.Error("all geocoder services failed", "error", lastErr.Error())
		}
		if geoRes != nil {
			dev.Address = &geoRes.Address
		}
	} else if len(geo.WifiInfos) > 0 {
		locT = "WIFI"
		if dev.SimCardSignal != nil && *dev.SimCardSignal == 0 {
			//猜测v53应该是共用天线，wifi定位时无信号强度数据,不采用该数据
			dev.SimCardSignal = nil
		}
		//请求位置
		wifiJSON, _ := json.Marshal(geo.WifiInfos)
		var wifis []*mxm.WiFiInfo
		json.Unmarshal(wifiJSON, &wifis)
		req := services.LocationRequest{
			WifiInfo: wifis,
		}

		var locRes *services.LocationResult
		var lastErr error

		// 循环尝试不同的位置服务
		for i, locService := range h.locS {
			var err error
			locRes, err = locService.LocateByNetwork(req, 2*time.Second)
			if err == nil {
				break // 成功则跳出循环
			}
			lastErr = err
			if i < len(h.locS)-1 {
				slog.Warn("invoking loc service failed, trying next service...", "error", err.Error())
			}
		}

		if locRes != nil {
			dev.Address = &locRes.Address
			dev.Longitude = &locRes.Location.Longitude
			dev.Latitude = &locRes.Location.Latitude
		} else if lastErr != nil {
			slog.Error("all loc services failed", "error", lastErr.Error())
		}
	} else if len(geo.LBSInfos) > 0 {
		locT = "LBS"
	}
	dev.LocType = &locT
}

func handle0002(status *mxm.DeviceStatus1, data interface{}) {
	// 心跳推送回来的也是整个geo，直接按0200处理 //TODO 后续再完善细化
	//但是只处理电量更新，其它的都是旧值，不更新
	bytes, e := json.Marshal(data)
	if e != nil {
		slog.Error("Failed to marshal message", "error", e)
		return
	}
	geo := DeviceGeo{}
	json.Unmarshal(bytes, &geo)
	if status.Device == nil {
		tp := "v53"
		status.Device = &mxm.Device{
			Type: &tp,
		}
	}
	dev := status.Device
	dev.OriginSN = &geo.Phone
	status.OriginSN = geo.Phone
	if geo.Battery != nil {
		ele := int(geo.Battery.BatteryLevel)
		dev.Electricity = &ele
	}
	dev.LastOnline = &geo.Time
}

var _v53 = "v53"

type v53_params struct {
	DevicePhone    string `json:"device_phone"`   // 关联device phone
	ReportInterval int    `json:"reportInterval"` // 上报间隔(s)
}

func handle0104(status *mxm.DeviceStatus1, data interface{}) {
	if status.Device == nil {
		status.Device = &mxm.Device{}
	}
	status.Type, status.Device.Type = _v53, &_v53

	bytes, e := json.Marshal(data)
	if e != nil {
		slog.Error("Failed to marshal message", "error", e)
		return
	}
	params := v53_params{}
	if err := json.Unmarshal(bytes, &params); err != nil {
		slog.Error("Failed to unmarshal message", "error", err)
		return
	}
	status.OriginSN = params.DevicePhone
	status.Device.OriginSN = &params.DevicePhone
	status.Device.Interval = &params.ReportInterval
}

func handle8103_reply(status *mxm.DeviceStatus1, data interface{}) {
	type cmdResult struct {
		PhoneNumber string `json:"phone_number"`
		CommandID   uint32 `json:"command_id"`
		Succeed     bool   `json:"succeed"`
		Msg         string `json:"msg"`
	}
	cr := cmdResult{}
	bytes, e := json.Marshal(data)
	if e != nil {
		slog.Error("Failed to marshal message", "error", e)
		return
	}
	if err := json.Unmarshal(bytes, &cr); err != nil {
		slog.Error("Failed to unmarshal message", "error", err)
		return
	}
	status.OriginSN = cr.PhoneNumber
	status.Command = &mxm.Command{
		Result: &mxm.CommandResult{
			CommandID: int64(cr.CommandID),
			Succeed:   cr.Succeed,
		},
	}

	refreshParams(status.OriginSN)
}

// 文本指令应答
func handle8300_reply(status *mxm.DeviceStatus1, data interface{}) {
	slog.Debug("handle8300_reply", "data", data)
	type cmdResult struct {
		PhoneNumber string `json:"phone_number"`
		CommandID   uint32 `json:"command_id"`
		Succeed     bool   `json:"succeed"`
		Msg         string `json:"msg"`
	}
	cr := cmdResult{}
	bytes, e := json.Marshal(data)
	if e != nil {
		slog.Error("Failed to marshal message", "error", e)
		return
	}
	if err := json.Unmarshal(bytes, &cr); err != nil {
		slog.Error("Failed to unmarshal message", "error", err)
		return
	}
	status.OriginSN = cr.PhoneNumber
	status.Command = &mxm.Command{
		Result: &mxm.CommandResult{
			CommandID: int64(cr.CommandID),
			Succeed:   cr.Succeed,
		},
	}
}

func refreshParams(phoneNumber string) {
	_, err := utils.HttpGet("http://localhost:8008/device/" + phoneNumber + "/params")
	if err != nil {
		slog.Error("Failed to get refresh params", "error", err)
		return
	}
}

func NewV53Handler(port int) vendors.VendorDriver {

	return &v53_Handler{
		listenerPort: port,
		locS:         []services.LocationService{services.NewTxLocationService(), services.NewWzLocationService()},
	}
}

func (h *v53_Handler) SetMessageHandler(handler vendors.MessageHandler) {
	h.messageHandler = handler
}

// 厂商自己的启动逻辑
func (h *v53_Handler) Start() error {
	// 订阅/侦听来自808平台的协议推送
	r := mux.NewRouter()
	// 配置 HTTPS 服务器
	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", h.listenerPort),
		Handler: r, // 使用 ServeMux 作为请求处理器
	}

	r.HandleFunc("/v53/notify", h.HandleNotify).Methods("POST")

	go func() {
		slog.Info("Starting v53 driver HTTP server:" + server.Addr + "...")
		if err := server.ListenAndServe(); err != nil {
			slog.Error("Failed to start HTTP server:" + err.Error())
		}
	}()

	return nil
}
func (h *v53_Handler) Activate(originSN string) error {
	//Do nothing
	return nil
}
func (h *v53_Handler) Deactivate(originSN string) error {
	//Do nothing
	return nil
}

func (h *v53_Handler) SetReportInterval(CommandID int64, originSN string, interval int) error {
	defer func() {
		go func() {
			<-time.After(time.Second * 1)
			utils.HttpGet(config.GetConfig().JT808Url + originSN + "/params")
		}()
	}()

	bytes, _ := json.Marshal(map[string]interface{}{
		"command_id":   CommandID,
		"text_content": fmt.Sprintf("upload,%v#", interval)})

	return utils.HttpPost(config.GetConfig().JT808Url+originSN+"/text", bytes)
}

// 定时开机tm格式为hh:mm,指令内容AUTOMATIC,1,1,09:00# 表示9点开机
func (h *v53_Handler) AutoStart(CommandID int64, originSN string, tm string, enable bool) error {
	enableS := "1"
	if !enable {
		enableS = "0"
	}
	bytes, _ := json.Marshal(map[string]interface{}{
		"command_id":   CommandID,
		"text_content": fmt.Sprintf("AUTOMATIC,%s,1,%s#", enableS, tm)})
	return utils.HttpPost(config.GetConfig().JT808Url+originSN+"/text", bytes)
}

// 定时开机tm格式为hh:mm,指令内容AUTOMATIC,1,0,09:00# 表示9点关机
func (h *v53_Handler) AutoShut(CommandID int64, originSN string, tm string, enable bool) error {
	enableS := "1"
	if !enable {
		enableS = "0"
	}
	bytes, _ := json.Marshal(map[string]interface{}{
		"command_id":   CommandID,
		"text_content": fmt.Sprintf("AUTOMATIC,%s,0,%s#", enableS, tm)})
	return utils.HttpPost(config.GetConfig().JT808Url+originSN+"/text", bytes)
}

func (h *v53_Handler) Locate(CommandID int64, originSN string) error {
	bytes, _ := json.Marshal(map[string]interface{}{
		"command_id":   CommandID,
		"text_content": "LJDW#"})
	return utils.HttpPost(config.GetConfig().JT808Url+originSN+"/text", bytes)
}
func (h *v53_Handler) Reboot(CommandID int64, originSN string) error {
	bytes, _ := json.Marshal(map[string]interface{}{
		"command_id":   CommandID,
		"text_content": "RESET#"})
	return utils.HttpPost(config.GetConfig().JT808Url+originSN+"/text", bytes)
}
func (h *v53_Handler) PowerOff(CommandID int64, originSN string) error {
	bytes, _ := json.Marshal(map[string]interface{}{
		"command_id":   CommandID,
		"text_content": "SHUTDOWN#"})
	return utils.HttpPost(config.GetConfig().JT808Url+originSN+"/text", bytes)
}

// 查找设备（寻宠）
func (h *v53_Handler) Find(CommandID int64, originSN string) error {
	// 调用808平台接口，设置设备上报间隔
	bytes, _ := json.Marshal(map[string]interface{}{
		"command_id":   CommandID,
		"text_content": "bon,1#"})
	return utils.HttpPost(config.GetConfig().JT808Url+originSN+"/text", bytes)
}
