package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/model"
)

// GetMe 返回当前登录用户的完整信息：等级/称号/经验进度 + 阅读、收藏、评论、获赞统计。
func (h *Handler) GetMe(c *gin.Context) {
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
	u.FillLevel()
	u.FillMember()

	var favCount, commentCount int64
	var likeSum int64
	h.DB.Model(&model.Favorite{}).Where("user_id = ?", uid).Count(&favCount)
	h.DB.Model(&model.Comment{}).Where("user_id = ?", uid).Count(&commentCount)
	// 获赞总数：该用户所有评论的点赞数之和。
	h.DB.Model(&model.Comment{}).Where("user_id = ?", uid).
		Select("COALESCE(SUM(like_count),0)").Scan(&likeSum)

	ok(c, gin.H{
		"user":          u,
		"readMinutes":   u.ReadSeconds / 60,
		"favoriteCount": favCount,
		"commentCount":  commentCount,
		"likeReceived":  likeSum,
	})
}

// ClaimMember 首次免费领取 1 个月会员（每个用户仅一次）。
func (h *Handler) ClaimMember(c *gin.Context) {
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
	until := time.Now().AddDate(0, 1, 0)
	// 用 member_claimed=false 条件做一次原子领取，避免并发重复领取。
	res := h.DB.Model(&model.User{}).Where("id = ? AND member_claimed = ?", uid, false).
		Updates(map[string]any{"member_until": until, "member_claimed": true})
	if res.Error != nil {
		fail(c, http.StatusInternalServerError, res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		fail(c, http.StatusBadRequest, "已领取过免费会员")
		return
	}
	u.MemberUntil = &until
	u.MemberClaimed = true
	u.FillLevel()
	u.FillMember()
	ok(c, u)
}

// ListMyComments 返回当前用户发表过的评论/诗话（含所属诗标题）。
// sort=like 按获赞排序（用于「被赞」列表），否则按时间倒序。
func (h *Handler) ListMyComments(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	sort := c.DefaultQuery("sort", "new")

	tx := h.DB.Model(&model.Comment{}).Where("user_id = ?", uid)
	if sort == "like" {
		tx = tx.Order("like_count desc, created_at desc")
	} else {
		tx = tx.Order("created_at desc")
	}
	var comments []model.Comment
	tx.Find(&comments)
	if len(comments) == 0 {
		ok(c, []any{})
		return
	}

	// 批量取所属诗标题。
	idset := make(map[uint]struct{}, len(comments))
	for _, cm := range comments {
		idset[cm.PoemID] = struct{}{}
	}
	ids := make([]uint, 0, len(idset))
	for id := range idset {
		ids = append(ids, id)
	}
	var poems []model.Poem
	h.DB.Model(&model.Poem{}).Select("id,title,author").Where("id IN ?", ids).Find(&poems)
	titleOf := make(map[uint]model.Poem, len(poems))
	for _, p := range poems {
		titleOf[p.ID] = p
	}

	list := make([]gin.H, 0, len(comments))
	for _, cm := range comments {
		p := titleOf[cm.PoemID]
		list = append(list, gin.H{
			"id":         cm.ID,
			"content":    cm.Content,
			"kind":       cm.Kind,
			"likeCount":  cm.LikeCount,
			"replyCount": cm.ReplyCount,
			"isPinned":   cm.IsPinned,
			"isFeatured": cm.IsFeatured,
			"createdAt":  cm.CreatedAt,
			"poemId":     cm.PoemID,
			"poemTitle":  p.Title,
			"poemAuthor": p.Author,
		})
	}
	ok(c, list)
}

// ReportRead 上报一次阅读停留时长（秒），累计阅读时长并按分钟折算经验。
// 单次上报做上限保护，避免异常值刷经验。
func (h *Handler) ReportRead(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	var body struct {
		Seconds int `json:"seconds"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Seconds <= 0 {
		ok(c, gin.H{"gainExp": 0})
		return
	}
	if body.Seconds > 1800 { // 单次最多计 30 分钟
		body.Seconds = 1800
	}

	// 累计阅读时长（完整记录，用于个人中心展示），经验则按每日任务上限发放。
	h.DB.Model(&model.User{}).Where("id = ?", uid).
		Update("read_seconds", gorm.Expr("read_seconds + ?", body.Seconds))
	gain := h.grantDaily(uid, "read", body.Seconds/60)
	ok(c, gin.H{"gainExp": gain})
}
