package model

import "time"

// Feedback 用户在个人中心提交的建议与反馈。
type Feedback struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"userId"`
	Nickname  string    `gorm:"size:64" json:"nickname"`
	Phone     string    `gorm:"size:20" json:"phone"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}
