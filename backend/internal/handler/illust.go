package handler

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/img"
	"shijing/internal/llm"
	"shijing/internal/model"
	"shijing/internal/search"
)

func (h *Handler) imgClient() *img.Client {
	return img.New(h.Cfg.ImgBaseURL, h.Cfg.ImgAPIKey, h.Cfg.ImgModel, h.Cfg.ImgSize)
}

func (h *Handler) llmClient() *llm.Client {
	return llm.New(h.Cfg.LLMBaseURL, h.Cfg.LLMAPIKey, h.Cfg.LLMModel)
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// buildIllustPrompt 依据诗的内容/意象生成文生图提示词（以画面意境为主，不强制人物）。
func buildIllustPrompt(p *model.Poem) string {
	imagery := firstNonEmpty(p.ScenePunch, p.FirstVerse, p.IntentLead)
	return fmt.Sprintf(
		"中国传统水墨画，写意留白，表现《%s》（%s·%s）的意境：%s。"+
			"山水/花鸟/人物依诗意自然取舍，淡墨渲染、意境高远、构图雅致。"+
			"严禁出现任何文字、题字、签名、印章、水印、边框或 logo。",
		p.Title, p.Dynasty, p.Author, imagery,
	)
}

func webImageDir() string { return "./web/images" }

// saveImageBytes 存图到磁盘，并写入数据库。返回可访问路径。
func (h *Handler) saveImageBytes(data []byte, contentType string) (string, error) {
	ext := ".png"
	switch {
	case strings.Contains(contentType, "jpeg"), strings.Contains(contentType, "jpg"):
		ext = ".jpg"
	case strings.Contains(contentType, "webp"):
		ext = ".webp"
	}
	dir := webImageDir()
	if e := os.MkdirAll(dir, 0o755); e != nil {
		return "", e
	}
	name := fmt.Sprintf("illust-%d%s", time.Now().UnixNano(), ext)
	if e := os.WriteFile(filepath.Join(dir, name), data, 0o644); e != nil {
		return "", e
	}
	if e := h.DB.Save(&model.ImageFile{Name: name, ContentType: contentType, Data: data}).Error; e != nil {
		return "", e
	}
	return "/images/" + name, nil
}

// ServeImage 优先读磁盘；磁盘没有时从数据库取出（重新部署后仍能看到插画）。
func (h *Handler) ServeImage(c *gin.Context) { h.serveStored(c, webImageDir()) }

// ServeAvatar 头像同样落在容器磁盘，重新部署后改从数据库读。
func (h *Handler) ServeAvatar(c *gin.Context) { h.serveStored(c, "./web/avatars") }

// ServeAudio 朗诵文件。磁盘被清掉后从数据库读出再播。
func (h *Handler) ServeAudio(c *gin.Context) { h.serveStored(c, "./web/audio") }

func (h *Handler) serveStored(c *gin.Context, dir string) {
	name := filepath.Base(c.Param("name"))
	if name == "" || name == "." || strings.Contains(name, "..") {
		c.Status(http.StatusNotFound)
		return
	}
	path := filepath.Join(dir, name)
	// 插画和朗诵文件名在更新时会变，可以长期缓存，换卡时直接用本地副本。
	c.Header("Cache-Control", "public, max-age=2592000")
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		c.File(path)
		return
	}
	var img model.ImageFile
	if err := h.DB.Where("name = ?", name).First(&img).Error; err != nil || len(img.Data) == 0 {
		c.Status(http.StatusNotFound)
		return
	}
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(path, img.Data, 0o644)
	ct := img.ContentType
	if ct == "" {
		if strings.HasSuffix(strings.ToLower(name), ".mp3") {
			ct = "audio/mpeg"
		} else {
			ct = "image/png"
		}
	}
	c.Data(http.StatusOK, ct, img.Data)
}

