package model

import "time"

// Favorite 用户收藏一首诗。(user_id,poem_id) 唯一，保证一人一诗一条、可切换。
type Favorite struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:uniq_user_poem" json:"userId"`
	PoemID    uint      `gorm:"uniqueIndex:uniq_user_poem;index" json:"poemId"`
	CreatedAt time.Time `json:"createdAt"`
}
