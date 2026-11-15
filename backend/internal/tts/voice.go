package tts

import "strings"

// Recitation 一首诗的朗诵方案：音色按诗人性别，再按内容情绪微调。
// 只用已在当前豆包密钥下试通的 seed-tts 2.0 音色。
type Recitation struct {
	Speaker string // 豆包 speaker
	Rate    int    // 语速，负值更慢
	Key     string // 写入音频地址，换音色后自动重合成
	Label   string
}

const (
	maleDeep   = "zh_male_m191_uranus_bigtts"           // 云舟，沉稳
	maleCool   = "zh_male_qingcang_uranus_bigtts"       // 青苍，清朗
	maleWarm   = "zh_male_taocheng_uranus_bigtts"       // 小天，温厚
	maleBright = "zh_male_shaonianzixin_uranus_bigtts"  // 少年，明快
	femaleSoft = "zh_female_xiaohe_uranus_bigtts"       // 小何，柔和
	femaleCool = "zh_female_gaolengyujie_uranus_bigtts" // 御姐，清冷
	femaleLive = "zh_female_cancan_uranus_bigtts"       // 灿灿，明快
	femaleWarm = "zh_female_vv_uranus_bigtts"           // Vivi，从容
)

// 女诗人。未收录的作者按男声处理（古典诗作者绝大多数为男性）。
var femalePoets = map[string]bool{
	"李清照": true, "朱淑真": true, "薛涛": true, "鱼玄机": true, "李冶": true,
	"蔡文姬": true, "蔡琰": true, "卓文君": true, "花蕊夫人": true, "上官婉儿": true,
	"武则天": true, "秋瑾": true, "冰心": true, "席慕蓉": true, "舒婷": true,
	"林徽因": true, "萧红": true, "石评梅": true, "贺双卿": true, "柳如是": true,
	"顾太清": true, "吴藻": true, "徐灿": true, "魏夫人": true, "李季兰": true,
	"聂隐娘": true, "管道升": true, "黄峨": true,
}

// ChooseRecitation 按诗人性别和诗的情绪选出音色与语速。
func ChooseRecitation(author string, moods, tags []string, text string) Recitation {
	female := femalePoets[strings.TrimSpace(author)]
	mood := detectMood(moods, tags, text)
	r := Recitation{Rate: -25, Key: "m-steady"}
	switch mood {
	case "grief":
		r.Speaker, r.Rate, r.Key, r.Label = maleDeep, -35, "m-grief", "沉郁"
		if female {
			r.Speaker, r.Key, r.Label = femaleCool, "f-grief", "清冷"
		}
	case "longing":
		r.Speaker, r.Rate, r.Key, r.Label = maleWarm, -30, "m-longing", "温厚"
		if female {
			r.Speaker, r.Key, r.Label = femaleSoft, "f-longing", "柔和"
		}
	case "lonely":
		r.Speaker, r.Rate, r.Key, r.Label = maleCool, -32, "m-lonely", "清朗"
		if female {
			r.Speaker, r.Key, r.Label = femaleCool, "f-lonely", "清冷"
		}
	case "joy":
		r.Speaker, r.Rate, r.Key, r.Label = maleBright, -12, "m-joy", "明快"
		if female {
			r.Speaker, r.Key, r.Label = femaleLive, "f-joy", "明快"
		}
	case "free":
		r.Speaker, r.Rate, r.Key, r.Label = maleCool, -18, "m-free", "洒脱"
		if female {
			r.Speaker, r.Key, r.Label = femaleWarm, "f-free", "从容"
		}
	default:
		r.Speaker, r.Label = maleDeep, "沉稳"
		if female {
			r.Speaker, r.Key, r.Label = femaleWarm, "f-steady", "从容"
		}
	}
	return r
}

func detectMood(moods, tags []string, text string) string {
	blob := strings.Join(append(append([]string{}, moods...), tags...), " ") + " " + text
	switch {
	case containsAny(blob, "忧国", "愤懑", "悲壮", "沉郁", "国破", "烽火", "战", "愤", "恨"):
		return "grief"
	case containsAny(blob, "喜悦", "狂喜", "豪放", "豪迈", "欢", "少年"):
		return "joy"
	case containsAny(blob, "思乡", "离别", "爱情", "感伤", "思家", "送别", "家书", "归"):
		return "longing"
	case containsAny(blob, "孤独", "寂", "冷", "空山", "孤"):
		return "lonely"
	case containsAny(blob, "旷达", "闲适", "哲理", "洒脱"):
		return "free"
	default:
		return ""
	}
}

func containsAny(s string, keys ...string) bool {
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