// downloadIllustration 校验并下载外链图片（真实可访问的图片）。
func (h *Handler) downloadIllustration(u string) (string, error) {
	if !strings.HasPrefix(u, "http") {
		return "", errors.New("非法图片地址")
	}
	cli := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	req.Header.Set("Referer", "https://image.baidu.com/")
	resp, err := cli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("图片不可访问(%d)", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		return "", fmt.Errorf("不是图片: %s", ct)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", err
	}
	if len(data) < 2048 {
		return "", errors.New("图片过小，疑似无效")
	}
	return h.saveImageBytes(data, ct)
}

// generateIllustration 文生图兜底（检索不到时按诗意生成）。
func (h *Handler) generateIllustration(p *model.Poem) (string, error) {
	c := h.imgClient()
	if !c.Configured() {
		return "", errors.New("未配置文生图(IMG_API_KEY)")
	}
	data, err := c.Generate(buildIllustPrompt(p))
	if err != nil {
		return "", err
	}
	return h.saveImageBytes(data, "image/png")
}

// 文生图接口并发容易被限流。批量新增时排队配图，避免大多数诗落库时没有插画。
var illustGate = make(chan struct{}, 1)

var illustBackfillMu sync.Mutex

// StartIllustBackfill 每 3 分钟检查一次。每次各补一处：缺插画、缺诗人介绍、缺诗文原文、缺句读，并核对一首诗是否收全、文字是否与通行本一致。
func (h *Handler) StartIllustBackfill() {
	go func() {
		log.Printf("[cron] 定时检查已启动，立即执行一轮，之后每 3 分钟一次")
		h.fillGaps()
		ticker := time.NewTicker(3 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			h.fillGaps()
		}
	}()
}

func (h *Handler) fillGaps() {
	if !illustBackfillMu.TryLock() {
		log.Printf("[cron] 上一轮还没结束，本轮跳过")
		return
	}
	defer illustBackfillMu.Unlock()
	start := time.Now()
	log.Printf("[cron] 开始检查")
	h.fillMissingIllustrations()
	h.fillMissingPoetIntro()
	h.fillMissingPoemContent()
	h.fillShortPoem()
	h.fillPunctuation()
	h.checkPoemText()
	log.Printf("[cron] 本轮结束，用时 %s", time.Since(start).Round(time.Second))
}

func (h *Handler) fillMissingIllustrations() {
	var poems []model.Poem
	if err := h.DB.Select("id,title,dynasty,author,first_verse,scene_punch,intent_lead").
		Where("illust_url IS NULL OR illust_url = ''").
		Order("id asc").
		Limit(1).
		Find(&poems).Error; err != nil {
		log.Printf("[illust] 查询缺图诗词失败: %v", err)
		return
	}
	if len(poems) == 0 {
		return
	}
	for i := range poems {
		p := &poems[i]
		url, err := h.resolveIllustration(p)
		if err != nil {
			log.Printf("[illust] 补充失败 %s·%s: %v", p.Title, p.Author, err)
			continue
		}
		if err := h.DB.Model(&model.Poem{}).Where("id = ?", p.ID).Update("illust_url", url).Error; err != nil {
			log.Printf("[illust] 写入插画地址失败 id=%d: %v", p.ID, err)
			continue
		}
		log.Printf("[illust] 已补充 %s·%s", p.Title, p.Author)
	}
}

// fillMissingPoetIntro 补一位还没有介绍的诗人。佚名没有可写的生平，跳过。
func (h *Handler) fillMissingPoetIntro() {
	type row struct {
		Author  string
		Dynasty string
	}
	var r row
	err := h.DB.Raw(`
SELECT author, dynasty FROM poems
WHERE status <> 'draft' AND author <> '' AND author NOT IN ('佚名', '无名氏')
AND NOT EXISTS (
  SELECT 1 FROM poets WHERE poets.name = poems.author AND poets.intro <> ''
)
ORDER BY id ASC LIMIT 1`).Scan(&r).Error
	if err != nil {
		log.Printf("[poet] 查询缺介绍诗人失败: %v", err)
		return
	}
	if strings.TrimSpace(r.Author) == "" {
		return
	}
	if intro := h.ensurePoetIntro(r.Author, model.NormalizeDynasty(r.Dynasty)); intro != "" {
		log.Printf("[poet] 已补充介绍 %s", r.Author)
	}
}

