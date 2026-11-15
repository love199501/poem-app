package model

import "time"

// MemberOrder 会员充值订单。金额在下单时从月价抄下，支付成功后再延长会员。
type MemberOrder struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	OutTradeNo    string     `gorm:"size:32;uniqueIndex" json:"outTradeNo"`
	UserID        uint       `gorm:"index" json:"userId"`
	OpenID        string     `gorm:"size:64" json:"-"`
	Amount        int        `json:"amount"` // 分
	Months        int        `json:"months"`
	Status        string     `gorm:"size:16;index" json:"status"` // pending / paid
	TransactionID string     `gorm:"size:64" json:"transactionId"`
	CreatedAt     time.Time  `json:"createdAt"`
	PaidAt        *time.Time `json:"paidAt"`
}
