package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// 用 httptest 伪渠道源验证 Fetch 的解析/缓存/降级逻辑（不访问真实外网）。
func TestNewsServiceFetch(t *testing.T) {
	srv := &NewsService{client: &http.Client{Timeout: 2 * time.Second}}

	// 1) 不支持的渠道 → 报错。
	if _, err := srv.Fetch(context.Background(), "weibo", false); err == nil {
		t.Fatal("期望不支持的渠道报错，实际 nil")
	}

	// 2) 正常解析：把渠道端点指向伪 server。
	body, _ := json.Marshal(map[string]any{
		"status": "200",
		"data": []map[string]string{
			{"title": "热点一", "url": "https://s.weibo.com/x1", "content": "热点一", "publish_time": "2026-10-17 10:00:00"},
			{"title": "热点二", "url": "https://s.weibo.com/x2", "content": "热点二", "publish_time": "2026-10-17 10:00:00"},
		},
	})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer ts.Close()
	newsPlatforms["tenxunwang"] = ts.URL
	defer delete(newsPlatforms, "tenxunwang") // 还原，避免污染其它用例

	items, err := srv.Fetch(context.Background(), "tenxunwang", false)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(items) != 2 || items[0].Title != "热点一" || items[0].URL != "https://s.weibo.com/x1" {
		t.Fatalf("解析结果不符合预期: %+v", items)
	}

	// 3) 缓存命中：渠道源挂掉后仍返回缓存（降级）。
	ts.Close()
	cached, err := srv.Fetch(context.Background(), "tenxunwang", false)
	if err != nil {
		t.Fatalf("缓存降级不应报错: %v", err)
	}
	if len(cached) != 2 {
		t.Fatalf("缓存数据丢失: %+v", cached)
	}

	// 4) force 拉取 + 源挂掉 → 报错（无可用缓存降级之外的路径）。
	if _, err := srv.Fetch(context.Background(), "tenxunwang", true); err == nil {
		t.Fatal("force 且源挂掉应报错")
	}
}
