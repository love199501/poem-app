package handler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/llm"
	"shijing/internal/model"
)

const (
	batchMaxItems = 100 // 单次最多新增首数
	batchWorkers  = 10  // 同时生成的首数
)

// AdminAuth 管理员鉴权中间件：要求登录凭证对应的用户 is_admin。
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

// AdminLogin 网页后台登录：校验口令，签发会话凭证。后续请求带 X-Auth-Token。
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
	if err := h.issueSession(&u); err != nil {
		fail(c, http.StatusInternalServerError, "登录失败")
		return
	}
	u.FillLevel()
	u.FillMember()
	ok(c, u)
}

// AdminListPoems 诗词列表（含草稿），按 id 倒序分页。
// query: page 从 1 开始，pageSize 默认 10，最大 100。
func (h *Handler) AdminListPoems(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}
	q := strings.TrimSpace(c.Query("q"))
	filter := func(db *gorm.DB) *gorm.DB {
		if q == "" {
			return db
		}
		like := "%" + q + "%"
		return db.Where("title LIKE ? OR author LIKE ? OR dynasty LIKE ? OR first_verse LIKE ?", like, like, like, like)
	}
	var total int64
	if err := h.DB.Model(&model.Poem{}).Scopes(filter).Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	var poems []model.Poem
	if err := h.DB.Model(&model.Poem{}).Scopes(filter).
		Select("id,title,dynasty,author,form,status,comment_count,created_at").
		Order("id desc").
		Offset((page - 1) * size).Limit(size).
		Find(&poems).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"list": poems, "total": total, "page": page, "pageSize": size})
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
	prepareNewPoem(&poem, "draft")
	poemCreateMu.Lock()
	defer poemCreateMu.Unlock()
	if dup, err := h.findDuplicate(poem.Title, poem.Author, 0); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	} else if dup != nil {
		fail(c, http.StatusBadRequest, dupMessage(dup))
		return
	}
	if err := h.DB.Create(&poem).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, poem)
}

// batchItem 批量新增的一首。Text 为「诗名 诗人」，也可直接传 title、author。
type batchItem struct {
	Title  string `json:"title"`
	Author string `json:"author"`
}

type batchResult struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	ID     uint   `json:"id,omitempty"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
	Warn   string `json:"warn,omitempty"`
}

// AdminBatchCreate 批量新增：解析多首诗名+诗人，并发生成内容、配图并落库。
func (h *Handler) AdminBatchCreate(c *gin.Context) {
	var body struct {
		Text    string      `json:"text"`
		Items   []batchItem `json:"items"`
		Publish bool        `json:"publish"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "格式错误")
		return
	}
	items := body.Items
	if len(items) == 0 {
		items = parseBatchText(body.Text)
	}
	if len(items) == 0 {
		fail(c, http.StatusBadRequest, "请按「诗名 诗人」每行填写至少一首")
		return
	}
	if len(items) > batchMaxItems {
		fail(c, http.StatusBadRequest, "单次最多 100 首")
		return
	}
	pending, skipped := dedupeBatch(items)
	if len(pending) == 0 && len(skipped) == 0 {
		fail(c, http.StatusBadRequest, "请按「诗名 诗人」每行填写至少一首")
		return
	}

	status := "draft"
	if body.Publish {
		status = "published"
	}
	results := append([]batchResult{}, skipped...)
	made := make([]batchResult, len(pending))
	sem := make(chan struct{}, batchWorkers)
	var wg sync.WaitGroup
	for i, it := range pending {
		wg.Add(1)
		go func(i int, it batchItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			made[i] = h.generateAndSave(it.Title, it.Author, status)
		}(i, it)
	}
	wg.Wait()
	results = append(results, made...)

	okN := 0
	for _, r := range results {
		if r.Error == "" {
			okN++
		}
	}
	ok(c, gin.H{
		"total":   len(results),
		"ok":      okN,
		"fail":    len(results) - okN,
		"results": results,
	})
}

