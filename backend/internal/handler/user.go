package handler

import (
	"net/http"
	"strconv"
	"strings"
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

	var favCount, commentCount, readCount int64
	var likeSum int64
	h.DB.Model(&model.Favorite{}).Where("user_id = ?", uid).Count(&favCount)
	h.DB.Model(&model.Comment{}).Where("user_id = ?", uid).Count(&commentCount)
	h.DB.Model(&model.ReadHistory{}).Where("user_id = ?", uid).Count(&readCount)
	// 获赞总数：该用户所有评论的点赞数之和。
	h.DB.Model(&model.Comment{}).Where("user_id = ?", uid).
		Select("COALESCE(SUM(like_count),0)").Scan(&likeSum)

	ok(c, gin.H{
		"user":             u,
		"readMinutes":      u.ReadSeconds / 60,
		"readCount":        readCount,
		"favoriteCount":    favCount,
		"commentCount":     commentCount,
		"likeReceived":     likeSum,
		"memberMonthPrice": h.memberMonthPriceFen(),
	})
}

// memberMonthPriceFen 会员月价，单位分。没有记录时写入默认 600（6 元）。
func (h *Handler) memberMonthPriceFen() int {
	const key = "member_month_price"
	const fallback = 600
	var s model.AppSetting
	err := h.DB.Where("name = ?", key).First(&s).Error
	if err == gorm.ErrRecordNotFound {
		s = model.AppSetting{Name: key, Value: "600"}
		_ = h.DB.Create(&s).Error
		return fallback
	}
	if err != nil {
		return fallback
	}
	n, convErr := strconv.Atoi(strings.TrimSpace(s.Value))
	if convErr != nil || n < 0 {
		return 0
	}
	return n
}

// profileReady 昵称、头像、手机号都已填写。默认昵称「诗友」不算完善。
func profileReady(u *model.User) bool {
	nick := strings.TrimSpace(u.Nickname)
	return nick != "" && nick != "诗友" && strings.TrimSpace(u.AvatarURL) != "" && strings.TrimSpace(u.Phone) != ""
}

// ClaimMember 完善资料后，首次免费领取 7 天会员（每个用户仅一次）。
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
	if u.MemberClaimed {
		fail(c, http.StatusBadRequest, "已领取过免费会员")
		return
	}
	if !profileReady(&u) {
		fail(c, http.StatusBadRequest, "请先完善昵称、头像和手机号")
		return
	}
	base := time.Now()
	if u.MemberUntil != nil && u.MemberUntil.After(base) {
		base = *u.MemberUntil
	}
	until := base.AddDate(0, 0, model.TrialMemberDays)
	// 未领取包含 0 和 NULL。只按已领取的记录拒绝，避免空值被误判成领过。
	res := h.DB.Model(&model.User{}).
		Where("id = ? AND (member_claimed = ? OR member_claimed IS NULL)", uid, false).
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
// 带 poemId 时写入阅读历史；停留不足不计时长与经验，仍记下读过这首。
// 单次上报做上限保护，避免异常值刷经验。
func (h *Handler) ReportRead(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	var body struct {
		Seconds int  `json:"seconds"`
		PoemID  uint `json:"poemId"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.PoemID > 0 {
		h.markRead(uid, body.PoemID)
	}
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

// ListReadPoems 阅读历史：按最近阅读时间倒序返回诗的元信息。
func (h *Handler) ListReadPoems(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}
	var rows []model.ReadHistory
	h.DB.Where("user_id = ?", uid).Order("updated_at desc").Find(&rows)
	if len(rows) == 0 {
		ok(c, []model.Poem{})
		return
	}
	ids := make([]uint, len(rows))
	for i, row := range rows {
		ids[i] = row.PoemID
	}
	var poems []model.Poem
	h.DB.Model(&model.Poem{}).
		Select("id,title,dynasty,author,form,year_label,first_verse,audio_url,illust_url,tags,moods,comment_count,like_count,favorite_count,created_at").
		Where("id IN ?", ids).Find(&poems)
	byID := make(map[uint]model.Poem, len(poems))
	for _, p := range poems {
		byID[p.ID] = p
	}
	ordered := make([]model.Poem, 0, len(rows))
	for _, row := range rows {
		p, ok := byID[row.PoemID]
		if !ok {
			continue
		}
		readAt := row.UpdatedAt
		p.ReadAt = &readAt
		ordered = append(ordered, p)
	}
	h.fillDisplayVerses(ordered)
	normalizeDynasties(ordered)
	ok(c, ordered)
}

// ListReads 返回当前用户已读过的诗 id，供首页冷启动、换一换和滑动避开。
func (h *Handler) ListReads(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}
	ids := make([]uint, 0)
	h.DB.Model(&model.ReadHistory{}).Where("user_id = ?", uid).
		Order("updated_at desc").Pluck("poem_id", &ids)
	ok(c, ids)
}

// markRead 记下用户读过这首诗；同一首重复阅读只更新时间。
func (h *Handler) markRead(uid, poemID uint) {
	var n int64
	if err := h.DB.Model(&model.Poem{}).Where("id = ?", poemID).Count(&n).Error; err != nil || n == 0 {
		return
	}
	var row model.ReadHistory
	err := h.DB.Where("user_id = ? AND poem_id = ?", uid, poemID).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		h.DB.Create(&model.ReadHistory{UserID: uid, PoemID: poemID})
		return
	}
	if err == nil {
		h.DB.Model(&row).Update("updated_at", time.Now())
	}
}
