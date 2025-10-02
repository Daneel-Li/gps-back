package dao

import (
	"fmt"

	mxm "github.com/Daneel-Li/gps-back/internal/models"
)

func (d *MysqlRepository) CreateFeedback(feedback *mxm.Feedback) error {
	if err := d.db.Create(feedback).Error; err != nil {
		return fmt.Errorf("create feedback failed: %w", err)
	}
	return nil
}

func (d *MysqlRepository) AddFeedback(feedback *mxm.Feedback) error {
	if err := d.db.Create(feedback).Error; err != nil {
		return fmt.Errorf("add feedback failed: %w", err)
	}
	return nil
}

func (d *MysqlRepository) GetFeedbacksByUserID(userId uint, limit, offset int) ([]*mxm.Feedback, error) {
	var feedbacks []*mxm.Feedback
	query := d.db.Model(mxm.Feedback{}).Where("user_id=?", userId)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&feedbacks).Error; err != nil {
		return nil, fmt.Errorf("get feedbacks failed: %w", err)
	}
	return feedbacks, nil
}

func (d *MysqlRepository) GetFeedbacksByUserId(userId uint, limit, offset int) ([]*mxm.Feedback, error) {
	return d.GetFeedbacksByUserID(userId, limit, offset)
}

// GetFeedbacks 获取所有反馈
func (d *MysqlRepository) GetFeedbacks(limit, offset int) ([]*mxm.Feedback, error) {
	var feedbacks []*mxm.Feedback
	query := d.db.Model(mxm.Feedback{})
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&feedbacks).Error; err != nil {
		return nil, fmt.Errorf("get feedbacks failed: %w", err)
	}
	return feedbacks, nil
}