func (h *Handler) generateAndSave(title, author, status string) batchResult {
	res := batchResult{Title: title, Author: author, Status: status}
	if dup, err := h.findDuplicate(title, author, 0); err != nil {
		res.Error = err.Error()
		return res
	} else if dup != nil {
		res.Error = dupMessage(dup)
		return res
	}
	client := llm.New(h.Cfg.LLMBaseURL, h.Cfg.LLMAPIKey, h.Cfg.LLMModel)
	poem, err := client.Generate(title, author)
	if err != nil {
		res.Error = "生成失败：" + err.Error()
		return res
	}
	if url, e := h.resolveIllustration(poem); e == nil {
		poem.IllustURL = url
	} else {
		res.Warn = "插画未生成：" + e.Error()
		log.Printf("[illust] 配图失败 %s·%s: %v", title, author, e)
	}
	prepareNewPoem(poem, status)
	// 生成后的诗名可能和输入略有差别，落库前再查一次，并挡住并发重复写入。
	poemCreateMu.Lock()
	defer poemCreateMu.Unlock()
	if dup, err := h.findDuplicate(poem.Title, poem.Author, 0); err != nil {
		res.Error = err.Error()
		return res
	} else if dup != nil {
		res.Error = dupMessage(dup)
		return res
	}
	if dup, err := h.findDuplicate(title, author, 0); err != nil {
		res.Error = err.Error()
		return res
	} else if dup != nil {
		res.Error = dupMessage(dup)
		return res
	}
	if err := h.DB.Create(poem).Error; err != nil {
		res.Error = err.Error()
		return res
	}
	res.ID = poem.ID
	return res
}

// poemCreateMu 保证「查重 + 写入」不被并发批量请求插进同一首。
var poemCreateMu sync.Mutex

// findDuplicate 按去掉书名号、空格后的「诗名 + 诗人」判断是否已有同一首。
func (h *Handler) findDuplicate(title, author string, exceptID uint) (*model.Poem, error) {
	key := poemKey(title, author)
	if key == "\n" || strings.HasPrefix(key, "\n") || strings.HasSuffix(key, "\n") {
		return nil, nil
	}
	var poems []model.Poem
	if err := h.DB.Model(&model.Poem{}).Select("id,title,author,status").Find(&poems).Error; err != nil {
		return nil, err
	}
	for i := range poems {
		if poems[i].ID == exceptID {
			continue
		}
		if poemKey(poems[i].Title, poems[i].Author) == key {
			return &poems[i], nil
		}
	}
	return nil, nil
}

func dupMessage(p *model.Poem) string {
	return fmt.Sprintf("与已有诗词重复：%s · %s（#%d）", p.Title, p.Author, p.ID)
}

func poemKey(title, author string) string {
	return normPoemName(title) + "\n" + normPoemName(author)
}

func normPoemName(s string) string {
	s = strings.TrimSpace(s)
	return strings.NewReplacer(
		" ", "", "\t", "", "\n", "", "\r", "", "　", "",
		"《", "", "》", "", "〈", "", "〉", "",
		"「", "", "」", "", "『", "", "』", "",
		"\"", "", "'", "", "“", "", "”", "", "‘", "", "’", "",
		"·", "", "・", "", "•", "",
	).Replace(s)
}

func prepareNewPoem(poem *model.Poem, status string) {
	poem.ID = 0
	poem.Status = status
	poem.CommentCount = 0
	for i := range poem.Lines {
		poem.Lines[i].ID = 0
		poem.Lines[i].Seq = i
	}
	poem.FirstVerse = model.CompleteDisplayVerse(poem.FirstVerse, poem.Lines)
	poem.Dynasty = model.NormalizeDynasty(poem.Dynasty)
}

// parseBatchText 每行「诗名 诗人」，诗人取最后一个词。
func parseBatchText(text string) []batchItem {
	var items []batchItem
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			items = append(items, batchItem{Title: line})
			continue
		}
		items = append(items, batchItem{
			Title:  strings.Join(fields[:len(fields)-1], ""),
			Author: fields[len(fields)-1],
		})
	}
	return items
}

// dedupeBatch 去掉本次提交里的空行和重复行。重复按诗名+诗人判断，忽略书名号和空格。
func dedupeBatch(items []batchItem) (pending []batchItem, skipped []batchResult) {
	seen := map[string]bool{}
	for _, it := range items {
		it.Title = strings.TrimSpace(it.Title)
		it.Author = strings.TrimSpace(it.Author)
		if it.Title == "" || it.Author == "" {
			skipped = append(skipped, batchResult{Title: it.Title, Author: it.Author, Error: "缺少诗名或诗人"})
			continue
		}
		key := poemKey(it.Title, it.Author)
		if seen[key] {
			skipped = append(skipped, batchResult{Title: it.Title, Author: it.Author, Error: "与本次提交重复，已跳过"})
			continue
		}
		seen[key] = true
		pending = append(pending, it)
	}
	return pending, skipped
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
	if dup, err := h.findDuplicate(body.Title, body.Author, id); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	} else if dup != nil {
		fail(c, http.StatusBadRequest, dupMessage(dup))
		return
	}

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
