package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/config"
	"shijing/internal/model"
	"shijing/internal/wechat"
)

// Handler 聚合依赖，所有接口方法挂在其上。
type Handler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func New(db *gorm.DB, cfg *config.Config) *Handler { return &Handler{DB: db, Cfg: cfg} }

// AppConfig 给小程序的云控开关。评论入口由 COMMENT_ENABLED 决定。
func (h *Handler) AppConfig(c *gin.Context) {
	ok(c, gin.H{
		"commentEnabled": h.Cfg.CommentEnabled,
		"audioEnabled":   h.Cfg.AudioEnabled,
		"rewardAdUnit":   h.Cfg.RewardAdUnit,
	})
}

// ok / fail 统一响应格式：{code,msg,data}。
func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": data})
}

func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"code": status, "msg": msg})
}

// AuthContext 用登录时下发的 X-Auth-Token 认出用户。不信任客户端自报的用户编号或 openid。
func (h *Handler) AuthContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.GetHeader("X-Auth-Token"))
		if token != "" {
			var u model.User
			err := h.DB.Select("id").Where("session_token = ?", token).First(&u).Error
			if err == nil && u.ID != 0 {
				c.Set("uid", u.ID)
			}
		}
		c.Next()
	}
}

// currentUserID 从鉴权中间件写入的登录态取用户；返回 0 表示未登录。
func currentUserID(c *gin.Context) uint {
	v, ok := c.Get("uid")
	if !ok {
		return 0
	}
	id, _ := v.(uint)
	return id
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *Handler) issueSession(u *model.User) error {
	token, err := newSessionToken()
	if err != nil {
		return err
	}
	if err := h.DB.Model(u).Update("session_token", token).Error; err != nil {
		return err
	}
	u.SessionToken = token
	return nil
}

func reservedOpenID(id string) bool {
	return id == "" || id == "admin-root" || strings.HasPrefix(id, "dev-") || strings.HasPrefix(id, "seed-")
}

func paramUint(c *gin.Context, key string) uint {
	id, _ := strconv.ParseUint(c.Param(key), 10, 64)
	return uint(id)
}

// addExp 为用户累加经验（delta 可为负）。用户不存在或 delta=0 时忽略。
func (h *Handler) addExp(userID uint, delta int) {
	if userID == 0 || delta == 0 {
		return
	}
	h.DB.Model(&model.User{}).Where("id = ?", userID).
		UpdateColumn("exp", gorm.Expr("exp + ?", delta))
}

// Login 微信登录：配置了小程序密钥时，只用 wx.login 的 code 向微信换 openid。
// 不采用请求头里自带的 openid，避免公网请求伪造身份。未配置密钥时才走本地兜底。
func (h *Handler) Login(c *gin.Context) {
	var body struct {
		Code      string `json:"code"`
		Nickname  string `json:"nickname"`
		AvatarURL string `json:"avatarUrl"`
	}
	_ = c.ShouldBindJSON(&body)
	code := strings.TrimSpace(body.Code)

	var openID string
	if h.Cfg.WechatAppID != "" && h.Cfg.WechatSecret != "" {
		if code == "" || code == "guest" {
			fail(c, http.StatusUnauthorized, "请重新登录")
			return
		}
		sess, err := wechat.Jscode2Session(h.Cfg.WechatAppID, h.Cfg.WechatSecret, code)
		if err != nil {
			fail(c, http.StatusBadGateway, err.Error())
			return
		}
		openID = sess.OpenID
		if reservedOpenID(openID) {
			fail(c, http.StatusUnauthorized, "登录失败")
			return
		}
	} else {
		headerID := strings.TrimSpace(c.GetHeader("x-wx-openid"))
		if headerID != "" && !reservedOpenID(headerID) {
			openID = headerID
		} else {
			if code == "" {
				code = "guest"
			}
			openID = "dev-" + code
		}
	}

	var u model.User
	err := h.DB.Where("open_id = ?", openID).First(&u).Error
	nickIn := strings.TrimSpace(body.Nickname)
	if nickIn != "" && nickIn != "诗友" && (err == gorm.ErrRecordNotFound || nickIn != u.Nickname) {
		if !h.ensureTextSafe(c, &model.User{OpenID: openID}, nickIn, wechat.SceneProfile) {
			return
		}
	}
	if err == gorm.ErrRecordNotFound {
		nick := body.Nickname
		if nick == "" {
			nick = "诗友"
		}
		u = model.User{
			OpenID: openID, Nickname: nick, AvatarChar: firstRune(nick),
			AvatarColor: "linear-gradient(140deg,#5c5142,#8a7a5e)", AvatarURL: body.AvatarURL,
		}
		if e := h.DB.Create(&u).Error; e != nil {
			fail(c, http.StatusInternalServerError, e.Error())
			return
		}
	} else if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	} else {
		upd := map[string]any{}
		// 自动登录会带默认昵称「诗友」，不能盖掉用户已经保存的名字。
		if nickIn != "" && nickIn != "诗友" && nickIn != u.Nickname {
			upd["nickname"] = nickIn
			upd["avatar_char"] = firstRune(nickIn)
		}
		if body.AvatarURL != "" && body.AvatarURL != u.AvatarURL {
			upd["avatar_url"] = body.AvatarURL
		}
		if len(upd) > 0 {
			h.DB.Model(&u).Updates(upd)
			h.DB.First(&u, u.ID)
		}
	}
	if err := h.issueSession(&u); err != nil {
		fail(c, http.StatusInternalServerError, "登录失败")
		return
	}
	u.FillLevel()
	u.FillMember()
	ok(c, u)
}

func firstRune(s string) string {
	for _, r := range s {
		return string(r)
	}
	return "诗"
}
