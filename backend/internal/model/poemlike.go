package model

import "time"

// PoemLike 用户给一首诗点赞。(poem_id,user_id) 唯一，保证一人一赞可切换。
type PoemLike struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PoemID    uint      `gorm:"uniqueIndex:uniq_poem_user;index" json:"poemId"`
	UserID    uint      `gorm:"uniqueIndex:uniq_poem_user" json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
}
