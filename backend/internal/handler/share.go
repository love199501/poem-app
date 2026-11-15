package handler

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"shijing/internal/wechat"
)

// 小程序码按诗缓存，避免每次打开诗签都去打微信接口。
var (
	wxaMu    sync.Mutex
	wxaCache = map[uint]string{}
)

// PoemWxaCode 返回这首诗的小程序码（base64）。扫码进入对应详情页。
func (h *Handler) PoemWxaCode(c *gin.Context) {
	id := paramUint(c, "id")
	if id == 0 {
		fail(c, http.StatusBadRequest, "诗词不存在")
		return
	}
	wxaMu.Lock()
	if img, hit := wxaCache[id]; hit && img != "" {
		wxaMu.Unlock()
		ok(c, gin.H{"image": img})
		return
	}
	wxaMu.Unlock()

	scene := fmt.Sprintf("id=%d", id)
	page := "pages/poem/detail"
	// 正式版页面还没发过时，微信会拒绝 release。再试体验版和开发版，保证诗签上能画出码。
	versions := []string{c.DefaultQuery("env", "release"), "release", "trial", "develop"}
	seen := map[string]bool{}
	var png []byte
	var err error
	for _, env := range versions {
		if env != "develop" && env != "trial" && env != "release" {
			env = "release"
		}
		if seen[env] {
			continue
		}
		seen[env] = true
		if h.Cfg.WechatAppID != "" && h.Cfg.WechatSecret != "" {
			png, err = wechat.WxaCodeUnlimited(h.Cfg.WechatAppID, h.Cfg.WechatSecret, scene, page, env)
		}
		if len(png) == 0 {
			var cloudErr error
			png, cloudErr = wechat.WxaCodeUnlimitedInCloud(scene, page, env)
			if err == nil {
				err = cloudErr
			}
		}
		if len(png) > 0 {
			break
		}
		log.Printf("[share] 小程序码 %s 失败 poem=%d: %v", env, id, err)
	}
	if len(png) == 0 {
		fail(c, http.StatusBadGateway, "小程序码暂不可用")
		return
	}
	img := base64.StdEncoding.EncodeToString(png)
	wxaMu.Lock()
	wxaCache[id] = img
	wxaMu.Unlock()
	ok(c, gin.H{"image": img})
}

// ReportShare 分享给朋友计一次每日任务。每天最多 3 次，每次 5 经验，超出不再加。
func (h *Handler) ReportShare(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	gain := h.grantDaily(uid, "share", 1)
	ok(c, gin.H{"gainExp": gain})
}
