package search

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// BaiduImage 调用百度图片的 acjson 接口做文搜图（无需 Key）。
type BaiduImage struct {
	HTTP *http.Client
}

func NewBaidu() *BaiduImage {
	return &BaiduImage{HTTP: &http.Client{Timeout: 20 * time.Second}}
}

type acResp struct {
	Data []struct {
		ThumbURL  string `json:"thumbURL"`
		MiddleURL string `json:"middleURL"`
		HoverURL  string `json:"hoverURL"`
	} `json:"data"`
}

// Search 返回候选图片直链（百度 CDN 缓存图，稳定可取），按清晰度优先排序。
func (b *BaiduImage) Search(query string) ([]string, error) {
	u := fmt.Sprintf(
		"https://image.baidu.com/search/acjson?tn=resultjson_com&ipn=rj&word=%s&pn=0&rn=30",
		url.QueryEscape(query),
	)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	req.Header.Set("Referer", "https://image.baidu.com/")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := b.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var r acResp
	if e := json.Unmarshal(body, &r); e != nil {
		return nil, fmt.Errorf("百度图片响应解析失败")
	}
	var urls []string
	for _, it := range r.Data {
		for _, cand := range []string{it.MiddleURL, it.HoverURL, it.ThumbURL} {
			if len(cand) > 8 && cand[:4] == "http" {
				urls = append(urls, cand)
				break
			}
		}
	}
	return urls, nil
}
