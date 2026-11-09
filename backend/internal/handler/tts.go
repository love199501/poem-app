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
	id := paramUint(c, "id")
	force := c.Query("force") == "1" || c.Query("force") == "true"
	var poem model.Poem
	if err := h.DB.Preload("Lines", func(db *gorm.DB) *gorm.DB { return db.Order("seq asc") }).First(&poem, id).Error; err != nil {
		fail(c, http.StatusNotFound, "诗词不存在")
		return
	}
	if poem.AudioURL != "" && !force {
		ok(c, gin.H{"audioUrl": poem.AudioURL})
		return
	}

	client := h.pickTTS()
	if client == nil {
		log.Printf("[tts] 没有可用供应商: TTS_PROVIDER=%q volcano=%v azure=%v openai=%v",
			h.Cfg.TTSProvider, h.Cfg.VolcAppID != "" && h.Cfg.VolcToken != "",
			h.Cfg.AzureKey != "", h.Cfg.TTSAPIKey != "")
		fail(c, http.StatusServiceUnavailable,
			"未配置情感朗诵（当前 TTS_PROVIDER="+h.Cfg.TTSProvider+"；火山需 VOLC_APPID+VOLC_ACCESS_TOKEN，Azure 需 AZURE_SPEECH_KEY，OpenAI 需 TTS_API_KEY）")
		return
	}

	text := buildReadText(&poem)
	if text == "" {
		fail(c, http.StatusBadRequest, "暂无可朗诵内容")
		return
	}

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

	// 带版本号，换音色/语速重新生成后能绕过小程序与缓存。
	url := fmt.Sprintf("/audio/%s?v=%d", name, time.Now().Unix())
	h.DB.Model(&model.Poem{}).Where("id = ?", id).Update("audio_url", url)
	ok(c, gin.H{"audioUrl": url})
}

// pickTTS 按配置选择朗诵供应商。未配置返回 nil。
func (h *Handler) pickTTS() tts.Synthesizer {
	az := tts.NewAzure(h.Cfg.AzureKey, h.Cfg.AzureRegion, h.Cfg.AzureVoice, h.Cfg.AzureStyle)
	vo := tts.NewVolcano(h.Cfg.VolcAppID, h.Cfg.VolcToken, h.Cfg.VolcCluster, h.Cfg.VolcVoiceType, h.Cfg.VolcEmotion)
	vb := tts.NewVolcanoBig(h.Cfg.VolcBigKey, h.Cfg.VolcResourceID, h.Cfg.VolcSpeaker, h.Cfg.VolcSpeechRate, h.Cfg.VolcBigEmotion)
	oa := tts.New(h.Cfg.TTSBaseURL, h.Cfg.TTSAPIKey, h.Cfg.TTSModel, h.Cfg.TTSVoice)
	switch h.Cfg.TTSProvider {
	case "azure":
		if az.Configured() {
			return az
		}
	case "volcano":
		if vo.Configured() {
			return vo
		}
	case "volcano_bigtts":
		if vb.Configured() {
			return vb
		}
	case "openai":
		if oa.Configured() {
			return oa
		}
	default:
		if vb.Configured() {
			return vb
		}
		if vo.Configured() {
			return vo
		}
		if az.Configured() {
			return az
		}
		if oa.Configured() {
			return oa
		}
	}
	return nil
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
