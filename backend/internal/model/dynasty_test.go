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
