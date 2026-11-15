package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/model"
	"shijing/internal/wechat"
)

const maxAvatarBytes = 2 << 20

func sniffAvatar(data []byte) (ext, contentType string, ok bool) {
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return ".jpg", "image/jpeg", true
	}
	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return ".png", "image/png", true
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return ".webp", "image/webp", true
	}
	return "", "", false
}

// BindPhone 用微信手机号授权 code 换取号码并绑定到当前用户。
func (h *Handler) BindPhone(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Code == "" {
		fail(c, http.StatusBadRequest, "请先授权微信手机号")
		return
	}
	phone, err := h.exchangePhone(body.Code)
	if err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	var other model.User
	err = h.DB.Where("phone = ? AND id <> ?", phone, uid).First(&other).Error
	if err == nil {
		fail(c, http.StatusBadRequest, "该手机号已绑定其他账号")
		return
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if e := h.DB.Model(&model.User{}).Where("id = ?", uid).Update("phone", phone).Error; e != nil {
		fail(c, http.StatusInternalServerError, e.Error())
		return
	}
	var u model.User
	h.DB.First(&u, uid)
	u.FillLevel()
	u.FillMember()
	ok(c, u)
}

// exchangePhone 有小程序密钥时走 access_token；云托管未配密钥时走免鉴权接口。
func (h *Handler) exchangePhone(code string) (string, error) {
	if h.Cfg.WechatAppID != "" && h.Cfg.WechatSecret != "" {
		return wechat.GetPhoneNumber(h.Cfg.WechatAppID, h.Cfg.WechatSecret, code)
	}
	phone, err := wechat.GetPhoneNumberInCloud(code)
	if err != nil {
		return "", fmt.Errorf("云托管换取手机号失败：%s", err.Error())
	}
	return phone, nil
}

// UpdateProfile 更新昵称/头像（微信「头像昵称填写」能力拿到的值）。
func (h *Handler) UpdateProfile(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	var body struct {
		Nickname  string `json:"nickname"`
		AvatarURL string `json:"avatarUrl"`
	}
	_ = c.ShouldBindJSON(&body)
	upd := map[string]any{}
	nick := strings.TrimSpace(body.Nickname)
	if nick != "" {
		var u model.User
		if err := h.DB.First(&u, uid).Error; err != nil {
			fail(c, http.StatusNotFound, "用户不存在")
			return
		}
		if !h.ensureTextSafe(c, &u, nick, wechat.SceneProfile) {
			return
		}
		upd["nickname"] = nick
		upd["avatar_char"] = firstRune(nick)
	}
	if body.AvatarURL != "" {
		upd["avatar_url"] = body.AvatarURL
	}
	if len(upd) == 0 {
		fail(c, http.StatusBadRequest, "无更新内容")
		return
	}
	h.DB.Model(&model.User{}).Where("id = ?", uid).Updates(upd)
	var u model.User
	h.DB.First(&u, uid)
	u.FillLevel()
	u.FillMember()
	ok(c, u)
}

// UploadAvatar 接收 chooseAvatar 选中的头像文件，保存并写入用户头像地址。
func (h *Handler) UploadAvatar(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "缺少头像文件")
		return
	}
	if fh.Size <= 0 || fh.Size > maxAvatarBytes {
		fail(c, http.StatusBadRequest, "头像请使用 2MB 以内的 JPG、PNG 或 WEBP")
		return
	}
	dir := "./web/avatars"
	if e := os.MkdirAll(dir, 0o755); e != nil {
		fail(c, http.StatusInternalServerError, e.Error())
		return
	}
	tmp := filepath.Join(dir, fmt.Sprintf("u%d-%d.tmp", uid, time.Now().UnixNano()))
	if e := c.SaveUploadedFile(fh, tmp); e != nil {
		fail(c, http.StatusInternalServerError, e.Error())
		return
	}
	data, e := os.ReadFile(tmp)
	if e != nil {
		os.Remove(tmp)
		fail(c, http.StatusInternalServerError, e.Error())
		return
	}
	ext, ct, okImg := sniffAvatar(data)
	if !okImg || len(data) > maxAvatarBytes {
		os.Remove(tmp)
		fail(c, http.StatusBadRequest, "头像请使用 2MB 以内的 JPG、PNG 或 WEBP")
		return
	}
	name := fmt.Sprintf("u%d-%d%s", uid, time.Now().Unix(), ext)
	path := filepath.Join(dir, name)
	if e := os.Rename(tmp, path); e != nil {
		os.Remove(tmp)
		fail(c, http.StatusInternalServerError, e.Error())
		return
	}
	if e := h.DB.Save(&model.ImageFile{Name: name, ContentType: ct, Data: data}).Error; e != nil {
		os.Remove(path)
		fail(c, http.StatusInternalServerError, e.Error())
		return
	}
	url := "/avatars/" + name
	h.DB.Model(&model.User{}).Where("id = ?", uid).Update("avatar_url", url)
	ok(c, gin.H{"avatarUrl": url})
}
