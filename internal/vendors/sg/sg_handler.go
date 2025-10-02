package sg

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/Daneel-Li/gps-back/internal/models/sg"

	"github.com/Daneel-Li/gps-back/internal/handlers"

	"github.com/gorilla/mux"
)

//sg 驱动

// 定义所有厂商必须实现的强制接口
type sg_Handler struct {
	listenerPort   int                     //监听端口
	messageHandler handlers.MessageHandler //统一消息处理接口
}

/*
*
索工平台数据推送
*/
func (h *sg_Handler) UpdateBySuogong(w http.ResponseWriter, r *http.Request) {
	// 读取请求体中的数据
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	slog.Info("Received POST request with data:" + string(body))
	//在这里处理 POST 请求逻辑
	//body := `{"appKey":"b766c722fffa49f88983ac2ffa75a35d","data":{"electricity":98,"signal":87,"acc":true,"imei":363801011153321,"latitude":"39.97832905402682","longitude":"116.47458999684423","time":1721505597000,"type":"WIFI","wifiList":[{"mac":"7a:2b:46:c1:53:7c","signal":100,"ssid":"7a2b46c1537c"},{"mac":"02:5c:c2:a0:e2:44","signal":67,"ssid":"025cc2a0e244"},{"mac":"c8:f7:42:94:f8:dc","signal":57,"ssid":"c8f74294f8dc"},{"mac":"a8:50:81:9c:3c:c0","signal":51,"ssid":"a850819c3cc0"},{"mac":"04:88:5f:55:6b:f0","signal":50,"ssid":"04885f556bf0"},{"mac":"30:ae:7b:e5:e6:11","signal":49,"ssid":"30ae7be5e611"},{"mac":"22:87:ec:68:35:a0","signal":45,"ssid":"2287ec6835a0"},{"mac":"28:93:7d:12:9d:3e","signal":44,"ssid":"28937d129d3e"}]},"type":"location"}`
	var data sg.PushData
	err = json.Unmarshal([]byte(body), &data)
	if err != nil {
		slog.Error("Error parsing JSON:" + err.Error())
		http.Error(w, fmt.Sprintf("Error parsing JSON:%s", err.Error()), http.StatusInternalServerError)
		return
	}
	//根据不同推送类型更新不同表

	switch data.Type {
	case "location":
		var loc sg.Location
		if err = json.Unmarshal(data.Data, &loc); err != nil {
			slog.Error("Unmarshal sg location error:" + err.Error())
		}
		if err := h.mysql.AddSgLocation(loc); err != nil {
			slog.Error("addSgLocation error:" + err.Error())
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}
	default:
		slog.Error("未能解析推送(类型未知）：", "body", body)

	}

	w.Header().Set("Content-Type", "application/json") // 设置响应头，指明内容类型为JSON
	w.WriteHeader(http.StatusOK)                       // 设置状态码为200
	response := map[string]interface{}{"code": 0, "msg": "success"}
	jsonResponse, _ := json.Marshal(response) // 将map编码为JSON
	w.Write(jsonResponse)
}

var _sg = "sg"

type sg_params struct {
	DevicePhone    string `json:"device_phone"`   // 关联device phone
	ReportInterval int    `json:"reportInterval"` // 上报间隔(s)
}

func NewSgHandler(port int) handlers.VendorDriver {

	return &sg_Handler{
		listenerPort: port,
	}
}

func (h *sg_Handler) SetMessageHandler(handler handlers.MessageHandler) {
	h.messageHandler = handler
}

// 厂商自己的启动逻辑
func (h *sg_Handler) Start() error {
	// 订阅/侦听来自sg平台的协议推送
	r := mux.NewRouter()
	// 配置 HTTPS 服务器
	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", h.listenerPort),
		Handler: r, // 使用 ServeMux 作为请求处理器
	}

	r.HandleFunc("/api/v1/data", h.UpdateBySuogong).Methods("POST")

	go func() {
		slog.Info("Starting sg driver HTTP server:" + server.Addr + "...")
		if err := server.ListenAndServe(); err != nil {
			slog.Error("Failed to start HTTP server:" + err.Error())
		}
	}()

	return nil
}
func (h *sg_Handler) Activate(originSN string) error {
	//Do nothing
	return nil
}
func (h *sg_Handler) Deactivate(originSN string) error {
	//Do nothing
	return nil
}

func (h *sg_Handler) SetReportInterval(CommandID int64, originSN string, interval int) error {
	// TODO
	panic("implement me")
}

func (h *sg_Handler) Locate(CommandID int64, originSN string) error {
	bytes, _ := json.Marshal(map[string]interface{}{
		"command_id":   CommandID,
		"text_content": "LJDW#"})
	return httpPost("http://localhost:8008/device/"+originSN+"/text", bytes)
}
func (h *sg_Handler) Reboot(CommandID int64, originSN string) error {
	bytes, _ := json.Marshal(map[string]interface{}{
		"command_id":   CommandID,
		"text_content": "RESET#"})
	return httpPost("http://localhost:8008/device/"+originSN+"/text", bytes)
}
func (h *sg_Handler) PowerOff(CommandID int64, originSN string) error {
	bytes, _ := json.Marshal(map[string]interface{}{
		"command_id":   CommandID,
		"text_content": "SHUTDOWN#"})
	return httpPost("http://localhost:8008/device/"+originSN+"/text", bytes)
}

// 查找设备（寻宠）
func (h *sg_Handler) Find(CommandID int64, originSN string) error {
	// 调用808平台接口，设置设备上报间隔
	bytes, _ := json.Marshal(map[string]interface{}{
		"command_id":   CommandID,
		"text_content": "bon,1#"})
	return httpPost("http://localhost:8008/device/"+originSN+"/text", bytes)
}
