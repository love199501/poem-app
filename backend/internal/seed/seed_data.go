package seed

import (
	"time"

	"gorm.io/gorm"

	"shijing/internal/model"
)

// buildChunWang 组装《春望》的完整富内容，数据取自原型图。
func buildChunWang() model.Poem {
	return model.Poem{
		Title:      "春望",
		Dynasty:    "唐",
		Author:     "杜甫",
		Form:       "五言律诗",
		YearLabel:  "公元 757 年",
		FirstVerse: "感时花溅泪，恨别鸟惊心。",
		Tags:       []string{"安史之乱", "忧国", "思家"},
		Moods:      []string{"忧国", "思乡"},
		Lines: []model.PoemLine{
			{Seq: 0, Text: "国破山河在，城春草木深。",
				NoteBody: "<b>国破</b>：国都长安被叛军攻陷。<b>山河在</b>：山河依旧存在。<b>草木深</b>：草木茂盛，暗写人烟稀少、市井荒芜。",
				NoteSrc:  "出处：《杜诗详注》卷四 · 仇兆鳌 注"},
			{Seq: 1, Text: "感时花溅泪，恨别鸟惊心。",
				NoteBody: "<b>感时</b>：感伤时局。<b>溅泪</b>：落泪。<b>恨别</b>：怅恨离别。<b>惊心</b>：内心震动。因感伤时局，见花开反而落泪；因怅恨离别，闻鸟鸣反而心惊。",
				NoteSrc:  "出处：《杜诗详注》卷四 · 仇兆鳌 注"},
			{Seq: 2, Text: "烽火连三月，家书抵万金。",
				NoteBody: "<b>烽火</b>：战火。<b>连三月</b>：接连数月不断。<b>家书</b>：家信。<b>抵万金</b>：抵得上万两黄金，极言珍贵。",
				NoteSrc:  "出处：《杜诗详注》卷四 · 仇兆鳌 注"},
			{Seq: 3, Text: "白头搔更短，浑欲不胜簪。",
				NoteBody: "<b>白头</b>：白发。<b>搔</b>：用手指抓挠。<b>短</b>：稀疏。<b>浑</b>：简直。<b>不胜簪</b>：插不住簪子，极写白发稀疏、衰老之态。",
				NoteSrc:  "出处：《杜诗详注》卷四 · 仇兆鳌 注"},
		},
		Translation: []string{
			"国都沦陷了，山河依旧在；长安城里春天来了，草木疯长，人烟稀少。",
			"感伤时局，看见花开也落泪；怅恨离别，听见鸟鸣也心惊。",
			"战火接连数月不断，一封家书抵得上万两黄金。",
			"白发越抓越稀疏，简直连簪子都插不住了。",
		},
		TransSource: "译注参考：中华书局《杜甫诗选》",

		// 境
		SceneTitle: "《春望》· 现场档案",
		SceneSub:   "THE SCENE · 公元 757 年",
		SceneRows: []model.SceneRow{
			{Icon: "📅", Text: "757 年春 · 唐至德二载"},
			{Icon: "📍", Text: "长安 · 已沦陷的都城"},
			{Icon: "🌦️", Text: "春深，草木疯长，街上人烟稀少"},
			{Icon: "👤", Text: "杜甫 46 岁 · 被叛军所俘，困于城中"},
			{Icon: "💔", Text: "妻儿远在羌村，音讯断绝已数月"},
		},
		ScenePunch:      "写这首诗的时候，他最想知道的不是战局，是羌村里那几个人还在不在。",
		SceneBackground: "至德二载春天，长安被安史叛军占了将近一年。杜甫在投奔唐肃宗的路上被俘，押回城里，人可以在坊巷里走，城门出不去。这一年他四十六岁。妻子杨氏带着孩子逃到鄜州羌村，从去年秋天失散后再无音讯。春天照常来了，宫苑和街道却没有人，草木长得很高。他站在这座还叫长安的空城里，不知道家里人是否还活着，也不知道朝廷还能不能回来。",
		Emotions: []model.Emotion{
			{Name: "忧国", Value: 92, Color: "#b0392c"},
			{Name: "思家", Value: 88, Color: "#b0392c"},
			{Name: "悲己", Value: 74, Color: "#3f6f62"},
			{Name: "愤懑", Value: 60, Color: "#c9a227"},
		},
		LifeCurve: []model.LifePoint{
			{Age: "24岁", Title: "望岳", Level: 82, Highlight: false},
			{Age: "46岁", Title: "春望", Level: 20, Highlight: true},
			{Age: "49岁", Title: "茅屋", Level: 40, Highlight: false},
			{Age: "57岁", Title: "登高", Level: 28, Highlight: false},
			{Age: "59岁", Title: "病逝湘江", Level: 10, Highlight: false},
		},

		// 意
		IntentLead: "他想说的不是「我很难过」，而是「这样的春天，不该是这个样子」。",
		IntentSub:  "一个被困在沦陷都城里的中年人，把家国之痛与骨肉之思，压缩进四十个字。",
		Reasons: []model.Reason{
			{Title: "亲眼所见", Body: "他就在长安城里，看着春天照常来，人却回不来。"},
			{Title: "无处可说", Body: "与家人音讯断绝数月，写给谁？只能写给诗。"},
			{Title: "必须留下", Body: "他要让后人知道，这一年长安是什么样子。"},
		},
		Strategies: []model.Strategy{
			{Name: "以乐景写哀", Desc: "花与鸟本是春日喜物，他偏让它们落泪、惊心。"},
			{Name: "一「在」一「深」", Desc: "山河还在，人却没了。两个虚词，写尽荒凉。"},
			{Name: "由大及小", Desc: "从国破写到一封家书，把宏大落到一个人的枕边。"},
			{Name: "结于细节", Desc: "不喊痛，只写头发白到插不住簪子。"},
		},
		Audiences: []model.Audience{
			{Role: "自己", Text: "把说不出口的痛，安放成四十个字。"},
			{Role: "家人", Text: "若有一天家人读到，便知道他还在、还念着。"},
			{Role: "后人", Text: "为这个破碎的春天，留一份证词。"},
		},
		Void: "全诗没有出现一个「妻」字、一个「儿」字，却用「家书抵万金」把思念写到极致；他没写自己是否哭过，只写「白头搔更短」——那只反复抓头发的手，比眼泪更重。",

		// 悟
		Layers: []model.Layer{
			{Key: "face", Title: "字面义", Body: "国都沦陷，山河仍在；春来草木深，花落鸟鸣；战火连月，家书难得；白发稀疏，簪不能束。"},
			{Key: "heart", Title: "诗人心", Body: "一个「在」字，是庆幸也是讽刺；一个「深」字，写草木，讲的却是人没了。五六句把镜头从国家拉回自己——这一刻他只是一个想知道老婆孩子还活着没有的中年男人。"},
			{Key: "era", Title: "时代音", Body: "安史之乱把盛唐劈成两半。春望写的不只是杜甫一个人，而是整整一代人被战争碾过的春天。"},
		},
		Quotes: []model.Quote{
			{Text: "山河在，明无余物矣；草木深，明无人矣。", Source: "司马光《温公续诗话》"},
		},
		Duels: []model.Duel{
			{Side: "a", Title: "观点 A · 移情于物", Body: "花鸟本无情，诗人把自己的泪与惊投射上去。景语皆情语。"},
			{Side: "b", Title: "观点 B · 触景生情", Body: "花鸟自在地开着叫着，正因它们「无情」，才更衬出人的有情与痛。"},
		},
		AiPrompts: []string{"他真的穷到那种地步吗？", "家书后来送到了吗？", "为什么是五言律诗？"},
	}
}

