package model

import "time"

// ImageFile 插画文件。云托管容器磁盘会在重新部署后清空，图片字节放在数据库里才能继续访问。
type ImageFile struct {
	Name        string    `gorm:"primaryKey;size:128" json:"name"`
	ContentType string    `gorm:"size:64" json:"contentType"`
	Data        []byte    `gorm:"type:longblob" json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
}
