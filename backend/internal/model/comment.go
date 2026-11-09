package model

import "time"

// Comment 评论（短评 / 诗话）。IsPinned 置顶、IsFeatured 精品，
// 对应「精品点赞可以置顶」：高赞或作者精选的内容会被标记并排在最前。
type Comment struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	PoemID      uint       `gorm:"index" json:"poemId"`
	ParentID    uint       `gorm:"index" json:"parentId"`      // 0=顶层评论；>0=某条评论的楼中楼回复
	ReplyToName string     `gorm:"size:64" json:"replyToName"` // 回复 @谁（展示用）
	UserID      uint       `json:"userId"`
	AuthorName  string     `gorm:"size:64" json:"authorName"`
	AuthorBadge string     `gorm:"size:32" json:"authorBadge"` // 如「语文教研」
	AvatarChar  string     `gorm:"size:8" json:"avatarChar"`
	AvatarColor string     `gorm:"size:96" json:"avatarColor"`
	Content     string     `gorm:"type:text" json:"content"`
	Kind        string     `gorm:"size:16;index" json:"kind"` // short=短评 shihua=诗话 reply=回复
	LikeCount   int        `json:"likeCount"`
	ReplyCount  int        `json:"replyCount"`
	IsFeatured  bool       `json:"isFeatured"` // 精品
	IsPinned    bool       `gorm:"index" json:"isPinned"`
	PinnedAt    *time.Time `json:"pinnedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`

	// Liked 非数据库字段：当前请求用户是否已点赞，查询时按 user 填充。
	Liked bool `gorm:"-" json:"liked"`
	// AuthorLevel / AuthorTitle 非数据库字段：作者当前等级与身份称号，查询时按 user 的经验填充。
	AuthorLevel int    `gorm:"-" json:"authorLevel"`
	AuthorTitle string `gorm:"-" json:"authorTitle"`
	// AuthorMember 非数据库字段：作者当前是否为有效会员（展示皇冠）。
	AuthorMember bool `gorm:"-" json:"authorMember"`
	// Replies 非数据库字段：顶层评论挂载的楼中楼回复（按时间升序）。
	Replies []Comment `gorm:"-" json:"replies"`
}

// CommentLike 点赞记录，(comment_id,user_id) 唯一，保证一人一赞可切换。
type CommentLike struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CommentID uint      `gorm:"uniqueIndex:uniq_comment_user" json:"commentId"`
	UserID    uint      `gorm:"uniqueIndex:uniq_comment_user" json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
}

// FeatureLikeThreshold 达到该赞数即视为「精品」，前端可展示精品标。
const FeatureLikeThreshold = 100
