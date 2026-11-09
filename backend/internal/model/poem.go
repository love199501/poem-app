package model

import "time"

// —— 诗词富内容的子结构，均以 JSON 序列化方式存入单列（GORM serializer:json）——

type SceneRow struct {
	Icon string `json:"icon"`
	Text string `json:"text"`
}

// Emotion 情绪坐标：名称 + 0-100 强度 + 进度条配色。
type Emotion struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
	Color string `json:"color"`
}

// LifePoint 人生曲线节点。Level 为 0-100 的相对高度，Highlight 标记本诗所在点。
type LifePoint struct {
	Age       string `json:"age"`
	Title     string `json:"title"`
	Level     int    `json:"level"`
	Highlight bool   `json:"highlight"`
}

type Reason struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type Strategy struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

type Audience struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// Layer 三层解析：key=face/heart/era，字面义/诗人心/时代音。
type Layer struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

type Quote struct {
	Text   string `json:"text"`
	Source string `json:"source"`
}

// Duel 争议并列的一方。Side=a/b 控制配色。
type Duel struct {
	Side  string `json:"side"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// PoemLine 逐句原文 + 注释，支撑「读」里点击诗句看注释。
type PoemLine struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	PoemID   uint   `gorm:"index" json:"poemId"`
	Seq      int    `json:"seq"`
	Text     string `gorm:"size:255" json:"text"`
	NoteBody string `gorm:"type:text" json:"noteBody"`
	NoteSrc  string `gorm:"size:255" json:"noteSrc"`
}

// Poem 一首诗的全部内容：元信息 + 读/境/意/悟 四段富内容。
// 列表接口只返回元信息，详情接口返回全部（含 Lines）。
type Poem struct {
	ID         uint     `gorm:"primaryKey" json:"id"`
	Title      string   `gorm:"size:64" json:"title"`
	Dynasty    string   `gorm:"size:16;index" json:"dynasty"`
	Author     string   `gorm:"size:32;index" json:"author"`
	Form       string   `gorm:"size:32" json:"form"`      // 体裁
	YearLabel  string   `gorm:"size:64" json:"yearLabel"` // 如「公元 757 年」
	FirstVerse string   `gorm:"size:128" json:"firstVerse"`
	AudioURL   string   `gorm:"size:512" json:"audioUrl"`        // 名家朗诵音频（mp3等），为空则前端用 TTS 合成
	IllustURL  string   `gorm:"size:512" json:"illustrationUrl"` // 人物水墨插画（AI生成）
	Tags       []string `gorm:"serializer:json" json:"tags"`
	Moods      []string `gorm:"serializer:json;" json:"moods"` // 心情筛选维度

	// 读
	Lines       []PoemLine `gorm:"constraint:OnDelete:CASCADE" json:"lines,omitempty"`
	Translation []string   `gorm:"serializer:json" json:"translation,omitempty"`
	TransSource string     `gorm:"size:255" json:"transSource"`

	// 境
	SceneTitle      string      `gorm:"size:64" json:"sceneTitle"`
	SceneSub        string      `gorm:"size:64" json:"sceneSub"`
	SceneRows       []SceneRow  `gorm:"serializer:json" json:"sceneRows,omitempty"`
	ScenePunch      string      `gorm:"type:text" json:"scenePunch"`
	SceneBackground string      `gorm:"type:text" json:"sceneBackground"` // 当时的创作背景，供读者理解作者处境
	Emotions        []Emotion   `gorm:"serializer:json" json:"emotions,omitempty"`
	LifeCurve       []LifePoint `gorm:"serializer:json" json:"lifeCurve,omitempty"`

	// 意
	IntentLead string     `gorm:"type:text" json:"intentLead"`
	IntentSub  string     `gorm:"type:text" json:"intentSub"`
	Reasons    []Reason   `gorm:"serializer:json" json:"reasons,omitempty"`
	Strategies []Strategy `gorm:"serializer:json" json:"strategies,omitempty"`
	Audiences  []Audience `gorm:"serializer:json" json:"audiences,omitempty"`
	Void       string     `gorm:"type:text" json:"void"`

	// 悟
	Layers    []Layer  `gorm:"serializer:json" json:"layers,omitempty"`
	Quotes    []Quote  `gorm:"serializer:json" json:"quotes,omitempty"`
	Duels     []Duel   `gorm:"serializer:json" json:"duels,omitempty"`
	AiPrompts []string `gorm:"serializer:json" json:"aiPrompts,omitempty"`

	CommentCount  int       `json:"commentCount"`
	LikeCount     int       `gorm:"default:0" json:"likeCount"`                    // 诗被点赞数（受欢迎度）
	FavoriteCount int       `gorm:"default:0" json:"favoriteCount"`                // 诗被收藏数（受欢迎度）
	Status        string    `gorm:"size:16;index;default:published" json:"status"` // draft=草稿 published=已发布
	CreatedAt     time.Time `json:"createdAt"`

	// Favored / Liked 非数据库字段：当前请求用户是否已收藏 / 已点赞，详情接口按 user 填充。
	Favored bool `gorm:"-" json:"favored"`
	Liked   bool `gorm:"-" json:"liked"`
}
