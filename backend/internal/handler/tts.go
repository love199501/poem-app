package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/model"
	"shijing/internal/tts"
)

// SynthesizePoemAudio 为一首诗合成「有感情的朗诵」音频并缓存。
// 已有 audioUrl 直接返回；带 ?force=1 则忽略缓存、重新生成并覆盖（换音色/语速后用）。
func (h *Handler) SynthesizePoemAudio(c *gin.Context) {
	if !h.Cfg.AudioEnabled {
		fail(c, http.StatusForbidden, "朗诵暂未开放")
		return
	}
	if currentUserID(c) == 0 {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}
	id := paramUint(c, "id")
	force := c.Query("force") == "1" || c.Query("force") == "true"
	if force {
		var me model.User
		if err := h.DB.Select("is_admin").First(&me, currentUserID(c)).Error; err != nil || !me.IsAdmin {
			fail(c, http.StatusForbidden, "不能重新合成")
			return
		}
	}
	var poem model.Poem
	if err := h.DB.Preload("Lines", func(db *gorm.DB) *gorm.DB { return db.Order("seq asc") }).First(&poem, id).Error; err != nil {
		fail(c, http.StatusNotFound, "诗词不存在")
		return
	}
	text := buildReadText(&poem)
	if text == "" {
		fail(c, http.StatusBadRequest, "暂无可朗诵内容")
		return
	}

	voice := tts.ChooseRecitation(poem.Author, poem.Moods, poem.Tags, text+poem.Title)
	// 地址还在、文件或数据库里也在、音色和引擎都未变，才用缓存。否则重新合成。
	if poem.AudioURL != "" && !force && h.audioMatches(poem.AudioURL, voice.Key) && h.audioReady(poem.AudioURL) {
		ok(c, gin.H{"audioUrl": poem.AudioURL, "speaker": voice.Label})
		return
	}
	if poem.AudioURL != "" && !h.audioReady(poem.AudioURL) {
		log.Printf("[tts] 朗诵文件丢失，重新合成 poem=%d", poem.ID)
	}
	client := h.pickTTSFor(voice)
	if client == nil {
		log.Printf("[tts] 没有可用供应商: TTS_PROVIDER=%q", h.Cfg.TTSProvider)
		fail(c, http.StatusServiceUnavailable, "未配置情感朗诵")
		return
	}
	speaker := voice.Speaker
	if h.Cfg.TTSProvider == "cosyvoice" {
		speaker = h.cosyVoice()
	}
	log.Printf("[tts] %s《%s》音色 %s（%s）语速 %d", poem.Author, poem.Title, speaker, voice.Label, voice.Rate)

	audio, err := client.Synthesize(text, buildInstruction(&poem))
	if err != nil {
		log.Printf("[tts] 合成失败: %v", err)
		fail(c, http.StatusBadGateway, err.Error())
		return
	}

	dir := "./web/audio"
	if e := os.MkdirAll(dir, 0o755); e != nil {
		fail(c, http.StatusInternalServerError, e.Error())
		return
	}
	name := strconv.FormatUint(uint64(id), 10) + ".mp3"
	if e := os.WriteFile(filepath.Join(dir, name), audio, 0o644); e != nil {
		fail(c, http.StatusInternalServerError, e.Error())
		return
	}
	if e := h.DB.Save(&model.ImageFile{Name: name, ContentType: "audio/mpeg", Data: audio}).Error; e != nil {
		log.Printf("[tts] 音频写入数据库失败 poem=%d: %v", id, e)
	}

	// 带版本号，换音色/语速重新生成后能绕过小程序与缓存。
	url := fmt.Sprintf("/audio/%s?v=%d&spk=%s%s", name, time.Now().Unix(), voice.Key, h.audioEngineMark())
	h.DB.Model(&model.Poem{}).Where("id = ?", id).Update("audio_url", url)
	ok(c, gin.H{"audioUrl": url, "speaker": voice.Label})
}

// pickTTSFor 按这首诗选出的音色构造朗诵客户端。cosyvoice 走硅基流动。
func (h *Handler) pickTTSFor(voice tts.Recitation) tts.Synthesizer {
	if h.Cfg.TTSProvider == "cosyvoice" {
		if c := h.cosyClient(voice); c != nil {
			return c
		}
	}
	if h.Cfg.TTSProvider == "azure" && h.Cfg.AzureKey != "" {
		azVoice := "zh-CN-YunxiNeural"
		if strings.HasPrefix(voice.Key, "f-") {
			azVoice = "zh-CN-XiaoxiaoNeural"
		}
		return tts.NewAzure(h.Cfg.AzureKey, h.Cfg.AzureRegion, azVoice, h.Cfg.AzureStyle)
	}
	if h.Cfg.TTSProvider == "openai" && h.Cfg.TTSAPIKey != "" {
		return tts.New(h.Cfg.TTSBaseURL, h.Cfg.TTSAPIKey, h.Cfg.TTSModel, h.Cfg.TTSVoice)
	}
	if h.Cfg.VolcBigKey != "" && h.Cfg.TTSProvider != "volcano" && h.Cfg.TTSProvider != "azure" && h.Cfg.TTSProvider != "openai" {
		return tts.NewVolcanoBig(h.Cfg.VolcBigKey, "", voice.Speaker, voice.Rate, "")
	}
	if h.Cfg.VolcAppID != "" && h.Cfg.VolcToken != "" {
		return tts.NewVolcano(h.Cfg.VolcAppID, h.Cfg.VolcToken, h.Cfg.VolcCluster, h.Cfg.VolcVoiceType, h.Cfg.VolcEmotion)
	}
	if h.Cfg.VolcBigKey != "" {
		return tts.NewVolcanoBig(h.Cfg.VolcBigKey, "", voice.Speaker, voice.Rate, "")
	}
	return nil
}

