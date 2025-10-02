package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Daneel-Li/gps-back/internal/config"
	mxm "github.com/Daneel-Li/gps-back/internal/models"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/google/uuid"
	"github.com/taosdata/driver-go/v3/errors"
	"golang.org/x/time/rate"
)

// LocationService 位置服务接口
type LocationService interface {
	// LocateByNetwork 通过网络信息进行定位
	LocateByNetwork(params LocationRequest, timeout time.Duration) (*LocationResult, error)

	// Geocode 逆地址解析
	Geocode(latitude, longitude float64, timeout time.Duration) (*GeoCoderResult, error)
}

// txLocationService 腾讯地图位置服务实现
type txLocationService struct{}

// NewTxLocationService 创建腾讯地图位置服务
func NewTxLocationService() *txLocationService {
	return &txLocationService{}
}

// LocateByNetwork 通过网络信息进行定位
func (s *txLocationService) LocateByNetwork(params LocationRequest, timeout time.Duration) (*LocationResult, error) {
	return txLocNetwork(params, timeout)
}

// Geocode 逆地址解析
func (s *txLocationService) Geocode(latitude, longitude float64, timeout time.Duration) (*GeoCoderResult, error) {
	return txGeocoder(latitude, longitude, timeout)
}

/**
调用外部服务
*/

const (
	_TX_NETWORK  = "https://apis.map.qq.com/ws/location/v1/network"
	_TX_GEOCODER = "https://apis.map.qq.com/ws/geocoder/v1/"
)

// TODO 看是否需要封装
var (
	goPool            gopool.Pool
	txLocNetLimiter   *rate.Limiter // tx 的融合定位限流器，官网免费qps为5
	txGeocoderLimiter *rate.Limiter // tx 的逆地址解析限流器，官网免费qps为100
	once              sync.Once
	wzLocNetLimiter   *rate.Limiter // wz 的融合定位限流器，免费qps为1
)

func InitService() {
	if goPool != nil {
		return
	}
	initTxService()
	initWzService()
	initGoPool()
}

// 允许用户手动刷新配置
func initWzService() {
	maxLocNet := config.GetConfig().WzLocNetMaxConCurrent
	if maxLocNet < 1 {
		maxLocNet = 1
	}
	wzLocNetLimiter = rate.NewLimiter(rate.Limit(maxLocNet), int(maxLocNet))
}

func initGoPool() {
	once.Do(func() {
		goPool = gopool.NewPool("tx_Pool", 200, gopool.NewConfig())
	})
}

func initTxService() {
	maxLocNet := config.GetConfig().TxLocNetMaxConCurrent
	if maxLocNet < 1 {
		maxLocNet = 5
	}
	maxGeoCoder := config.GetConfig().TxGeocoderMaxConCurrent
	if maxGeoCoder < 1 {
		maxGeoCoder = 100
	}

	txGeocoderLimiter = rate.NewLimiter(rate.Limit(maxGeoCoder), int(maxGeoCoder)) //每秒产生100个令牌，桶的大小为100
	txLocNetLimiter = rate.NewLimiter(rate.Limit(maxLocNet), int(maxLocNet))       //每秒产生maxC个令牌，桶的大小为maxC
}

func getTxMapApiKey() string {
	return config.GetConfig().TxAppKey
}

func getWayzMapApiKey() string {
	return config.GetConfig().WayzAppKey
}

/**
腾讯地图服务，智能硬件定位
URL：https://apis.map.qq.com/ws/location/v1/network
Method：POST
Header：Content-Type:application/json
*/

func getTXMapApiResponse(body []byte) (*TxApiResponse, error) {
	// 解析响应数据到 Response 结构体
	var response TxApiResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	// 检查状态码
	if response.Status != 0 {
		return nil, fmt.Errorf("error occurred: %s", response.Message)
	}
	return &response, nil
}

