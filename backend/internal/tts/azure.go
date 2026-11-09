package tts

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Synthesizer 统一的朗诵合成接口，OpenAI 兼容与 Azure 两种实现都满足。
type Synthesizer interface {
	Configured() bool
	Synthesize(text, instruction string) ([]byte, error)
}

// AzureClient 调用 Azure 认知服务语音合成，使用 mstts express-as 的 poetry-reading 风格。
type AzureClient struct {
	Key    string
	Region string
	Voice  string
	Style  string
	HTTP   *http.Client
}

func NewAzure(key, region, voice, style string) *AzureClient {
	if voice == "" {
		voice = "zh-CN-XiaoxiaoNeural"
	}
	if style == "" {
		style = "poetry-reading"
	}
	return &AzureClient{
		Key:    key,
		Region: region,
		Voice:  voice,
		Style:  style,
		HTTP:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *AzureClient) Configured() bool { return c.Key != "" && c.Region != "" }

// Synthesize 用 SSML + poetry-reading 风格合成 mp3。instruction 在 Azure 下不使用（情感由 style 决定）。
func (c *AzureClient) Synthesize(text, _ string) ([]byte, error) {
	ssml := c.buildSSML(text)
	url := fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/v1", c.Region)

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(ssml))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", c.Key)
	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("X-Microsoft-OutputFormat", "audio-24khz-48kbitrate-mono-mp3")
	req.Header.Set("User-Agent", "shijing")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Azure TTS 失败(%d): %s", resp.StatusCode, string(data))
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("Azure TTS 返回空音频")
	}
	return data, nil
}

// buildSSML 逐句之间插入停顿，整体略放慢，套用 poetry-reading 风格。
func (c *AzureClient) buildSSML(text string) string {
	lines := strings.Split(text, "\n")
	var body bytes.Buffer
	for i, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if i > 0 {
			body.WriteString(`<break time="600ms"/>`)
		}
		body.WriteString(xmlEscape(ln))
	}
	return `<speak version="1.0" xmlns="http://www.w3.org/2001/10/synthesis" ` +
		`xmlns:mstts="https://www.w3.org/2001/mstts" xml:lang="zh-CN">` +
		`<voice name="` + c.Voice + `">` +
		`<mstts:express-as style="` + c.Style + `" styledegree="2">` +
		`<prosody rate="-8%">` + body.String() + `</prosody>` +
		`</mstts:express-as></voice></speak>`
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}
