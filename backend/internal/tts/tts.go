package tts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client 调用 OpenAI 兼容的 /audio/speech 做情感语音合成（朗诵）。
type Client struct {
	BaseURL string
	APIKey  string
	Model   string
	Voice   string
	HTTP    *http.Client
}

func New(baseURL, apiKey, model, voice string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		Voice:   voice,
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *Client) Configured() bool { return c.APIKey != "" }

type speechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format"`
	Speed          float64 `json:"speed"`
	Instructions   string  `json:"instructions,omitempty"` // OpenAI gpt-4o-mini-tts 支持：控制语气/情感
}

// Synthesize 合成为 mp3 字节。instructions 用于指定朗诵情感（兼容服务会忽略未知字段）。
func (c *Client) Synthesize(text, instructions string) ([]byte, error) {
	reqBody := speechRequest{
		Model:          c.Model,
		Input:          text,
		Voice:          c.Voice,
		ResponseFormat: "mp3",
		Speed:          0.92, // 略慢，更有朗诵的抑扬与停顿
		Instructions:   instructions,
	}
	raw, _ := json.Marshal(reqBody)

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
		return nil, fmt.Errorf("TTS 失败(%d): %s", resp.StatusCode, string(data))
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("TTS 返回空音频")
	}
	return data, nil
}
