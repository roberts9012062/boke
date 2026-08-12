// internal/handler/plugin_test.go
// 插件前端资源静态服务单元测试（M3.6）：正常访问 / 路径穿越拒绝 / 不存在 404 / ID 不合法。
package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/internal/service"
)

// newAssetTestHandler 构造 Asset handler（真实 BinStore 于临时目录 + 预置 frontend 文件）。
func newAssetTestHandler(t *testing.T) (*PluginHandler, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dataDir := t.TempDir()
	store := plugin.NewBinStore(dataDir)
	// 预置插件前端资源（模拟 .bpk 解包落盘）
	base := filepath.Join(dataDir, "plugins", "test-plugin", "frontend")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("创建资源目录失败：%v", err)
	}
	manifest := filepath.Join(base, "manifest.json")
	if err := os.WriteFile(manifest, []byte(`{"extensionPoints":[{"slot":"post.footer","entry":"index.js"}]}`), 0o644); err != nil {
		t.Fatalf("写入 manifest 失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(base, "index.js"), []byte("export default function register(){}"), 0o644); err != nil {
		t.Fatalf("写入 index.js 失败：%v", err)
	}
	svc := service.NewPluginService(nil, nil, nil, nil, nil, store, nil)
	return NewPluginHandler(svc, nil), base
}

// TestAssetNormal 正常访问 frontend/manifest.json → 200 + 内容。
func TestAssetNormal(t *testing.T) {
	h, _ := newAssetTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/plugin-assets/test-plugin/frontend/manifest.json", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{
		{Key: "id", Value: "test-plugin"},
		{Key: "filepath", Value: "/frontend/manifest.json"},
	}
	h.Asset(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", rec.Code)
	}
	if body := rec.Body.String(); !contains(body, "extensionPoints") {
		t.Fatalf("响应内容不符：%s", body)
	}
}

// TestAssetPathTraversal 路径穿越（../ 逃逸插件目录）→ 404 拒绝。
func TestAssetPathTraversal(t *testing.T) {
	h, base := newAssetTestHandler(t)
	// 在插件目录外放一个敏感文件（验证穿越无法读取）
	_ = os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(base)), "secret.txt"), []byte("secret"), 0o644)
	req := httptest.NewRequest(http.MethodGet, "/plugin-assets/test-plugin/../../secret.txt", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{
		{Key: "id", Value: "test-plugin"},
		{Key: "filepath", Value: "/../../secret.txt"},
	}
	h.Asset(c)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("穿越应 404，实际 %d", rec.Code)
	}
	if contains(rec.Body.String(), "secret") {
		t.Fatal("穿越读取到了插件目录外文件")
	}
}

// TestAssetInvalidID 非法插件 ID（含路径字符）→ 404。
func TestAssetInvalidID(t *testing.T) {
	h, _ := newAssetTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/plugin-assets/../etc/passwd", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "../etc"}, {Key: "filepath", Value: "/passwd"}}
	h.Asset(c)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("非法 ID 应 404，实际 %d", rec.Code)
	}
}

// TestAssetNotFound 不存在文件 → 404。
func TestAssetNotFound(t *testing.T) {
	h, _ := newAssetTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/plugin-assets/test-plugin/frontend/nope.js", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "test-plugin"}, {Key: "filepath", Value: "/frontend/nope.js"}}
	h.Asset(c)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在应 404，实际 %d", rec.Code)
	}
}

// contains 简单子串判断（测试辅助）。
func contains(s string, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
