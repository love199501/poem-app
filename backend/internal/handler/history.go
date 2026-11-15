package handler

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"shijing/internal/model"
)

// HistoryScroll 历史卷轴：按朝代时间线列出诗人，诗人按作品热度排序。
func (h *Handler) HistoryScroll(c *gin.Context) {
	var poems []model.Poem
	err := h.DB.Model(&model.Poem{}).
		Select("dynasty,author,like_count,favorite_count").
		Where("status <> ?", "draft").
		Find(&poems).Error
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	type stat struct {
		poems int
		heat  int
	}
	byDyn := map[string]map[string]*stat{}
	seen := newFreq()
	for _, p := range poems {
		dynasty := model.NormalizeDynasty(p.Dynasty)
		author := strings.TrimSpace(p.Author)
		if dynasty == "" || author == "" {
			continue
		}
		if byDyn[dynasty] == nil {
			byDyn[dynasty] = map[string]*stat{}
			seen.add(dynasty)
		}
		if byDyn[dynasty][author] == nil {
			byDyn[dynasty][author] = &stat{}
		}
		byDyn[dynasty][author].poems++
		byDyn[dynasty][author].heat += p.LikeCount + p.FavoriteCount
	}

	type poet struct {
		Name  string `json:"name"`
		Poems int    `json:"poems"`
		Heat  int    `json:"heat"`
	}
	type era struct {
		Name      string `json:"name"`
		PoemCount int    `json:"poemCount"`
		Poets     []poet `json:"poets"`
	}
	eras := make([]era, 0, len(byDyn))
	for _, name := range orderByPref(dynastyOrder, seen) {
		authors := byDyn[name]
		list := make([]poet, 0, len(authors))
		poemCount := 0
		for author, st := range authors {
			list = append(list, poet{Name: author, Poems: st.poems, Heat: st.heat})
			poemCount += st.poems
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].Heat != list[j].Heat {
				return list[i].Heat > list[j].Heat
			}
			if list[i].Poems != list[j].Poems {
				return list[i].Poems > list[j].Poems
			}
			return list[i].Name < list[j].Name
		})
		eras = append(eras, era{Name: name, PoemCount: poemCount, Poets: list})
	}
	ok(c, gin.H{"dynasties": eras})
}
