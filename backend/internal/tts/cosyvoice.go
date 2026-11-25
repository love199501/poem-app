package tts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
)

const CosyModel = "FunAudioLLM/CosyVoice2-0.5B"

// CosyClient 调用硅基流动 CosyVoice 的 /audio/speech，返回 mp3。
type CosyClient struct {
	BaseURL string
	APIKey  string
	Model   string
	Voice   string
	Speed   float64
	HTTP    *http.Client
}

func NewCosy(baseURL, apiKey, model, voice string, speed float64) *CosyClient {
	if model == "" {
		model = CosyModel
	}
	if speed <= 0 {
		speed = 0.9
	}
	return &CosyClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		Voice:   voice,
		Speed:   speed,
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *CosyClient) Configured() bool { return c.APIKey != "" && c.Voice != "" }

type cosyRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format"`
	Speed          float64 `json:"speed"`
	Stream         bool    `json:"stream"`
}

// Synthesize 整首一次朗读。每个句读后加换气，避免两句被连读后重新断句。
func (c *CosyClient) Synthesize(text, _ string) ([]byte, error) {
	spoken := speakText(text)
	if spoken == "" {
		return nil, fmt.Errorf("没有可朗诵的文本")
	}
	return c.speak(spoken)
}

func (c *CosyClient) speak(line string) ([]byte, error) {
	raw, _ := json.Marshal(cosyRequest{
		Model:          c.Model,
		Input:          line,
		Voice:          c.Voice,
		ResponseFormat: "mp3",
		Speed:          c.Speed,
		Stream:         false,
	})
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/audio/speech", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("CosyVoice 失败(%d): %s", resp.StatusCode, string(data))
	}
	if len(data) == 0 || data[0] == '{' {
		return nil, fmt.Errorf("CosyVoice 未返回音频: %s", string(data))
	}
	return data, nil
}

// CosyVoiceName 朗诵统一用 diana。传入已是「模型:音色」时只取音色名。
func CosyVoiceName(model, voice string) string {
	if model == "" {
		model = CosyModel
	}
	name := strings.TrimSpace(voice)
	if i := strings.LastIndex(name, ":"); i >= 0 {
		name = name[i+1:]
	}
	if name == "" || name == "onyx" || strings.HasPrefix(name, "m-") || strings.HasPrefix(name, "f-") {
		name = "diana"
	}
	return model + ":" + name
}

// CosySpeed 比正常说话稍慢。不再放到 0.85 以下，避免吞字。
func CosySpeed(int) float64 { return 0.92 }

func speakText(text string) string {
	flat := strings.TrimSpace(text)
	if flat == "" {
		return ""
	}
	var b strings.Builder
	var clause strings.Builder
	flush := func() {
		line := strings.TrimSpace(clause.String())
		clause.Reset()
		if line == "" {
			return
		}
		rs := []rune(line)
		if !unicode.IsPunct(rs[len(rs)-1]) {
			line += "。"
		}
		b.WriteString(line)
		b.WriteString("[breath]\n")
	}
	for _, r := range flat {
		if r == '\n' || r == '\r' {
			flush()
			continue
		}
		clause.WriteRune(r)
		if r == '，' || r == '。' || r == '？' || r == '！' || r == '；' {
			flush()
		}
	}
	flush()
	return strings.TrimSpace(b.String())
}
