package model

import "testing"

func TestNormalizeDynasty(t *testing.T) {
	if got := NormalizeDynasty(" 唐代 "); got != "唐" {
		t.Fatalf("唐代 => %q", got)
	}
	if got := NormalizeDynasty("唐朝"); got != "唐" {
		t.Fatalf("唐朝 => %q", got)
	}
	if got := NormalizeDynasty("五代"); got != "五代" {
		t.Fatalf("五代应保持原样, got %q", got)
	}
	if got := NormalizeDynasty("近现代"); got != "近现代" {
		t.Fatalf("近现代应保持原样, got %q", got)
	}
	if got := NormalizeDynasty("现代"); got != "近现代" {
		t.Fatalf("现代 => %q", got)
	}
	if got := NormalizeDynasty("民国"); got != "近现代" {
		t.Fatalf("民国 => %q", got)
	}
	if got := NormalizeDynasty("现"); got != "近现代" {
		t.Fatalf("现 => %q", got)
	}
	if got := NormalizeDynasty("战国"); got != "先秦" {
		t.Fatalf("战国 => %q", got)
	}
	if got := NormalizeDynasty("南朝宋"); got != "南北朝" {
		t.Fatalf("南朝宋 => %q", got)
	}
	if got := NormalizeDynasty("宋"); got != "宋" {
		t.Fatalf("宋应保持原样, got %q", got)
	}
	if got := NormalizeDynasty("南唐"); got != "五代" {
		t.Fatalf("南唐 => %q", got)
	}
	if got := NormalizeDynasty("魏"); got != "魏晋" {
		t.Fatalf("魏 => %q", got)
	}
}

func TestDynastyVariants(t *testing.T) {
	got := DynastyVariants("唐代")
	want := map[string]bool{"唐": true, "唐代": true, "唐朝": true}
	if len(got) != len(want) {
		t.Fatalf("variants %v", got)
	}
	for _, v := range got {
		if !want[v] {
			t.Fatalf("unexpected %q in %v", v, got)
		}
	}
}
