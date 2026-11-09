package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/model"
)

// ListComments 讨论区列表：只返回顶层评论（置顶优先），每条挂载其楼中楼回复。
// query: sort=hot|new, kind=shihua(仅诗话)
func (h *Handler) ListComments(c *gin.Context) {
	poemID := paramUint(c, "id")
	sort := c.DefaultQuery("sort", "hot")
	kind := c.Query("kind")

	tx := h.DB.Where("poem_id = ? AND parent_id = 0", poemID)
	if kind == "shihua" {
		tx = tx.Where("kind = ?", "shihua")
	}
	// 置顶恒在最前；其余按热门(赞)或最新(时间)。
	if sort == "new" {
		tx = tx.Order("is_pinned DESC, pinned_at DESC, created_at DESC")
	} else {
		tx = tx.Order("is_pinned DESC, pinned_at DESC, like_count DESC, created_at DESC")
	}

	var tops []model.Comment
	if err := tx.Find(&tops).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 一次性取出这首诗的全部回复，按父评论分组、时间升序挂载。
	var replies []model.Comment
	h.DB.Where("poem_id = ? AND parent_id > 0", poemID).Order("created_at asc").Find(&replies)
	byParent := make(map[uint][]model.Comment)
	for _, r := range replies {
		byParent[r.ParentID] = append(byParent[r.ParentID], r)
	}
	for i := range tops {
		tops[i].Replies = byParent[tops[i].ID]
	}

	h.fillLiked(c, tops, replies)
	h.fillAuthorLevel(tops, replies)
	ok(c, tops)
}

// fillAuthorLevel 按作者的经验值填充每条评论/回复的等级与身份称号。
func (h *Handler) fillAuthorLevel(tops, replies []model.Comment) {
	uids := make(map[uint]struct{})
	for _, cm := range tops {
		if cm.UserID > 0 {
			uids[cm.UserID] = struct{}{}
		}
	}
	for _, cm := range replies {
		if cm.UserID > 0 {
			uids[cm.UserID] = struct{}{}
		}
	}
	if len(uids) == 0 {
		return
	}
	ids := make([]uint, 0, len(uids))
	for id := range uids {
		ids = append(ids, id)
	}
	var users []model.User
	// 取完整用户行：会员标识按「当前是否有效会员」回填，开通后历史评论也会带皇冠。
	h.DB.Where("id IN ?", ids).Find(&users)
	expOf := make(map[uint]int, len(users))
	memberOf := make(map[uint]bool, len(users))
	for i := range users {
		users[i].FillMember()
		expOf[users[i].ID] = users[i].Exp
		memberOf[users[i].ID] = users[i].IsMember
	}
	apply := func(cm *model.Comment) {
		lv, title := model.LevelTitleByExp(expOf[cm.UserID])
		cm.AuthorLevel = lv
		cm.AuthorTitle = title
		cm.AuthorMember = memberOf[cm.UserID]
	}
	for i := range tops {
		apply(&tops[i])
		for j := range tops[i].Replies {
			apply(&tops[i].Replies[j])
		}
	}
}

// fillLiked 回填当前用户对顶层评论与回复的点赞态。
func (h *Handler) fillLiked(c *gin.Context, tops, replies []model.Comment) {
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	ids := make([]uint, 0, len(tops)+len(replies))
	for _, cm := range tops {
		ids = append(ids, cm.ID)
	}
	for _, cm := range replies {
		ids = append(ids, cm.ID)
	}
	if len(ids) == 0 {
		return
	}
	var liked []uint
	h.DB.Model(&model.CommentLike{}).Where("user_id = ? AND comment_id IN ?", uid, ids).Pluck("comment_id", &liked)
	set := make(map[uint]bool, len(liked))
	for _, id := range liked {
		set[id] = true
	}
	for i := range tops {
		tops[i].Liked = set[tops[i].ID]
		for j := range tops[i].Replies {
			tops[i].Replies[j].Liked = set[tops[i].Replies[j].ID]
		}
	}
}

