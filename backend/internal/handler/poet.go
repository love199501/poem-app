package handler

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"shijing/internal/llm"
	"shijing/internal/model"
)

var (
	yearNumRe = regexp.MustCompile(`(\d{3,4})`)
	ageNumRe  = regexp.MustCompile(`(\d+)\s*岁`)
)

// PoetScroll 一位诗人的人生卷轴：按年龄列出其已发布诗文。
func (h *Handler) PoetScroll(c *gin.Context) {
	author := strings.TrimSpace(c.Query("author"))
	if author == "" || author == "全部" {
		fail(c, http.StatusBadRequest, "请指定诗人")
		return
	}
	dynasty := model.NormalizeDynasty(c.Query("dynasty"))
	tx := h.DB.Model(&model.Poem{}).
		Select("id,title,dynasty,author,form,year_label,first_verse,life_curve").
		Where("status <> ? AND author = ?", "draft", author)
	if dynasty != "" {
		tx = tx.Where("dynasty IN ?", model.DynastyVariants(dynasty))
	}
	var poems []model.Poem
	err := tx.Find(&poems).Error
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.fillDisplayVerses(poems)
	normalizeDynasties(poems)

	type item struct {
		poem model.Poem
		year int
		age  int
	}
	items := make([]item, 0, len(poems))
	var births []int
	for _, p := range poems {
		year := parseYear(p.YearLabel)
		age := highlightAge(p.LifeCurve)
		if year > 0 && age > 0 {
			births = append(births, year-age)
		}
		items = append(items, item{poem: p, year: year, age: age})
	}
	birth := medianInt(births)
	for i := range items {
		if items[i].age == 0 && birth > 0 && items[i].year > birth {
			items[i].age = items[i].year - birth
		}
	}

	type stage struct {
		Age   int     `json:"age"`
		Label string  `json:"label"`
		Poems []gin.H `json:"poems"`
	}
	order := []int{}
	byAge := map[int]*stage{}
	for _, it := range items {
		key := it.age
		if key <= 0 {
			key = 0
		}
		st := byAge[key]
		if st == nil {
			label := "年代不详"
			if key > 0 {
				label = strconv.Itoa(key) + "岁"
			}
			st = &stage{Age: key, Label: label}
			byAge[key] = st
			order = append(order, key)
		}
		p := it.poem
		st.Poems = append(st.Poems, gin.H{
			"id":         p.ID,
			"title":      p.Title,
			"form":       p.Form,
			"yearLabel":  p.YearLabel,
			"firstVerse": p.FirstVerse,
		})
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i] == 0 {
			return false
		}
		if order[j] == 0 {
			return true
		}
		return order[i] < order[j]
	})
	stages := make([]stage, 0, len(order))
	for _, age := range order {
		stages = append(stages, *byAge[age])
	}
	if dynasty == "" && len(poems) > 0 {
		dynasty = poems[0].Dynasty
	}
	ok(c, gin.H{
		"author":  author,
		"dynasty": dynasty,
		"count":   len(poems),
		"stages":  stages,
		"intro":   h.cachedPoetIntro(author, dynasty),
	})
}

// PoetIntro 返回诗人介绍。库里没有时生成一次并保存。生成会调用大模型，必须登录。
func (h *Handler) PoetIntro(c *gin.Context) {
	if currentUserID(c) == 0 {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}
	author := strings.TrimSpace(c.Query("author"))
	if author == "" {
		fail(c, http.StatusBadRequest, "请指定诗人")
		return
	}
	dynasty := strings.TrimSpace(c.Query("dynasty"))
	intro := h.ensurePoetIntro(author, dynasty)
	ok(c, gin.H{"author": author, "intro": intro})
}

func poetStoreName(author, dynasty string) string {
	if (author == "佚名" || author == "无名氏") && dynasty != "" {
		return dynasty + "·" + author
	}
	return author
}

func (h *Handler) cachedPoetIntro(author, dynasty string) string {
	var p model.Poet
	if h.DB.Where("name = ?", poetStoreName(author, dynasty)).First(&p).Error != nil {
		return ""
	}
	return strings.TrimSpace(p.Intro)
}

var poetIntroMu sync.Mutex

