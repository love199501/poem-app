package wechat

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// PayParams 小程序 wx.requestPayment 需要的字段。
type PayParams struct {
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
}

// Merchant 微信支付 API v3 商户。云托管代支付不支持「在线销售」类目，会员充值走这里。
type Merchant struct {
	AppID      string
	MchID      string
	Serial     string
	APIv3Key   string
	NotifyURL  string
	PrivateKey *rsa.PrivateKey
	HTTP       *http.Client
}

// NewMerchant 校验证书材料。缺任何一项都返回给调用方看的说明。
func NewMerchant(appID, mchID, serial, apiV3Key, privateKeyPEM, notifyURL string) (*Merchant, error) {
	switch {
	case strings.TrimSpace(mchID) == "":
		return nil, fmt.Errorf("未读取到商户号，请配置 WX_MCH_ID")
	case strings.TrimSpace(appID) == "":
		return nil, fmt.Errorf("未配置小程序 AppID")
	case strings.TrimSpace(serial) == "":
		return nil, fmt.Errorf("请配置商户证书序列号 WX_MCH_CERT_SERIAL")
	case strings.TrimSpace(apiV3Key) == "":
		return nil, fmt.Errorf("请配置微信支付 APIv3 密钥 WX_API_V3_KEY")
	case strings.TrimSpace(privateKeyPEM) == "":
		return nil, fmt.Errorf("请配置商户私钥 WX_MCH_PRIVATE_KEY")
	case strings.TrimSpace(notifyURL) == "":
		return nil, fmt.Errorf("请配置支付回调地址 WX_PAY_NOTIFY_URL")
	}
	key, err := parsePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, err
	}
	if len(apiV3Key) != 32 {
		return nil, fmt.Errorf("APIv3 密钥需要 32 位")
	}
	return &Merchant{
		AppID: appID, MchID: mchID, Serial: serial, APIv3Key: apiV3Key,
		NotifyURL: notifyURL, PrivateKey: key,
		HTTP: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// Jsapi 小程序下单，返回调起支付的参数。金额单位为分。
func (m *Merchant) Jsapi(openid, orderNo, description string, total int) (*PayParams, error) {
	payload := map[string]any{
		"appid":        m.AppID,
		"mchid":        m.MchID,
		"description":  description,
		"out_trade_no": orderNo,
		"notify_url":   m.NotifyURL,
		"amount":       map[string]any{"total": total, "currency": "CNY"},
		"payer":        map[string]string{"openid": openid},
	}
	raw, _ := json.Marshal(payload)
	const path = "/v3/pay/transactions/jsapi"
	body, err := m.call(http.MethodPost, path, raw)
	if err != nil {
		return nil, err
	}
	var resp struct {
		PrepayID string `json:"prepay_id"`
	}
	if e := json.Unmarshal(body, &resp); e != nil || resp.PrepayID == "" {
		return nil, fmt.Errorf("微信支付未返回预支付单")
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := nonceStr()
	if err != nil {
		return nil, err
	}
	pkg := "prepay_id=" + resp.PrepayID
	sign, err := m.sign(m.AppID + "\n" + ts + "\n" + nonce + "\n" + pkg + "\n")
	if err != nil {
		return nil, err
	}
	return &PayParams{TimeStamp: ts, NonceStr: nonce, Package: pkg, SignType: "RSA", PaySign: sign}, nil
}

// Paid 查询订单是否支付成功，并核对金额（分）。返回微信订单号。
func (m *Merchant) Paid(orderNo string, expectFee int) (bool, string, error) {
	path := "/v3/pay/transactions/out-trade-no/" + url.PathEscape(orderNo) + "?mchid=" + url.QueryEscape(m.MchID)
	body, err := m.call(http.MethodGet, path, nil)
	if err != nil {
		return false, "", err
	}
	var resp struct {
		TradeState    string `json:"trade_state"`
		TransactionID string `json:"transaction_id"`
		Amount        struct {
			Total int `json:"total"`
		} `json:"amount"`
	}
	if e := json.Unmarshal(body, &resp); e != nil {
		return false, "", fmt.Errorf("支付查询响应无法解析")
	}
	if resp.TradeState == "SUCCESS" && resp.Amount.Total == expectFee {
		return true, resp.TransactionID, nil
	}
	return false, resp.TransactionID, nil
}

// DecryptResource 解密支付结果通知里的 resource。
func DecryptResource(apiV3Key, nonce, associated, ciphertext string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher([]byte(apiV3Key))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, []byte(nonce), raw, []byte(associated))
}

func (m *Merchant) call(method, path string, body []byte) ([]byte, error) {
	auth, err := m.authorization(method, path, body)
	if err != nil {
		return nil, err
	}
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, "https://api.mch.weixin.qq.com"+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "shijing")
	resp, err := m.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("暂时无法连接微信支付")
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return out, nil
	}
	var er struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(out, &er)
	if er.Message == "" {
		er.Message = "支付下单失败"
	}
	return nil, fmt.Errorf("%s", er.Message)
}

