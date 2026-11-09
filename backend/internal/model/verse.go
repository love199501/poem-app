package model

import (
	"strings"
	"unicode"
)

const versePunct = "，、；。！？,.!?;:："

// CompleteDisplayVerse 保证卡片/列表展示的是完整一句（五七言则补成一联），避免「此地别燕丹」这种半句。
func CompleteDisplayVerse(verse string, lines []PoemLine) string {
	v := strings.TrimSpace(verse)
	if displayVerseComplete(v) {
		return ensureVerseStop(v)
	}
	idx := findVerseLine(v, lines)
	if idx < 0 {
		if len(lines) == 0 {
			return v
		}
		idx = 0
	}
	cur := strings.TrimSpace(lines[idx].Text)
	if displayVerseComplete(cur) {
		return ensureVerseStop(cur)
	}
	if idx+1 < len(lines) {
		return joinCouplet(cur, strings.TrimSpace(lines[idx+1].Text))
	}
	return ensureVerseStop(cur)
}

func displayVerseComplete(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	body := strings.TrimRight(s, versePunct+" ")
	if strings.ContainsAny(body, "，、；。！？,;") {
		return true
	}
	// 五言联 10 字、七言联 14 字；≥9 视为已是完整一联/词句。
	return hanCount(body) >= 9
}

func hanCount(s string) int {
	n := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			n++
		}
	}
	return n
}

func findVerseLine(v string, lines []PoemLine) int {
	key := stripVersePunct(v)
	if key == "" {
		return -1
	}
	for i, l := range lines {
		t := stripVersePunct(l.Text)
		if t == key || strings.HasPrefix(t, key) || strings.HasPrefix(key, t) {
			return i
		}
	}
	return -1
}

func stripVersePunct(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(versePunct+" \n\t「」“”\"'", r) {
			return -1
		}
		return r
	}, s)
}

func joinCouplet(a, b string) string {
	a = strings.TrimRight(a, versePunct+" ")
	b = strings.TrimRight(b, versePunct+" ")
	if a == "" {
		return ensureVerseStop(b)
	}
	if b == "" {
		return ensureVerseStop(a)
	}
	return a + "，" + b + "。"
}

func ensureVerseStop(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	rs := []rune(s)
	if strings.ContainsRune(versePunct, rs[len(rs)-1]) {
		return s
	}
	return s + "。"
}
