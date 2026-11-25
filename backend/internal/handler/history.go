package handler

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"shijing/internal/model"
)

// majorDynasty 把初唐、北宋这类阶段收成卷轴上的一级朝代。
var majorDynasty = map[string]string{
	"初唐": "唐", "盛唐": "唐", "中唐": "唐", "晚唐": "唐",
	"北宋": "宋", "南宋": "宋",
	"西汉": "汉", "东汉": "汉",
	"明初": "明", "晚明": "明",
	"清初": "清", "晚清": "清",
}

// dynastyPeriods 朝代内部的出场阶段。birth 用生年，否则用作品年份。
var dynastyPeriods = map[string][2][]yearSpan{
	"唐": {
		{{"初唐", 0, 689}, {"盛唐", 690, 745}, {"中唐", 746, 800}, {"晚唐", 801, 907}},
		{{"初唐", 0, 711}, {"盛唐", 712, 765}, {"中唐", 766, 835}, {"晚唐", 836, 960}},
	},
	"宋": {
		{{"北宋前期", 0, 1020}, {"北宋中叶", 1021, 1080}, {"南渡之际", 1081, 1124}, {"南宋", 1125, 1300}},
		{{"北宋前期", 0, 1050}, {"北宋中叶", 1051, 1110}, {"南渡之际", 1111, 1154}, {"南宋", 1155, 1300}},
	},
	"元": {
		{{"元初", 0, 1259}, {"元中期", 1260, 1300}, {"元末", 1301, 1400}},
		{{"元初", 0, 1290}, {"元中期", 1291, 1330}, {"元末", 1331, 1400}},
	},
	"明": {
		{{"明初", 0, 1419}, {"明中叶", 1420, 1520}, {"晚明", 1521, 1700}},
		{{"明初", 0, 1450}, {"明中叶", 1451, 1560}, {"晚明", 1561, 1700}},
	},
	"清": {
		{{"清初", 0, 1699}, {"清中叶", 1700, 1820}, {"晚清", 1821, 2100}},
		{{"清初", 0, 1730}, {"清中叶", 1731, 1850}, {"晚清", 1851, 2100}},
	},
	"汉": {
		{{"西汉", 0, 24}, {"东汉", 25, 220}},
		{{"西汉", 0, 24}, {"东汉", 25, 220}},
	},
}

type yearSpan struct {
	name     string
	from, to int
}

// HistoryScroll 历史卷轴：一级是大朝代，展开后按出场阶段排列诗人和代表句。
func (h *Handler) HistoryScroll(c *gin.Context) {
	var poems []model.Poem
	err := h.DB.Model(&model.Poem{}).
		Select("id,dynasty,author,year_label,first_verse,like_count,favorite_count,life_curve").
		Where("status <> ?", "draft").
		Find(&poems).Error
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.fillDisplayVerses(poems)

	type agg struct {
		poems int
		heat  int
		birth []int
		years []int
		verse string
		vHeat int
		vRank int
	}
	byDyn := map[string]map[string]*agg{}
	seen := newFreq()
	for _, p := range poems {
		dynasty := scrollDynasty(p.Dynasty)
		author := strings.TrimSpace(p.Author)
		if dynasty == "" || author == "" {
			continue
		}
		if byDyn[dynasty] == nil {
			byDyn[dynasty] = map[string]*agg{}
			seen.add(dynasty)
		}
		st := byDyn[dynasty][author]
		if st == nil {
			st = &agg{}
			byDyn[dynasty][author] = st
		}
		st.poems++
		heat := p.LikeCount + p.FavoriteCount
		st.heat += heat
		year := parseYear(p.YearLabel)
		if year > 0 {
			st.years = append(st.years, year)
			if age := highlightAge(p.LifeCurve); age > 0 && year > age {
				born := year - age
				if born > 0 {
					st.birth = append(st.birth, born)
				}
			}
		}
		verse := strings.TrimSpace(p.FirstVerse)
		rank := verseRank(verse)
		if verse != "" && (st.verse == "" || heat > st.vHeat || (heat == st.vHeat && rank > st.vRank)) {
			st.verse = verse
			st.vHeat = heat
			st.vRank = rank
		}
	}

	type poet struct {
		Name     string `json:"name"`
		Year     int    `json:"year"`
		YearText string `json:"yearText"`
		Verse    string `json:"verse"`
		Poems    int    `json:"poems"`
	}
	type period struct {
		Name  string `json:"name"`
		Poets []poet `json:"poets"`
	}
	type era struct {
		Name      string   `json:"name"`
		PoemCount int      `json:"poemCount"`
		PoetCount int      `json:"poetCount"`
		Periods   []period `json:"periods"`
	}
	eras := make([]era, 0, len(byDyn))
	for _, name := range orderByPref(dynastyOrder, seen) {
		authors := byDyn[name]
		type row struct {
			poet
			period string
			order  int
		}
		rows := make([]row, 0, len(authors))
		poemCount := 0
		for author, st := range authors {
			year, birth := appearYear(st.birth, st.years)
			label, order := periodOf(name, year, birth)
			text := ""
			if year > 0 {
				text = strconv.Itoa(year)
			}
			rows = append(rows, row{
				poet:   poet{Name: author, Year: year, YearText: text, Verse: st.verse, Poems: st.poems},
				period: label,
				order:  order,
			})
			poemCount += st.poems
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].order != rows[j].order {
				return rows[i].order < rows[j].order
			}
			if (rows[i].Year == 0) != (rows[j].Year == 0) {
				return rows[i].Year != 0
			}
			if rows[i].Year != rows[j].Year {
				return rows[i].Year < rows[j].Year
			}
			return rows[i].Name < rows[j].Name
		})
		var periods []period
		for _, r := range rows {
			if len(periods) == 0 || periods[len(periods)-1].Name != r.period {
				periods = append(periods, period{Name: r.period})
			}
			periods[len(periods)-1].Poets = append(periods[len(periods)-1].Poets, r.poet)
		}
		eras = append(eras, era{Name: name, PoemCount: poemCount, PoetCount: len(rows), Periods: periods})
	}
	ok(c, gin.H{"dynasties": eras})
}

func scrollDynasty(s string) string {
	s = model.NormalizeDynasty(s)
	if v, ok := majorDynasty[s]; ok {
		return v
	}
	return s
}

func appearYear(births, years []int) (year int, fromBirth bool) {
	if len(births) > 0 {
		return medianInt(births), true
	}
	if len(years) == 0 {
		return 0, false
	}
	y := years[0]
	for _, n := range years[1:] {
		if n < y {
			y = n
		}
	}
	return y, false
}

func periodOf(dynasty string, year int, birth bool) (string, int) {
	if year <= 0 {
		return "年代未详", 1000
	}
	pair, ok := dynastyPeriods[dynasty]
	if !ok {
		return "", 0
	}
	spans := pair[1]
	if birth {
		spans = pair[0]
	}
	for i, sp := range spans {
		if year >= sp.from && year <= sp.to {
			return sp.name, i
		}
	}
	return spans[len(spans)-1].name, len(spans) - 1
}

func verseRank(s string) int {
	rank := 0
	if strings.Contains(s, "，") {
		rank += 2
	}
	if strings.ContainsAny(s, "。？！") {
		rank += 1
	}
	return rank
}
