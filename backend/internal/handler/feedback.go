package handler

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"shijing/internal/model"
	"shijing/internal/wechat"
)

// CreateFeedback 用户提交建议与反馈。
func (h *Handler) CreateFeedback(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "格式错误")
		return
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		fail(c, http.StatusBadRequest, "请写下建议或反馈")
		return
	}
	if utf8.RuneCountInString(content) > 500 {
		fail(c, http.StatusBadRequest, "请控制在 500 字以内")
		return
	}
	var u model.User
	if err := h.DB.First(&u, uid).Error; err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if !h.ensureTextSafe(c, &u, content, wechat.SceneSocial) {
		return
	}
	fb := model.Feedback{
		UserID:   uid,
		Nickname: u.Nickname,
		Phone:    u.Phone,
		Content:  content,
	}
	if err := h.DB.Create(&fb).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, fb)
}

// AdminListFeedbacks 后台查看建议与反馈，按时间倒序。
func (h *Handler) AdminListFeedbacks(c *gin.Context) {
	var list []model.Feedback
	if err := h.DB.Order("id desc").Limit(100).Find(&list).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, list)
}
