package handler

import "testing"

func TestPoemKeyIgnoresMarks(t *testing.T) {
	a := poemKey("《登高》", "杜甫")
	b := poemKey("登 高", "杜 甫")
	c := poemKey("登高", "李白")
	if a != b {
		t.Fatalf("书名号和空格应视为同一首: %q vs %q", a, b)
	}
	if a == c {
		t.Fatalf("不同诗人不应判重: %q", a)
	}
}
