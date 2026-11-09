package model

import "time"

// —— 每日任务：限制每类经验来源的单日获取量，避免无限刷经验 ——

// 每日各任务的「单位」上限（read 的单位是分钟，其余是次数）。
const (
	DailyReadMinCap  = 10 // 阅读每天最多计 10 分钟
	DailyFavoriteCap = 3  // 每天最多 3 次收藏计经验
	DailyCommentCap  = 3  // 每天最多 3 条评论计经验
	DailyLikeCap     = 5  // 每天最多 5 次点赞计经验
)

// DailyTask 记录某用户某天某类任务的完成单位数与已获经验。
// (user_id,date,task) 唯一，天级重置（新的一天即新记录）。
type DailyTask struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:uniq_user_date_task" json:"userId"`
	Date      string    `gorm:"size:10;uniqueIndex:uniq_user_date_task" json:"date"` // YYYY-MM-DD
	Task      string    `gorm:"size:16;uniqueIndex:uniq_user_date_task" json:"task"` // read/favorite/comment/like
	Count     int       `json:"count"`                                               // 已完成单位（分钟或次数）
	Exp       int       `json:"exp"`                                                 // 当天通过该任务已获经验
	UpdatedAt time.Time `json:"updatedAt"`
}

// TaskDef 任务定义：展示信息 + 每日上限 + 单位经验。
type TaskDef struct {
	Task   string
	Title  string
	Desc   string
	Icon   string
	Cap    int // 每日单位上限
	PerExp int // 每单位经验
}

// DailyTaskDefs 全部每日任务（展示顺序即此顺序）。
var DailyTaskDefs = []TaskDef{
	{"read", "沉浸阅读", "阅读诗词，按分钟累计", "📖", DailyReadMinCap, ExpReadPerMin},
	{"favorite", "收藏好诗", "收藏喜欢的诗作", "⭐", DailyFavoriteCap, ExpFavorite},
	{"comment", "畅所欲言", "发表评论或诗话", "💬", DailyCommentCap, ExpComment},
	{"like", "点赞佳评", "为他人的佳评点赞", "♥", DailyLikeCap, ExpLike},
}

// FindTaskDef 按任务名查定义，不存在返回 nil。
func FindTaskDef(task string) *TaskDef {
	for i := range DailyTaskDefs {
		if DailyTaskDefs[i].Task == task {
			return &DailyTaskDefs[i]
		}
	}
	return nil
}
