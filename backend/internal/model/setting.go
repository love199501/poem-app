package model

// AppSetting 运营配置。会员月价等由数据库字段控制，改值后无需发版。
type AppSetting struct {
	Name  string `gorm:"primaryKey;size:64" json:"name"`
	Value string `gorm:"size:64" json:"value"`
}
