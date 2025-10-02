package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Daneel-Li/gps-back/internal/dao"
	mxm "github.com/Daneel-Li/gps-back/internal/models"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	wechatpay_utils "github.com/wechatpay-apiv3/wechatpay-go/utils"

	"github.com/Daneel-Li/gps-back/internal/config"
	"github.com/Daneel-Li/gps-back/pkg/utils"
)

type PayRequest struct {
	Package   string `json:"package"`
	PaySign   string `json:"paySign"`
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
}

type PaymentHandler struct {
	cfg  *config.WechatPaymentConfig
	repo dao.Repository
	// 可以注入其他依赖如数据库等
}

func NewPaymentHandler(cfg *config.WechatPaymentConfig, repo dao.Repository) *PaymentHandler {
	return &PaymentHandler{cfg: cfg, repo: repo}
}

func (h *PaymentHandler) Renew(userid uint, payerOpenID string, deviceId string, amount int) (*PayRequest, error) {
	// 1. 生成订单信息
	// 1. 初始化微信支付客户端
	client, err := h.initWechatPayClient()
	if err != nil {
		slog.Error("failed to init wechat pay client", "error", err)
		return nil, fmt.Errorf("failed to init wechat pay client: %v", err)
	}

	// 2. 获取请求参数（可以从请求中获取或使用默认值）
	openID := payerOpenID

	// 3. 生成订单信息
	order := &mxm.Order{
		OrderNo:     h.generateOrderNo(),
		Description: "设备续费", // tODO 可根据业务自定义
		Amount:      amount, // 单位:分
		OpenID:      openID,
		Status:      mxm.OrderStatusCreated,
		UserID:      userid,
		DeviceInfo:  deviceId, // 可根据业务自定义
	}

	// 保存订单到数据库
	if err := h.repo.CreateOrder(order); err != nil {
		return nil, fmt.Errorf("create order failed: %w", err)
	}

	// 4. 调用微信支付接口
	svc := jsapi.JsapiApiService{Client: client}
	resp, result, err := svc.Prepay(
		context.Background(),
		jsapi.PrepayRequest{
			Appid:       core.String(h.cfg.AppID),
			Mchid:       core.String(h.cfg.MchID),
			Description: core.String(order.Description),
			OutTradeNo:  core.String(order.OrderNo),
			Attach:      core.String(order.Attach),
			NotifyUrl:   core.String(strings.Replace(h.cfg.NotifyURL, `/{device_id}/`, deviceId, -1)),
			Payer:       &jsapi.Payer{Openid: core.String(openID)},
			Amount:      &jsapi.Amount{Total: core.Int64(int64(order.Amount))},
		},
	)

	if err != nil {
		slog.Error("wechat pay prepay failed",
			"error", err,
			"status", result.Response.StatusCode)
		return nil, fmt.Errorf("wechat pay prepay failed: error=%v, status=%d", err, result.Response.StatusCode)
	}

	// 5. 生成支付签名
	ts := time.Now().Format("20060102150405")
	nonceStr := utils.RandomString(32)
	source := fmt.Sprintf("%s\n%s\n%s\n%s\n",
		h.cfg.AppID,
		ts,
		nonceStr,
		*resp.PrepayId,
	)

	res, err := client.Sign(context.Background(), source)
	if err != nil {
		return nil, fmt.Errorf("sign error: %w", err)
	}

	// 6. 返回支付参数
	return &PayRequest{
		//OrderNo:   order.OrderNo,
		Package:   "prepay_id=" + *resp.PrepayId,
		PaySign:   res.Signature,
		TimeStamp: ts,
		NonceStr:  nonceStr,
	}, nil
}

// 初始化微信支付客户端
func (h *PaymentHandler) initWechatPayClient() (*core.Client, error) {
	publicKey, err := wechatpay_utils.LoadPublicKeyWithPath(h.cfg.WechatpayPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load public key error: %w", err)
	}

	mchPrivateKey, err := wechatpay_utils.LoadPrivateKeyWithPath(h.cfg.MchPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load private key error: %w", err)
	}

	opts := []core.ClientOption{
		option.WithWechatPayPublicKeyAuthCipher(
			h.cfg.MchID,
			h.cfg.MchCertificateSerial,
			mchPrivateKey,
			h.cfg.WechatpayPublicKeyID,
			publicKey,
		),
	}

	return core.NewClient(context.Background(), opts...)
}

// 生成订单号
func (h *PaymentHandler) generateOrderNo() string {
	return fmt.Sprintf("%s%d", time.Now().Format("20060102"), utils.RandomInt(100000, 999999))
}

// 生成支付签名
func (h *PaymentHandler) generatePaySign(client *core.Client, prepayID *string) (string, error) {
	source := fmt.Sprintf("%s\n%s\n%s\n%s\n",
		h.cfg.AppID,
		time.Now().Format("20060102150405"),
		utils.RandomString(32),
		*prepayID,
	)

	res, err := client.Sign(context.Background(), source)
	if err != nil {
		return "", fmt.Errorf("sign error: %w", err)
	}

	return res.Signature, nil
}
