package wechat

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const vpModeGoods = "short_series_goods"

// VirtualPay 拉起小程序虚拟支付所需参数。signData 必须原样传给客户端。
type VirtualPay struct {
	Mode      string `json:"mode"`
	SignData  string `json:"signData"`
	PaySig    string `json:"paySig"`
	Signature string `json:"signature"`
}

// BuildVirtualPay 按道具直购拼签名。goodsPrice 单位为分，须与虚拟支付后台的道具价格一致。
func BuildVirtualPay(offerID, appKey, sessionKey, productID, outTradeNo, attach string, env, goodsPrice int) (*VirtualPay, error) {
	if offerID == "" || appKey == "" || productID == "" {
		return nil, fmt.Errorf("请配置虚拟支付 OfferID、AppKey 和道具 ID")
	}
	if sessionKey == "" {
		return nil, fmt.Errorf("请重新登录后再充值")
	}
	signData, err := marshalVP(map[string]any{
		"offerId":      offerID,
		"buyQuantity":  1,
		"env":          env,
		"currencyType": "CNY",
		"productId":    productID,
		"goodsPrice":   goodsPrice,
		"outTradeNo":   outTradeNo,
		"attach":       attach,
	}, []string{"offerId", "buyQuantity", "env", "currencyType", "productId", "goodsPrice", "outTradeNo", "attach"})
	if err != nil {
		return nil, err
	}
	return &VirtualPay{
		Mode:      vpModeGoods,
		SignData:  signData,
		PaySig:    paySig("requestVirtualPayment", signData, appKey),
		Signature: userSig(signData, sessionKey),
	}, nil
}

// VirtualOrder 查询现金订单。status：2 已支付待发货，3 发货中，4 已发货。
type VirtualOrder struct {
	Status    int
	OrderFee  int
	WxOrderID string
}

func QueryVirtualOrder(appID, secret, appKey, openID, orderID string, env int) (*VirtualOrder, error) {
	body, err := marshalVP(map[string]any{
		"openid":   openID,
		"env":      env,
		"order_id": orderID,
	}, []string{"openid", "env", "order_id"})
	if err != nil {
		return nil, err
	}
	raw, err := postXPay(appID, secret, appKey, "/xpay/query_order", body, true)
	if err != nil {
		return nil, err
	}
	var resp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Order   struct {
			Status    int    `json:"status"`
			OrderFee  int    `json:"order_fee"`
			WxOrderID string `json:"wx_order_id"`
		} `json:"order"`
	}
	if e := json.Unmarshal(raw, &resp); e != nil {
		return nil, fmt.Errorf("虚拟支付查单响应无法解析")
	}
	if resp.ErrCode != 0 {
		if resp.ErrMsg == "" {
			resp.ErrMsg = "查单失败"
		}
		return nil, fmt.Errorf("%s", resp.ErrMsg)
	}
	return &VirtualOrder{Status: resp.Order.Status, OrderFee: resp.Order.OrderFee, WxOrderID: resp.Order.WxOrderID}, nil
}

// NotifyProvided 通知虚拟支付道具已发货。
func NotifyProvided(appID, secret, orderID string, env int) error {
	body, err := marshalVP(map[string]any{
		"order_id": orderID,
		"env":      env,
	}, []string{"order_id", "env"})
	if err != nil {
		return err
	}
	raw, err := postXPay(appID, secret, "", "/xpay/notify_provide_goods", body, false)
	if err != nil {
		return err
	}
	var resp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if json.Unmarshal(raw, &resp) == nil && resp.ErrCode != 0 && resp.ErrCode != 268490004 {
		if resp.ErrMsg == "" {
			resp.ErrMsg = "发货通知失败"
		}
		return fmt.Errorf("%s", resp.ErrMsg)
	}
	return nil
}

func postXPay(appID, secret, appKey, uri, body string, withPaySig bool) ([]byte, error) {
	token, err := accessToken(appID, secret)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("access_token", token)
	if withPaySig {
		q.Set("pay_sig", paySig(uri, body, appKey))
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.weixin.qq.com"+uri+"?"+q.Encode(), bytes.NewReader([]byte(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 15 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("暂时无法连接虚拟支付")
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if len(out) == 0 {
		return nil, fmt.Errorf("虚拟支付没有返回")
	}
	return out, nil
}

func paySig(uri, body, appKey string) string {
	mac := hmac.New(sha256.New, []byte(appKey))
	_, _ = mac.Write([]byte(uri + "&" + body))
	return hex.EncodeToString(mac.Sum(nil))
}

func userSig(body, sessionKey string) string {
	mac := hmac.New(sha256.New, []byte(sessionKey))
	_, _ = mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func marshalVP(fields map[string]any, order []string) (string, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range order {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(key)
		if err != nil {
			return "", err
		}
		vb, err := json.Marshal(fields[key])
		if err != nil {
			return "", err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.String(), nil
}
