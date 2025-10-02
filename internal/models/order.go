package mxm

import (
	"time"
)

type OrderStatus string

const (
	OrderStatusCreated   OrderStatus = "created"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusRefunding OrderStatus = "refunding"
	OrderStatusRefunded  OrderStatus = "refunded"
	OrderStatusClosed    OrderStatus = "closed"
	OrderStatusFailed    OrderStatus = "failed"
)

type Order struct {
	ID            int         `gorm:"primaryKey;type:int;autoIncrement;comment:订单ID"`
	OrderNo       string      `gorm:"uniqueIndex;type:varchar(32);not null;comment:商户订单号"`
	Description   string      `gorm:"type:varchar(255);not null;comment:订单描述"`
	Amount        int         `gorm:"not null;comment:订单金额(单位:分)"`
	Currency      string      `gorm:"type:varchar(3);not null;default:'CNY';comment:货币类型"`
	OpenID        string      `gorm:"column:openid;type:varchar(32);comment:用户openid"`
	TransactionID string      `gorm:"type:varchar(32);comment:微信支付订单号"`
	Status        OrderStatus `gorm:"type:enum('created','paid','refunding','refunded','closed','failed');not null;default:'created';comment:订单状态"`
	Attach        string      `gorm:"type:varchar(255);comment:附加数据"`
	PrepayID      string      `gorm:"type:varchar(64);comment:预支付交易会话标识"`
	CreatedAt     time.Time   `gorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
	UpdatedAt     time.Time   `gorm:"not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;comment:更新时间"`
	PaidAt        *time.Time  `gorm:"comment:支付成功时间"`
	ExpireTime    *time.Time  `gorm:"comment:订单过期时间"`
	RefundAmount  int         `gorm:"default:0;comment:已退款金额(单位:分)"`
	UserID        uint        `gorm:"type:int;comment:关联用户ID"`
	DeviceInfo    string      `gorm:"type:varchar(32);comment:设备信息"`
}

func (Order) TableName() string {
	return "orders"
}
