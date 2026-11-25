package wechat

import "testing"

func TestVirtualPaySignatureMatchesDoc(t *testing.T) {
	body := `{"openid": "xxx", "user_ip": "127.0.0.1", "env": 0}`
	if got := paySig("/xpay/query_user_balance", body, "12345"); got != "c37809f27c6d7fd1837ad2500a04512b66b34fd793a39a385fade56dca89a4b5" {
		t.Fatalf("paySig=%s", got)
	}
	if got := userSig(body, "9hAb/NEYUlkaMBEsmFgzig=="); got != "089d9e8dc5d308977360c4b79ec600a93d736802802a807d634192328032f6c7" {
		t.Fatalf("signature=%s", got)
	}
}
