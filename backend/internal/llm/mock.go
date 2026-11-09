package llm

import "shijing/internal/model"

// mockPoem 在未配置大模型 Key 时生成占位草稿，保证后台流程可跑通。
// 管理员可在后台手动完善，或配置 LLM_API_KEY 后重新用真实模型生成。
func mockPoem(title, author string) *model.Poem {
	return &model.Poem{
		Title:      title,
		Author:     author,
		Dynasty:    "待补充",
		Form:       "待补充",
		YearLabel:  "待补充",
		FirstVerse: "（示例）" + title + " 的代表名句",
		Tags:       []string{"待补充"},
		Moods:      []string{"忧国"},
		Lines: []model.PoemLine{
			{Seq: 0, Text: "（示例）" + title + " 第一句。", NoteBody: "<b>示例</b>：此为本地 mock 生成，请配置大模型或手动完善。", NoteSrc: "—"},
		},
		Translation: []string{"（示例）第一句的白话译文。"},
		TransSource: "—",
		SceneTitle:  title + " · 现场档案",
		SceneSub:    "THE SCENE",
		SceneRows: []model.SceneRow{
			{Icon: "📅", Text: "时间：待补充"},
			{Icon: "📍", Text: "地点：待补充"},
		},
		ScenePunch:      "（示例）一句点睛的处境描述。",
		SceneBackground: "（示例）写明他在何时何地、多大年纪、刚经历了什么，以及落笔时最挂心的事。配置大模型后会按诗自动生成。",
		IntentLead:      "（示例）他想说的是……",
		IntentSub:       "本地 mock 内容，配置大模型后可自动生成。",
		Audiences: []model.Audience{
			{Role: "自己", Text: "待补充"},
		},
		Void: "（示例）他没写出来的……",
		Layers: []model.Layer{
			{Key: "face", Title: "字面义", Body: "待补充"},
			{Key: "heart", Title: "诗人心", Body: "待补充"},
			{Key: "era", Title: "时代音", Body: "待补充"},
		},
		AiPrompts: []string{"这首诗写于何时？"},
	}
}
