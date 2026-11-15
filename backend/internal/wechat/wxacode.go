package wechat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// WxaCodeUnlimited 生成小程序码，扫码打开指定页面。scene 最长 32 个字符。
// envVersion 取 develop、trial、release，对应开发版、体验版、正式版。
func WxaCodeUnlimited(appID, secret, scene, page, envVersion string) ([]byte, error) {
	token, err := accessToken(appID, secret)
	if err != nil {
		return nil, err
	}
	api := "https://api.weixin.qq.com/wxa/getwxacodeunlimit?access_token=" + url.QueryEscape(token)
	return postWxaCode(api, scene, page, envVersion)
}

// WxaCodeUnlimitedInCloud 在微信云托管内免密钥生成小程序码。
func WxaCodeUnlimitedInCloud(scene, page, envVersion string) ([]byte, error) {
	return postWxaCode("http://api.weixin.qq.com/wxa/getwxacodeunlimit", scene, page, envVersion)
}

func postWxaCode(api, scene, page, envVersion string) ([]byte, error) {
	if envVersion != "develop" && envVersion != "trial" && envVersion != "release" {
		envVersion = "release"
	}
	payload, _ := json.Marshal(map[string]any{
		"scene":       scene,
		"page":        page,
		"width":       280,
		"check_path":  false,
		"env_version": envVersion,
	})
	req, err := http.NewRequest(http.MethodPost, api, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 15 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("小程序码为空")
	}
	if body[0] == '{' {
		var r struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		_ = json.Unmarshal(body, &r)
		if r.ErrMsg == "" {
			r.ErrMsg = "未知错误"
		}
		return nil, fmt.Errorf("小程序码失败(%d): %s", r.ErrCode, r.ErrMsg)
	}
	return body, nil
}