func callService(body any, args url.Values, url string, method string, timeout time.Duration, limiter *rate.Limiter) ([]byte, error) {
	if goPool == nil {
		panic("you shuold invoke InitService() first")
	}
	// 将请求参数编码为 JSON
	var bodyRaw []byte
	var err error
	if body != nil {
		bodyRaw, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	if len(args) > 0 {
		url = url + "?" + args.Encode()
	}
	// 创建一个 HTTP POST 请求
	req, err := http.NewRequest(method, url, bytes.NewBuffer(bodyRaw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求并获取响应
	client := &http.Client{}

	ch1 := make(chan *http.Response)
	ch2 := make(chan error)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	goPool.Go(func() {

		// 1. 内部快速检查（防止无效操作）
		if ctx.Err() != nil {
			return
		}

		//等令牌，限制最高并发量
		if err := limiter.Wait(ctx); err != nil {
			ch2 <- errors.NewError(-1, "time out while waiting token")
			return
		}

		resp, err := client.Do(req.WithContext(ctx))
		if err != nil {
			ch2 <- err
			return
		}
		select { //确保ch1不会阻塞（万一写ch1时外层已经返回，goroutine就会阻塞，占用一个pool）
		case ch1 <- resp:
		default:
			resp.Body.Close()
		}
	})
	select {
	case <-ctx.Done():
		return nil, errors.NewError(-1, "time out")
	case err := <-ch2:
		return nil, err
	case resp := <-ch1:
		defer resp.Body.Close()

		// 读取响应数据
		return io.ReadAll(resp.Body)
	}
}

func txLocNetwork(params LocationRequest, timeout time.Duration) (*LocationResult, error) {
	for _, w := range params.WifiInfo {
		strings.ReplaceAll(w.Mac, ":", "")
	}
	req := struct {
		LocationRequest
		Key  string `json:"key"`
		Test string `json:"test"`
	}{
		LocationRequest: params,
		Key:             getTxMapApiKey(),
	}

	body, err := callService(req, nil, _TX_NETWORK, "POST", 2*time.Second, txLocNetLimiter)
	if err != nil {
		return nil, err
	}
	resp, err := getTXMapApiResponse(body)
	if err != nil {
		return nil, err
	}
	var res LocationResult
	if err := json.Unmarshal(*resp.Result, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

/*
*
逆地址解析 https://apis.map.qq.com/ws/geocoder/v1/?location=
*/
func txGeocoder(latitude float64, longitude float64, timeout time.Duration) (*GeoCoderResult, error) {
	args := url.Values{}
	args.Add("location", fmt.Sprintf("%f,%f", latitude, longitude))
	args.Add("key", getTxMapApiKey())
	body, err := callService(nil, args, _TX_GEOCODER, "GET", 2*time.Second, txGeocoderLimiter)
	if err != nil {
		return nil, err
	}
	resp, err := getTXMapApiResponse(body)
	if err != nil {
		return nil, err
	}
	var res GeoCoderResult
	if err := json.Unmarshal(*resp.Result, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

type wzLocationService struct{}

// NewWzLocationService 创建微智地图位置服务
func NewWzLocationService() *wzLocationService {
	return &wzLocationService{}
}

// LocateByNetwork 通过网络信息进行定位
func (s *wzLocationService) LocateByNetwork(params LocationRequest, timeout time.Duration) (*LocationResult, error) {
	return wzLocNetwork(params, timeout)
}

// Geocode 逆地址解析
func (s *wzLocationService) Geocode(latitude, longitude float64, timeout time.Duration) (*GeoCoderResult, error) {
	return wz_Geocoder(latitude, longitude, timeout)
}

/*
https://api.newayz.com/location/hub/v1/track_points?access_key=&response_sprf=gcj02
文档见 https://lothub.newayz.com/pdf/api.pdf
*/
func wzLocNetwork(params LocationRequest, timeout time.Duration) (*LocationResult, error) {
	// API 基础URL
	baseURL := "https://api.newayz.com/location/hub/v1/track_points"

	type Wifi struct {
		MacAddress     string `json:"macAddress"`
		SignalStrength int    `json:"signalStrength"`
	}
	type Asset struct {
		ID string `json:"id"`
	}
	type Desc struct {
		Wifis []Wifi `json:"wifis"`
	}

	//构建body参数
	wayzParams := struct {
		Asset    Asset `json:"asset"`
		Location Desc  `json:"location"`
	}{
		Asset: Asset{
			ID: uuid.New().String(),
		},
		Location: Desc{
			Wifis: []Wifi{},
		},
	}
	for _, w := range params.WifiInfo {
		r := w.Rssi
		if r < 0 {
			r = -r
		}
		wayzParams.Location.Wifis = append(wayzParams.Location.Wifis, Wifi{
			MacAddress:     w.Mac,
			SignalStrength: r,
		})
	}

	// 构建查询参数
	queryParams := url.Values{}
	queryParams.Add("access_key", getWayzMapApiKey())
	queryParams.Add("response_sprf", "gcj02")

	// 调用通用服务
	body, err := callService(wayzParams, queryParams, baseURL, "POST", timeout, wzLocNetLimiter)
	if err != nil {
		return nil, fmt.Errorf("call Wayz location service failed: %v", err)
	}

	// 解析响应
	type TmpLocationResult struct {
		Location struct {
			Address struct {
				Name string `json:"name"`
			} `json:"address"`
			Position struct {
				Point struct {
					Longitude float64 `json:"longitude"`
					Latitude  float64 `json:"latitude"`
				} `json:"point"`
				Accuracy float64 `json:"accuracy"`
			} `json:"position"`
		} `json:"location"`
	}
	var tmp TmpLocationResult
	if err := json.Unmarshal(body, &tmp); err != nil {
		return nil, fmt.Errorf("unmarshal Wayz response failed: %v", err)
	}
	if tmp.Location.Address.Name == "" {
		return nil, fmt.Errorf("wayz location service return empty address")
	}

	result := LocationResult{
		Location: &Location{
			Latitude:  tmp.Location.Position.Point.Latitude,
			Longitude: tmp.Location.Position.Point.Longitude,
			Accuracy:  tmp.Location.Position.Accuracy,
		},
		Address: tmp.Location.Address.Name,
	}

	return &result, nil
}

/*
*
逆地址解析 https://api.newayz.com/location/hub/v1/track_points?access_key=
*/
func wz_Geocoder(latitude float64, longitude float64, timeout time.Duration) (*GeoCoderResult, error) {
	//构建查询参数
	args := url.Values{}
	args.Add("access_key", getWayzMapApiKey())

	//构建body参数
	reqBody := map[string]interface{}{
		"asset": uuid.New().String(),
		"location": WzLocation{
			Position: WzPosition{
				Point: WzPoint{
					Latitude:  latitude,
					Longitude: longitude,
				},
			},
		},
	}

	body, err := callService(reqBody, args, _TX_GEOCODER, "GET", 2*time.Second, txGeocoderLimiter)
	if err != nil {
		return nil, err
	}
	resp, err := getWzMapApiResponse(body)
	if err != nil {
		return nil, err
	}

	if resp.Location.Address.Name == "" {
		return nil, fmt.Errorf("wayz geocoder service return empty address")
	}
	return &GeoCoderResult{
		Address: resp.Location.Address.Name,
	}, nil
}

func getWzMapApiResponse(body []byte) (*WzApiResponse, error) {
	var res WzApiResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// 定义基站信息结构体
type CellInfo struct {
	Mcc    int     `json:"mcc"`
	Mnc    int     `json:"mnc"`
	Lac    int     `json:"lac"`    // GSM制式传lac，CDMA制式传nid，LTE和5G传tac
	Cellid int     `json:"cellid"` // CDMA制式传bid，5G传nci，其它制式传cellid
	Rss    float64 `json:"rss"`
}

/**
* 返回wifilist的macs拼接串
 */

// 定义蓝牙信息结构体
type BeaconInfo struct {
	Mac   string    `json:"mac"`
	Major int       `json:"major"`
	Minor int       `json:"minor"`
	Rssi  float64   `json:"rssi"`
	Time  time.Time `json:"time"`
}

// 定义GPS信息结构体
type GPSInfo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Accuracy  float64 `json:"accuracy"`
	Speed     float64 `json:"speed"`
	Bearing   float64 `json:"bearing"`
	Viewstar  int     `json:"viewstar"`
	Usedstar  int     `json:"usedstar"`
}

// 定义请求参数结构体
type LocationRequest struct {
	DeviceID   string          `json:"device_id"`
	GetPOI     *int            `json:"get_poi,omitempty"` // 使用指针以便可以区分0和未设置的情况
	GPSInfo    GPSInfo         `json:"gpsinfo,omitempty"`
	CellInfo   []CellInfo      `json:"cellinfo,omitempty"`
	WifiInfo   []*mxm.WiFiInfo `json:"wifiinfo,omitempty"`
	BeaconInfo []*BeaconInfo   `json:"beaconinfo,omitempty"`
}

// 定义定位结果中的 location 结构体
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Accuracy  float64 `json:"accuracy"`
}

// 定义行政区划信息结构体
type AdInfo struct {
	Adcode   string `json:"adcode"`
	Nation   string `json:"nation"`
	Province string `json:"province"`
	City     string `json:"city"`
	District string `json:"district"`
}

// 定义 POI（地点信息）结构体
type POI struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Address  string    `json:"address"`
	Category string    `json:"category"`
	Location *Location `json:"location"`
	// Distance and DirDesc 只在数组 pois 中使用
	// _distance    float64 `json:"_distance"`
	// _dirDesc     string  `json:"_dir_desc"`
}

// 定义请求结果的结构体
type LocationResult struct {
	Address  string    `json:"address,omitempty"`
	Location *Location `json:"location,omitempty"`
	AdInfo   *AdInfo   `json:"ad_info,omitempty"`
	POIs     []POI     `json:"pois,omitempty"`
}

// 定义响应结果的结构体
type TxApiResponse struct {
	Status    int              `json:"status"`
	Message   string           `json:"message"`
	RequestID string           `json:"request_id"`
	Result    *json.RawMessage `json:"result,omitempty"`
}

// 用于检查 Result 是否为空
func (r *LocationResult) IsEmpty() bool {
	// 根据实际字段判断是否为空
	return r.Address == "" && r.Location == nil && r.AdInfo == nil && len(r.POIs) == 0
}

// 用于检查 Result 中的 Location 是否为空
func (r *LocationResult) LocationIsEmpty() bool {
	if r.Location == nil {
		return true
	}
	return r.Location.Latitude == 0 && r.Location.Longitude == 0 && r.Location.Altitude == 0 && r.Location.Accuracy == 0
}

type GeoCoderResult struct {
	Address string `json:"address"`
}

// Wz结构体
type WzAddress struct {
	Name string `json:"name"`
}
type WzPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
type WzPosition struct {
	Point WzPoint `json:"point"`
}
type WzLocation struct {
	Address  WzAddress  `json:"address"`
	Position WzPosition `json:"position"`
}
type WzApiResponse struct {
	Location WzLocation `json:"location"`
}
