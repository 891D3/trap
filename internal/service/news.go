// news.go —— 热点资讯服务（v0.3.1+ 新增）：
// 接入 news.orz.ai 聚合热点接口，按平台（渠道）拉取榜单；支持内存缓存、
// 定时后台刷新与手动刷新（前端标签页旁的刷新按钮走这里）。
//
// 网络边界说明（AGENTS.md）：这是用户明确要求接入的新闻功能，网络访问仅
// 指向用户配置的渠道源（当前只有 weibo），不涉及用户数据上传。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// NewsItem 单条热点（对齐接口字段，仅取前端展示所需）。
type NewsItem struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Content     string `json:"content,omitempty"`
	PublishTime string `json:"publishTime,omitempty"`
}

// 支持的渠道（标签页）定义：key = 接口 platform 参数。
var newsPlatforms = map[string]string{
	"tenxunwang": "https://news.orz.ai/api/v1/dailynews/",
	"douyin":     "https://news.orz.ai/api/v1/dailynews/",
	"eastmoney":  "https://news.orz.ai/api/v1/dailynews/",
	"xueqiu":     "https://news.orz.ai/api/v1/dailynews/",
}

const (
	newsFetchTimeout = 8 * time.Second  // 单次拉取超时
	newsCacheTTL     = 10 * time.Minute // 定时刷新周期（也是缓存有效期）
	newsMaxItems     = 50               // 榜单最多展示条数
)

// NewsService 提供热点资讯（缓存 + 手动刷新）。
//
// 生命周期：main Bind 时注册零值；WhenReady 中 StartNewsScheduler 启动
// 定时刷新（goroutine 可经 stop 取消，符合 AGENTS.md goroutine 退出约束）。
type NewsService struct {
	mu      sync.RWMutex
	cache   map[string][]NewsItem // platform → 最近一次成功拉取的榜单
	fetched map[string]time.Time  // platform → 缓存时间
	client  *http.Client
	stop    chan struct{}
}

// NewsChanged 是新闻更新事件：定时/手动刷新成功后广播，前端刷新列表。
var NewsChanged = mygo.NewEvent[NewsChangedPayload]("news.changed")

// NewsChangedPayload 新闻刷新事件载荷。
type NewsChangedPayload struct {
	Platform string `json:"platform"`
	Count    int    `json:"count"`
}

// Fetch 返回指定渠道的热点榜单（优先缓存；force=true 时绕过 TTL 强制拉取）。
func (s *NewsService) Fetch(ctx context.Context, platform string, force bool) ([]NewsItem, error) {
	if newsPlatforms[platform] == "" {
		return nil, fmt.Errorf("不支持的资讯渠道: %s", platform)
	}
	s.mu.RLock()
	items, ok := s.cache[platform]
	fetchedAt := s.fetched[platform]
	s.mu.RUnlock()
	if !force && ok && time.Since(fetchedAt) < newsCacheTTL {
		return items, nil
	}
	fresh, err := s.fetchRemote(ctx, platform)
	if err != nil {
		// 拉取失败但有旧缓存 → 降级返回缓存（错误提示给前端 toast）。
		if ok {
			return items, fmt.Errorf("刷新失败，展示缓存数据: %w", err)
		}
		return nil, err
	}
	s.mu.Lock()
	if s.cache == nil { // 零值服务懒初始化（main 与测试都直接用零值）
		s.cache = make(map[string][]NewsItem)
		s.fetched = make(map[string]time.Time)
	}
	s.cache[platform] = fresh
	s.fetched[platform] = time.Now()
	s.mu.Unlock()
	return fresh, nil
}

// fetchRemote 请求渠道源并解析。
func (s *NewsService) fetchRemote(ctx context.Context, platform string) ([]NewsItem, error) {
	endpoint, err := url.Parse(newsPlatforms[platform])
	if err != nil {
		return nil, fmt.Errorf("渠道地址非法: %w", err)
	}
	q := endpoint.Query()
	q.Set("platform", platform)
	endpoint.RawQuery = q.Encode()

	fetchCtx, cancel := context.WithTimeout(ctx, newsFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := (func() (*http.Response, error) {
		s.mu.RLock()
		c := s.client
		s.mu.RUnlock()
		if c != nil {
			return c.Do(req)
		}
		return http.DefaultClient.Do(req)
	})()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("渠道返回 %d", resp.StatusCode)
	}
	var body struct {
		Status string `json:"status"`
		Data   []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Content     string `json:"content"`
			PublishTime string `json:"publish_time"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	items := make([]NewsItem, 0, len(body.Data))
	for i, d := range body.Data {
		if i >= newsMaxItems {
			break
		}
		items = append(items, NewsItem{Title: d.Title, URL: d.URL, Content: d.Content, PublishTime: d.PublishTime})
	}
	return items, nil
}

// StartNewsScheduler 启动定时刷新（启动即拉一次；每 TTL 周期刷新）。
// 返回停止函数（WhenReady 中 defer/退出时调用；App 退出即进程退出，简单起见
// 由 main 持有）。
func (s *NewsService) StartNewsScheduler(ctx context.Context) {
	if s.client == nil {
		s.client = &http.Client{Timeout: newsFetchTimeout}
	}
	if s.stop == nil {
		s.stop = make(chan struct{})
	}
	go func() {
		ticker := time.NewTicker(newsCacheTTL)
		defer ticker.Stop()
		// 启动即预热一次（失败只记日志，等首次前端 Fetch 或下个周期再试）。
		for platform := range newsPlatforms {
			if _, err := s.Fetch(ctx, platform, true); err != nil {
				log.Printf("新闻预热失败(%s): %v", platform, err)
			}
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			case <-ticker.C:
				for platform := range newsPlatforms {
					items, err := s.Fetch(ctx, platform, true)
					if err != nil {
						log.Printf("新闻定时刷新失败(%s): %v", platform, err)
						continue
					}
					_ = NewsChanged.Broadcast(NewsChangedPayload{Platform: platform, Count: len(items)})
				}
			}
		}
	}()
}