func (m *Merchant) authorization(method, path string, body []byte) (string, error) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := nonceStr()
	if err != nil {
		return "", err
	}
	msg := method + "\n" + path + "\n" + ts + "\n" + nonce + "\n" + string(body) + "\n"
	sig, err := m.sign(msg)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		m.MchID, nonce, sig, ts, m.Serial,
	), nil
}

func (m *Merchant) sign(msg string) (string, error) {
	sum := sha256.Sum256([]byte(msg))
	sig, err := rsa.SignPKCS1v15(rand.Reader, m.PrivateKey, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func nonceStr() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

func parsePrivateKey(text string) (*rsa.PrivateKey, error) {
	text = normalizeKeyText(text)
	if text == "" {
		return nil, fmt.Errorf("请配置商户私钥 WX_MCH_PRIVATE_KEY")
	}
	if !strings.Contains(text, "BEGIN") {
		if b, err := os.ReadFile(text); err == nil {
			text = normalizeKeyText(string(b))
		}
	}
	if !strings.Contains(text, "BEGIN") {
		compact := keepBase64(text)
		if raw, err := base64.StdEncoding.DecodeString(compact); err == nil && bytes.Contains(raw, []byte("BEGIN")) {
			text = normalizeKeyText(string(raw))
		} else if key := parseDER(raw); err == nil && key != nil {
			return key, nil
		} else {
			text = wrapPEM("PRIVATE KEY", compact)
		}
	}
	text = rebuildPEM(text)
	block, _ := pem.Decode([]byte(text))
	if block == nil {
		return nil, fmt.Errorf("商户私钥不完整，请粘贴 apiclient_key.pem 全文")
	}
	if block.Type == "CERTIFICATE" || block.Type == "PUBLIC KEY" {
		return nil, fmt.Errorf("请填写 apiclient_key.pem 私钥，不要填证书")
	}
	if key := parseDER(block.Bytes); key != nil {
		return key, nil
	}
	return nil, fmt.Errorf("商户私钥格式不正确")
}

func normalizeKeyText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\uFEFF")
	if len(s) >= 2 {
		q := s[0]
		if (q == '"' || q == '\'') && s[len(s)-1] == q {
			s = s[1 : len(s)-1]
		}
	}
	for i := 0; i < 4; i++ {
		next := strings.ReplaceAll(s, `\r\n`, "\n")
		next = strings.ReplaceAll(next, `\n`, "\n")
		next = strings.ReplaceAll(next, `\r`, "")
		next = strings.ReplaceAll(next, "\\\n", "\n")
		if next == s {
			break
		}
		s = next
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}

func rebuildPEM(s string) string {
	begin := strings.Index(s, "-----BEGIN")
	end := strings.Index(s, "-----END")
	if begin < 0 || end < 0 || end < begin {
		return s
	}
	rest := s[begin+5:]
	mark := strings.Index(rest, "-----")
	if mark < 0 {
		return s
	}
	header := strings.TrimSpace(s[begin : begin+5+mark+5])
	body := keepBase64(s[begin+5+mark+5 : end])
	endLine := s[end:]
	if nl := strings.Index(endLine, "\n"); nl >= 0 {
		endLine = endLine[:nl]
	}
	var b strings.Builder
	b.WriteString(header)
	b.WriteByte('\n')
	for len(body) > 64 {
		b.WriteString(body[:64])
		b.WriteByte('\n')
		body = body[64:]
	}
	b.WriteString(body)
	b.WriteByte('\n')
	b.WriteString(strings.TrimSpace(endLine))
	b.WriteByte('\n')
	return b.String()
}

func wrapPEM(kind, body string) string {
	var b strings.Builder
	b.WriteString("-----BEGIN ")
	b.WriteString(kind)
	b.WriteString("-----\n")
	for len(body) > 64 {
		b.WriteString(body[:64])
		b.WriteByte('\n')
		body = body[64:]
	}
	b.WriteString(body)
	b.WriteString("\n-----END ")
	b.WriteString(kind)
	b.WriteString("-----\n")
	return b.String()
}

func keepBase64(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '+', r == '/', r == '=':
			return r
		default:
			return -1
		}
	}, s)
}

func parseDER(der []byte) *rsa.PrivateKey {
	if len(der) == 0 {
		return nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey
		}
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key
	}
	return nil
}
