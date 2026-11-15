package wechat

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
)

func TestParsePrivateKeyFormats(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	_ = pem.Encode(&b, &pem.Block{Type: "PRIVATE KEY", Bytes: der})
	pemText := b.String()

	cases := []string{
		pemText,
		`"` + strings.ReplaceAll(pemText, "\n", `\n`) + `"`,
		strings.ReplaceAll(pemText, "\n", ""),
		base64.StdEncoding.EncodeToString(der),
	}
	for i, in := range cases {
		got, err := parsePrivateKey(in)
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if got.N.Cmp(key.N) != 0 {
			t.Fatalf("case %d: key mismatch", i)
		}
	}
}
