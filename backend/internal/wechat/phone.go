package wechat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// GetPhoneNumber 用按钮 open-type="getPhoneNumber" 返回的 code 换取手机号。
func GetPhoneNumber(appID, secret, code string) (string, error) {
	token, err := accessToken(appID, secret)
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(map[string]string{"code": code})
	api := "https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token=" + url.QueryEscape(token)
	req, err := http.NewRequest(http.MethodPost, api, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	return doPhoneRequest(req)
}

// GetPhoneNumberInCloud 在微信云托管内免密钥换手机号。
// 容器里访问 http://api.weixin.qq.com 时，平台会自动带上调用凭证。
func GetPhoneNumberInCloud(code string) (string, error) {
	payload, _ := json.Marshal(map[string]string{"code": code})
	req, err := http.NewRequest(http.MethodPost, "http://api.weixin.qq.com/wxa/business/getuserphonenumber", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	return doPhoneRequest(req)
}

func doPhoneRequest(req *http.Request) (string, error) {
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var r struct {
		ErrCode   int    `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		PhoneInfo struct {
			PhoneNumber     string `json:"phoneNumber"`
			PurePhoneNumber string `json:"purePhoneNumber"`
		} `json:"phone_info"`
	}
	if e := json.Unmarshal(body, &r); e != nil {
		return "", fmt.Errorf("微信手机号响应解析失败: %s", string(body))
	}
	if r.ErrCode != 0 {
		return "", fmt.Errorf("获取手机号失败(%d): %s", r.ErrCode, r.ErrMsg)
	}
	phone := r.PhoneInfo.PurePhoneNumber
	if phone == "" {
		phone = r.PhoneInfo.PhoneNumber
	}
	if phone == "" {
		return "", fmt.Errorf("微信未返回手机号")
	}
	return phone, nil
}

type tokenEntry struct {
	token string
	until time.Time
}

var (
	tokenMu    sync.Mutex
	tokenCache = map[string]tokenEntry{}
)

func accessToken(appID, secret string) (string, error) {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	if e, ok := tokenCache[appID]; ok && time.Now().Before(e.until) && e.token != "" {
		return e.token, nil
	}
	api := fmt.Sprintf(
		"https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		url.QueryEscape(appID), url.QueryEscape(secret),
	)
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Get(api)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if e := json.Unmarshal(body, &r); e != nil {
		return "", fmt.Errorf("微信令牌响应解析失败: %s", string(body))
	}
	if r.ErrCode != 0 || r.AccessToken == "" {
		return "", fmt.Errorf("获取微信令牌失败(%d): %s", r.ErrCode, r.ErrMsg)
	}
	ttl := r.ExpiresIn - 120
	if ttl < 60 {
		ttl = 60
	}
	tokenCache[appID] = tokenEntry{token: r.AccessToken, until: time.Now().Add(time.Duration(ttl) * time.Second)}
	return r.AccessToken, nil
}
