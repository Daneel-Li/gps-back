package config

import (
	"encoding/json"
	"encoding/pem"
	"os"

	"golang.org/x/exp/slog"
)

// DBConfig 存储数据库连接信息
type MysqlConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	DBName   string `json:"dbname"`
}

// TaosConfig 存储数据库连接信息
type TaosConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	DBName   string `json:"dbname"`
	Param    string `json:"param"`
}

type MqttConfig struct {
	Broker   string `json:"broker"`
	ClientID string `json:"clientid"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Tls struct {
	CertPath string `json:"cert_path"`
	KeyPath  string `json:"key_path"`
}

// 微信支付相关参数
type WechatPaymentConfig struct {
	WechatpayPublicKeyID   string `json:"wechatpay_public_key_id"`
	WechatpayPublicKeyPath string `json:"wechatpay_public_key_path"`
	AppID                  string `json:"app_id"`
	MchID                  string `json:"mch_id"`
	MchCertificateSerial   string `json:"mch_certificate_serial"`
	MchPrivateKeyPath      string `json:"mch_private_key_path"`
	NotifyURL              string `json:"notify_url"`
	DefaultPayerOpenID     string `json:"default_payer_open_id"` // 可选，用于测试
	MchAPIV3Key            string `json:"mch_apiv3_key"`
}

type Config struct {
	JT808Url                string      `json:"jt808_url"`
	TrialDeviceID           string      `json:"trial_device_id"` //TODO: should be a list
	JwtIssuer               string      `json:"jwt_issuer"`
	MxmAPIKey               string      `json:"api_key"`
	Tls                     Tls         `json:"tls"`
	Mysql                   MysqlConfig `json:"mysql"`
	Taos                    TaosConfig  `json:"taos"`
	Mqtt                    MqttConfig  `json:"mqtt"`
	Loglevel                string      `json:"log_level"`
	TxAppKey                string      `json:"tx_app_key"`
	WayzAppKey              string      `json:"wayz_app_key"`
	TxLocNetMaxConCurrent   int32       `json:"tx_max_loc_net_qps"`
	TxGeocoderMaxConCurrent int32       `json:"tx_max_geocoder_qps"`
	WzLocNetMaxConCurrent   int32       `json:"wz_max_loc_net_qps"`
	ServerPort              int32       `json:"server_port"`
	AppID                   string      `json:"app_id"`
	AppSecret               string      `json:"app_secret"`
	JwtKeyPath              string      `json:"jwt_key_path"` // jwt加密密钥路径
	JwtKey                  []byte
	DataPath                string              `json:"data_path"`      //数据路径
	AvatarPath              string              `json:"avatar_path"`    //	头像存储路径
	WechatPayment           WechatPaymentConfig `json:"wechat_payment"` // 微信支付相关参数
}

var (
	config *Config
)

func LoadConfig(path string) {
	configData, err := os.ReadFile(path)
	if err != nil {
		slog.Error("error load config:"+err.Error(), "path", path)
		return
	}

	config = &Config{}
	if err := json.Unmarshal(configData, config); err != nil {
		slog.Error("error load config:"+err.Error(), "path", path)
		return
	}

	pemData, err := os.ReadFile(config.JwtKeyPath)
	if err != nil {
		slog.Error("无法读取jwt私钥文件:" + err.Error())
	}
	// 解码PEM格式的密钥
	block, _ := pem.Decode(pemData)
	if block == nil {
		slog.Error("无效的PEM格式")
	} else {
		config.JwtKey = block.Bytes
	}
}

func GetConfig() *Config {
	if config == nil {
		LoadConfig("./config.json")
	}
	return config
}