func (h *Handler) ensurePoetIntro(author, dynasty string) string {
	if intro := h.cachedPoetIntro(author, dynasty); intro != "" {
		return intro
	}
	poetIntroMu.Lock()
	defer poetIntroMu.Unlock()
	if intro := h.cachedPoetIntro(author, dynasty); intro != "" {
		return intro
	}
	client := llm.New(h.Cfg.LLMBaseURL, h.Cfg.LLMAPIKey, h.Cfg.LLMModel)
	if client.HTTP != nil {
		client.HTTP.Timeout = 25 * time.Second
	}
	intro, err := client.PoetIntro(author, dynasty)
	if err != nil || strings.TrimSpace(intro) == "" {
		if err != nil {
			log.Printf("[poet] 介绍生成失败 %s: %v", author, err)
		}
		return ""
	}
	if e := h.DB.Save(&model.Poet{Name: poetStoreName(author, dynasty), Dynasty: dynasty, Intro: intro}).Error; e != nil {
		log.Printf("[poet] 介绍保存失败 %s: %v", author, e)
	}
	return intro
}

func parseYear(s string) int {
	m := yearNumRe.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	if n < 100 || n > 2100 {
		return 0
	}
	return n
}

func parseAge(s string) int {
	m := ageNumRe.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	if n <= 0 || n > 120 {
		return 0
	}
	return n
}

func highlightAge(curve []model.LifePoint) int {
	for _, p := range curve {
		if p.Highlight {
			if n := parseAge(p.Age); n > 0 {
				return n
			}
		}
	}
	return 0
}

// poetChatMaterial 给「与诗人对话」准备人物介绍，以及按年纪抽出的人生坐标。
func (h *Handler) poetChatMaterial(author, dynasty string) (intro, life string) {
	intro = h.cachedPoetIntro(author, dynasty)
	tx := h.DB.Model(&model.Poem{}).
		Select("id,title,dynasty,author,year_label,first_verse,life_curve").
		Where("status <> ? AND author = ?", "draft", author)
	if dynasty != "" {
		tx = tx.Where("dynasty IN ?", model.DynastyVariants(dynasty))
	}
	var poems []model.Poem
	if tx.Find(&poems).Error != nil || len(poems) == 0 {
		return intro, ""
	}
	type row struct {
		age   int
		year  int
		title string
		verse string
		label string
	}
	rows := make([]row, 0, len(poems))
	var births []int
	for _, p := range poems {
		year := parseYear(p.YearLabel)
		age := highlightAge(p.LifeCurve)
		if year > 0 && age > 0 {
			births = append(births, year-age)
		}
		verse := strings.TrimSpace(p.FirstVerse)
		rows = append(rows, row{age: age, year: year, title: p.Title, verse: verse, label: p.YearLabel})
	}
	birth := medianInt(births)
	for i := range rows {
		if rows[i].age == 0 && birth > 0 && rows[i].year > birth {
			rows[i].age = rows[i].year - birth
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		ai, aj := rows[i].age, rows[j].age
		if ai == 0 {
			ai = 10000
		}
		if aj == 0 {
			aj = 10000
		}
		if ai != aj {
			return ai < aj
		}
		if rows[i].year != rows[j].year {
			return rows[i].year < rows[j].year
		}
		return rows[i].title < rows[j].title
	})
	const cap = 24
	picked := rows
	omitted := 0
	if len(rows) > cap {
		picked = make([]row, 0, cap)
		for i := 0; i < cap; i++ {
			picked = append(picked, rows[i*(len(rows)-1)/(cap-1)])
		}
		omitted = len(rows) - cap
	}
	var b strings.Builder
	for _, r := range picked {
		age := "年代不详"
		if r.age > 0 {
			age = strconv.Itoa(r.age) + "岁"
		}
		fmt.Fprintf(&b, "- %s", age)
		if y := strings.TrimSpace(r.label); y != "" {
			fmt.Fprintf(&b, " · %s", y)
		}
		fmt.Fprintf(&b, " · 《%s》", r.title)
		if v := clipRunes(r.verse, 22); v != "" {
			fmt.Fprintf(&b, "：%s", v)
		}
		b.WriteString("\n")
	}
	if omitted > 0 {
		fmt.Fprintf(&b, "（另有 %d 首未列出，以上只是按年纪抽出的坐标）\n", omitted)
	}
	return intro, strings.TrimSpace(b.String())
}

func medianInt(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	sort.Ints(xs)
	return xs[len(xs)/2]
}
