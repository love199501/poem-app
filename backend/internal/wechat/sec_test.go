package wechat

import "testing"

func TestSplitRunes(t *testing.T) {
	got := splitRunes("你好世界", 2)
	if len(got) != 2 || got[0] != "你好" || got[1] != "世界" {
		t.Fatalf("got %#v", got)
	}
	if splitRunes("诗", 10)[0] != "诗" {
		t.Fatal("short text")
	}
}
