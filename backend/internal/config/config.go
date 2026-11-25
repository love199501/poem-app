package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// loadDotEnv 从当前工作目录读取 .env（存在才加载），不覆盖已显式设置的环境变量。
// 这样在 backend/ 目录执行 `go run ./cmd/server` 即可自动读取 backend/.env。
func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		if _, ok := os.LookupEnv(key); !ok {
			_ = os.Setenv(key, val)
		}
	}
}

// Config 保存运行期配置，全部来自环境变量，便于容器化部署。
type Config struct {
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	AppPort     string
	AutoMigrate bool
	Seed        bool

	AdminKey   string // 网页后台登录口令
	LLMBaseURL string // OpenAI 兼容服务地址，如 https://api.deepseek.com/v1
	LLMAPIKey  string // 大模型 API Key（为空则走本地 mock 生成）
	LLMModel   string // 模型名，如 deepseek-chat / gpt-4o-mini

	TTSBaseURL string // 朗诵合成（OpenAI 兼容 /audio/speech）地址
	TTSAPIKey  string // 为空则不启用情感朗诵，前端回退到插件 TTS
	TTSModel   string // 如 gpt-4o-mini-tts / tts-1
	TTSVoice   string // 音色，如 onyx / alloy / ash

	TTSProvider string // 朗诵供应商：azure / openai / volcano / 留空=自动
	AzureKey    string // Azure 语音服务 Key
	AzureRegion string // 区域，如 eastus / eastasia
	AzureVoice  string // 音色，如 zh-CN-XiaoxiaoNeural
	AzureStyle  string // 风格，如 poetry-reading

	VolcAppID     string // 火山引擎语音 AppID（经典语音合成）
	VolcToken     string // 火山引擎 Access Token（经典语音合成）
	VolcCluster   string // 集群，如 volcano_tts
	VolcVoiceType string // 音色，如 BV001_streaming / 多情感音色
	VolcEmotion   string // 可选情感（需音色支持），如 sad/happy

	VolcBigKey     string // 豆包大模型语音 X-Api-Key（seed-tts，只需一个 key）
	VolcResourceID string // 资源ID，如 seed-tts-2.0
	VolcSpeaker    string // 大模型音色 speaker
	VolcSpeechRate int    // 语速，-50~100，负值更慢
	VolcBigEmotion string // 可选情感（需音色支持），如 affectionate/sad

	ImgBaseURL string // 文生图（OpenAI 兼容 /images/generations）地址
	ImgAPIKey  string // 为空则不生成插画
	ImgModel   string // 如 dall-e-3 / Kwai-Kolors/Kolors / gpt-image-1
	ImgSize    string // 如 1024x1024
	ImgSearch  string // 插画检索：baidu / off

	WechatAppID  string // 微信小程序 AppID（配置后走真实 jscode2session）
	WechatSecret string // 微信小程序 AppSecret
	VpOfferID    string // 虚拟支付 OfferID
	VpAppKey     string // 虚拟支付现网 AppKey
	VpProductID  string // 会员道具 ID
	VpEnv        int    // 0 正式，1 沙箱。iOS 不支持沙箱
	RewardAdUnit string // 激励视频广告位 ID，如 adunit-xxxx

	// CommentEnabled 评论云控。审核期间设为 false 隐藏入口；过审后改为 true，无需再发版。
	CommentEnabled bool
	// AudioEnabled 在线播放云控。未取得额外资质时设为 false，隐藏朗诵入口。
	AudioEnabled bool
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func getenv(key, def string) string {
	// 用 LookupEnv 区分「未设置」与「显式设为空」，这样空密码（DB_PASSWORD=）也能生效。
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func getenvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

// Load 读取环境变量，缺省值面向本地开发。
func Load() *Config {
	loadDotEnv()

	// 数据库：优先用微信云托管注入的 MYSQL_ADDRESS/MYSQL_USERNAME/MYSQL_PASSWORD；否则用 DB_*。
	dbHost := getenv("DB_HOST", "127.0.0.1")
	dbPort := getenv("DB_PORT", "3306")
	dbUser := getenv("DB_USER", "root")
	dbPass := getenv("DB_PASSWORD", "root")
	if addr := os.Getenv("MYSQL_ADDRESS"); addr != "" {
		if i := strings.LastIndex(addr, ":"); i > 0 {
			dbHost, dbPort = addr[:i], addr[i+1:]
		} else {
			dbHost = addr
		}
	}
	if u := os.Getenv("MYSQL_USERNAME"); u != "" {
		dbUser = u
	}
	if p, ok := os.LookupEnv("MYSQL_PASSWORD"); ok {
		dbPass = p
	}

	return &Config{
		DBHost:      dbHost,
		DBPort:      dbPort,
		DBUser:      dbUser,
		DBPassword:  dbPass,
		DBName:      getenv("DB_NAME", "shijing"),
		AppPort:     getenv("PORT", getenv("APP_PORT", "8080")), // 云托管注入 PORT
		AutoMigrate: getenv("AUTO_MIGRATE", "true") == "true",
		Seed:        getenv("SEED", "true") == "true",

		AdminKey:   getenv("ADMIN_KEY", "admin123"),
		LLMBaseURL: getenv("LLM_BASE_URL", "https://api.deepseek.com/v1"),
		LLMAPIKey:  getenv("LLM_API_KEY", ""),
		LLMModel:   getenv("LLM_MODEL", "deepseek-chat"),

		TTSBaseURL: getenv("TTS_BASE_URL", "https://api.openai.com/v1"),
		TTSAPIKey:  getenv("TTS_API_KEY", ""),
		TTSModel:   getenv("TTS_MODEL", "gpt-4o-mini-tts"),
		TTSVoice:   getenv("TTS_VOICE", "onyx"),

		TTSProvider: getenv("TTS_PROVIDER", ""),
		AzureKey:    getenv("AZURE_SPEECH_KEY", ""),
		AzureRegion: getenv("AZURE_SPEECH_REGION", "eastus"),
		AzureVoice:  getenv("AZURE_VOICE", "zh-CN-XiaoxiaoNeural"),
		AzureStyle:  getenv("AZURE_STYLE", "poetry-reading"),

		VolcAppID:     getenv("VOLC_APPID", ""),
		VolcToken:     getenv("VOLC_ACCESS_TOKEN", ""),
		VolcCluster:   getenv("VOLC_CLUSTER", "volcano_tts"),
		VolcVoiceType: getenv("VOLC_VOICE_TYPE", "BV001_streaming"),
		VolcEmotion:   getenv("VOLC_EMOTION", ""),

		VolcBigKey:     getenv("VOLC_API_KEY", ""),
		VolcResourceID: getenv("VOLC_RESOURCE_ID", "seed-tts-2.0"),
		VolcSpeaker:    getenv("VOLC_SPEAKER", "zh_female_gaolengyujie_uranus_bigtts"),
		VolcSpeechRate: getenvInt("VOLC_SPEECH_RATE", -25),
		VolcBigEmotion: getenv("VOLC_BIG_EMOTION", ""),

		ImgBaseURL: getenv("IMG_BASE_URL", "https://api.openai.com/v1"),
		ImgAPIKey:  getenv("IMG_API_KEY", ""),
		ImgModel:   getenv("IMG_MODEL", "dall-e-3"),
		ImgSize:    getenv("IMG_SIZE", "1024x1024"),
		ImgSearch:  getenv("IMG_SEARCH", "baidu"),

		WechatAppID:  getenv("WECHAT_APPID", ""),
		WechatSecret: getenv("WECHAT_SECRET", ""),
		VpOfferID:    firstEnv("WX_VP_OFFER_ID", "VP_OFFER_ID"),
		VpAppKey:     firstEnv("WX_VP_APP_KEY", "VP_APP_KEY"),
		VpProductID:  firstEnv("WX_VP_PRODUCT_ID", "VP_PRODUCT_ID"),
		VpEnv:        getenvInt("WX_VP_ENV", 0),
		RewardAdUnit: getenv("REWARD_AD_UNIT", ""),

		CommentEnabled: envBool("COMMENT_ENABLED", false),
		AudioEnabled:   envBool("AUDIO_ENABLED", false),
	}
}

// DSN 构造 GORM MySQL 连接串。parseTime 让 time.Time 正常映射，loc=Local 用本地时区。
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName,
	)
}
