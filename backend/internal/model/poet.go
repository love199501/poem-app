package model

import "time"

// Poet 诗人介绍。生成一次后存放，人生卷轴直接读取。
type Poet struct {
	Name      string    `gorm:"primaryKey;size:32" json:"name"`
	Dynasty   string    `gorm:"size:16" json:"dynasty"`
	Intro     string    `gorm:"type:text" json:"intro"`
	UpdatedAt time.Time `json:"updatedAt"`
}
