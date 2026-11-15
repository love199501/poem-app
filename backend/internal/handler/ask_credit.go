package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/model"
)

var (
	errAskAdCap  = errors.New("今日看广告次数已用完")
	errAskAdSoon = errors.New("请稍后再看")
)

func poetAskKey(author, dynasty string) string {
	author = strings.TrimSpace(author)
	dynasty = model.NormalizeDynasty(dynasty)
	if (author == "佚名" || author == "无名氏") && dynasty != "" {
		return dynasty + "·" + author
	}
	return author
}

// GetAskCredit 当前用户与这位诗人还剩几次对话。会员返回 -1。
func (h *Handler) GetAskCredit(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	var u model.User
	if err := h.DB.First(&u, uid).Error; err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	u.FillMember()
	author := strings.TrimSpace(c.Query("author"))
	if author == "" {
		fail(c, http.StatusBadRequest, "请指定诗人")
		return
	}
	remain := 0
	if u.IsMember {
		remain = -1
	} else {
		var row model.PoetAskCredit
		h.DB.Where("user_id = ? AND poet_key = ?", uid, poetAskKey(author, c.Query("dynasty"))).First(&row)
		remain = row.Remain
	}
	ok(c, gin.H{"remain": remain, "perAd": model.AskPerAd})
}

// GrantAskCredit 非会员看完激励视频后，为当前诗人增加对话次数。
func (h *Handler) GrantAskCredit(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	var body struct {
		Author  string `json:"author"`
		Dynasty string `json:"dynasty"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "格式错误")
		return
	}
	author := strings.TrimSpace(body.Author)
	if author == "" || author == "全部" {
		fail(c, http.StatusBadRequest, "请指定诗人")
		return
	}
	var u model.User
	if err := h.DB.First(&u, uid).Error; err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	u.FillMember()
	if u.IsMember {
		ok(c, gin.H{"remain": -1, "gained": 0, "perAd": model.AskPerAd})
		return
	}
	key := poetAskKey(author, body.Dynasty)
	day := time.Now().Format("2006-01-02")
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&model.PoetAskGrant{}).Where("user_id = ? AND day = ?", uid, day).Count(&n).Error; err != nil {
			return err
		}
		if n >= model.AskAdDailyCap {
			return errAskAdCap
		}
		var last model.PoetAskGrant
		err := tx.Where("user_id = ?", uid).Order("id desc").First(&last).Error
		if err == nil && time.Since(last.CreatedAt) < 10*time.Second {
			return errAskAdSoon
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err := tx.Create(&model.PoetAskGrant{UserID: uid, PoetKey: key, Day: day}).Error; err != nil {
			return err
		}
		var row model.PoetAskCredit
		err = tx.Where("user_id = ? AND poet_key = ?", uid, key).First(&row).Error
		if err == gorm.ErrRecordNotFound {
			return tx.Create(&model.PoetAskCredit{UserID: uid, PoetKey: key, Remain: model.AskPerAd}).Error
		}
		if err != nil {
			return err
		}
		return tx.Model(&model.PoetAskCredit{}).Where("id = ?", row.ID).
			UpdateColumn("remain", gorm.Expr("remain + ?", model.AskPerAd)).Error
	})
	if err != nil {
		if errors.Is(err, errAskAdCap) || errors.Is(err, errAskAdSoon) {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		fail(c, http.StatusInternalServerError, "暂时无法增加次数")
		return
	}
	var row model.PoetAskCredit
	h.DB.Where("user_id = ? AND poet_key = ?", uid, key).First(&row)
	ok(c, gin.H{"remain": row.Remain, "gained": model.AskPerAd, "perAd": model.AskPerAd})
}

// useAskTurn 会员直接放行。非会员扣掉与这位诗人的 1 次对话，次数不够时要求先看广告。
func (h *Handler) useAskTurn(c *gin.Context, u *model.User, author, dynasty string) (remain int, ok bool) {
	u.FillMember()
	if u.IsMember {
		return -1, true
	}
	key := poetAskKey(author, dynasty)
	res := h.DB.Model(&model.PoetAskCredit{}).
		Where("user_id = ? AND poet_key = ? AND remain > 0", u.ID, key).
		UpdateColumn("remain", gorm.Expr("remain - 1"))
	if res.Error != nil {
		fail(c, http.StatusInternalServerError, "暂时无法回答，请稍后再试")
		return 0, false
	}
	if res.RowsAffected == 0 {
		fail(c, http.StatusForbidden, "看完广告后即可与这位诗人对话")
		return 0, false
	}
	var row model.PoetAskCredit
	h.DB.Where("user_id = ? AND poet_key = ?", u.ID, key).First(&row)
	return row.Remain, true
}

func (h *Handler) giveBackAskTurn(userID uint, author, dynasty string) {
	if userID == 0 {
		return
	}
	h.DB.Model(&model.PoetAskCredit{}).
		Where("user_id = ? AND poet_key = ?", userID, poetAskKey(author, dynasty)).
		UpdateColumn("remain", gorm.Expr("remain + 1"))
}