// CreateComment 发表评论或回复。parentId>0 表示楼中楼回复。
func (h *Handler) CreateComment(c *gin.Context) {
	poemID := paramUint(c, "id")
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}
	var body struct {
		Content     string `json:"content"`
		Kind        string `json:"kind"`
		ParentID    uint   `json:"parentId"`
		ReplyToName string `json:"replyToName"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Content == "" {
		fail(c, http.StatusBadRequest, "内容不能为空")
		return
	}
	if body.Kind == "" {
		body.Kind = "short"
	}
	if body.ParentID > 0 {
		body.Kind = "reply"
	}

	var user model.User
	h.DB.First(&user, uid)

	cm := model.Comment{
		PoemID: poemID, UserID: uid,
		ParentID:    body.ParentID,
		ReplyToName: body.ReplyToName,
		AuthorName:  user.Nickname,
		AvatarChar:  user.AvatarChar,
		AvatarColor: user.AvatarColor,
		Content:     body.Content,
		Kind:        body.Kind,
	}
	if err := h.DB.Create(&cm).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.DB.Model(&model.Poem{}).Where("id = ?", poemID).UpdateColumn("comment_count", gorm.Expr("comment_count + 1"))
	if body.ParentID > 0 {
		h.DB.Model(&model.Comment{}).Where("id = ?", body.ParentID).UpdateColumn("reply_count", gorm.Expr("reply_count + 1"))
	}
	h.grantDaily(uid, "comment", 1) // 每日上限内加经验
	user.FillLevel()
	user.FillMember()
	cm.AuthorLevel = user.Level
	cm.AuthorTitle = user.LevelName
	cm.AuthorMember = user.IsMember
	ok(c, cm)
}

// ToggleLike 点赞/取消点赞（一人一赞）。达到阈值自动标记为精品。
func (h *Handler) ToggleLike(c *gin.Context) {
	commentID := paramUint(c, "id")
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}

	var cm model.Comment
	if err := h.DB.First(&cm, commentID).Error; err != nil {
		fail(c, http.StatusNotFound, "评论不存在")
		return
	}

	var liked bool
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var existing model.CommentLike
		e := tx.Where("comment_id = ? AND user_id = ?", commentID, uid).First(&existing).Error
		if e == gorm.ErrRecordNotFound {
			if err := tx.Create(&model.CommentLike{CommentID: commentID, UserID: uid}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Comment{}).Where("id = ?", commentID).
				UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error; err != nil {
				return err
			}
			liked = true
		} else if e == nil {
			if err := tx.Delete(&existing).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Comment{}).Where("id = ? AND like_count > 0", commentID).
				UpdateColumn("like_count", gorm.Expr("like_count - 1")).Error; err != nil {
				return err
			}
			liked = false
		} else {
			return e
		}
		return nil
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if liked {
		h.grantDaily(uid, "like", 1) // 点赞给点赞者加经验（每日上限内），取消不扣
	}

	h.DB.First(&cm, commentID)
	// 高赞自动精品。
	if cm.LikeCount >= model.FeatureLikeThreshold && !cm.IsFeatured {
		h.DB.Model(&cm).Update("is_featured", true)
		cm.IsFeatured = true
	}
	cm.Liked = liked
	ok(c, gin.H{"liked": liked, "likeCount": cm.LikeCount, "isFeatured": cm.IsFeatured})
}

// PinComment 精选置顶 / 取消置顶（编辑或作者操作，演示未做鉴权）。
// 置顶即标记精品，并记录置顶时间用于排序。
func (h *Handler) PinComment(c *gin.Context) {
	commentID := paramUint(c, "id")
	var cm model.Comment
	if err := h.DB.First(&cm, commentID).Error; err != nil {
		fail(c, http.StatusNotFound, "评论不存在")
		return
	}

	updates := map[string]any{}
	if cm.IsPinned {
		updates["is_pinned"] = false
		updates["pinned_at"] = nil
	} else {
		now := time.Now()
		updates["is_pinned"] = true
		updates["pinned_at"] = &now
		updates["is_featured"] = true
	}
	if err := h.DB.Model(&cm).Updates(updates).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.DB.First(&cm, commentID)
	ok(c, cm)
}
