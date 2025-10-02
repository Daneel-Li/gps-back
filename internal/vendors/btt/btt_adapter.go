package btt

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Daneel-Li/gps-back/internal/config"
	mxm "github.com/Daneel-Li/gps-back/internal/models"
	"github.com/Daneel-Li/gps-back/internal/services"
	"github.com/Daneel-Li/gps-back/pkg/utils"

	"github.com/qichengzx/coordtransform"
)

const (
	_WIFI    = 1
	_GPS_BD  = 2
	_GPS     = 3
	_BD      = 4
	_LBS     = 5
	_PHONE   = 6
	_UNKNOWN = 99
	TYPE_BTT = "btt"
)

type DeviceStatusFactory interface {
	CreateDeviceStatus(m interface{}) (*mxm.DeviceStatus1, error)
}

type bttDeviceStatusFactory struct {
	cache services.LocalCache
	locS  []services.LocationService
}

func NewDeviceStatusFactory() DeviceStatusFactory {
	services.RegisterStruct(ElectWithTm{})
	services.RegisterStruct(mxm.Location{})
	return &bttDeviceStatusFactory{
		cache: services.NewLocalCache(filepath.Join(config.GetConfig().DataPath, "cache_data.json")),
		locS:  []services.LocationService{services.NewTxLocationService(), services.NewWzLocationService()},
	}
}

type ElectWithTm struct {
	Elect      int       //电量百分比
	Tm         time.Time //记录时间
	Status     int       //是否充电的状态位运算，001未充电，010可能充电，100才判断充电
	LazyStatus bool      //延迟的充电状态，如果当前status是010，它将残留上一次的状态
}

func (f *bttDeviceStatusFactory) CreateDeviceStatus(m interface{}) (*mxm.DeviceStatus1, error) {

	msg, ok := m.(*Message)
	if !ok || msg == nil {
		return nil, fmt.Errorf("invalid param:%v", m)
	}

	res := &mxm.DeviceStatus1{}
	res.OriginSN = msg.DeviceSN
	res.Type = TYPE_BTT
	res.RawMsg = msg.ToBytes()

	switch msg.DataType {
	case REPORT_HEARTBEAT:
		d, err := f.handleHeartBeat(msg)
		if err != nil {
			slog.Error(fmt.Sprintf("handleHeartBeat:%v", err.Error()))
			res.Device = nil
		} else {
			res.Device = d
		}
	case POWRER, SET_REPORTINTERVAL, FIND, CMD_REPLY:
		c, err := f.handleCommand(msg)
		if err != nil {
			slog.Error(fmt.Sprintf("handleCommand:%v", err.Error()))
			res.Command = nil
		} else {
			res.Command = c
		}
	}

	return res, nil
}

func (h *bttDeviceStatusFactory) handleCommand(msg *Message) (*mxm.Command, error) {
	cmd := &mxm.Command{}
	var data interface{}
	json.Unmarshal(msg.Data, &data)

	switch msg.DataType {
	case POWRER:
		cmd.Action = ""
		if data.([]interface{})[0].(map[string]interface{})["action"] == "reboot" {
			cmd.Action = "REBOOT"
		} else {
			cmd.Action = "POWER_OFF"
		}
	case SET_REPORTINTERVAL:
		cmd.Action = "SET_REPORTINTERVAL"
		cmd.Args = []string{data.([]interface{})[0].(map[string]interface{})["gpstime"].(string)}
	case FIND:
		cmd.Action = "FIND"
		args, _ := json.Marshal(data.([]interface{})[0])
		cmd.Args = []string{string(args)}
	case CMD_REPLY:
		suc := false
		if msg.Code == "1" {
			suc = true
		}
		cmd.Result = &mxm.CommandResult{
			CommandID: msg.MessageId,
			Succeed:   suc,
			Msg:       msg.Code,
			Extra:     []byte{},
		}
	}
	return cmd, nil
}

