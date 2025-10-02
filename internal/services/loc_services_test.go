package services

import (
	"testing"
	"time"

	mxm "github.com/Daneel-Li/gps-back/internal/models"

	"github.com/Daneel-Li/gps-back/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestTxLocNetwork(t *testing.T) {
	config.LoadConfig("../../config.json")
	InitService()
	req := LocationRequest{
		WifiInfo: []*mxm.WiFiInfo{
			&mxm.WiFiInfo{Mac: "C8:3A:35:43:2D:48", Rssi: -80},
			&mxm.WiFiInfo{Mac: "70:3A:73:04:15:3C", Rssi: -88},
			&mxm.WiFiInfo{Mac: "70:3A:73:0C:15:3C", Rssi: -90},
		},
	}
	locS := NewTxLocationService()
	locRes, err := locS.LocateByNetwork(req, 2*time.Second)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, locRes)
	assert.NotNil(t, locRes.Location)
	//assert.True(t, locRes.Location.Accuracy > 200)
}