// fillMissingPoemContent 补一首没有原文的诗，写回原记录，保留插画、音频和互动数据。
func (h *Handler) fillMissingPoemContent() {
	var id uint
	err := h.DB.Raw(`
SELECT id FROM poems
WHERE status <> 'draft'
AND NOT EXISTS (
  SELECT 1 FROM poem_lines
  WHERE poem_lines.poem_id = poems.id
    AND poem_lines.text IS NOT NULL
    AND CHAR_LENGTH(TRIM(poem_lines.text)) > 0
)
ORDER BY id ASC LIMIT 1`).Scan(&id).Error
	if err != nil {
		log.Printf("[poem] 查询缺原文诗词失败: %v", err)
		return
	}
	if id == 0 {
		return
	}
	if err := h.refillPoemContent(id); err != nil {
		if errors.Is(err, errNoPoemBody) {
			if e := h.deleteEmptyPoem(id); e != nil {
				log.Printf("[poem] 删除无原文诗词失败 id=%d: %v", id, e)
				return
			}
			log.Printf("[poem] 找不到原文，已删除 id=%d", id)
			return
		}
		log.Printf("[poem] 补充原文失败 id=%d: %v", id, err)
		return
	}
	log.Printf("[poem] 已补充原文 id=%d", id)
}

var (
	errNoPoemBody   = errors.New("生成结果没有原文")
	poemSpanChecked sync.Map
	punctChecked    sync.Map
	poemTextChecked sync.Map
)

// fillPunctuation 每次为最新一首缺句读的诗补上标点，并给难字注音。汉字保持原样。
func (h *Handler) fillPunctuation() {
	type row struct {
		ID     uint
		Title  string
		Author string
	}
	var rows []row
	err := h.DB.Raw(`
SELECT p.id, p.title, p.author
FROM poems p
WHERE p.status <> 'draft'
AND EXISTS (
  SELECT 1 FROM poem_lines l
  WHERE l.poem_id = p.id
    AND CHAR_LENGTH(TRIM(l.text)) > 0
    AND (
      RIGHT(TRIM(l.text), 1) NOT IN ('。','？','！','，','、','；','：')
      OR (l.text LIKE '%兮%' AND l.text NOT LIKE '%兮，%' AND l.text NOT LIKE '%兮。%' AND l.text NOT LIKE '%兮？%' AND l.text NOT LIKE '%兮！%')
    )
)
ORDER BY p.id DESC`).Scan(&rows).Error
	if err != nil {
		log.Printf("[poem] 查询缺句读的诗文失败: %v", err)
		return
	}
	if len(rows) == 0 {
		log.Printf("[cron] 句读：没有缺标点的诗")
		return
	}
	log.Printf("[cron] 句读：待补 %d 首", len(rows))
	client := llm.New(h.Cfg.LLMBaseURL, h.Cfg.LLMAPIKey, h.Cfg.LLMModel)
	for _, r := range rows {
		if _, ok := punctChecked.Load(r.ID); ok {
			continue
		}
		var lines []model.PoemLine
		if err := h.DB.Where("poem_id = ?", r.ID).Order("seq asc, id asc").Find(&lines).Error; err != nil {
			log.Printf("[poem] 读取诗句失败 id=%d: %v", r.ID, err)
			return
		}
		texts := make([]string, 0, len(lines))
		for _, l := range lines {
			texts = append(texts, strings.TrimSpace(l.Text))
		}
		if len(texts) == 0 {
			punctChecked.Store(r.ID, true)
			continue
		}
		changed := llm.NormalizePunct(lines)
		changed = llm.EnsureReadingPunct(lines) || changed
		if changed {
			if !savePunctLines(h, r.ID, lines) {
				return
			}
			punctChecked.Store(r.ID, true)
			log.Printf("[poem] 已补句读 id=%d %s", r.ID, r.Title)
			continue
		}
		if !client.Configured() {
			punctChecked.Store(r.ID, true)
			continue
		}
		gen, err := client.Punctuate(r.Title, r.Author, texts)
		if err != nil {
			log.Printf("[poem] 句读失败 %s·%s: %v", r.Title, r.Author, err)
			punctChecked.Store(r.ID, true)
			return
		}
		updated := 0
		for i := range lines {
			old := strings.TrimSpace(lines[i].Text)
			neu := strings.TrimSpace(gen[i].Text)
			if hanziOnly(old) == "" || hanziOnly(old) != hanziOnly(neu) || !hasPunct(neu) {
				log.Printf("[poem] 句读未采用 id=%d 第%d句", r.ID, i+1)
				continue
			}
			lines[i].Text = neu
			lines[i].Rubies = cleanRubies(neu, gen[i].Rubies)
			if err := h.DB.Save(&lines[i]).Error; err != nil {
				log.Printf("[poem] 保存句读失败 id=%d: %v", r.ID, err)
				return
			}
			updated++
		}
		var poem model.Poem
		if h.DB.Select("id, first_verse").First(&poem, r.ID).Error == nil {
			if verse := punctuatedVerse(poem.FirstVerse, lines); verse != poem.FirstVerse {
				h.DB.Model(&model.Poem{}).Where("id = ?", r.ID).Update("first_verse", verse)
			}
		}
		punctChecked.Store(r.ID, true)
		log.Printf("[poem] 已补句读与注音 id=%d %s，%d 句", r.ID, r.Title, updated)
		return
	}
}

