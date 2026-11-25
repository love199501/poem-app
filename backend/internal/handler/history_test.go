package handler

import "testing"

func TestSongPeriodsFollowAppearance(t *testing.T) {
	cases := []struct {
		year  int
		birth bool
		want  string
	}{
		{967, true, "北宋前期"},
		{1037, true, "北宋中叶"},
		{1084, true, "南渡之际"},
		{1125, true, "南宋"},
		{1000, false, "北宋前期"},
		{1080, false, "北宋中叶"},
		{1130, false, "南渡之际"},
		{1170, false, "南宋"},
	}
	for _, c := range cases {
		got, _ := periodOf("宋", c.year, c.birth)
		if got != c.want {
			t.Fatalf("宋 %d birth=%v 得到 %s，期望 %s", c.year, c.birth, got, c.want)
		}
	}
}

func TestTangBirthLandsInShengTang(t *testing.T) {
	if got, _ := periodOf("唐", 701, true); got != "盛唐" {
		t.Fatalf("李白生年应落在盛唐，得到 %s", got)
	}
	if got, _ := periodOf("唐", 757, false); got != "盛唐" {
		t.Fatalf("757 年的作品应落在盛唐，得到 %s", got)
	}
}
