package dao

import mxm "github.com/Daneel-Li/gps-back/internal/models"

func (d *MysqlRepository) GetOrdersByUserID(userId uint) (orders []*mxm.Order, err error) {
	err = d.db.Where("user_id = ?", userId).Find(&orders).Error
	return
}

func (d *MysqlRepository) CreateOrder(order *mxm.Order) (err error) {
	err = d.db.Debug().Omit("id").Create(order).Error
	return
}

func (d *MysqlRepository) UpdateOrder(order *mxm.Order) (err error) {
	err = d.db.Save(order).Error
	return
}

// GetOrderByID 根据ID获取订单
func (d *MysqlRepository) GetOrderByID(orderID string) (*mxm.Order, error) {
	var order mxm.Order
	if err := d.db.Where("id = ?", orderID).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// UpdateOrderStatus 更新订单状态
func (d *MysqlRepository) UpdateOrderStatus(orderID string, status string) error {
	return d.db.Model(&mxm.Order{}).Where("id = ?", orderID).Update("status", status).Error
}
