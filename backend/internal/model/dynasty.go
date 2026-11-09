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
