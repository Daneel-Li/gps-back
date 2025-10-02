package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Daneel-Li/gps-back/internal/config"
	mxm "github.com/Daneel-Li/gps-back/internal/models"
)

type WechatService interface {
	GetSession(code string) (*mxm.SessionResponse, error)
}

func NewWechatService() WechatService {
	return &wechatServiceImpl{}
}

type wechatServiceImpl struct {
}

// getSessionFromWechat 使用微信提供的 code 调用微信接口获取 session_key 和 openid
func (w *wechatServiceImpl) GetSession(code string) (*mxm.SessionResponse, error) {
	cfg := config.GetConfig()
	AppID, AppSecret := cfg.AppID, cfg.AppSecret
	url := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code", AppID, AppSecret, code)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var session mxm.SessionResponse
	err = json.Unmarshal(body, &session)
	if err != nil {
		return nil, err
	}

	return &session, nil
}
