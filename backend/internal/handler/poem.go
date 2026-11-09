package handler

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shijing/internal/model"
)

// ListPoems 诗库列表：支持 dynasty / mood / author / tag / q(标题或作者模糊) 过滤。
// 只返回元信息，不含 Lines 等富内容。
func (h *Handler) ListPoems(c *gin.Context) {
	q := c.Query("q")
	dynasty := c.Query("dynasty")
	mood := c.Query("mood")
	author := c.Query("author")
	tag := c.Query("tag")

	tx := h.DB.Model(&model.Poem{}).
		Select("id,title,dynasty,author,form,year_label,first_verse,audio_url,illust_url,tags,moods,comment_count,like_count,favorite_count,created_at").
		Where("status <> ?", "draft").
		Order("id asc")

	if dynasty != "" && dynasty != "全部" {
		// 「唐」同时命中库里可能残留的「唐代 / 唐朝」。
		tx = tx.Where("dynasty IN ?", model.DynastyVariants(dynasty))
	}
	if author != "" && author != "全部" {
		tx = tx.Where("author = ?", author)
	}
	if q != "" {
		like := "%" + q + "%"
		tx = tx.Where("title LIKE ? OR author LIKE ? OR first_verse LIKE ?", like, like, like)
	}
	// moods / tags 以 JSON 文本存储，用 LIKE 粗匹配即可满足演示需求。
	if mood != "" && mood != "全部" {
		tx = tx.Where("moods LIKE ?", "%\""+mood+"\"%")
	}
	if tag != "" && tag != "全部" {
		tx = tx.Where("tags LIKE ?", "%\""+tag+"\"%")
	}

	var poems []model.Poem
	if err := tx.Find(&poems).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.fillDisplayVerses(poems)
	normalizeDynasties(poems)
	ok(c, poems)
}

func normalizeDynasties(poems []model.Poem) {
	for i := range poems {
		poems[i].Dynasty = model.NormalizeDynasty(poems[i].Dynasty)
	}
}

// 朝代、心情的推荐展示顺序：只保留库中实际出现的，其余按出现顺序追加。
var dynastyOrder = []string{"先秦", "汉", "魏晋", "南北朝", "隋", "唐", "五代", "宋", "辽", "金", "元", "明", "清", "近现代", "现代"}
var moodOrder = []string{"忧国", "思乡", "旷达", "孤独", "喜悦", "离别", "爱情", "闲适", "哲理", "愤懑", "感伤", "豪放"}

// freq 记录字符串的出现顺序与计数，用于稳定聚合。
type freq struct {
	order []string
	count map[string]int
}

func newFreq() *freq { return &freq{count: map[string]int{}} }

func (f *freq) add(s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	if f.count[s] == 0 {
		f.order = append(f.order, s)
	}
	f.count[s]++
}

// orderByPref 先按 pref 预定义顺序输出出现过的，再按出现顺序补齐其余。
func orderByPref(pref []string, f *freq) []string {
	res := make([]string, 0, len(f.order))
	used := make(map[string]bool, len(f.order))
	for _, k := range pref {
		if f.count[k] > 0 {
			res = append(res, k)
			used[k] = true
		}
	}
	for _, k := range f.order {
		if !used[k] {
			res = append(res, k)
		}
	}
	return res
}

// topByCount 按出现次数降序取前 n（次数相同保持出现顺序）。
func topByCount(f *freq, n int) []string {
	ks := make([]string, len(f.order))
	copy(ks, f.order)
	sort.SliceStable(ks, func(i, j int) bool { return f.count[ks[i]] > f.count[ks[j]] })
	if n > 0 && len(ks) > n {
		ks = ks[:n]
	}
	return ks
}

// PoemFacets 实时聚合库中已发布诗词涉及的筛选维度：朝代 / 心情 / 诗人 / 标签。
// 写入、更新、发布后下次拉取即自动反映，无需手工维护。
func (h *Handler) PoemFacets(c *gin.Context) {
	var poems []model.Poem
	h.DB.Model(&model.Poem{}).
		Select("dynasty,author,moods,tags").
		Where("status <> ?", "draft").
		Find(&poems)

	dyn, auth, mood, tag := newFreq(), newFreq(), newFreq(), newFreq()
	for _, p := range poems {
		dyn.add(model.NormalizeDynasty(p.Dynasty))
		auth.add(p.Author)
		for _, m := range p.Moods {
			mood.add(m)
		}
		for _, t := range p.Tags {
			tag.add(t)
		}
	}

	withAll := func(ss []string) []string { return append([]string{"全部"}, ss...) }

	ok(c, gin.H{
		"dynasties": withAll(orderByPref(dynastyOrder, dyn)),
		"moods":     withAll(orderByPref(moodOrder, mood)),
		"authors":   withAll(topByCount(auth, 12)),
		"tags":      withAll(topByCount(tag, 20)),
	})
}

// GetPoem 诗词详情：含逐句注释与读/境/意/悟全部富内容。
func (h *Handler) GetPoem(c *gin.Context) {
	id := paramUint(c, "id")
	var poem model.Poem
	err := h.DB.Preload("Lines", func(db *gorm.DB) *gorm.DB {
		return db.Order("seq asc")
	}).First(&poem, id).Error
	if err != nil {
		fail(c, http.StatusNotFound, "诗词不存在")
		return
	}
	// 填充当前用户是否已收藏 / 已点赞。
	if uid := currentUserID(c); uid > 0 {
		var fc, lc int64
		h.DB.Model(&model.Favorite{}).Where("user_id = ? AND poem_id = ?", uid, poem.ID).Count(&fc)
		poem.Favored = fc > 0
		h.DB.Model(&model.PoemLike{}).Where("user_id = ? AND poem_id = ?", uid, poem.ID).Count(&lc)
		poem.Liked = lc > 0
	}
	poem.FirstVerse = model.CompleteDisplayVerse(poem.FirstVerse, poem.Lines)
	poem.Dynasty = model.NormalizeDynasty(poem.Dynasty)
	ok(c, poem)
}

// fillDisplayVerses 按逐句原文把 firstVerse 补成完整一句/一联，避免列表卡片露出半句。
func (h *Handler) fillDisplayVerses(poems []model.Poem) {
	if len(poems) == 0 {
		return
	}
	ids := make([]uint, len(poems))
	for i, p := range poems {
		ids[i] = p.ID
	}
	var lines []model.PoemLine
	h.DB.Select("poem_id,seq,text").Where("poem_id IN ?", ids).Order("poem_id, seq").Find(&lines)
	by := make(map[uint][]model.PoemLine, len(poems))
	for _, l := range lines {
		by[l.PoemID] = append(by[l.PoemID], l)
	}
	for i := range poems {
		poems[i].FirstVerse = model.CompleteDisplayVerse(poems[i].FirstVerse, by[poems[i].ID])
	}
}
