package wechat

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Jscode2SessionResp 微信 code2session 返回。
type Jscode2SessionResp struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// Jscode2Session 用 wx.login 的 code 换取 openid。
func Jscode2Session(appID, secret, code string) (*Jscode2SessionResp, error) {
	api := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		url.QueryEscape(appID), url.QueryEscape(secret), url.QueryEscape(code),
	)
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Get(api)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var r Jscode2SessionResp
	if e := json.Unmarshal(body, &r); e != nil {
		return nil, fmt.Errorf("微信登录响应解析失败: %s", string(body))
	}
	if r.ErrCode != 0 {
		return nil, fmt.Errorf("微信登录失败(%d): %s", r.ErrCode, r.ErrMsg)
	}
	if r.OpenID == "" {
		return nil, fmt.Errorf("微信未返回 openid")
	}
	return &r, nil
}
