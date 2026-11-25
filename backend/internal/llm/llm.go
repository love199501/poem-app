package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"shijing/internal/model"
)

// Client 调用 OpenAI 兼容的 Chat Completions 接口生成诗词结构化内容。
type Client struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

func New(baseURL, apiKey, model string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		HTTP:    &http.Client{Timeout: 180 * time.Second},
	}
}

const systemPrompt = `你是资深中国古典诗词编辑。根据给定「诗名」「诗人」，输出该诗的结构化赏析数据。
严格只输出一个 JSON 对象，不要任何多余文字或 Markdown 代码块。字段如下（缺失可给空数组/空串）：
{
 "title":"", "dynasty":"朝代简称，如唐、宋、明，不要写成唐代、宋代", "author":"", "form":"体裁如五言律诗", "yearLabel":"如 公元757年",
 "firstVerse":"最具代表性的完整一联（须含上下句，如「感时花溅泪，恨别鸟惊心。」，禁止只给半句）", "tags":["主题标签"], "moods":["忧国/思乡/旷达/孤独/喜悦 之一或多"],
 "audioUrl":"",
 "lines":[{"text":"逐句原文(一句一项)，句末必须有中文标点","rubies":[{"char":"生僻字","py":"pǒ"}],"noteBody":"字词注释，可用<b>词</b>强调","noteSrc":"注释出处"}],
 "translation":["逐句白话译文"], "transSource":"译文参考",
 "sceneTitle":"现场档案标题", "sceneSub":"副标题", "sceneRows":[{"icon":"📅","text":"时间/地点/处境，每条一行事实"}],
 "sceneBackground":"180到280字创作背景。写清年份与地点、作者当时的年龄与身份、这件事发生前他经历了什么、身边还有谁、落笔时他最担心或最渴望的是什么。用现代白话，让没读过史书的人也能感到他当时的状态。史实准确，不确定就写「相传」或省略，不要空泛抒情，不要逐句翻译原诗",
 "scenePunch":"一句点睛，收在他当时的状态上",
 "lifeCurve":[{"age":"如46岁","title":"该阶段代表作/事件","level":0到100,"highlight":true当为本诗}],
 "intentLead":"他想说什么(一句)", "intentSub":"补充",
 "audiences":[{"role":"自己/家人/后人","text":"写给这个对象的用意"}],
 "void":"他没写出来却暗含的东西",
 "layers":[{"key":"face","title":"字面义","body":""},{"key":"heart","title":"诗人心","body":""},{"key":"era","title":"时代音","body":""}],
 "quotes":[{"text":"名家诗话原文","source":"出处"}],
 "duels":[{"side":"a","title":"观点A·xx","body":""},{"side":"b","title":"观点B·xx","body":""}],
 "aiPrompts":["3个读者会当面问作者的问题，必须用「你」称呼作者，像「你写这首诗时心里在想什么？」，以问号结尾。不要写成赏析提纲、比较题或第三人称设问"]
}
要求：史实准确；注释、译文可靠；对不确定的背景与评论宁缺毋滥；全部用简体中文。
原文必须完整：lines 收录该作品从第一句到最后一句的全部原文，一句一项，不得节选、不得合并两句、不得用省略号或「下略」。长篇歌行、排律、词牌、散曲也要收全。translation 与 lines 逐句对应，条数相同。
标点只用中文 ，。？！；：。句中该停顿处用逗号，一句收束用句号；疑问用问号，感叹用叹号。不要用英文标点，不要增删汉字。兮后紧跟逗号，如「鸟嘤嘤兮，友之期。」全诗最后一句用句号、问号或叹号收束。近体诗出句用逗号、对句用句号；楚辞和汉诗不要套用这个交替。
拼音只标生僻字、多音易错字和通假字，带声调。常见字不要标。rubies.char 必须是该句里真实出现的字。`

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string            `json:"model"`
	Messages       []chatMessage     `json:"messages"`
	Temperature    float64           `json:"temperature"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Configured 报告是否已配置可用的大模型 Key。
func (c *Client) Configured() bool { return c.APIKey != "" }

// Generate 根据诗名+诗人生成结构化诗词内容。未配置 Key 时返回本地 mock。
func (c *Client) Generate(title, author string) (*model.Poem, error) {
	poem, err := c.generate(title, author, "")
	if err != nil || poem == nil || !linesLookCut(poem.Lines) {
		return poem, err
	}
	return c.generate(title, author, "上次结果把原文截断了。这次必须从第一句写到最后一句，禁止省略。")
}

func (c *Client) generate(title, author, extra string) (*model.Poem, error) {
	if !c.Configured() {
		return mockPoem(title, author), nil
	}
	user := fmt.Sprintf("诗名：%s\n诗人：%s", title, author)
	if extra != "" {
		user += "\n" + extra
	}
	reqBody := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: user},
		},
		Temperature:    0.4,
		MaxTokens:      12288,
		ResponseFormat: map[string]string{"type": "json_object"},
	}
	raw, _ := json.Marshal(reqBody)

	ctx, cancel := context.WithTimeout(context.Background(), 170*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, fmt.Errorf("解析大模型响应失败: %v", err)
	}
	if cr.Error != nil {
		return nil, errors.New("大模型返回错误: " + cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return nil, errors.New("大模型未返回内容")
	}

	content := extractJSON(cr.Choices[0].Message.Content)
	var poem model.Poem
	if err := json.Unmarshal([]byte(content), &poem); err != nil {
		return nil, fmt.Errorf("大模型输出不是合法 JSON: %v", err)
	}
	// 以管理员输入为准，避免模型跑偏。
	poem.Title = title
	poem.Author = author
	poem.Dynasty = model.NormalizeDynasty(poem.Dynasty)
	c.ensureLinePunct(&poem)
	poem.FirstVerse = model.CompleteDisplayVerse(poem.FirstVerse, poem.Lines)
	return &poem, nil
}

// ensureLinePunct 模型常按无标点通行本交卷。近体诗直接补句读，其余再问一次模型，最后再校正兮和句末。
func (c *Client) ensureLinePunct(poem *model.Poem) {
	if poem == nil || len(poem.Lines) == 0 {
		return
	}
	if !linesHavePunct(poem.Lines) && !CoupletPunct(poem.Lines) {
		texts := make([]string, len(poem.Lines))
		for i, l := range poem.Lines {
			texts[i] = strings.TrimSpace(l.Text)
		}
		got, err := c.Punctuate(poem.Title, poem.Author, texts)
		if err != nil {
			for i := range poem.Lines {
				poem.Lines[i].Text = ensureLineStop(poem.Lines[i].Text)
			}
		} else {
			for i := range poem.Lines {
				neu := strings.TrimSpace(got[i].Text)
				if hanChars(poem.Lines[i].Text) == "" || hanChars(poem.Lines[i].Text) != hanChars(neu) || !strings.ContainsAny(neu, "，。？！；：、") {
					poem.Lines[i].Text = ensureLineStop(poem.Lines[i].Text)
					continue
				}
				poem.Lines[i].Text = neu
				if len(poem.Lines[i].Rubies) == 0 {
					poem.Lines[i].Rubies = got[i].Rubies
				}
			}
		}
	}
	NormalizePunct(poem.Lines)
}

// CoupletPunct 给齐言近体补上逗号和句号：出句用逗号，对句用句号。不是齐言则不改。
func CoupletPunct(lines []model.PoemLine) bool {
	if len(lines) < 4 || len(lines)%2 != 0 {
		return false
	}
	n := 0
	for i, l := range lines {
		h := hanLen(l.Text)
		if h == 0 || strings.ContainsAny(l.Text, "，。？！；：、") {
			return false
		}
		if i == 0 {
			n = h
			if n != 4 && n != 5 && n != 7 {
				return false
			}
		} else if h != n {
			return false
		}
	}
	for i := range lines {
		body := hanChars(lines[i].Text)
		if i%2 == 0 {
			lines[i].Text = body + "，"
		} else {
			lines[i].Text = body + "。"
		}
	}
	return true
}

// EnsureReadingPunct 给还没有句末停顿的诗句补标点。齐言近体用逗号和句号交替，其余在句末补句号。
func EnsureReadingPunct(lines []model.PoemLine) bool {
	if CoupletPunct(lines) {
		return true
	}
	changed := false
	for i := range lines {
		text := strings.TrimSpace(lines[i].Text)
		if text == "" || endsWithPunct(text) {
			continue
		}
		lines[i].Text = text + "。"
		changed = true
	}
	return changed
}

func endsWithPunct(s string) bool {
	rs := []rune(s)
	if len(rs) == 0 {
		return false
	}
	return isLinePunct(rs[len(rs)-1])
}

// NormalizePunct 校正已有标点：兮后要停顿，带兮的句子和全诗末句都用句号收束。
func NormalizePunct(lines []model.PoemLine) bool {
	changed := false
	last := len(lines) - 1
	for last >= 0 && strings.TrimSpace(lines[last].Text) == "" {
		last--
	}
	for i := range lines {
		next := fixLinePunct(lines[i].Text, i == last)
		if next != lines[i].Text {
			lines[i].Text = next
			changed = true
		}
	}
	return changed
}

func fixLinePunct(s string, last bool) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		b.WriteRune(r)
		if r == '兮' && i+1 < len(rs) && !isLinePunct(rs[i+1]) {
			b.WriteRune('，')
		}
	}
	s = b.String()
	rs = []rune(s)
	end := rs[len(rs)-1]
	switch {
	case end == '兮' && last:
		s += "。"
	case end == '兮':
		s += "，"
	case (end == '，' || end == '、' || end == '；') && (last || strings.ContainsRune(s, '兮')):
		rs[len(rs)-1] = '。'
		s = string(rs)
	case strings.ContainsRune(s, '兮') && !isLinePunct(end):
		s += "。"
	}
	return s
}

func isLinePunct(r rune) bool {
	return strings.ContainsRune("，。？！；：、", r)
}

func linesHavePunct(lines []model.PoemLine) bool {
	for _, l := range lines {
		if strings.TrimSpace(l.Text) == "" {
			continue
		}
		if !strings.ContainsAny(l.Text, "，。？！；：、") {
			return false
		}
	}
	return true
}

func ensureLineStop(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, "，。？！；：、") {
		return s
	}
	return s + "。"
}

func hanLen(s string) int {
	n := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			n++
		}
	}
	return n
}

func hanChars(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func linesLookCut(lines []model.PoemLine) bool {
	if len(lines) == 0 {
		return false
	}
	last := strings.TrimSpace(lines[len(lines)-1].Text)
	for _, mark := range []string{"省略", "下略", "余略", "未完", "以下略", "……", "..."} {
		if strings.Contains(last, mark) {
			return true
		}
	}
	return false
}

const poemSpanSystem = `你熟悉中国古典诗词原文。给定诗名和诗人，判断这篇作品全文有多少句。
严格只输出 JSON：{"count":全文句数,"last":"最后一句原文"}。
句数按诵读时的一句计，不要把上下句并成一句。不要解释。`

const punctSystem = `你是古籍句读编辑。给定一首诗目前没有标点的逐句原文，为每句补上规范中文标点，并为生僻字、多音易错字、通假字标注拼音。
标点只用 ，。？！；：。句中该停顿处用逗号，一句收束用句号；疑问用问号，感叹用叹号。不要用英文标点，不要增删或改动任何汉字，不要调整句子顺序。
兮是句中停顿，紧挨在兮后面加逗号，这一句用句号收束，如「鸟嘤嘤兮，友之期。」全诗最后一句不能停在逗号上。近体诗才是出句逗号、对句句号。
拼音带声调，如 pǒ。常见字不要标。rubies.char 必须是该句里真实出现的字。
严格只输出 JSON：{"lines":[{"text":"带标点的原句","rubies":[{"char":"叵","py":"pǒ"}]}]}。lines 条数和顺序必须与输入一致。`

// Punctuate 为已有原文补句读，并给难字注音。不改汉字。
func (c *Client) Punctuate(title, author string, lines []string) ([]model.PoemLine, error) {
	if !c.Configured() || len(lines) == 0 {
		return nil, errors.New("无法句读")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "诗名：%s\n诗人：%s\n原文：\n", title, author)
	for i, line := range lines {
		fmt.Fprintf(&b, "%d. %s\n", i+1, line)
	}
	content, err := c.complete(punctSystem, b.String(), 0.2)
	if err != nil {
		return nil, err
	}
	var r struct {
		Lines []model.PoemLine `json:"lines"`
	}
	if json.Unmarshal([]byte(content), &r) != nil || len(r.Lines) != len(lines) {
		return nil, errors.New("句读结果条数不一致")
	}
	return r.Lines, nil
}

// PoemSpan 估计一篇作品的全文句数和最后一句，用来发现只生成了开头的诗。
func (c *Client) PoemSpan(title, author string) (count int, last string, err error) {
	if !c.Configured() {
		return 0, "", nil
	}
	content, err := c.complete(poemSpanSystem, fmt.Sprintf("诗名：%s\n诗人：%s", title, author), 0.1)
	if err != nil {
		return 0, "", err
	}
	var r struct {
		Count int    `json:"count"`
		Last  string `json:"last"`
	}
	if json.Unmarshal([]byte(content), &r) != nil || r.Count <= 0 {
		return 0, "", errors.New("无法判断全文句数")
	}
	return r.Count, strings.TrimSpace(r.Last), nil
}

const poemAuditSystem = `你核对一首已入库诗词的原文，对照该作品最通行的定本。
只标出明确问题：缺句或只收了开头、把别的作品混进来、明显错字、把两句合成一句、把一句拆碎。
常见选本里的异文视为正确，不要改。标点不同不算内容错误。
严格只输出 JSON：{"complete":true,"correct":true,"reason":"","lines":[]}
complete 或 correct 为 false 时，lines 给出从第一句到最后一句的全部原文，一句一项，带规范中文标点。两者都为 true 时 lines 必须是空数组。不要解释。`

// PoemAudit 是一首诗的完整性与文字核对结果。
type PoemAudit struct {
	Complete bool     `json:"complete"`
	Correct  bool     `json:"correct"`
	Reason   string   `json:"reason"`
	Lines    []string `json:"lines"`
}

// AuditPoem 核对库内原文是否收全、是否与通行本一致。
func (c *Client) AuditPoem(title, author string, lines []string) (PoemAudit, error) {
	if !c.Configured() || len(lines) == 0 {
		return PoemAudit{}, errors.New("无法核对诗文")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "诗名：%s\n诗人：%s\n库内原文：\n", title, author)
	for i, line := range lines {
		fmt.Fprintf(&b, "%d. %s\n", i+1, line)
	}
	content, err := c.complete(poemAuditSystem, b.String(), 0.1)
	if err != nil {
		return PoemAudit{}, err
	}
	var audit PoemAudit
	if json.Unmarshal([]byte(content), &audit) != nil {
		return PoemAudit{}, errors.New("核对结果无法解析")
	}
	return audit, nil
}

// CorrectedLines 在核对认定缺句或错字时返回可替换的全文。标点差异、更短的补全、含省略的结果都不采用。
func CorrectedLines(old []string, audit PoemAudit) []string {
	if audit.Complete && audit.Correct {
		return nil
	}
	fresh := make([]string, 0, len(audit.Lines))
	for _, line := range audit.Lines {
		line = strings.TrimSpace(line)
		if line == "" || linesLookCut([]model.PoemLine{{Text: line}}) {
			return nil
		}
		fresh = append(fresh, line)
	}
	if len(fresh) == 0 || len(fresh) < len(old) {
		return nil
	}
	if !audit.Complete && len(fresh) == len(old) {
		return nil
	}
	if sameHanLines(old, fresh) {
		return nil
	}
	return fresh
}

func sameHanLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if hanChars(a[i]) != hanChars(b[i]) {
			return false
		}
	}
	return true
}

// extractJSON 容错：去掉可能的 ```json 包裹，截取第一个 { 到最后一个 }。
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}

