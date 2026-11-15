package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/llm"
	"shijing/internal/model"
)

// AskAuthor 会员向这首诗的作者追问。回答由后台 DeepSeek 生成，并尽量标注出处。
func (h *Handler) AskAuthor(c *gin.Context) {
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
	var body struct {
		Question string `json:"question"`
		History  []struct {
			Question string `json:"question"`
			Answer   string `json:"answer"`
		} `json:"history"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "格式错误")
		return
	}
	q := strings.TrimSpace(body.Question)
	if q == "" {
		fail(c, http.StatusBadRequest, "请先写下一个问题")
		return
	}
	if len([]rune(q)) > 200 {
		fail(c, http.StatusBadRequest, "问题请控制在 200 字以内")
		return
	}
	id := paramUint(c, "id")
	var poem model.Poem
	err := h.DB.Preload("Lines", func(db *gorm.DB) *gorm.DB {
		return db.Order("seq asc")
	}).First(&poem, id).Error
	if err != nil {
		fail(c, http.StatusNotFound, "诗词不存在")
		return
	}
	history := make([][2]string, 0, 6)
	start := 0
	if n := len(body.History); n > 6 {
		start = n - 6
	}
	for _, h := range body.History[start:] {
		qq := strings.TrimSpace(h.Question)
		aa := strings.TrimSpace(h.Answer)
		if qq != "" && aa != "" {
			history = append(history, [2]string{qq, aa})
		}
	}
	remain, allowed := h.useAskTurn(c, &u, poem.Author, poem.Dynasty)
	if !allowed {
		return
	}
	client := llm.New(h.Cfg.LLMBaseURL, h.Cfg.LLMAPIKey, h.Cfg.LLMModel)
	answer, source, err := client.AskAuthor(&poem, q, history)
	if err != nil {
		if !u.IsMember {
			h.giveBackAskTurn(u.ID, poem.Author, poem.Dynasty)
		}
		fail(c, http.StatusBadGateway, "暂时无法回答，请稍后再试")
		return
	}
	ok(c, gin.H{"answer": answer, "source": source, "askRemain": remain})
}

// AskPoet 会员向这位诗人追问。场景是他的一生，不围着某一首诗。
func (h *Handler) AskPoet(c *gin.Context) {
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
	var body struct {
		Author   string `json:"author"`
		Dynasty  string `json:"dynasty"`
		Question string `json:"question"`
		History  []struct {
			Question string `json:"question"`
			Answer   string `json:"answer"`
		} `json:"history"`
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
	q := strings.TrimSpace(body.Question)
	if q == "" {
		fail(c, http.StatusBadRequest, "请先写下一个问题")
		return
	}
	if len([]rune(q)) > 200 {
		fail(c, http.StatusBadRequest, "问题请控制在 200 字以内")
		return
	}
	dynasty := model.NormalizeDynasty(body.Dynasty)
	intro, life := h.poetChatMaterial(author, dynasty)
	history := make([][2]string, 0, 6)
	start := 0
	if n := len(body.History); n > 6 {
		start = n - 6
	}
	for _, turn := range body.History[start:] {
		qq := clipRunes(strings.TrimSpace(turn.Question), 200)
		aa := clipRunes(strings.TrimSpace(turn.Answer), 300)
		if qq != "" && aa != "" {
			history = append(history, [2]string{qq, aa})
		}
	}
	remain, allowed := h.useAskTurn(c, &u, author, dynasty)
	if !allowed {
		return
	}
	client := llm.New(h.Cfg.LLMBaseURL, h.Cfg.LLMAPIKey, h.Cfg.LLMModel)
	answer, source, err := client.AskPoet(author, dynasty, intro, life, q, history)
	if err != nil {
		if !u.IsMember {
			h.giveBackAskTurn(u.ID, author, dynasty)
		}
		fail(c, http.StatusBadGateway, "暂时无法回答，请稍后再试")
		return
	}
	ok(c, gin.H{"answer": answer, "source": source, "askRemain": remain})
}

func clipRunes(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n])
}
