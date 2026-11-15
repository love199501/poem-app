package model

import "time"

// ReadHistory 用户读过的诗。一人一首一条，供首页避开已读内容。
type ReadHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:uniq_user_read_poem" json:"userId"`
	PoemID    uint      `gorm:"uniqueIndex:uniq_user_read_poem;index" json:"poemId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
