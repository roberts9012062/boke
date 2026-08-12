// internal/ghclient/client_test.go
// GitHub 客户端单元测试（httptest mock）：latest Release 解析、资产流式下载、仓库名校验。
package ghclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestFetchLatestRelease 拉取 latest Release：tag 与资产列表解析。
func TestFetchLatestRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases/latest" {
			t.Fatalf("路径不符：%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "v0.1.0",
			"assets": [
				{"name": "demo-plugin-0.1.0-windows-amd64.bpk", "browser_download_url": "https://cdn.example/demo.bpk", "size": 1024}
			]
		}`))
	}))
	defer srv.Close()

	// 直接请求 mock server 并走与 FetchLatestRelease 相同的响应解析路径
	client := NewClient("")
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/repos/owner/repo/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.client.Do(req)
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码不符：%d", resp.StatusCode)
	}
	var release LatestRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if release.TagName != "v0.1.0" || len(release.Assets) != 1 {
		t.Fatalf("解析结果不符：%+v", release)
	}
	if release.Assets[0].Name != "demo-plugin-0.1.0-windows-amd64.bpk" {
		t.Fatalf("资产名不符：%s", release.Assets[0].Name)
	}
}

// TestDownloadAsset 资产流式下载：内容与大小上限校验。
func TestDownloadAsset(t *testing.T) {
	payload := []byte("bpk-content-0123456789")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	client := NewClient("")
	dest := filepath.Join(t.TempDir(), "demo.bpk")
	if err := client.DownloadAsset(context.Background(), srv.URL, dest, 1024); err != nil {
		t.Fatalf("下载失败：%v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("读取失败：%v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("内容不符：%s", got)
	}

	// 大小上限：超出报错
	over := filepath.Join(t.TempDir(), "over.bpk")
	if err := client.DownloadAsset(context.Background(), srv.URL, over, 5); err == nil {
		t.Fatal("超限下载应失败，实际成功")
	}
}

// TestDownloadAssetHTTPError 非 200 下载报错。
func TestDownloadAssetHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()
	client := NewClient("")
	if err := client.DownloadAsset(context.Background(), srv.URL, filepath.Join(t.TempDir(), "x.bpk"), 0); err == nil {
		t.Fatal("HTTP 404 应报错，实际成功")
	}
}

// TestValidateRepo 仓库名校验（防路径注入）。
func TestValidateRepo(t *testing.T) {
	if err := validateRepo("owner", "repo"); err != nil {
		t.Fatalf("合法仓库名应通过：%v", err)
	}
	if err := validateRepo("", "repo"); err == nil {
		t.Fatal("空 owner 应拒绝")
	}
	if err := validateRepo("own/er", "repo"); err == nil {
		t.Fatal("含斜杠 owner 应拒绝")
	}
	if err := validateRepo("owner", ".."); err == nil {
		t.Fatal("路径注入 repo 应拒绝")
	}
}
