package model

import "strings"

// dynastyAlias 把「唐代 / 唐朝」这类写法收成统一简称，避免筛选项同时出现「唐」和「唐代」。
// 「五代」「当代」「近现代」等本身以「代」结尾的名称不在此列。
var dynastyAlias = map[string]string{
	"唐朝": "唐", "唐代": "唐",
	"宋朝": "宋", "宋代": "宋",
	"元朝": "元", "元代": "元",
	"明朝": "明", "明代": "明",
	"清朝": "清", "清代": "清",
	"汉朝": "汉", "汉代": "汉",
	"隋朝": "隋", "隋代": "隋",
	"秦朝": "秦", "秦代": "秦",
	"晋朝": "晋", "晋代": "晋",
	"周朝": "周", "周代": "周",
	"辽朝": "辽", "辽代": "辽",
	"金朝": "金", "金代": "金",
	// 近现代已经包含现代、民国。截断写法一并收进去。
	"现代": "近现代",
	"当代": "近现代",
	"民国": "近现代", "中华民国": "近现代",
	"现":  "近现代",
	"近":  "近现代",
	// 先秦里的诸侯与阶段。
	"周": "先秦", "西周": "先秦", "东周": "先秦",
	"春秋": "先秦", "战国": "先秦", "楚": "先秦",
	// 魏晋、南北朝里的小朝代。南唐是五代十国，不是唐朝。
	"魏": "魏晋", "曹魏": "魏晋",
	"晋": "魏晋", "西晋": "魏晋", "东晋": "魏晋",
	"南朝宋": "南北朝", "刘宋": "南北朝",
	"南齐": "南北朝", "萧齐": "南北朝",
	"南朝": "南北朝", "北朝": "南北朝",
	"梁": "南北朝", "南梁": "南北朝", "陈": "南北朝",
	"北魏": "南北朝", "东魏": "南北朝", "西魏": "南北朝",
	"北齐": "南北朝", "北周": "南北朝",
	"南唐": "五代",
}

// NormalizeDynasty 返回朝代的统一简称；无法识别时原样返回（去掉首尾空白）。
func NormalizeDynasty(s string) string {
	s = strings.TrimSpace(s)
	if v, ok := dynastyAlias[s]; ok {
		return v
	}
	return s
}

// DynastyVariants 返回某个朝代在库里可能出现的全部写法，供筛选时一并命中。
func DynastyVariants(s string) []string {
	canon := NormalizeDynasty(s)
	if canon == "" {
		return nil
	}
	out := []string{canon}
	seen := map[string]bool{canon: true}
	for alias, c := range dynastyAlias {
		if c == canon && !seen[alias] {
			out = append(out, alias)
			seen[alias] = true
		}
	}
	return out
}
