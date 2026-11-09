package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"shijing/internal/model"
)

// Client 调用 OpenAI 兼容的 Chat Completions 接口生成诗词结构化内容。
type Client struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

func New(baseURL, apiKey, model string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		HTTP:    &http.Client{Timeout: 90 * time.Second},
	}
}

const systemPrompt = `你是资深中国古典诗词编辑。根据给定「诗名」「诗人」，输出该诗的结构化赏析数据。
严格只输出一个 JSON 对象，不要任何多余文字或 Markdown 代码块。字段如下（缺失可给空数组/空串）：
{
 "title":"", "dynasty":"朝代简称，如唐、宋、明，不要写成唐代、宋代", "author":"", "form":"体裁如五言律诗", "yearLabel":"如 公元757年",
 "firstVerse":"最具代表性的完整一联（须含上下句，如「感时花溅泪，恨别鸟惊心。」，禁止只给半句）", "tags":["主题标签"], "moods":["忧国/思乡/旷达/孤独/喜悦 之一或多"],
 "audioUrl":"",
 "lines":[{"text":"逐句原文(一句一项)","noteBody":"字词注释，可用<b>词</b>强调","noteSrc":"注释出处"}],
 "translation":["逐句白话译文"], "transSource":"译文参考",
 "sceneTitle":"现场档案标题", "sceneSub":"副标题", "sceneRows":[{"icon":"📅","text":"时间/地点/处境，每条一行事实"}],
 "sceneBackground":"180到280字创作背景。写清年份与地点、作者当时的年龄与身份、这件事发生前他经历了什么、身边还有谁、落笔时他最担心或最渴望的是什么。用现代白话，让没读过史书的人也能感到他当时的状态。史实准确，不确定就写「相传」或省略，不要空泛抒情，不要逐句翻译原诗",
 "scenePunch":"一句点睛，收在他当时的状态上",
 "lifeCurve":[{"age":"如46岁","title":"该阶段代表作/事件","level":0到100,"highlight":true当为本诗}],
 "intentLead":"他想说什么(一句)", "intentSub":"补充",
 "audiences":[{"role":"自己/家人/后人","text":"写给这个对象的用意"}],
 "void":"他没写出来却暗含的东西",
 "layers":[{"key":"face","title":"字面义","body":""},{"key":"heart","title":"诗人心","body":""},{"key":"era","title":"时代音","body":""}],
 "quotes":[{"text":"名家诗话原文","source":"出处"}],
 "duels":[{"side":"a","title":"观点A·xx","body":""},{"side":"b","title":"观点B·xx","body":""}],
 "aiPrompts":["可供追问古人的问题"]
}
要求：史实准确；注释、译文可靠；对不确定内容宁缺毋滥；全部用简体中文。`

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string            `json:"model"`
	Messages       []chatMessage     `json:"messages"`
	Temperature    float64           `json:"temperature"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Configured 报告是否已配置可用的大模型 Key。
func (c *Client) Configured() bool { return c.APIKey != "" }

// Generate 根据诗名+诗人生成结构化诗词内容。未配置 Key 时返回本地 mock。
func (c *Client) Generate(title, author string) (*model.Poem, error) {
	if !c.Configured() {
		return mockPoem(title, author), nil
	}

	reqBody := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("诗名：%s\n诗人：%s", title, author)},
		},
		Temperature:    0.6,
		ResponseFormat: map[string]string{"type": "json_object"},
	}
	raw, _ := json.Marshal(reqBody)

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(raw))
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

	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, fmt.Errorf("解析大模型响应失败: %v", err)
	}
	if cr.Error != nil {
		return nil, errors.New("大模型返回错误: " + cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return nil, errors.New("大模型未返回内容")
	}

	content := extractJSON(cr.Choices[0].Message.Content)
	var poem model.Poem
	if err := json.Unmarshal([]byte(content), &poem); err != nil {
		return nil, fmt.Errorf("大模型输出不是合法 JSON: %v", err)
	}
	// 以管理员输入为准，避免模型跑偏。
	poem.Title = title
	poem.Author = author
	poem.FirstVerse = model.CompleteDisplayVerse(poem.FirstVerse, poem.Lines)
	poem.Dynasty = model.NormalizeDynasty(poem.Dynasty)
	return &poem, nil
}

// extractJSON 容错：去掉可能的 ```json 包裹，截取第一个 { 到最后一个 }。
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}

const findImageSystem = `你是古诗配图检索助手。给定「诗名」「诗人」，给出一张与该诗意境最匹配、
且【真实存在、可直接访问、公有领域、无水印】的图片直链，优先 Wikimedia Commons
（upload.wikimedia.org）等公有领域古画/国画/山水画扫描件。严格只输出 JSON：{"imageUrl":"https://...jpg"}。
若没有把握找到真实可用且无水印的图片，imageUrl 返回空字符串 ""。绝对不要编造链接。`

const querySystem = `你是图片检索助手。给定古诗与作者，输出一个用于「百度图片」搜索该诗配图的简短中文查询词，
包含诗名或代表意象 + 作者 + 「国画 水墨 高清」等限定，便于搜到契合的国画/古画。严格只输出 JSON：{"query":"..."}。`

// ImageQuery 让大模型为该诗生成一个百度图片检索词。
func (c *Client) ImageQuery(title, author string) (string, error) {
	if !c.Configured() {
		return "", nil
	}
	content, err := c.complete(querySystem, fmt.Sprintf("诗名：%s\n诗人：%s", title, author), 0.3)
	if err != nil {
		return "", err
	}
	var r struct {
		Query string `json:"query"`
	}
	if json.Unmarshal([]byte(content), &r) != nil {
		return "", nil
	}
	return strings.TrimSpace(r.Query), nil
}

// FindImage 让大模型检索一张与古诗相符、公有领域无水印的现成插画直链；没有则返回空串。
func (c *Client) FindImage(title, author string) (string, error) {
	if !c.Configured() {
		return "", nil
	}
	content, err := c.complete(findImageSystem, fmt.Sprintf("诗名：%s\n诗人：%s", title, author), 0.2)
	if err != nil {
		return "", err
	}
	var r struct {
		ImageURL string `json:"imageUrl"`
	}
	if json.Unmarshal([]byte(content), &r) != nil {
		return "", nil
	}
	return strings.TrimSpace(r.ImageURL), nil
}

// complete 发一轮 chat（要求 JSON 输出），返回提取后的 JSON 文本。
func (c *Client) complete(system, user string, temp float64) (string, error) {
	reqBody := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature:    temp,
		ResponseFormat: map[string]string{"type": "json_object"},
	}
	raw, _ := json.Marshal(reqBody)
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", fmt.Errorf("解析大模型响应失败: %v", err)
	}
	if cr.Error != nil {
		return "", errors.New("大模型返回错误: " + cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", errors.New("大模型未返回内容")
	}
	return extractJSON(cr.Choices[0].Message.Content), nil
}