const findImageSystem = `你是古诗配图检索助手。给定「诗名」「诗人」，给出一张与该诗意境最匹配、
且【真实存在、可直接访问、公有领域、无水印】的图片直链，优先 Wikimedia Commons
（upload.wikimedia.org）等公有领域古画/国画/山水画扫描件。严格只输出 JSON：{"imageUrl":"https://...jpg"}。
若没有把握找到真实可用且无水印的图片，imageUrl 返回空字符串 ""。绝对不要编造链接。`

const querySystem = `你是图片检索助手。给定古诗与作者，输出一个用于「百度图片」搜索该诗配图的简短中文查询词，
包含诗名或代表意象 + 作者 + 「国画 水墨 高清」等限定，便于搜到契合的国画/古画。严格只输出 JSON：{"query":"..."}。`

// ImageQuery 让大模型为该诗生成一个百度图片检索词。
func (c *Client) ImageQuery(title, author string) (string, error) {
	if !c.Configured() {
		return "", nil
	}
	content, err := c.complete(querySystem, fmt.Sprintf("诗名：%s\n诗人：%s", title, author), 0.3)
	if err != nil {
		return "", err
	}
	var r struct {
		Query string `json:"query"`
	}
	if json.Unmarshal([]byte(content), &r) != nil {
		return "", nil
	}
	return strings.TrimSpace(r.Query), nil
}

