package model

import "testing"

func TestCompleteDisplayVerse(t *testing.T) {
	yiShui := []PoemLine{
		{Seq: 0, Text: "此地别燕丹"},
		{Seq: 1, Text: "壮士发冲冠"},
		{Seq: 2, Text: "昔时人已没"},
		{Seq: 3, Text: "今日水犹寒"},
	}
	e := []PoemLine{
		{Seq: 0, Text: "鹅，鹅，鹅，"},
		{Seq: 1, Text: "曲项向天歌。"},
		{Seq: 2, Text: "白毛浮绿水，"},
		{Seq: 3, Text: "红掌拨清波。"},
	}
	chunWang := []PoemLine{
		{Seq: 0, Text: "国破山河在，城春草木深。"},
		{Seq: 1, Text: "感时花溅泪，恨别鸟惊心。"},
	}

	cases := []struct {
		name  string
		verse string
		lines []PoemLine
		want  string
	}{
		{"五言半句补成联", "此地别燕丹", yiShui, "此地别燕丹，壮士发冲冠。"},
		{"已是完整联", "感时花溅泪，恨别鸟惊心。", chunWang, "感时花溅泪，恨别鸟惊心。"},
		{"咏鹅中间句", "白毛浮绿水", e, "白毛浮绿水，红掌拨清波。"},
		{"词句已有顿号保持", "独立寒秋，湘江北去，橘子洲头。", nil, "独立寒秋，湘江北去，橘子洲头。"},
		{"缺句号补上", "海内存知己，天涯若比邻", nil, "海内存知己，天涯若比邻。"},
	}
	for _, tc := range cases {
		got := CompleteDisplayVerse(tc.verse, tc.lines)
		if got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}
