package tts

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// VolcanoClient 调用字节火山引擎语音合成（HTTP 一次性合成），返回 mp3。
type VolcanoClient struct {
	AppID     string
	Token     string
	Cluster   string
	VoiceType string
	Emotion   string
	HTTP      *http.Client
}

func NewVolcano(appID, token, cluster, voiceType, emotion string) *VolcanoClient {
	if cluster == "" {
		cluster = "volcano_tts"
	}
	if voiceType == "" {
		voiceType = "BV001_streaming"
	}
	return &VolcanoClient{
		AppID:     appID,
		Token:     token,
		Cluster:   cluster,
		VoiceType: voiceType,
		Emotion:   emotion,
		HTTP:      &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *VolcanoClient) Configured() bool { return c.AppID != "" && c.Token != "" }

func reqID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type volcTTSRequest struct {
	App struct {
		AppID   string `json:"appid"`
		Token   string `json:"token"`
		Cluster string `json:"cluster"`
	} `json:"app"`
	User struct {
		UID string `json:"uid"`
	} `json:"user"`
	Audio struct {
		VoiceType     string  `json:"voice_type"`
		Encoding      string  `json:"encoding"`
		SpeedRatio    float64 `json:"speed_ratio"`
		Emotion       string  `json:"emotion,omitempty"`
		EnableEmotion bool    `json:"enable_emotion,omitempty"`
	} `json:"audio"`
	Request struct {
		ReqID     string `json:"reqid"`
		Text      string `json:"text"`
		TextType  string `json:"text_type"`
		Operation string `json:"operation"`
	} `json:"request"`
}

type volcTTSResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"` // base64 编码的音频
}

// Synthesize 合成 mp3。火山不吃自然语言 instruction，情感由音色 + emotion 字段决定。
func (c *VolcanoClient) Synthesize(text, _ string) ([]byte, error) {
	var r volcTTSRequest
	r.App.AppID = c.AppID
	r.App.Token = c.Token
	r.App.Cluster = c.Cluster
	r.User.UID = "shijing"
	r.Audio.VoiceType = c.VoiceType
	r.Audio.Encoding = "mp3"
	r.Audio.SpeedRatio = 0.9 // 略慢，更有朗诵韵味
	if c.Emotion != "" {
		r.Audio.Emotion = c.Emotion
		r.Audio.EnableEmotion = true
	}
	r.Request.ReqID = reqID()
	r.Request.Text = strings.ReplaceAll(text, "\n", "，") // 换行转停顿
	r.Request.TextType = "plain"
	r.Request.Operation = "query"

	raw, _ := json.Marshal(&r)
	req, err := http.NewRequest(http.MethodPost, "https://openspeech.bytedance.com/api/v1/tts", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	// 火山的特殊鉴权格式：Bearer;{token}
	req.Header.Set("Authorization", "Bearer;"+c.Token)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var vr volcTTSResponse
	if e := json.Unmarshal(body, &vr); e != nil {
		return nil, fmt.Errorf("火山 TTS 响应解析失败: %s", string(body))
	}
	if vr.Code != 3000 {
		return nil, fmt.Errorf("火山 TTS 失败(%d): %s", vr.Code, vr.Message)
	}
	audio, e := base64.StdEncoding.DecodeString(vr.Data)
	if e != nil || len(audio) == 0 {
		return nil, fmt.Errorf("火山 TTS 音频解码失败")
	}
	return audio, nil
}
