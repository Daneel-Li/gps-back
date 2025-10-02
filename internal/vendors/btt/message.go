package btt

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
)

const (
	SET_REPORTINTERVAL = "3002"
	LOCATE             = "2005"
	FIND               = "3006"
	CMD_REPLY          = "8001" //指令响应
	POWRER             = "3900"
	QUERY_ELECTRICITY  = "2006"

	REPORT_HEARTBEAT = "1002" //心跳上报
	// {
	// 	"messageId": 102,
	// 	"deviceSN": "868909071429404",
	// 	"dataType": "1002",
	// 	"data": {
	// 		"LTE": {
	// 			"csq": "13"
	// 		},
	// 		"GNSS": [
	// 			{
	// 				"lng": "",
	// 				"lat": "",
	// 				"alt": "102",
	// 				"speed": "",
	// 				"direc": "",
	// 				"bssid": "5C:02:14:FD:89:74|EC:CF:70:6A:E4:46|48:E5:33:36:0A:6E|F0:55:01:31:E8:E0|F0:55:01:31:E8:E1|F0:55:01:31:E8:E5|E8:D7:65:CC:E4:44|D8:3D:CC:05:5A:8D|AC:88:66:6D:3C:11|F4:E4:51:CF:13:3C|98:0D:51:3A:91:FC|CC:08:FB:8D:12:CC|30:CC:21:35:42:57|32:CC:21:35:42:57|F4:E4:51:36:C1:70|D4:35:38:D0:CC:3E|14:00:7D:E8:E6:C8",
	// 				"rssi": "-46|-47|-55|-64|-64|-65|-71|-74|-76|-82|-83|-86|-91|-91|-91|-96|-97",
	// 				"sates": "0",
	// 				"snr": "",
	// 				"time": "2025-04-03 13:43:42",
	// 				"type": "1"
	// 			}
	// 		],
	// 		"BAT": {
	// 			"vol": "4020"
	// 		},
	// 		"other": {
	// 			"gpstime": "1800"
	// 		},
	// 		"health": {
	// 			"step": "0"
	// 		}
	// 	}
	// }

	DEVICE_INFO = "1001" //设备信息上报
	// {
	//     "messageId": 101,
	//     "deviceSN": "869861062618140",
	//     "dataType": "1001",
	//     "data": {
	//         "DeviceInfo": {
	//             "productKey": "Air780EG",
	//             "project": "dwq_fanqie",
	//             "luatos_version": "V1110",
	//             "deviceType": "dingweiqi",
	//             "hwVersion": "1.0.0",
	//             "swVersion": "1.81",
	//             "imsi": "460083005400269",
	//             "imei": "869861062618140",
	//             "iccid": "898604B0162380390269",
	//             "buildDate": "Jun  7 2024"
	//         },
	//         "Heartbeat": {
	//             "gpstime": "1800"
	//         },
	//         "BAT": {
	//             "vol": "3404"
	//         }
	//     }
	// }

	ALARM = "1003" //报警上报
	// {
	//     "messageId": 103,
	//     "deviceSN": "869861062618140",
	//     "dataType": "1003",
	//     "data": {
	//         "Alarm": {
	//             "type": "1",     // 报警类型(1:SOS 2:低电 3:拆除)
	//             "value": "3",    // 报警级别
	//             "vol": "3032"    // 当前电压(mV)
	//         }
	//     }
	// }

	REPLY = "8001" //回复消息（TODO：可能不严谨）
	// {
	// 	"messageId":108,"deviceSN":"869861062618140",
	//  "dataType":"8001",
	//  "code":"1",
	//  "repDataType":"2005"}
	// }
)

type Message struct {
	MessageId   int64           `json:"messageId"`
	DeviceSN    string          `json:"deviceSN"`
	DataType    string          `json:"dataType"`
	Data        json.RawMessage `json:"data"`
	Code        string          `json:"code"`
	RepDataType string          `json:"repDataType"`
}

func (m Message) ToBytes() []byte {
	b, err := json.Marshal(m)
	if err != nil {
		slog.Error("message marshal failed: ", "err", err)
		return []byte{}
	}
	return b
}

// 专为处理btt小程序有些消息的id是字符串的情况而设计的
type FlexibleMessage struct {
	*Message                 // 嵌入原始结构体
	RawMessageID interface{} `json:"messageId"` // 捕获原始字段
}

func (m *FlexibleMessage) UnmarshalJSON(data []byte) error {
	// 创建临时类型避免递归调用
	type Alias FlexibleMessage
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// 统一处理messageId
	switch v := m.RawMessageID.(type) {
	case float64:
		m.Message.MessageId = int64(v)
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid messageId format: %v", v)
		}
		m.Message.MessageId = parsed
	case nil:
		// 处理字段缺失情况
		m.Message.MessageId = 0 // 或其它默认值
	default:
		return fmt.Errorf("unsupported messageId type: %T", v)
	}

	return nil
}