func (h *bttDeviceStatusFactory) handleGnss(gnss *Gnss) (*mxm.Location, error) {

	tp, _ := strconv.Atoi(gnss.Type)

	loc := mxm.Location{
		Type:       TypeAsString(int(utils.ParseFloatWithDefault(gnss.Type, 0))),
		Altitude:   utils.ParseFloatWithDefault(gnss.Alt, 0),
		Satellites: int(utils.ParseFloatWithDefault(gnss.Sates, 0)),
	}

	//请求地图服务解析地址，需要考虑调用失败重试
	if tp == _GPS || tp == _GPS_BD || tp == _LBS { //GPS 或gps+北斗
		//转换坐标
		if len(gnss.Lng) == 0 || len(gnss.Lat) == 0 {
			return nil, fmt.Errorf("wrong gnss, lng,lati is invalid:(%s,%s)", gnss.Lng, gnss.Lat)
		}
		longi, err := strconv.ParseFloat(gnss.Lng, 64)
		if err != nil {
			return nil, fmt.Errorf("wrong gnss, lng,lati are invalid:(%s,%s)", gnss.Lng, gnss.Lat)
		}
		lati, err := strconv.ParseFloat(gnss.Lat, 64)
		if err != nil {
			return nil, fmt.Errorf("wrong gnss, lng,lati are invalid:(%s,%s)", gnss.Lng, gnss.Lat)
		}
		loc.Longitude, loc.Latitude = coordtransform.WGS84toGCJ02(longi, lati)
		var geoRes *services.GeoCoderResult = nil
		var lastErr error

		// 循环尝试不同的地理编码服务
		for i, locService := range h.locS {
			var err error
			geoRes, err = locService.Geocode(loc.Latitude, loc.Longitude, 2*time.Second)
			if err == nil {
				break // 成功则跳出循环
			}
			lastErr = err
			if i < len(h.locS)-1 {
				slog.Warn("invoking geocoder service failed, trying next service...", "error", err.Error())
			}
		}

		if geoRes == nil {
			return nil, fmt.Errorf("all geocoder services failed, last error: %v", lastErr)
		}
		loc.Address = geoRes.Address
	} else if tp == _WIFI {
		macs := strings.Split(gnss.Bssid, "|")
		rssi := strings.Split(gnss.Rssi, "|")
		wifiList := make([]*mxm.WiFiInfo, len(macs))
		for i := 0; i < len(macs); i++ {
			num, err := strconv.Atoi(rssi[i])
			if err != nil {
				return nil, fmt.Errorf("invalid rssi:%v", rssi[i])
			}
			wifiList[i] = &mxm.WiFiInfo{Mac: macs[i], Rssi: num}
		}
		//排序
		sort.Slice(wifiList, func(i, j int) bool {
			return wifiList[i].Mac > wifiList[j].Mac
		})
		print := mxm.WifiList(wifiList).JoinedMacs()
		locCache, ok := h.cache.GetCache("wifi", print)
		if !ok {
			succ := false
			defer func() {
				if succ {
					h.cache.SetCache("wifi", print, loc)
				}
			}()
			req := services.LocationRequest{
				WifiInfo: wifiList,
			}

			//等待结果/或失效
			var locRes *services.LocationResult
			var lastErr error

			// 循环尝试不同的位置服务
			for i, locService := range h.locS {
				var err error
				locRes, err = locService.LocateByNetwork(req, 2*time.Second)
				if err == nil {
					succ = true
					loc.Address = locRes.Address
					loc.Longitude = locRes.Location.Longitude
					loc.Latitude = locRes.Location.Latitude
					loc.Accuracy = locRes.Location.Accuracy
					break // 成功则跳出循环
				}
				lastErr = err
				if i < len(h.locS)-1 {
					slog.Warn("invoking loc service failed, trying next service...", "error", err.Error())
				}
			}

			if !succ {
				slog.Error("invoking loc service failed(all services): " + lastErr.Error())
			}
		} else {
			// TODO 相似度校验，若超过阈值也需要更新
			slog.Info("got cache")
			old := locCache.(mxm.Location)
			loc.Address = old.Address
			loc.Longitude = old.Longitude
			loc.Latitude = old.Latitude
			loc.Accuracy = old.Accuracy
		}
	} else if tp == _UNKNOWN {
		slog.Warn("unknown gnss type")
	}
	// 解析时间
	t, err := time.ParseInLocation("2006-01-02 15:04:05", gnss.TimeStr, time.Local)
	if err != nil {
		slog.Warn("wrong heartbeat, time is invalid:" + gnss.TimeStr)
		gnss.Time = time.Now()
	} else {
		gnss.Time = t
	}
	loc.LocTime = gnss.Time
	return &loc, nil
}

