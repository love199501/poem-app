package img

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client 调用 OpenAI 兼容的 /images/generations 生成插画。
type Client struct {
	BaseURL string
	APIKey  string
	Model   string
	Size    string
	HTTP    *http.Client
}

func New(baseURL, apiKey, model, size string) *Client {
	if size == "" {
		size = "1024x1024"
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		Size:    size,
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *Client) Configured() bool { return c.APIKey != "" }

type imgResponse struct {
	Data []struct {
		URL     string `json:"url"`
		B64JSON string `json:"b64_json"`
	} `json:"data"`
	Images []struct {
		URL string `json:"url"`
	} `json:"images"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Generate 生成图片，返回图片字节（png/jpg）。
// 兼容两类参数：OpenAI(size/n/response_format) 与 硅基流动等(image_size/batch_size)。
func (c *Client) Generate(prompt string) ([]byte, error) {
	var payload map[string]any
	if strings.Contains(c.BaseURL, "openai.com") || strings.Contains(c.BaseURL, "azure") {
		payload = map[string]any{"model": c.Model, "prompt": prompt, "size": c.Size, "n": 1, "response_format": "b64_json"}
	} else {
		// 硅基流动 / Kolors 等：image_size + batch_size，返回 images[].url
		payload = map[string]any{"model": c.Model, "prompt": prompt, "image_size": c.Size, "batch_size": 1}
	}
	raw, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/images/generations", bytes.NewReader(raw))
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
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("文生图失败(%d): %s", resp.StatusCode, string(body))
	}

	var r imgResponse
	if e := json.Unmarshal(body, &r); e != nil {
		return nil, fmt.Errorf("文生图响应解析失败: %s", string(body))
	}
	if r.Error != nil {
		return nil, fmt.Errorf("文生图错误: %s", r.Error.Message)
	}
	if len(r.Data) > 0 {
		if r.Data[0].B64JSON != "" {
			return base64.StdEncoding.DecodeString(r.Data[0].B64JSON)
		}
		if r.Data[0].URL != "" {
			return c.fetch(r.Data[0].URL)
		}
	}
	if len(r.Images) > 0 && r.Images[0].URL != "" {
		return c.fetch(r.Images[0].URL)
	}
	return nil, fmt.Errorf("文生图未返回图片")
}

func (c *Client) fetch(url string) ([]byte, error) {
	resp, err := c.HTTP.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