func savePunctLines(h *Handler, id uint, lines []model.PoemLine) bool {
	for i := range lines {
		if err := h.DB.Model(&model.PoemLine{}).Where("id = ?", lines[i].ID).Update("text", lines[i].Text).Error; err != nil {
			log.Printf("[poem] 保存句读失败 id=%d: %v", id, err)
			return false
		}
	}
	var poem model.Poem
	if h.DB.Select("id, first_verse").First(&poem, id).Error == nil {
		if verse := model.CompleteDisplayVerse(poem.FirstVerse, lines); verse != poem.FirstVerse {
			h.DB.Model(&model.Poem{}).Where("id = ?", id).Update("first_verse", verse)
		}
	}
	return true
}

func linesHaveAnyPunct(lines []model.PoemLine) bool {
	for _, l := range lines {
		if strings.ContainsAny(l.Text, "，。？！；：、") {
			return true
		}
	}
	return false
}

func hanziOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func hasPunct(s string) bool {
	return strings.ContainsAny(s, "，。？！；：、")
}

func cleanRubies(text string, in []model.Ruby) []model.Ruby {
	out := make([]model.Ruby, 0, len(in))
	for _, r := range in {
		r.Char = strings.TrimSpace(r.Char)
		r.Py = strings.TrimSpace(r.Py)
		if len([]rune(r.Char)) != 1 || !strings.Contains(text, r.Char) || !validPy(r.Py) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func validPy(py string) bool {
	if py == "" || len([]rune(py)) > 8 {
		return false
	}
	for _, r := range py {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case strings.ContainsRune("āáǎàēéěèīíǐìōóǒòūúǔùǖǘǚǜüÜńňǹ", r):
		default:
			return false
		}
	}
	return true
}

func punctuatedVerse(old string, lines []model.PoemLine) string {
	target := hanziOnly(old)
	if target == "" || len(lines) == 0 {
		return old
	}
	acc := ""
	han := ""
	for _, l := range lines {
		acc += strings.TrimSpace(l.Text)
		han += hanziOnly(l.Text)
		if han == target {
			return acc
		}
		if len([]rune(han)) > len([]rune(target)) {
			break
		}
	}
	return old
}

// fillShortPoem 每次核对一首可能被截断的诗。全文句数明显多于库里的，就按原记录补全。
func (h *Handler) fillShortPoem() {
	type row struct {
		ID     uint
		Title  string
		Author string
		Form   string
		N      int
	}
	var rows []row
	err := h.DB.Raw(`
SELECT p.id, p.title, p.author, p.form,
  (SELECT COUNT(*) FROM poem_lines l
    WHERE l.poem_id = p.id AND l.text IS NOT NULL AND CHAR_LENGTH(TRIM(l.text)) > 0) AS n
FROM poems p
WHERE p.status <> 'draft' AND p.title <> '' AND p.author <> ''
ORDER BY p.id ASC`).Scan(&rows).Error
	if err != nil {
		log.Printf("[poem] 查询可能不全的诗文失败: %v", err)
		return
	}
	client := llm.New(h.Cfg.LLMBaseURL, h.Cfg.LLMAPIKey, h.Cfg.LLMModel)
	if !client.Configured() {
		return
	}
	for _, r := range rows {
		if _, ok := poemSpanChecked.Load(r.ID); ok {
			continue
		}
		if r.N == 0 || strings.Contains(r.Form, "绝句") {
			poemSpanChecked.Store(r.ID, true)
			continue
		}
		count, last, err := client.PoemSpan(r.Title, r.Author)
		if err != nil || count <= 0 {
			log.Printf("[poem] 无法判断全文 %s·%s: %v", r.Title, r.Author, err)
			return
		}
		if r.N+2 >= count {
			poemSpanChecked.Store(r.ID, true)
			log.Printf("[poem] 原文句数正常 id=%d %s：库内 %d 句，全文约 %d 句", r.ID, r.Title, r.N, count)
			return
		}
		log.Printf("[poem] 原文可能不全 %s·%s：库内 %d 句，全文约 %d 句，末句「%s」", r.Title, r.Author, r.N, count, last)
		if err := h.refillPoemContent(r.ID); err != nil {
			log.Printf("[poem] 补全原文失败 id=%d: %v", r.ID, err)
			return
		}
		poemSpanChecked.Store(r.ID, true)
		log.Printf("[poem] 已补全原文 id=%d %s", r.ID, r.Title)
		return
	}
}

// checkPoemText 每次核对一首诗：原文是否收全，文字是否与通行本一致。错则替换原文，保留注释和互动数据。
func (h *Handler) checkPoemText() {
	type row struct {
		ID     uint
		Title  string
		Author string
	}
	var rows []row
	err := h.DB.Raw(`
SELECT id, title, author FROM poems
WHERE status <> 'draft' AND title <> '' AND author <> ''
AND EXISTS (
  SELECT 1 FROM poem_lines l
  WHERE l.poem_id = poems.id AND CHAR_LENGTH(TRIM(l.text)) > 0
)
ORDER BY id DESC`).Scan(&rows).Error
	if err != nil {
		log.Printf("[poem] 查询待核对诗文失败: %v", err)
		return
	}
	client := llm.New(h.Cfg.LLMBaseURL, h.Cfg.LLMAPIKey, h.Cfg.LLMModel)
	if !client.Configured() {
		return
	}
	for _, r := range rows {
		if _, ok := poemTextChecked.Load(r.ID); ok {
			continue
		}
		var stored []model.PoemLine
		if err := h.DB.Where("poem_id = ?", r.ID).Order("seq asc, id asc").Find(&stored).Error; err != nil {
			log.Printf("[poem] 读取待核对诗文失败 id=%d: %v", r.ID, err)
			return
		}
		texts := make([]string, 0, len(stored))
		for _, l := range stored {
			if t := strings.TrimSpace(l.Text); t != "" {
				texts = append(texts, t)
			}
		}
		if len(texts) == 0 {
			poemTextChecked.Store(r.ID, true)
			continue
		}
		audit, err := client.AuditPoem(r.Title, r.Author, texts)
		if err != nil {
			log.Printf("[poem] 核对失败 %s·%s: %v", r.Title, r.Author, err)
			poemTextChecked.Store(r.ID, true)
			return
		}
		fresh := llm.CorrectedLines(texts, audit)
		poemTextChecked.Store(r.ID, true)
		if len(fresh) == 0 {
			log.Printf("[poem] 原文核对通过 id=%d %s", r.ID, r.Title)
			return
		}
		if err := h.replacePoemLines(r.ID, stored, fresh); err != nil {
			log.Printf("[poem] 改正原文失败 id=%d: %v", r.ID, err)
			return
		}
		log.Printf("[poem] 已改正原文 id=%d %s：%s", r.ID, r.Title, audit.Reason)
		return
	}
}

func (h *Handler) replacePoemLines(poemID uint, old []model.PoemLine, fresh []string) error {
	byHan := map[string]model.PoemLine{}
	for _, l := range old {
		k := hanziOnly(l.Text)
		if k != "" {
			if _, ok := byHan[k]; !ok {
				byHan[k] = l
			}
		}
	}
	rows := make([]model.PoemLine, 0, len(fresh))
	for _, text := range fresh {
		row := model.PoemLine{PoemID: poemID, Seq: len(rows), Text: strings.TrimSpace(text)}
		if prev, ok := byHan[hanziOnly(text)]; ok {
			row.NoteBody = prev.NoteBody
			row.NoteSrc = prev.NoteSrc
			row.Rubies = prev.Rubies
		}
		rows = append(rows, row)
	}
	llm.NormalizePunct(rows)
	return h.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Where("poem_id = ?", poemID).Delete(&model.PoemLine{}).Error; e != nil {
			return e
		}
		if e := tx.Create(&rows).Error; e != nil {
			return e
		}
		var poem model.Poem
		if e := tx.Select("id, first_verse").First(&poem, poemID).Error; e != nil {
			return e
		}
		verse := model.CompleteDisplayVerse(poem.FirstVerse, rows)
		if verse != poem.FirstVerse {
			return tx.Model(&model.Poem{}).Where("id = ?", poemID).Update("first_verse", verse).Error
		}
		return nil
	})
}

