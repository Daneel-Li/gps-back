package mxm

import "time"

type Feedback struct {
	UserID    uint      `json:"userId"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Contact   string    `json:"contact"`
	Reply     string    `json:"reply"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Feedback) TableName() string {
	return "feedback"
}
