// internal/ghclient/tree_test.go
// 文件树拉取单元测试（httptest mock）：路径列表解析、空仓库 409 报错、截断树报错。
package ghclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFetchTree 完整树解析：返回全量路径列表。
func TestFetchTree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/git/trees/HEAD" || r.URL.Query().Get("recursive") != "1" {
			t.Fatalf("路径或参数不符：%s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"sha": "abc",
			"truncated": false,
			"tree": [
				{"path": "market.json", "type": "blob"},
				{"path": "seo-optimizer", "type": "tree"},
				{"path": "seo-optimizer/plugin.json", "type": "blob"},
				{"path": "seo-optimizer/README.md", "type": "blob"}
			]
		}`))
	}))
	defer srv.Close()

	client := NewClient("")
	client.baseURL = srv.URL
	paths, err := client.FetchTree(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("拉取失败：%v", err)
	}
	if len(paths) != 4 || paths[2] != "seo-optimizer/plugin.json" {
		t.Fatalf("路径列表不符：%v", paths)
	}
}

// TestFetchTreeEmpty 空仓库 409：返回友好错误。
func TestFetchTreeEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Git Repository is empty", http.StatusConflict)
	}))
	defer srv.Close()

	client := NewClient("")
	client.baseURL = srv.URL
	if _, err := client.FetchTree(context.Background(), "owner", "repo"); err == nil {
		t.Fatal("空仓库应报错，实际成功")
	}
}

// TestFetchTreeTruncated 截断树报错（防遗漏插件）。
func TestFetchTreeTruncated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sha":"abc","truncated":true,"tree":[]}`))
	}))
	defer srv.Close()

	client := NewClient("")
	client.baseURL = srv.URL
	if _, err := client.FetchTree(context.Background(), "owner", "repo"); err == nil {
		t.Fatal("截断树应报错，实际成功")
	}
}