func poemHasBody(p *model.Poem) bool {
	if p == nil {
		return false
	}
	for _, l := range p.Lines {
		if strings.TrimSpace(l.Text) != "" {
			return true
		}
	}
	return false
}

func (h *Handler) refillPoemContent(id uint) error {
	var existing model.Poem
	if err := h.DB.First(&existing, id).Error; err != nil {
		return err
	}
	client := llm.New(h.Cfg.LLMBaseURL, h.Cfg.LLMAPIKey, h.Cfg.LLMModel)
	gen, err := client.Generate(existing.Title, existing.Author)
	if err != nil {
		return err
	}
	if !poemHasBody(gen) {
		return errNoPoemBody
	}
	gen.ID = existing.ID
	gen.Title = existing.Title
	gen.Author = existing.Author
	gen.CreatedAt = existing.CreatedAt
	gen.Status = existing.Status
	gen.CommentCount = existing.CommentCount
	gen.LikeCount = existing.LikeCount
	gen.FavoriteCount = existing.FavoriteCount
	gen.AudioURL = existing.AudioURL
	gen.IllustURL = existing.IllustURL
	for i := range gen.Lines {
		gen.Lines[i].ID = 0
		gen.Lines[i].PoemID = existing.ID
		gen.Lines[i].Seq = i
	}
	gen.FirstVerse = model.CompleteDisplayVerse(gen.FirstVerse, gen.Lines)
	gen.Dynasty = model.NormalizeDynasty(gen.Dynasty)
	return h.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Where("poem_id = ?", existing.ID).Delete(&model.PoemLine{}).Error; e != nil {
			return e
		}
		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(gen).Error
	})
}

