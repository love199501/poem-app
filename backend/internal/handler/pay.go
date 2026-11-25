package handler

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"regexp"
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
	var body struct {
		Code     string `json:"code"`
		Platform string `json:"platform"`
	}
	_ = c.ShouldBindJSON(&body)
	code := strings.TrimSpace(body.Code)
	if code == "" || code == "guest" {
		fail(c, http.StatusBadRequest, "请重新登录后再充值")
		return
	}
	sess, err := wechat.Jscode2Session(h.Cfg.WechatAppID, h.Cfg.WechatSecret, code)
	if err != nil {
		log.Printf("[pay] 换取登录态失败 uid=%d: %v", uid, err)
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	var u model.User
	if err := h.DB.Where("open_id = ?", sess.OpenID).First(&u).Error; err != nil || u.ID != uid {
		fail(c, http.StatusBadRequest, "请重新登录后再充值")
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
	offerID, appKey, productID, env := h.virtualPayConf()
	platform := strings.ToLower(strings.TrimSpace(body.Platform))
	// Apple 支付不支持沙箱，且金额不能低于 1 元。
	if platform == "ios" {
		if price < 100 {
			log.Printf("[pay] iOS 金额不足 uid=%d price=%d分", uid, price)
			fail(c, http.StatusBadRequest, "iOS 开通会员金额不能低于 1 元")
			return
		}
		env = 0
	}
	pay, err := wechat.BuildVirtualPay(offerID, appKey, sess.SessionKey, productID, orderNo, "member", env, price)
	if err != nil {
		log.Printf("[pay] 签发失败 uid=%d platform=%s env=%d price=%d offer=%t product=%q: %v", uid, platform, env, price, offerID != "", productID, err)
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	log.Printf("[pay] 已签发 uid=%d order=%s platform=%s env=%d price=%d product=%s session=%t signData=%s", uid, orderNo, platform, env, price, productID, sess.SessionKey != "", pay.SignData)
	ok(c, gin.H{
		"outTradeNo": orderNo,
		"mode":       pay.Mode,
		"signData":   pay.SignData,
		"paySig":     pay.PaySig,
		"signature":  pay.Signature,
	})
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
		log.Printf("[pay] 确认订单失败 uid=%d order=%s: %v", uid, no, err)
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	log.Printf("[pay] 已开通会员 uid=%d order=%s", uid, no)
	ok(c, gin.H{"paid": true})
}

// PayNotifyCheck 微信消息推送的地址校验。
func (h *Handler) PayNotifyCheck(c *gin.Context) {
	c.String(http.StatusOK, c.Query("echostr"))
}

// PayNotify 虚拟支付道具发货推送。查单确认已支付后开通会员，并回复发货成功。
func (h *Handler) PayNotify(c *gin.Context) {
	raw, _ := io.ReadAll(c.Request.Body)
	no := virtualOrderNo(raw)
	if no == "" {
		log.Printf("[pay] 发货通知里没有订单号")
		c.JSON(http.StatusOK, gin.H{"ErrCode": 0, "ErrMsg": "success"})
		return
	}
	if err := h.fulfillMemberOrder(no, 0); err != nil {
		log.Printf("[pay] 发货通知处理失败 order=%s: %v", no, err)
		c.JSON(http.StatusOK, gin.H{"ErrCode": 1, "ErrMsg": err.Error()})
		return
	}
	log.Printf("[pay] 发货通知已处理 order=%s", no)
	c.JSON(http.StatusOK, gin.H{"ErrCode": 0, "ErrMsg": "success"})
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
	_, appKey, _, env := h.virtualPayConf()
	queried, err := wechat.QueryVirtualOrder(h.Cfg.WechatAppID, h.Cfg.WechatSecret, appKey, order.OpenID, order.OutTradeNo, env)
	if err != nil {
		log.Printf("[pay] 查单失败 order=%s env=%d: %v", order.OutTradeNo, env, err)
		return err
	}
	// 2 已支付待发货，3 发货中，4 已发货。金额须与下单时抄下的月价一致。
	if queried.Status < 2 || queried.Status > 4 || (queried.OrderFee > 0 && queried.OrderFee != order.Amount) {
		log.Printf("[pay] 订单未支付 order=%s status=%d fee=%d expect=%d", order.OutTradeNo, queried.Status, queried.OrderFee, order.Amount)
		return errNotPaid
	}
	txid := queried.WxOrderID
	now := time.Now()
	err = h.DB.Transaction(func(tx *gorm.DB) error {
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
	if err != nil {
		return err
	}
	if queried.Status == 2 || queried.Status == 3 {
		_ = wechat.NotifyProvided(h.Cfg.WechatAppID, h.Cfg.WechatSecret, order.OutTradeNo, env)
	}
	return nil
}

func (h *Handler) virtualPayConf() (offerID, appKey, productID string, env int) {
	offerID = h.settingOr("vp_offer_id", h.Cfg.VpOfferID)
	appKey = h.settingOr("vp_app_key", h.Cfg.VpAppKey)
	productID = h.settingOr("vp_product_id", h.Cfg.VpProductID)
	env = h.Cfg.VpEnv
	if env != 0 && env != 1 {
		env = 0
	}
	return
}

func (h *Handler) settingOr(name, fallback string) string {
	if v := strings.TrimSpace(fallback); v != "" {
		return v
	}
	var s model.AppSetting
	if h.DB.Where("name = ?", name).First(&s).Error == nil {
		return strings.TrimSpace(s.Value)
	}
	return ""
}

var virtualOrderNoRe = regexp.MustCompile(`(?i)"(?:OutTradeNo|out_trade_no)"\s*:\s*"([^"]+)"|<OutTradeNo>(?:<!\[CDATA\[)?([^<\]]+)`)

func virtualOrderNo(raw []byte) string {
	m := virtualOrderNoRe.FindSubmatch(raw)
	if m == nil {
		return ""
	}
	if len(m) > 1 && len(m[1]) > 0 {
		return string(m[1])
	}
	if len(m) > 2 {
		return string(m[2])
	}
	return ""
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