// extraPoems 诗库里的其它几首（内容从简，供列表/筛选演示）。
var extraPoems = []model.Poem{
	{
		Title: "闻官军收河南河北", Dynasty: "唐", Author: "杜甫", Form: "七言律诗",
		YearLabel: "公元 763 年", FirstVerse: "白日放歌须纵酒，青春作伴好还乡。",
		Tags: []string{"安史之乱", "狂喜"}, Moods: []string{"喜悦"},
		Lines: []model.PoemLine{{Seq: 0, Text: "剑外忽传收蓟北，初闻涕泪满衣裳。"}},
	},
	{
		Title: "茅屋为秋风所破歌", Dynasty: "唐", Author: "杜甫", Form: "古体诗",
		YearLabel: "公元 761 年", FirstVerse: "安得广厦千万间，大庇天下寒士俱欢颜。",
		Tags: []string{"忧国", "济世"}, Moods: []string{"忧国"},
		Lines: []model.PoemLine{{Seq: 0, Text: "八月秋高风怒号，卷我屋上三重茅。"}},
	},
	{
		Title: "登高", Dynasty: "唐", Author: "杜甫", Form: "七言律诗",
		YearLabel: "公元 767 年", FirstVerse: "万里悲秋常作客，百年多病独登台。",
		Tags: []string{"漂泊", "悲己"}, Moods: []string{"孤独"},
		Lines: []model.PoemLine{{Seq: 0, Text: "风急天高猿啸哀，渚清沙白鸟飞回。"}},
	},
}