// deleteEmptyPoem 删掉补不出原文的诗，并清掉逐句、收藏、点赞、评论和阅读记录。
func (h *Handler) deleteEmptyPoem(id uint) error {
	return h.DB.Transaction(func(tx *gorm.DB) error {
		var commentIDs []uint
		if e := tx.Model(&model.Comment{}).Where("poem_id = ?", id).Pluck("id", &commentIDs).Error; e != nil {
			return e
		}
		if len(commentIDs) > 0 {
			if e := tx.Where("comment_id IN ?", commentIDs).Delete(&model.CommentLike{}).Error; e != nil {
				return e
			}
		}
		if e := tx.Where("poem_id = ?", id).Delete(&model.Comment{}).Error; e != nil {
			return e
		}
		if e := tx.Where("poem_id = ?", id).Delete(&model.PoemLine{}).Error; e != nil {
			return e
		}
		if e := tx.Where("poem_id = ?", id).Delete(&model.Favorite{}).Error; e != nil {
			return e
		}
		if e := tx.Where("poem_id = ?", id).Delete(&model.PoemLike{}).Error; e != nil {
			return e
		}
		if e := tx.Where("poem_id = ?", id).Delete(&model.ReadHistory{}).Error; e != nil {
			return e
		}
		return tx.Delete(&model.Poem{}, id).Error
	})
}

