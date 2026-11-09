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
	"time"

	"github.com/gin-gonic/gin"

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

// saveImageBytes 按内容类型存图到 ./web/images，返回可访问路径。
func saveImageBytes(data []byte, contentType string) (string, error) {
	ext := ".png"
	switch {
	case strings.Contains(contentType, "jpeg"), strings.Contains(contentType, "jpg"):
		ext = ".jpg"
	case strings.Contains(contentType, "webp"):
		ext = ".webp"
	}
	dir := "./web/images"
	if e := os.MkdirAll(dir, 0o755); e != nil {
		return "", e
	}
	name := fmt.Sprintf("illust-%d%s", time.Now().UnixNano(), ext)
	if e := os.WriteFile(filepath.Join(dir, name), data, 0o644); e != nil {
		return "", e
	}
	return "/images/" + name, nil
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
	return saveImageBytes(data, ct)
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
	return saveImageBytes(data, "image/png")
}

// resolveIllustration 先用「大模型生成检索词 + 百度图片检索」找现成插画，检索不到再按诗意生成。
func (h *Handler) resolveIllustration(p *model.Poem) (string, error) {
	if !strings.EqualFold(h.Cfg.ImgSearch, "off") {
		query, _ := h.llmClient().ImageQuery(p.Title, p.Author)
		if query == "" {
			query = fmt.Sprintf("%s %s 国画 水墨 高清", p.Title, p.Author)
		}
		urls, err := search.NewBaidu().Search(query)
		if err != nil {
			log.Printf("[illust] 百度检索失败: %v", err)
		} else {
			tried := 0
			for _, u := range urls {
				if tried >= 8 {
					break
				}
				tried++
				if path, e := h.downloadIllustration(u); e == nil {
					log.Printf("[illust] 百度检索命中(query=%s): %s", query, u)
					return path, nil
				}
			}
			log.Printf("[illust] 百度检索无可用图(query=%s)，改为生成", query)
		}
	}
	return h.generateIllustration(p)
}

// GenerateIllustration 为已存在的诗(重新)配图：优先检索，兜底生成。
func (h *Handler) GenerateIllustration(c *gin.Context) {
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
