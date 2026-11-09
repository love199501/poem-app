package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/config"
	"shijing/internal/model"
)

// Handler 聚合依赖，所有接口方法挂在其上。
type Handler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func New(db *gorm.DB, cfg *config.Config) *Handler { return &Handler{DB: db, Cfg: cfg} }

// ok / fail 统一响应格式：{code,msg,data}。
func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": data})
}

func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"code": status, "msg": msg})
}

// currentUserID 从 X-User-Id 头取登录用户；返回 0 表示未登录。
func currentUserID(c *gin.Context) uint {
	raw := c.GetHeader("X-User-Id")
	id, _ := strconv.ParseUint(raw, 10, 64)
	return uint(id)
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

// Login 开发版登录：用 code 当作 openid（线上应调用微信 code2session 换取真实 openid）。
// 无用户则创建一个访客，返回用户信息，前端保存 user.id 并在后续请求带 X-User-Id。
func (h *Handler) Login(c *gin.Context) {
	var body struct {
		Code     string `json:"code"`
		Nickname string `json:"nickname"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Code == "" {
		body.Code = "guest"
	}
	openID := "wx-" + body.Code

	var u model.User
	err := h.DB.Where("open_id = ?", openID).First(&u).Error
	if err == gorm.ErrRecordNotFound {
		nick := body.Nickname
		if nick == "" {
			nick = "诗友"
		}
		u = model.User{OpenID: openID, Nickname: nick, AvatarChar: firstRune(nick), AvatarColor: "linear-gradient(140deg,#5c5142,#8a7a5e)"}
		if e := h.DB.Create(&u).Error; e != nil {
			fail(c, http.StatusInternalServerError, e.Error())
			return
		}
	} else if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
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