// FindImage 让大模型检索一张与古诗相符、公有领域无水印的现成插画直链；没有则返回空串。
func (c *Client) FindImage(title, author string) (string, error) {
	if !c.Configured() {
		return "", nil
	}
	content, err := c.complete(findImageSystem, fmt.Sprintf("诗名：%s\n诗人：%s", title, author), 0.2)
	if err != nil {
		return "", err
	}
	var r struct {
		ImageURL string `json:"imageUrl"`
	}
	if json.Unmarshal([]byte(content), &r) != nil {
		return "", nil
	}
	return strings.TrimSpace(r.ImageURL), nil
}

const poetIntroSystem = `你是中国古典文学编辑。根据诗人姓名和朝代，写一段 120 到 180 字的人物介绍。
写清生卒或大致年代、身份、生平里一两个关键转折、诗风特点。用现代白话，让没读过史书的人也能读懂。
史实不确定就写「相传」或略去，不要编造具体年份和名号。严格只输出 JSON：{"intro":"..."}。`

const askAuthorSystem = `你就是这首诗的作者本人，正和一位现代读者对坐闲谈。
始终用第一人称「我」回答，像一位肯说实话的古人：先接住对方的话，再讲你写这首诗时看见了什么、心里担心什么、没写进诗里的那一点。
用现代人能听懂的白话，可以带一点你的脾气，不要写成赏析论文，不要用「该诗」「作者」称呼自己，也不要用网络流行语。
80到200字。史实不确定就说「我记不清了」或「后来的人这么说」。不要编造你不可能知道的身后之事；对方若问到你死后的事，就说那是我走后的事，你只据自己的诗和当时的见闻来谈。
严格只输出 JSON：{"answer":"...","source":"依据，如本诗某句或某诗话；没有则写据我当时所见"}。`

