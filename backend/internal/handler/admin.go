package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/llm"
	"shijing/internal/model"
)

// AdminAuth 管理员鉴权中间件：要求 X-User-Id 对应的用户 is_admin。
func (h *Handler) AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := currentUserID(c)
		if uid == 0 {
			fail(c, http.StatusUnauthorized, "未登录")
			c.Abort()
			return
		}
		var u model.User
		if err := h.DB.First(&u, uid).Error; err != nil || !u.IsAdmin {
			fail(c, http.StatusForbidden, "需要管理员权限")
			c.Abort()
			return
		}
		c.Next()
	}
}

// AdminLogin 网页后台登录：校验口令，返回管理员用户（其 id 作为后续 X-User-Id）。
func (h *Handler) AdminLogin(c *gin.Context) {
	var body struct {
		Key string `json:"key"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Key == "" || body.Key != h.Cfg.AdminKey {
		fail(c, http.StatusUnauthorized, "口令错误")
		return
	}
	var u model.User
	if err := h.DB.Where("open_id = ?", "admin-root").First(&u).Error; err != nil {
		fail(c, http.StatusInternalServerError, "管理员账号未初始化")
		return
	}
	ok(c, u)
}

// AdminListPoems 全部诗词（含草稿），供后台管理。
func (h *Handler) AdminListPoems(c *gin.Context) {
	var poems []model.Poem
	h.DB.Model(&model.Poem{}).
		Select("id,title,dynasty,author,form,status,comment_count,created_at").
		Order("id desc").Find(&poems)
	ok(c, poems)
}

// AdminGenerate 输入诗名+诗人，调用大模型生成结构化内容（不落库，供后台预览/编辑）。
func (h *Handler) AdminGenerate(c *gin.Context) {
	var body struct {
		Title  string `json:"title"`
		Author string `json:"author"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Title == "" || body.Author == "" {
		fail(c, http.StatusBadRequest, "请填写诗名与诗人")
		return
	}
	client := llm.New(h.Cfg.LLMBaseURL, h.Cfg.LLMAPIKey, h.Cfg.LLMModel)
	poem, err := client.Generate(body.Title, body.Author)
	if err != nil {
		fail(c, http.StatusBadGateway, "生成失败："+err.Error())
		return
	}
	// 生成内容时一并配图：先让大模型检索现成插画，检索不到再按诗意生成（配了 IMG_API_KEY 才兜底生成）。
	if url, e := h.resolveIllustration(poem); e == nil {
		poem.IllustURL = url
	} else {
		log.Printf("[illust] 配图失败: %v", e)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "mock": !client.Configured(), "data": poem})
}

// AdminCreatePoem 保存为草稿（status=draft）。
func (h *Handler) AdminCreatePoem(c *gin.Context) {
	var poem model.Poem
	if err := c.ShouldBindJSON(&poem); err != nil || poem.Title == "" {
		fail(c, http.StatusBadRequest, "内容为空或格式错误")
		return
	}
	poem.ID = 0
	poem.Status = "draft"
	poem.CommentCount = 0
	for i := range poem.Lines {
		poem.Lines[i].ID = 0
		poem.Lines[i].Seq = i
	}
	poem.FirstVerse = model.CompleteDisplayVerse(poem.FirstVerse, poem.Lines)
	poem.Dynasty = model.NormalizeDynasty(poem.Dynasty)
	if err := h.DB.Create(&poem).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, poem)
}

// AdminUpdatePoem 更新草稿/已发布内容（整体覆盖，含逐句）。
func (h *Handler) AdminUpdatePoem(c *gin.Context) {
	id := paramUint(c, "id")
	var existing model.Poem
	if err := h.DB.First(&existing, id).Error; err != nil {
		fail(c, http.StatusNotFound, "诗词不存在")
		return
	}
	var body model.Poem
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "格式错误")
		return
	}
	// 保留不可被编辑表单覆盖的字段。
	body.ID = id
	body.CreatedAt = existing.CreatedAt
	body.CommentCount = existing.CommentCount
	body.Status = existing.Status

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Where("poem_id = ?", id).Delete(&model.PoemLine{}).Error; e != nil {
			return e
		}
		for i := range body.Lines {
			body.Lines[i].ID = 0
			body.Lines[i].PoemID = id
			body.Lines[i].Seq = i
		}
		body.FirstVerse = model.CompleteDisplayVerse(body.FirstVerse, body.Lines)
		body.Dynasty = model.NormalizeDynasty(body.Dynasty)
		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(&body).Error
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, body)
}

// AdminPublish / AdminUnpublish 审批发布 / 下架。
func (h *Handler) AdminPublish(c *gin.Context) {
	h.setStatus(c, "published")
}
func (h *Handler) AdminUnpublish(c *gin.Context) {
	h.setStatus(c, "draft")
}
func (h *Handler) setStatus(c *gin.Context, status string) {
	id := paramUint(c, "id")
	if err := h.DB.Model(&model.Poem{}).Where("id = ?", id).Update("status", status).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"id": id, "status": status})
}

// AdminDeletePoem 删除诗词及其逐句、评论。
func (h *Handler) AdminDeletePoem(c *gin.Context) {
	id := paramUint(c, "id")
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		tx.Where("poem_id = ?", id).Delete(&model.PoemLine{})
		tx.Where("poem_id = ?", id).Delete(&model.Comment{})
		return tx.Delete(&model.Poem{}, id).Error
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"id": id})
}
