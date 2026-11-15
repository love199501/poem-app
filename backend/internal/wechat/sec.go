package wechat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
	"unicode/utf8"
)

// ErrContentUnsafe 文本未通过内容安全检查。对用户只展示这一句。
var ErrContentUnsafe = errors.New("所发布内容含违规信息")

const (
	SceneProfile = 1 // 资料
	SceneComment = 2 // 评论
	SceneForum   = 3 // 论坛
	SceneSocial  = 4 // 社交日志
)

// MsgSecCheck 调用微信 msgSecCheck 2.0。未通过返回 ErrContentUnsafe。
func MsgSecCheck(appID, secret, openID, content string, scene int) error {
	token, err := accessToken(appID, secret)
	if err != nil {
		return err
	}
	api := "https://api.weixin.qq.com/wxa/msg_sec_check?access_token=" + url.QueryEscape(token)
	return msgSecCheck(api, openID, content, scene)
}

// MsgSecCheckInCloud 在微信云托管内免密钥调用同一接口。
func MsgSecCheckInCloud(openID, content string, scene int) error {
	return msgSecCheck("http://api.weixin.qq.com/wxa/msg_sec_check", openID, content, scene)
}

func msgSecCheck(api, openID, content string, scene int) error {
	if scene < SceneProfile || scene > SceneSocial {
		scene = SceneComment
	}
	for _, chunk := range splitRunes(content, 2400) {
		if err := msgSecCheckOnce(api, openID, chunk, scene); err != nil {
			return err
		}
	}
	return nil
}

func msgSecCheckOnce(api, openID, content string, scene int) error {
	payload, _ := json.Marshal(map[string]any{
		"openid":  openID,
		"scene":   scene,
		"version": 2,
		"content": content,
	})
	req, err := http.NewRequest(http.MethodPost, api, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 8 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var r struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			Suggest string `json:"suggest"`
			Label   int    `json:"label"`
		} `json:"result"`
	}
	if e := json.Unmarshal(body, &r); e != nil {
		return fmt.Errorf("内容安全响应解析失败")
	}
	// 1.0 接口用 87014 表示违规；2.0 用 result.suggest。
	if r.ErrCode == 87014 || r.Result.Suggest == "risky" || r.Result.Suggest == "review" {
		return ErrContentUnsafe
	}
	if r.ErrCode != 0 {
		return fmt.Errorf("内容安全接口错误(%d)", r.ErrCode)
	}
	return nil
}

func splitRunes(s string, n int) []string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return []string{s}
	}
	var out []string
	buf := make([]rune, 0, n)
	for _, r := range s {
		buf = append(buf, r)
		if len(buf) == n {
			out = append(out, string(buf))
			buf = buf[:0]
		}
	}
	if len(buf) > 0 {
		out = append(out, string(buf))
	}
	return out
}