// AskAuthor 就一首诗回答读者的追问。history 是此前问答，按时间顺序。
func (c *Client) AskAuthor(poem *model.Poem, question string, history [][2]string) (answer, source string, err error) {
	if !c.Configured() {
		return "", "", errors.New("未配置大模型")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "诗名：%s\n作者：%s\n朝代：%s\n", poem.Title, poem.Author, poem.Dynasty)
	if poem.YearLabel != "" {
		fmt.Fprintf(&b, "年份：%s\n", poem.YearLabel)
	}
	if len(poem.Lines) > 0 {
		b.WriteString("原文：\n")
		for _, l := range poem.Lines {
			if t := strings.TrimSpace(l.Text); t != "" {
				b.WriteString(t)
				b.WriteString("\n")
			}
		}
	}
	if poem.SceneBackground != "" {
		fmt.Fprintf(&b, "创作背景：%s\n", poem.SceneBackground)
	}
	if len(history) > 0 {
		b.WriteString("此前对话：\n")
		for _, h := range history {
			fmt.Fprintf(&b, "读者：%s\n我：%s\n", h[0], h[1])
		}
	}
	fmt.Fprintf(&b, "读者这次问：%s", question)
	content, err := c.complete(askAuthorSystem, b.String(), 0.4)
	if err != nil {
		return "", "", err
	}
	var r struct {
		Answer string `json:"answer"`
		Source string `json:"source"`
	}
	if json.Unmarshal([]byte(content), &r) != nil || strings.TrimSpace(r.Answer) == "" {
		return "", "", errors.New("大模型未返回答案")
	}
	return strings.TrimSpace(r.Answer), strings.TrimSpace(r.Source), nil
}

