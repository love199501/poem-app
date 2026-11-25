package tts

import "testing"

func TestCosyVoiceName(t *testing.T) {
	if got := CosyVoiceName("", "m-grief"); got != CosyModel+":diana" {
		t.Fatalf("朗诵应固定为 diana: %s", got)
	}
	if got := CosyVoiceName(CosyModel, "diana"); got != CosyModel+":diana" {
		t.Fatalf("已指定 diana: %s", got)
	}
}

func TestCosySpeed(t *testing.T) {
	if got := CosySpeed(-25); got != 0.92 {
		t.Fatalf("语速应为 0.92，得到 %v", got)
	}
}

func TestSpeakTextPausesAtClauses(t *testing.T) {
	got := speakText("绿树村边合，青山郭外斜。\n开轩面场圃，把酒话桑麻。")
	want := "绿树村边合，[breath]\n青山郭外斜。[breath]\n开轩面场圃，[breath]\n把酒话桑麻。[breath]"
	if got != want {
		t.Fatalf("got %q", got)
	}
}
