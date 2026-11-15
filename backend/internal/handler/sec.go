package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"shijing/internal/model"
	"shijing/internal/wechat"
)

// ensureTextSafe 对用户发布的文本调用微信内容安全接口。
// 违规时只向调用方返回固定说明，不附带标签或错误码。
func (h *Handler) ensureTextSafe(c *gin.Context, user *model.User, content string, scene int) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return true
	}
	openID := ""
	if user != nil {
		openID = strings.TrimSpace(user.OpenID)
	}
	// 本地开发用的伪 openid 无法通过微信校验，跳过以免挡掉联调。
	if openID == "" || strings.HasPrefix(openID, "dev-") || strings.HasPrefix(openID, "seed-") || openID == "admin-root" {
		return true
	}
	var err error
	if h.Cfg.WechatAppID != "" && h.Cfg.WechatSecret != "" {
		err = wechat.MsgSecCheck(h.Cfg.WechatAppID, h.Cfg.WechatSecret, openID, content, scene)
	} else {
		err = wechat.MsgSecCheckInCloud(openID, content, scene)
	}
	if err == nil {
		return true
	}
	if errors.Is(err, wechat.ErrContentUnsafe) {
		fail(c, http.StatusBadRequest, wechat.ErrContentUnsafe.Error())
		return false
	}
	log.Printf("[sec] 内容安全接口调用失败: %v", err)
	fail(c, http.StatusBadGateway, "发布失败，请稍后再试")
	return false
}
