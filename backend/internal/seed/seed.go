package seed

import (
	"log"

	"gorm.io/gorm"

	"shijing/internal/model"
)

// Migrate 自动建表。
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.Poem{},
		&model.PoemLine{},
		&model.Comment{},
		&model.CommentLike{},
		&model.Favorite{},
		&model.PoemLike{},
		&model.DailyTask{},
	)
}

// SyncCounts 依据关联表回填诗的收藏数 / 点赞数（幂等，供历史数据对齐）。
func SyncCounts(db *gorm.DB) error {
	if err := db.Exec(
		"UPDATE poems SET favorite_count = (SELECT COUNT(*) FROM favorites WHERE favorites.poem_id = poems.id)").Error; err != nil {
		return err
	}
	if err := db.Exec(
		"UPDATE poems SET like_count = (SELECT COUNT(*) FROM poem_likes WHERE poem_likes.poem_id = poems.id)").Error; err != nil {
		return err
	}
	return nil
}

// EnsureAdmin 保证存在一个管理员用户（网页后台登录后以此身份鉴权），幂等。
func EnsureAdmin(db *gorm.DB) error {
	var u model.User
	err := db.Where("open_id = ?", "admin-root").First(&u).Error
	if err == gorm.ErrRecordNotFound {
		u = model.User{OpenID: "admin-root", Nickname: "管理员", AvatarChar: "管", IsAdmin: true}
		return db.Create(&u).Error
	}
	if err == nil && !u.IsAdmin {
		return db.Model(&u).Update("is_admin", true).Error
	}
	return err
}

// Run 写入初始数据（幂等：已有诗词则跳过）。主案例《春望》+ 讨论区示例。
func Run(db *gorm.DB) error {
	var count int64
	db.Model(&model.Poem{}).Count(&count)
	if count > 0 {
		log.Println("[seed] poems already exist, skip")
		return nil
	}

	chunqwang := buildChunWang()
	chunqwang.Status = "published"
	if err := db.Create(&chunqwang).Error; err != nil {
		return err
	}
	for i := range extraPoems {
		extraPoems[i].Status = "published"
		if err := db.Create(&extraPoems[i]).Error; err != nil {
			return err
		}
	}

	seedCommunity(db, chunqwang.ID)
	log.Println("[seed] done")
	return nil
}
