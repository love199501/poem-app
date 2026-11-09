package model

import "time"

// User 极简用户模型。原生小程序通过 wx.login 拿 code 换取 openid，
// 这里仅保存 openid 与展示信息，便于点赞/评论归属。
type User struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	OpenID        string     `gorm:"size:64;uniqueIndex" json:"-"`
	Nickname      string     `gorm:"size:64" json:"nickname"`
	AvatarChar    string     `gorm:"size:8" json:"avatarChar"`
	AvatarColor   string     `gorm:"size:96" json:"avatarColor"`
	IsAdmin       bool       `gorm:"index" json:"isAdmin"`
	Exp           int        `gorm:"default:0;index" json:"exp"`         // 经验值（阅读/点赞/评论/收藏累计）
	ReadSeconds   int        `gorm:"default:0" json:"readSeconds"`       // 累计阅读时长（秒）
	MemberUntil   *time.Time `json:"memberUntil"`                        // 会员到期时间（nil 或过去=非会员）
	MemberClaimed bool       `gorm:"default:false" json:"memberClaimed"` // 是否已领取过免费会员
	CreatedAt     time.Time  `json:"createdAt"`

	// —— 以下为非数据库字段，返回前计算填充 ——
	Level     int    `gorm:"-" json:"level"`     // 当前等级（1 起）
	LevelName string `gorm:"-" json:"levelName"` // 身份称号，如「秀才」
	CurBase   int    `gorm:"-" json:"curBase"`   // 当前等级起点经验
	NextExp   int    `gorm:"-" json:"nextExp"`   // 升到下一级所需总经验（满级则=curBase）
	IsMax     bool   `gorm:"-" json:"isMax"`     // 是否已满级
	IsMember  bool   `gorm:"-" json:"isMember"`  // 当前是否为有效会员
}

// FillMember 依据 MemberUntil 计算当前是否为有效会员。
func (u *User) FillMember() {
	u.IsMember = u.MemberUntil != nil && u.MemberUntil.After(time.Now())
}

// 经验奖励规则（可按运营需要调整）。
const (
	ExpLike       = 2 // 点赞一次
	ExpComment    = 5 // 发表评论/回复
	ExpFavorite   = 3 // 收藏一首诗
	ExpReadPerMin = 2 // 每阅读满 1 分钟
)

// levelDef 等级定义：达到 Threshold 经验即进入该等级，Name 为身份称号。
type levelDef struct {
	Threshold int
	Name      string
}

// levels 诗词主题的身份等级（由低到高）。
// 曲线：前期升级快带来正反馈，高阶需长期坚持（诗仙按每日满额约需半年）。
var levels = []levelDef{
	{0, "白丁"},
	{50, "学童"},
	{150, "秀才"},
	{400, "举人"},
	{900, "进士"},
	{1800, "翰林"},
	{3500, "诗人"},
	{6000, "诗豪"},
	{10000, "诗仙"},
}

// FillLevel 依据 Exp 计算并填充等级、称号、进度相关字段。
func (u *User) FillLevel() {
	lv := 1
	base := 0
	name := levels[0].Name
	next := levels[len(levels)-1].Threshold
	isMax := false

	for i, d := range levels {
		if u.Exp >= d.Threshold {
			lv = i + 1
			base = d.Threshold
			name = d.Name
			if i+1 < len(levels) {
				next = levels[i+1].Threshold
				isMax = false
			} else {
				next = d.Threshold
				isMax = true
			}
		}
	}
	u.Level = lv
	u.CurBase = base
	u.LevelName = name
	u.NextExp = next
	u.IsMax = isMax
}

// LevelTitleByExp 按经验值快速取出身份称号与等级（用于评论作者标识等轻量场景）。
func LevelTitleByExp(exp int) (int, string) {
	lv := 1
	name := levels[0].Name
	for i, d := range levels {
		if exp >= d.Threshold {
			lv = i + 1
			name = d.Name
		}
	}
	return lv, name
}
