package btt

import (
	"strings"
	"time"
)

type Lte struct {
	Csq string `json:"csq"`
}

type Gnss struct {
	Lng     string `json:"lng"`
	Lat     string `json:"lat"`
	Alt     string `json:"alt"`
	Speed   string `json:"speed"`
	Direc   string `json:"direc"`
	Bssid   string `json:"bssid"`
	Rssi    string `json:"rssi"`
	Sates   string `json:"sates"`
	Snr     string `json:"snr"`
	TimeStr string `json:"time"`
	Time    time.Time
	Type    string `json:"type"`
}

type Bat struct {
	Vol      string `json:"vol"`
	Charging int    `json:"charging"`
}

type Other struct {
	Temp    string `json:"temp"`
	Gpstime string `json:"gpstime"`
}

type Health struct {
	Step string `json:"step"`
}

type HeartBeat struct {
	Lte    Lte    `json:"LTE"`
	GNSS   []Gnss `json:"GNSS"`
	BAT    Bat    `json:"BAT"`
	Other  Other  `json:"other"`
	Health Health `json:"health"`
}

func TypeAsInt(sType string) int {
	//1 wifi定位 2 GPS+北斗 定位 3GPS定位 4 北斗定位 5基站定位 6手机定位 99:未知定位
	switch strings.ToUpper(sType) {
	case "WIFI":
		return 1
	case "GPS+北斗":
		return 2
	case "GPS":
		return 3
	case "北斗":
		return 4
	case "LBS":
		return 5
	case "手机":
		return 6
	default:
		return 99
	}
}

func TypeAsString(iType int) string {
	//1 wifi定位 2 GPS+北斗 定位 3GPS定位 4 北斗定位 5基站定位 6手机定位 99:未知定位
	switch iType {
	case 1:
		return "WIFI"
	case 2:
		return "GPS"
	case 3:
		return "GPS"
	case 4:
		return "GPS"
	case 5:
		return "LBS"
	case 6:
		return "手机"
	default:
		return "未知"
	}
}

// 信号值csq换算成百分值
// 以下数值非专业换算，纯属个人预估
func CsqAsPercent(csq int) int {
	if csq >= 30 { //很强
		return 100
	} else if csq >= 25 { //强
		return int(85 + (csq-25)*(100-85)/(30-25))
	} else if csq >= 20 { //中高
		return int(65 + (csq-20)*(85-65)/(25-20))
	} else if csq >= 17 { //中
		return int(45 + (csq-17)*(65-45)/(20-17))
	} else if csq >= 14 { //中低
		return int(25 + (csq-14)*(45-25)/(17-14))
	} else if csq >= 10 { //低
		return int(15 + (csq-10)*(25-10)/(14-10))
	} else { //弱
		return int(0 + (csq-0)*(15-0)/(10-0))
	}
}

// Vol2Percent 根据电压值通过线性插值计算电量百分比
func Vol2Percent(vol int) int {
	// 找到电压所在的区间
	res := 100 //初始100%
	for _, level := range voltageLevels {
		if level.voltage <= vol {
			return level.percentage
		}
		res = level.percentage
	}
	// 如果电压不在任何区间内，默认返回0%
	return res
}

// 电压区间和对应的起始电量百分比
var voltageLevels = []struct {
	voltage    int
	percentage int
}{
	{4170, 100},
	{4140, 100},
	{4126, 99},
	{4112, 98},
	{4102, 97},
	{4086, 96},
	{4076, 95},
	{4064, 94},
	{4060, 93},
	{4050, 92},
	{4040, 91},
	{4034, 90},
	{4026, 89},
	{4024, 88},
	{4020, 87},
	{4018, 86},
	{4016, 85},
	{4012, 84},
	{4008, 83},
	{4002, 82},
	{3996, 81},
	{3993, 80},
	{3990, 79},
	{3984, 78},
	{3982, 77},
	{3976, 76},
	{3970, 75},
	{3962, 74},
	{3958, 73},
	{3952, 72},
	{3944, 71},
	{3936, 70},
	{3932, 69},
	{3926, 68},
	{3918, 67},
	{3910, 66},
	{3906, 65},
	{3898, 64},
	{3892, 63},
	{3884, 62},
	{3876, 61},
	{3868, 60},
	{3862, 59},
	{3858, 58},
	{3852, 57},
	{3856, 56},
	{3842, 55},
	{3838, 54},
	{3830, 53},
	{3822, 52},
	{3818, 51},
	{3816, 50},
	{3814, 49},
	{3810, 48},
	{3800, 47},
	{3796, 46},
	{3792, 45},
	{3786, 44},
	{3780, 43},
	{3772, 42},
	{3766, 41},
	{3756, 40},
	{3748, 39},
	{3742, 38},
	{3732, 37},
	{3722, 36},
	{3710, 35},
	{3700, 34},
	{3690, 33},
	{3680, 32},
	{3674, 31},
	{3662, 30},
	{3650, 29},
	{3638, 28},
	{3630, 27},
	{3618, 26},
	{3614, 25},
	{3604, 24},
	{3596, 23},
	{3590, 22},
	{3582, 21},
	{3576, 20},
	{3564, 19},
	{3554, 18},
	{3544, 17},
	{3534, 16},
	{3524, 15},
	{3514, 14},
	{3508, 13},
	{3496, 12},
	{3490, 11},
	{3484, 10},
	{3478, 9},
	{3472, 8},
	{3464, 7},
	{3456, 6},
	{3442, 5},
	{3420, 4},
	{3380, 3},
	{3326, 2},
	{3260, 1},
	{3200, 0},
}
