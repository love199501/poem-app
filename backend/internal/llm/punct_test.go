package llm

import (
	"testing"

	"shijing/internal/model"
)

func TestCoupletPunctQiyan(t *testing.T) {
	lines := []model.PoemLine{
		{Text: "众芳摇落独暄妍"},
		{Text: "占尽风情向小园"},
		{Text: "疏影横斜水清浅"},
		{Text: "暗香浮动月黄昏"},
		{Text: "霜禽欲下先偷眼"},
		{Text: "粉蝶如知合断魂"},
		{Text: "幸有微吟可相狎"},
		{Text: "不须檀板共金尊"},
	}
	if !CoupletPunct(lines) {
		t.Fatal("七言律诗应补上句读")
	}
	if lines[0].Text != "众芳摇落独暄妍，" || lines[1].Text != "占尽风情向小园。" {
		t.Fatalf("句读不符: %q / %q", lines[0].Text, lines[1].Text)
	}
	if lines[7].Text != "不须檀板共金尊。" {
		t.Fatalf("末句不符: %q", lines[7].Text)
	}
}

func TestNormalizeXiPause(t *testing.T) {
	lines := []model.PoemLine{
		{Text: "鸟嘤嘤兮友之期，"},
		{Text: "念高子兮仆怀思。"},
		{Text: "想念恢兮爰集兹，"},
	}
	if !NormalizePunct(lines) {
		t.Fatal("兮字句读应被校正")
	}
	want := []string{"鸟嘤嘤兮，友之期。", "念高子兮，仆怀思。", "想念恢兮，爰集兹。"}
	for i, w := range want {
		if lines[i].Text != w {
			t.Fatalf("第%d句不符: %q", i+1, lines[i].Text)
		}
	}
}

func TestNormalizeXiWithoutPunct(t *testing.T) {
	lines := []model.PoemLine{{Text: "鸟嘤嘤兮友之期"}}
	NormalizePunct(lines)
	if lines[0].Text != "鸟嘤嘤兮，友之期。" {
		t.Fatalf("无标点的兮字句不符: %q", lines[0].Text)
	}
}

func TestNormalizeKeepsLvshi(t *testing.T) {
	lines := []model.PoemLine{
		{Text: "众芳摇落独暄妍，"},
		{Text: "占尽风情向小园。"},
		{Text: "疏影横斜水清浅，"},
		{Text: "暗香浮动月黄昏。"},
	}
	if NormalizePunct(lines) {
		t.Fatal("近体出句的逗号应保留")
	}
}

func TestCorrectedLines(t *testing.T) {
	old := []string{"鸟嘤嘤兮友之期", "念高子兮仆怀思", "想念恢兮爰集兹"}
	if got := CorrectedLines(old, PoemAudit{Complete: true, Correct: true}); got != nil {
		t.Fatal("核对通过不应替换")
	}
	if got := CorrectedLines(old, PoemAudit{Complete: true, Correct: false, Lines: []string{"鸟嘤嘤兮，友之期。", "念高子兮，仆怀思。", "想念恢兮，爰集兹。"}}); got != nil {
		t.Fatal("只差标点不应替换")
	}
	got := CorrectedLines(old, PoemAudit{Complete: true, Correct: false, Lines: []string{"鸟嘤嘤兮，友之期。", "念高子兮，仆怀思。", "相念恢兮，爰集兹。"}})
	if len(got) != 3 || got[2] != "相念恢兮，爰集兹。" {
		t.Fatalf("错字应替换: %#v", got)
	}
	short := CorrectedLines(old[:1], PoemAudit{Complete: false, Correct: true, Lines: old})
	if len(short) != 3 {
		t.Fatalf("缺句应补全: %#v", short)
	}
	if CorrectedLines(old, PoemAudit{Complete: false, Correct: true, Lines: old[:2]}) != nil {
		t.Fatal("更短的结果不能当作补全")
	}
}

func TestEnsureReadingPunctAddsStop(t *testing.T) {
	denggao := []model.PoemLine{{Text: "风急天高猿啸哀，渚清沙白鸟飞回"}}
	if !EnsureReadingPunct(denggao) || denggao[0].Text != "风急天高猿啸哀，渚清沙白鸟飞回。" {
		t.Fatalf("句末应补句号: %q", denggao[0].Text)
	}
	wuyan := []model.PoemLine{
		{Text: "此地别燕丹"},
		{Text: "壮士发冲冠"},
		{Text: "昔时人已没"},
		{Text: "今日水犹寒"},
	}
	if !EnsureReadingPunct(wuyan) || wuyan[0].Text != "此地别燕丹，" || wuyan[1].Text != "壮士发冲冠。" {
		t.Fatalf("五言应交替标点: %q / %q", wuyan[0].Text, wuyan[1].Text)
	}
}

func TestCoupletPunctSkipsCi(t *testing.T) {
	lines := []model.PoemLine{
		{Text: "苔枝缀玉有翠禽小小枝上同宿"},
		{Text: "客里相逢篱角黄昏无言自倚修竹"},
		{Text: "昭君不惯胡沙远但暗忆江南江北"},
		{Text: "想佩环月夜归来化作此花幽独"},
	}
	if CoupletPunct(lines) {
		t.Fatal("长短不齐的词不该按近体句读")
	}
}
