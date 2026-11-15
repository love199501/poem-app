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
		h.settleCheckin(userID)
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
	h.settleCheckin(userID)
	return gain
}

// activeTaskDefs 当前对用户生效的每日任务。评论关闭时不要求评论。
func (h *Handler) activeTaskDefs() []model.TaskDef {
	out := make([]model.TaskDef, 0, len(model.DailyTaskDefs))
	for _, d := range model.DailyTaskDefs {
		if d.Task == "comment" && !h.Cfg.CommentEnabled {
			continue
		}
		out = append(out, d)
	}
	return out
}

func tasksDone(defs []model.TaskDef, cur map[string]model.DailyTask) bool {
	if len(defs) == 0 {
		return false
	}
	for _, d := range defs {
		if cur[d.Task].Count < d.Cap {
			return false
		}
	}
	return true
}

// settleCheckin 当天全部任务已完成且尚未打卡时，记一笔并加 15 经验。返回本次新得经验。
func (h *Handler) settleCheckin(userID uint) int {
	if userID == 0 {
		return 0
	}
	date := todayStr()
	var rows []model.DailyTask
	if err := h.DB.Where("user_id = ? AND date = ?", userID, date).Find(&rows).Error; err != nil {
		return 0
	}
	cur := make(map[string]model.DailyTask, len(rows))
	for _, r := range rows {
		cur[r.Task] = r
	}
	if !tasksDone(h.activeTaskDefs(), cur) {
		return 0
	}
	var exist model.Checkin
	err := h.DB.Where("user_id = ? AND date = ?", userID, date).First(&exist).Error
	if err == nil {
		return 0
	}
	if err != gorm.ErrRecordNotFound {
		return 0
	}
	rec := model.Checkin{UserID: userID, Date: date, Exp: model.ExpCheckin}
	if e := h.DB.Create(&rec).Error; e != nil {
		return 0
	}
	h.addExp(userID, model.ExpCheckin)
	return model.ExpCheckin
}

func monthBounds(now time.Time) (start, end string) {
	start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	end = time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	return start, end
}

func (h *Handler) monthCheckinCount(userID uint, now time.Time) int64 {
	start, end := monthBounds(now)
	var n int64
	h.DB.Model(&model.Checkin{}).Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).Count(&n)
	return n
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

	checkinGain := h.settleCheckin(uid)
	now := time.Now()
	month := now.Format("2006-01")
	var me model.User
	_ = h.DB.Select("checkin_reward_month").First(&me, uid).Error
	monthCount := h.monthCheckinCount(uid, now)
	checkedToday := false
	var todayRow model.Checkin
	if h.DB.Where("user_id = ? AND date = ?", uid, date).First(&todayRow).Error == nil {
		checkedToday = true
	}

	list := make([]gin.H, 0, len(model.DailyTaskDefs))
	gotExp, totalExp := 0, 0
	for _, d := range h.activeTaskDefs() {
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
	ok(c, gin.H{
		"date":        date,
		"tasks":       list,
		"gotExp":      gotExp,
		"totalExp":    totalExp,
		"checkinGain": checkinGain,
		"checkin": gin.H{
			"today":      checkedToday,
			"monthCount": monthCount,
			"need":       model.CheckinMemberAfter + 1,
			"canClaim":   monthCount > model.CheckinMemberAfter && me.CheckinRewardMonth != month,
			"claimed":    me.CheckinRewardMonth == month,
			"bonusExp":   model.ExpCheckin,
		},
	})
}

// ClaimCheckinMember 当月打卡超过 17 次时，领取一个月会员。每个自然月一次。
func (h *Handler) ClaimCheckinMember(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	now := time.Now()
	month := now.Format("2006-01")
	if h.monthCheckinCount(uid, now) <= model.CheckinMemberAfter {
		fail(c, http.StatusBadRequest, "本月打卡未超过 17 次")
		return
	}
	var u model.User
	if err := h.DB.First(&u, uid).Error; err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if u.CheckinRewardMonth == month {
		fail(c, http.StatusBadRequest, "本月已领取")
		return
	}
	base := now
	if u.MemberUntil != nil && u.MemberUntil.After(base) {
		base = *u.MemberUntil
	}
	until := base.AddDate(0, 1, 0)
	res := h.DB.Model(&model.User{}).
		Where("id = ? AND (checkin_reward_month IS NULL OR checkin_reward_month = '' OR checkin_reward_month <> ?)", uid, month).
		Updates(map[string]any{"member_until": until, "checkin_reward_month": month})
	if res.Error != nil {
		fail(c, http.StatusInternalServerError, res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		fail(c, http.StatusBadRequest, "本月已领取")
		return
	}
	u.MemberUntil = &until
	u.CheckinRewardMonth = month
	u.FillLevel()
	u.FillMember()
	ok(c, u)
}
