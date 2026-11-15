package model

import "time"

const (
	// AskPerAd 看完一次激励视频，可与当前诗人对话的次数。
	AskPerAd = 3
	// AskAdDailyCap 非会员每天最多通过广告增加对话次数的次数。
	AskAdDailyCap = 20
)

// PoetAskCredit 非会员与某位诗人剩余的对话次数。会员不走这张表。
type PoetAskCredit struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:uniq_ask_credit" json:"userId"`
	PoetKey   string    `gorm:"size:80;uniqueIndex:uniq_ask_credit" json:"poetKey"`
	Remain    int       `json:"remain"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PoetAskGrant 一次看完广告的记录，用来限制每天增加次数。
type PoetAskGrant struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index:idx_ask_grant,priority:1" json:"userId"`
	PoetKey   string    `gorm:"size:80" json:"poetKey"`
	Day       string    `gorm:"size:10;index:idx_ask_grant,priority:2" json:"day"`
	CreatedAt time.Time `json:"createdAt"`
}
