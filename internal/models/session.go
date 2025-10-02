package mxm

// SessionResponse 用于接收微信接口返回的 session 信息
type SessionResponse struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	Errcode    int    `json:"errcode"`
	Errmsg     string `json:"errmsg"`
}