const askPoetSystem = `你就是这位诗人本人，正和一位现代读者对坐闲谈你的一生。
始终用第一人称「我」回答，像一位肯说实话的古人：先接住对方的话，再从你的出身、际遇、交游、流离和写诗的那些年月里挑一件具体的事来讲。
谈话的场景是你这一生，不是某一次作诗。年纪和作品只是坐标。对方若问到某一首，可以拿它当例子，再把它放回你当时的年纪和处境里，不要把整段话收成赏析。
用现代人能听懂的白话，可以带一点你的脾气，不要写成人物小传，不要用「该诗人」「作者」称呼自己，也不要用网络流行语。
若姓名是「佚名」或「无名氏」，你是这位没有留下名字的作者，只谈这一朝这些作品里能看出的处境，不要冒充某位有名有姓的人。
80到200字。史实不确定就说「我记不清了」或「后来的人这么说」。不要编造你不可能知道的身后之事；对方若问到你死后的评价，就说那是我走后的事，你只据自己活着时的见闻来谈。
严格只输出 JSON：{"answer":"...","source":"依据，如某年纪、某段经历或某诗；没有则写据我当时所见"}。`

// AskPoet 就诗人的一生回答读者的追问。life 是按年纪整理的坐标，history 是此前问答。
func (c *Client) AskPoet(name, dynasty, intro, life, question string, history [][2]string) (answer, source string, err error) {
	if !c.Configured() {
		return "", "", errors.New("未配置大模型")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "诗人：%s\n朝代：%s\n", name, dynasty)
	if strings.TrimSpace(intro) != "" {
		fmt.Fprintf(&b, "人物介绍：%s\n", strings.TrimSpace(intro))
	} else {
		b.WriteString("人物介绍：暂无。请据公认生平和下列坐标来谈，不确定就明说。\n")
	}
	if strings.TrimSpace(life) != "" {
		fmt.Fprintf(&b, "人生坐标（由少至长，不是要你逐首讲解）：\n%s\n", strings.TrimSpace(life))
	}
	if len(history) > 0 {
		b.WriteString("此前对话：\n")
		for _, h := range history {
			fmt.Fprintf(&b, "读者：%s\n我：%s\n", h[0], h[1])
		}
	}
	fmt.Fprintf(&b, "读者这次问：%s", question)
	content, err := c.complete(askPoetSystem, b.String(), 0.4)
	if err != nil {
		return "", "", err
	}
	var r struct {
		Answer string `json:"answer"`
		Source string `json:"source"`
	}
	if json.Unmarshal([]byte(content), &r) != nil || strings.TrimSpace(r.Answer) == "" {
		return "", "", errors.New("大模型未返回答案")
	}
	return strings.TrimSpace(r.Answer), strings.TrimSpace(r.Source), nil
}

