package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/model"
)

// ToggleFavorite 收藏 / 取消收藏一首诗（一人一诗可切换）。首次收藏加经验，取消不扣。
func (h *Handler) ToggleFavorite(c *gin.Context) {
	poemID := paramUint(c, "id")
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}

	var fav model.Favorite
	err := h.DB.Where("user_id = ? AND poem_id = ?", uid, poemID).First(&fav).Error
	if err == gorm.ErrRecordNotFound {
		if e := h.DB.Create(&model.Favorite{UserID: uid, PoemID: poemID}).Error; e != nil {
			fail(c, http.StatusInternalServerError, e.Error())
			return
		}
		h.DB.Model(&model.Poem{}).Where("id = ?", poemID).
			UpdateColumn("favorite_count", gorm.Expr("favorite_count + 1"))
		gain := h.grantDaily(uid, "favorite", 1) // 每日上限内加经验，超限返回 0
		var p model.Poem
		h.DB.Select("favorite_count").First(&p, poemID)
		ok(c, gin.H{"favored": true, "gainExp": gain, "favoriteCount": p.FavoriteCount})
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if e := h.DB.Delete(&fav).Error; e != nil {
		fail(c, http.StatusInternalServerError, e.Error())
		return
	}
	h.DB.Model(&model.Poem{}).Where("id = ? AND favorite_count > 0", poemID).
		UpdateColumn("favorite_count", gorm.Expr("favorite_count - 1"))
	var p model.Poem
	h.DB.Select("favorite_count").First(&p, poemID)
	ok(c, gin.H{"favored": false, "favoriteCount": p.FavoriteCount})
}

// TogglePoemLike 诗级点赞 / 取消点赞（一人一赞可切换），维护诗的 LikeCount。
func (h *Handler) TogglePoemLike(c *gin.Context) {
	poemID := paramUint(c, "id")
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}

	var liked bool
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var existing model.PoemLike
		e := tx.Where("poem_id = ? AND user_id = ?", poemID, uid).First(&existing).Error
		if e == gorm.ErrRecordNotFound {
			if err := tx.Create(&model.PoemLike{PoemID: poemID, UserID: uid}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Poem{}).Where("id = ?", poemID).
				UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error; err != nil {
				return err
			}
			liked = true
		} else if e == nil {
			if err := tx.Delete(&existing).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Poem{}).Where("id = ? AND like_count > 0", poemID).
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
	var p model.Poem
	h.DB.Select("like_count").First(&p, poemID)
	ok(c, gin.H{"liked": liked, "likeCount": p.LikeCount})
}

// ListFavorites 返回当前用户收藏的诗（元信息），供个人中心「我的收藏」。
func (h *Handler) ListFavorites(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}
	var poemIDs []uint
	h.DB.Model(&model.Favorite{}).Where("user_id = ?", uid).
		Order("created_at desc").Pluck("poem_id", &poemIDs)
	if len(poemIDs) == 0 {
		ok(c, []model.Poem{})
		return
	}
	var poems []model.Poem
	h.DB.Model(&model.Poem{}).
		Select("id,title,dynasty,author,form,year_label,first_verse,audio_url,illust_url,tags,moods,comment_count,like_count,favorite_count,created_at").
		Where("id IN ?", poemIDs).Find(&poems)

	// Find 按主键返回，这里按收藏时间顺序(poemIDs 已 created_at desc)重排。
	byID := make(map[uint]model.Poem, len(poems))
	for _, p := range poems {
		byID[p.ID] = p
	}
	ordered := make([]model.Poem, 0, len(poemIDs))
	for _, id := range poemIDs {
		if p, ok := byID[id]; ok {
			ordered = append(ordered, p)
		}
	}
	h.fillDisplayVerses(ordered)
	normalizeDynasties(ordered)
	ok(c, ordered)
}
