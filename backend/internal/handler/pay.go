package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/model"
	"shijing/internal/wechat"
)

// RechargeMember 按数据库月价向微信统一下单，返回小程序调起支付的参数。
func (h *Handler) RechargeMember(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}
	price := h.memberMonthPriceFen()
	if price <= 0 {
		fail(c, http.StatusBadRequest, "暂未开放充值")
		return
	}
	mchID := h.wxMchID()
	if mchID == "" {
		fail(c, http.StatusBadRequest, "未读取到商户号，请配置 WX_MCH_ID")
		return
	}
	var u model.User
	if err := h.DB.First(&u, uid).Error; err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if u.OpenID == "" || strings.HasPrefix(u.OpenID, "dev-") || u.OpenID == "admin-root" {
		fail(c, http.StatusBadRequest, "请使用微信登录后再充值")
		return
	}
	orderNo, err := newOrderNo()
	if err != nil {
		fail(c, http.StatusInternalServerError, "暂时无法下单")
		return
	}
	order := model.MemberOrder{
		OutTradeNo: orderNo,
		UserID:     uid,
		OpenID:     u.OpenID,
		Amount:     price,
		Months:     1,
		Status:     "pending",
	}
	if err := h.DB.Create(&order).Error; err != nil {
		fail(c, http.StatusInternalServerError, "暂时无法下单")
		return
	}
	mch, err := h.payMerchant()
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	pay, err := mch.Jsapi(u.OpenID, orderNo, "诗趣小园会员", price)
	if err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	ok(c, gin.H{"outTradeNo": orderNo, "payment": pay})
}

// ConfirmMemberPay 支付完成后向微信核对订单，成功则延长一个月会员。
func (h *Handler) ConfirmMemberPay(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}
	var body struct {
		OutTradeNo string `json:"outTradeNo"`
	}
	_ = c.ShouldBindJSON(&body)
	no := strings.TrimSpace(body.OutTradeNo)
	if no == "" {
		fail(c, http.StatusBadRequest, "缺少订单号")
		return
	}
	if err := h.fulfillMemberOrder(no, uid); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"paid": true})
}

// PayNotify 微信支付 API v3 结果通知。解密出订单号后向微信查单，核对金额再入账。
func (h *Handler) PayNotify(c *gin.Context) {
	var note struct {
		EventType string `json:"event_type"`
		Resource  struct {
			Ciphertext     string `json:"ciphertext"`
			Nonce          string `json:"nonce"`
			AssociatedData string `json:"associated_data"`
		} `json:"resource"`
	}
	if err := c.ShouldBindJSON(&note); err != nil || note.Resource.Ciphertext == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "无效通知"})
		return
	}
	if note.EventType != "" && note.EventType != "TRANSACTION.SUCCESS" {
		c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
		return
	}
	plain, err := wechat.DecryptResource(h.Cfg.WxAPIv3Key, note.Resource.Nonce, note.Resource.AssociatedData, note.Resource.Ciphertext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "解密失败"})
		return
	}
	var tx struct {
		OutTradeNo string `json:"out_trade_no"`
	}
	if json.Unmarshal(plain, &tx) != nil || tx.OutTradeNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "缺少订单号"})
		return
	}
	if err := h.fulfillMemberOrder(tx.OutTradeNo, 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
}

func (h *Handler) fulfillMemberOrder(outTradeNo string, onlyUser uint) error {
	var order model.MemberOrder
	if err := h.DB.Where("out_trade_no = ?", outTradeNo).First(&order).Error; err != nil {
		return errOrderMissing
	}
	if onlyUser != 0 && order.UserID != onlyUser {
		return errOrderMissing
	}
	if order.Status == "paid" {
		return nil
	}
	mch, err := h.payMerchant()
	if err != nil {
		return err
	}
	paid, txid, err := mch.Paid(order.OutTradeNo, order.Amount)
	if err != nil {
		return err
	}
	if !paid {
		return errNotPaid
	}
	now := time.Now()
	return h.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.MemberOrder{}).
			Where("id = ? AND status = ?", order.ID, "pending").
			Updates(map[string]any{"status": "paid", "transaction_id": txid, "paid_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		var u model.User
		if err := tx.First(&u, order.UserID).Error; err != nil {
			return err
		}
		base := now
		if u.MemberUntil != nil && u.MemberUntil.After(base) {
			base = *u.MemberUntil
		}
		months := order.Months
		if months < 1 {
			months = 1
		}
		until := base.AddDate(0, months, 0)
		return tx.Model(&model.User{}).Where("id = ?", u.ID).Update("member_until", until).Error
	})
}

func (h *Handler) wxMchID() string {
	if v := strings.TrimSpace(h.Cfg.WxMchID); v != "" {
		return v
	}
	var s model.AppSetting
	if h.DB.Where("name = ?", "wx_mch_id").First(&s).Error == nil {
		return strings.TrimSpace(s.Value)
	}
	return ""
}

func (h *Handler) payMerchant() (*wechat.Merchant, error) {
	return wechat.NewMerchant(
		h.Cfg.WechatAppID,
		h.wxMchID(),
		h.Cfg.WxMchSerial,
		h.Cfg.WxAPIv3Key,
		h.Cfg.WxMchPrivateKey,
		h.Cfg.WxPayNotifyURL,
	)
}

func newOrderNo() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "SJ" + time.Now().Format("20060102150405") + hex.EncodeToString(b), nil
}

var (
	errOrderMissing = errText("订单不存在")
	errNotPaid      = errText("支付尚未完成")
)

type errText string

func (e errText) Error() string { return string(e) }
