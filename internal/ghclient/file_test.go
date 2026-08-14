// internal/ghclient/file_test.go
// 仓库文件拉取单元测试（httptest mock）：原始内容、404 哨兵错误、路径穿越拒绝。
package ghclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFetchFile 正常拉取：返回文件原文。
func TestFetchFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/contents/seo-optimizer/README.md" {
			t.Fatalf("路径不符：%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("# SEO 优化"))
	}))
	defer srv.Close()

	client := NewClient("")
	client.baseURL = srv.URL
	raw, err := client.FetchFile(context.Background(), "owner", "repo", "seo-optimizer/README.md", 0)
	if err != nil {
		t.Fatalf("拉取失败：%v", err)
	}
	if string(raw) != "# SEO 优化" {
		t.Fatalf("内容不符：%s", raw)
	}
}

// TestFetchFileNotFound 404 → ErrFileNotFound（上层区分「未提供 README」）。
func TestFetchFileNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient("")
	client.baseURL = srv.URL
	_, err := client.FetchFile(context.Background(), "owner", "repo", "x/README.md", 0)
	if !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("应返回 ErrFileNotFound，实际：%v", err)
	}
}

// TestValidateFilePath 路径校验（防穿越）。
func TestValidateFilePath(t *testing.T) {
	if err := validateFilePath("seo-optimizer/README.md"); err != nil {
		t.Fatalf("合法路径应通过：%v", err)
	}
	for _, bad := range []string{"", "/abs", "..", "a/../b", "a\\b", "a//b", "a/./b"} {
		if err := validateFilePath(bad); err == nil {
			t.Fatalf("非法路径应拒绝：%q", bad)
		}
	}
}