/*
*
btt 专有的充电状态检测算法，由于硬件不支持，只能动态检测
返回三种状态0b1000-充电，0b0110-模糊不确定, 0b0001-未充电，若波动大不好滤，可增加中间模糊位的数量
同时兼顾电量滤波功能
*/
func (h *bttDeviceStatusFactory) chargingStatus(originSN string, elect int) (bool, int) {

	var preE *ElectWithTm = nil
	now := time.Now()
	if pE, ok := h.cache.GetCache("btt_elect", originSN); ok {
		tmp, _ := pE.(ElectWithTm)
		preE = &tmp
	}

	// 电量判断置信模型
	// 如果上次的电量比较久（有可能曾经掉线），直接采用当前电量，但是不判断充电状态
	// 如果上次电量很近，且两次电量变化小于5%，则认为电量稳定，不更新状态位 -- 滤波
	// 如果上次电量比较近，且两次电量变化大于5%，则认为电量变化，更新状态位 -- 滤波
	// 如果上次电量比较近，且两次电量变化小于5%，则犹豫，谨慎 -- 滤波
	// 抽象：计算电量变化的速率并结合速率及时间差、电量差判断置信度
	// 时间差大，直接采信电量，但不判断充电状态；时间差中等，按速率判断；时间差小，电量差小时不采信

	if preE == nil {
		preE = &ElectWithTm{
			Elect:      elect,
			Tm:         now,
			Status:     0b0001,
			LazyStatus: false,
		}
	}
	tmDelta := now.Sub(preE.Tm).Minutes()            //时间差（分钟数）
	eDelta := float64(elect - preE.Elect)            // 电量差（百分比）
	if tmDelta <= 1 && eDelta <= 5 && eDelta >= -5 { // 时间倒置或很紧凑或电量无变化，直接不采信当前样本
		// do nothing
	} else if tmDelta >= 120 || eDelta <= -5 { //时间太长且电量下降，直接更新电量，但不判断充电状态
		preE.Elect = elect
		preE.Tm = now
	} else { //时间差中等，重点关注场景
		if eDelta == 0 && preE.LazyStatus { //电量持恒，削弱充电状态积累
			preE.Status >>= 1
			if preE.Status <= 0b0010 { //实锤时变更状态
				preE.Status = 0b0001
				preE.LazyStatus = false
			}
		}
		if eDelta > 5 { // 电量变化大于5%，认为充电
			preE.Status = 0b1000
			preE.LazyStatus = true
		} else if eDelta < -5 { // 电量变化小于-5%，认为放电
			preE.Status = 0b0001
			preE.LazyStatus = false
		} else if eDelta > 0 && preE.Status < 0b1000 {
			preE.Status <<= 1
			if preE.Status == 0b1000 { //实锤时变更状态
				preE.LazyStatus = true
			}
		} else if eDelta < 0 && preE.Status > 0b0010 { //对放电状态的累积要求比充电降低一点
			preE.Status >>= 1
			if preE.Status <= 0b0010 { //实锤时变更状态
				preE.Status = 0b0001
				preE.LazyStatus = false
			}
		}

		if preE.Status == 0b1000 || preE.Status == 0b0001 { //
			// 实锤状态达到或保持，则电量稳定，可以更新当前电量
			preE.Elect = elect
			preE.Tm = now
		}
	}

	h.cache.SetCache("btt_elect", originSN, *preE)
	return preE.LazyStatus, preE.Elect
}

func (h *bttDeviceStatusFactory) handleHeartBeat(msg *Message) (*mxm.Device, error) {

	var hb HeartBeat
	if err := json.Unmarshal(msg.Data, &hb); err != nil {
		return nil, fmt.Errorf("unmarshal:%v", err.Error())
	}

	if len(hb.GNSS) != 1 { //GNSS长度大于1，需要修改代码
		slog.Warn("length of GNSS is not 1" + string(msg.Data))
	} else if hb.GNSS[0].Lng == "" && hb.GNSS[0].Bssid == "" { //既无坐标又无wifi列表，gnss为无效数据
		slog.Warn("GNSS is empty:" + string(msg.Data))
	}

	vol, _ := strconv.Atoi(hb.BAT.Vol)
	csq, _ := strconv.Atoi(hb.Lte.Csq)
	interval, _ := strconv.Atoi(hb.Other.Gpstime)
	steps, _ := strconv.Atoi(hb.Health.Step)
	//locBytes, _ := json.Marshal(loc)
	var btt string = TYPE_BTT
	simSignal := CsqAsPercent(csq)
	elect := Vol2Percent(vol)
	d := mxm.Device{
		SimCardSignal: &simSignal,
		OriginSN:      &msg.DeviceSN,
		Type:          &btt,
		Interval:      &interval,
		Steps:         &steps,
	}

	//充电状态检测,由于电量波动大，此处电量做了滤波处理
	status, electFiltered := h.chargingStatus(msg.DeviceSN, elect)
	d.Charging = &status
	d.Electricity = &electFiltered

	// 坐标地址处理
	gnss := hb.GNSS[0]
	loc, err := h.handleGnss(&gnss)
	if err != nil {
		slog.Error("handleGnss failed", "gnss", gnss)
		t := time.Now()
		d.LastOnline = &t
	} else {
		d.Address = &loc.Address
		d.Latitude = &loc.Latitude
		d.Longitude = &loc.Longitude
		d.Altitude = &loc.Altitude
		d.Satellites = &loc.Satellites
		d.LocType = &loc.Type
		d.Accuracy = &loc.Accuracy
		d.LocTime = &loc.LocTime
		d.Speed = &loc.Speed
		d.Heading = &loc.Heading

		d.LastOnline = &loc.LocTime
	}
	return &d, nil
}