// resolveIllustration 先检索现成插画，没有再按诗意生成。生成失败会重试。
func (h *Handler) resolveIllustration(p *model.Poem) (string, error) {
	illustGate <- struct{}{}
	defer func() { <-illustGate }()

	if url, err := h.searchIllustration(p); err == nil && url != "" {
		return url, nil
	} else if err != nil {
		log.Printf("[illust] 检索未用上 %s·%s: %v", p.Title, p.Author, err)
	}

	var last error
	for i := 0; i < 3; i++ {
		if i > 0 {
			time.Sleep(time.Duration(i*3) * time.Second)
		}
		url, err := h.generateIllustration(p)
		if err == nil && url != "" {
			return url, nil
		}
		last = err
		log.Printf("[illust] 生成失败 %s·%s 第%d次: %v", p.Title, p.Author, i+1, err)
		if err != nil && strings.Contains(err.Error(), "未配置") {
			break
		}
	}
	if last == nil {
		last = errors.New("未生成插画")
	}
	return "", last
}

// searchIllustration 用检索词在百度图片里找一张能下载的插画。
func (h *Handler) searchIllustration(p *model.Poem) (string, error) {
	if strings.EqualFold(h.Cfg.ImgSearch, "off") {
		return "", errors.New("已关闭插画检索")
	}
	query, _ := h.llmClient().ImageQuery(p.Title, p.Author)
	if query == "" {
		query = fmt.Sprintf("%s %s 国画 水墨 高清", p.Title, p.Author)
	}
	urls, err := search.NewBaidu().Search(query)
	if err != nil {
		return "", err
	}
	tried := 0
	var last error
	for _, u := range urls {
		if tried >= 8 {
			break
		}
		tried++
		path, e := h.downloadIllustration(u)
		if e == nil {
			log.Printf("[illust] 百度检索命中(query=%s): %s", query, u)
			return path, nil
		}
		last = e
	}
	if last == nil {
		last = errors.New("百度检索无可用图")
	}
	log.Printf("[illust] 百度检索无可用图(query=%s)，改为生成", query)
	return "", last
}

// GenerateIllustration 为已存在的诗(重新)配图：优先检索，兜底生成。会调用付费接口，仅管理员可触发。
func (h *Handler) GenerateIllustration(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}
	var me model.User
	if err := h.DB.Select("is_admin").First(&me, uid).Error; err != nil || !me.IsAdmin {
		fail(c, http.StatusForbidden, "需要管理员权限")
		return
	}
	id := paramUint(c, "id")
	var poem model.Poem
	if err := h.DB.First(&poem, id).Error; err != nil {
		fail(c, http.StatusNotFound, "诗词不存在")
		return
	}
	url, err := h.resolveIllustration(&poem)
	if err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	h.DB.Model(&model.Poem{}).Where("id = ?", id).Update("illust_url", url)
	ok(c, gin.H{"illustrationUrl": url})
}