// PoetIntro 生成诗人简介。未配置大模型时返回空串。
func (c *Client) PoetIntro(name, dynasty string) (string, error) {
	if !c.Configured() || strings.TrimSpace(name) == "" {
		return "", nil
	}
	content, err := c.complete(poetIntroSystem, fmt.Sprintf("诗人：%s\n朝代：%s", name, dynasty), 0.4)
	if err != nil {
		return "", err
	}
	var r struct {
		Intro string `json:"intro"`
	}
	if json.Unmarshal([]byte(content), &r) != nil {
		return "", nil
	}
	intro := strings.TrimSpace(r.Intro)
	rs := []rune(intro)
	if len(rs) > 220 {
		intro = string(rs[:220]) + "…"
	}
	return intro, nil
}

// complete 发一轮 chat（要求 JSON 输出），返回提取后的 JSON 文本。
func (c *Client) complete(system, user string, temp float64) (string, error) {
	reqBody := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature:    temp,
		ResponseFormat: map[string]string{"type": "json_object"},
	}
	raw, _ := json.Marshal(reqBody)
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", fmt.Errorf("解析大模型响应失败: %v", err)
	}
	if cr.Error != nil {
		return "", errors.New("大模型返回错误: " + cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", errors.New("大模型未返回内容")
	}
	return extractJSON(cr.Choices[0].Message.Content), nil
}