func (h *Handler) cosyClient(voice tts.Recitation) tts.Synthesizer {
	key := strings.TrimSpace(h.Cfg.TTSAPIKey)
	if key == "" {
		key = strings.TrimSpace(h.Cfg.LLMAPIKey)
	}
	if key == "" {
		key = strings.TrimSpace(h.Cfg.ImgAPIKey)
	}
	if key == "" {
		return nil
	}
	base := strings.TrimSpace(h.Cfg.TTSBaseURL)
	if base == "" || strings.Contains(base, "openai.com") {
		base = "https://api.siliconflow.cn/v1"
	}
	model := strings.TrimSpace(h.Cfg.TTSModel)
	if model == "" || !strings.Contains(strings.ToLower(model), "cosyvoice") {
		model = tts.CosyModel
	}
	return tts.NewCosy(base, key, model, h.cosyVoice(), tts.CosySpeed(voice.Rate))
}

func (h *Handler) cosyVoice() string {
	return tts.CosyVoiceName(h.Cfg.TTSModel, h.Cfg.TTSVoice)
}

func (h *Handler) audioEngineMark() string {
	if h.Cfg.TTSProvider != "cosyvoice" {
		return ""
	}
	name := h.cosyVoice()
	if i := strings.LastIndex(name, ":"); i >= 0 {
		name = name[i+1:]
	}
	return "&eng=cosyvoice&voice=" + name + "&read=3"
}

func (h *Handler) audioMatches(audioURL, key string) bool {
	if !strings.Contains(audioURL, "spk="+key) {
		return false
	}
	if h.Cfg.TTSProvider == "cosyvoice" && !strings.Contains(audioURL, h.audioEngineMark()) {
		return false
	}
	return true
}

// audioReady 缓存地址对应的 mp3 在磁盘或数据库中是否还在。
func (h *Handler) audioReady(audioURL string) bool {
	name := audioFileName(audioURL)
	if name == "" {
		return false
	}
	if st, err := os.Stat(filepath.Join("./web/audio", name)); err == nil && !st.IsDir() && st.Size() > 0 {
		return true
	}
	var n int64
	h.DB.Model(&model.ImageFile{}).Where("name = ?", name).Count(&n)
	return n > 0
}

func audioFileName(audioURL string) string {
	path := audioURL
	if i := strings.Index(path, "?"); i >= 0 {
		path = path[:i]
	}
	name := filepath.Base(path)
	if name == "" || name == "." || strings.Contains(name, "..") {
		return ""
	}
	return name
}

// buildReadText 拼朗诵文本：逐句换行，借标点形成停顿。
func buildReadText(p *model.Poem) string {
	parts := make([]string, 0, len(p.Lines)+1)
	for _, l := range p.Lines {
		if strings.TrimSpace(l.Text) != "" {
			parts = append(parts, l.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// buildInstruction 依据诗的情绪/心情给出朗诵情感指令（供支持 instructions 的 TTS 使用）。
func buildInstruction(p *model.Poem) string {
	base := "请以中文古诗词朗诵的方式，深情、抑扬顿挫地朗读，字正腔圆，句末适当停顿与留白，切忌机械平读。"
	key := ""
	if len(p.Moods) > 0 {
		key = p.Moods[0]
	} else if len(p.Emotions) > 0 {
		key = p.Emotions[0].Name
	}
	tone := "语气深情庄重，如名家吟诵。"
	switch {
	case strings.ContainsAny(key, "忧愤"):
		tone = "语气沉郁顿挫、低沉而克制，饱含家国之痛，节奏缓慢厚重。"
	case strings.ContainsAny(key, "思乡家别"):
		tone = "语气温柔哀婉、略带思念的哽咽，节奏舒缓。"
	case strings.ContainsAny(key, "旷达"):
		tone = "语气豁达洒脱、从容大气。"
	case strings.ContainsAny(key, "孤独寂"):
		tone = "语气清冷孤寂、气息悠远，多留白。"
	case strings.ContainsAny(key, "喜悦狂欢"):
		tone = "语气欢快激动、神采飞扬。"
	}
	return base + tone
}
