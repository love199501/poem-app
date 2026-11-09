package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/model"
)

// todayStr 当前日期（本地时区），作为每日任务的分组键。
func todayStr() string { return time.Now().Format("2006-01-02") }

// grantDaily 在每日上限内为某任务发放经验，返回实际新增经验。
// units：本次完成的单位数（read 传分钟数，其余传 1）。超过当日剩余额度的部分不再计经验。
func (h *Handler) grantDaily(userID uint, task string, units int) int {
	def := model.FindTaskDef(task)
	if def == nil || userID == 0 || units <= 0 {
		return 0
	}
	date := todayStr()

	var dt model.DailyTask
	err := h.DB.Where("user_id = ? AND date = ? AND task = ?", userID, date, task).First(&dt).Error
	if err == gorm.ErrRecordNotFound {
		dt = model.DailyTask{UserID: userID, Date: date, Task: task}
	} else if err != nil {
		return 0
	}

	remain := def.Cap - dt.Count
	if remain <= 0 {
		return 0 // 今日该任务已达上限
	}
	if units > remain {
		units = remain
	}
	gain := units * def.PerExp
	dt.Count += units
	dt.Exp += gain

	if dt.ID == 0 {
		if e := h.DB.Create(&dt).Error; e != nil {
			return 0
		}
	} else {
		h.DB.Model(&model.DailyTask{}).Where("id = ?", dt.ID).
			Updates(map[string]any{"count": dt.Count, "exp": dt.Exp})
	}
	h.addExp(userID, gain)
	return gain
}

// DailyTasks 返回当前用户「今日任务」进度：各任务已完成单位 / 上限、已得经验 / 上限。
func (h *Handler) DailyTasks(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	date := todayStr()
	var rows []model.DailyTask
	h.DB.Where("user_id = ? AND date = ?", uid, date).Find(&rows)
	cur := make(map[string]model.DailyTask, len(rows))
	for _, r := range rows {
		cur[r.Task] = r
	}

	list := make([]gin.H, 0, len(model.DailyTaskDefs))
	gotExp, totalExp := 0, 0
	for _, d := range model.DailyTaskDefs {
		r := cur[d.Task]
		maxExp := d.Cap * d.PerExp
		list = append(list, gin.H{
			"task":   d.Task,
			"title":  d.Title,
			"desc":   d.Desc,
			"icon":   d.Icon,
			"count":  r.Count,
			"cap":    d.Cap,
			"exp":    r.Exp,
			"maxExp": maxExp,
			"perExp": d.PerExp,
			"done":   r.Count >= d.Cap,
		})
		gotExp += r.Exp
		totalExp += maxExp
	}
	ok(c, gin.H{"date": date, "tasks": list, "gotExp": gotExp, "totalExp": totalExp})
}