// seedCommunity 为《春望》写入讨论区示例：林间有风为「精品置顶」。
func seedCommunity(db *gorm.DB, poemID uint) {
	u := model.User{OpenID: "seed-demo", Nickname: "杜家小友", AvatarChar: "杜", AvatarColor: "linear-gradient(140deg,#5c5142,#8a7a5e)"}
	db.Create(&u)

	now := time.Now()
	comments := []model.Comment{
		{PoemID: poemID, UserID: u.ID, AuthorName: "林间有风", AuthorBadge: "诗话",
			AvatarChar: "林", AvatarColor: "linear-gradient(140deg,#7a9b8c,#4a7c6f)",
			Content: "「国破山河在」最狠的地方在于——山河根本不在乎谁当皇帝。它只是在那儿。所以「城春草木深」才那么冷：春天照常来了，人却没回来。",
			Kind:    "shihua", LikeCount: 1200, ReplyCount: 46,
			IsFeatured: true, IsPinned: true, PinnedAt: &now},
		{PoemID: poemID, UserID: u.ID, AuthorName: "陈老师", AuthorBadge: "语文教研",
			AvatarChar: "陈", AvatarColor: "linear-gradient(140deg,#c98f6a,#a8623b)",
			Content: "给学生的建议：先别看赏析，就问自己——如果你是杜甫，困在长安城里，你会先写什么？大部分孩子第一句写「我很想家」，而杜甫先写的是国家。",
			Kind:    "shihua", LikeCount: 86, ReplyCount: 23},
		{PoemID: poemID, UserID: u.ID, AuthorName: "杜家小友",
			AvatarChar: "杜", AvatarColor: "linear-gradient(140deg,#a99a7c,#7d6f52)",
			Content: "「白头搔更短，浑欲不胜簪」，46 岁头发白到簪子都别不住。古人平均寿命短，46 岁已经是老头了。",
			Kind:    "short", LikeCount: 43, ReplyCount: 5},
		{PoemID: poemID, UserID: u.ID, AuthorName: "阿棠",
			AvatarChar: "阿", AvatarColor: "linear-gradient(140deg,#6f8fa8,#4a6d88)",
			Content: "第一次知道「浑欲不胜簪」里的「簪」是真的插在头发上的。人到中年，连一根簪子都留不住。",
			Kind:    "short", LikeCount: 30, ReplyCount: 2},
	}
	for i := range comments {
		db.Create(&comments[i])
	}

	// 楼中楼示例：挂在第 1 条（林间有风）下的两条回复。
	top := comments[0]
	replies := []model.Comment{
		{PoemID: poemID, ParentID: top.ID, UserID: u.ID, ReplyToName: "林间有风",
			AuthorName: "陈老师", AvatarChar: "陈", AvatarColor: "linear-gradient(140deg,#c98f6a,#a8623b)",
			Content: "说得好。补一句：山河是客观的「在」，草木是荒芜的「深」，一喜一悲全在两个字里。", Kind: "reply", LikeCount: 12},
		{PoemID: poemID, ParentID: top.ID, UserID: u.ID, ReplyToName: "陈老师",
			AuthorName: "阿棠", AvatarChar: "阿", AvatarColor: "linear-gradient(140deg,#6f8fa8,#4a6d88)",
			Content: "所以越读越冷，表面写春景，底下全是空城。", Kind: "reply", LikeCount: 5},
		{PoemID: poemID, ParentID: top.ID, UserID: u.ID, ReplyToName: "林间有风",
			AuthorName: "杜家小友", AvatarChar: "杜", AvatarColor: "linear-gradient(140deg,#a99a7c,#7d6f52)",
			Content: "+1，山河不在乎谁当皇帝这句太戳了。", Kind: "reply", LikeCount: 3},
	}
	for i := range replies {
		db.Create(&replies[i])
	}
	db.Model(&model.Comment{}).Where("id = ?", top.ID).Update("reply_count", len(replies))

	total := len(comments) + len(replies)
	db.Model(&model.Poem{}).Where("id = ?", poemID).Update("comment_count", total)
}
