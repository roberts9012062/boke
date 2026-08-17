// internal/handler/plugin_test.go
// 插件前端资产静态服务单元测试（M3.6 + P0 加固）：
// 路径白名单清理（sanitizeAssetPath 纯函数表驱动）+ handler 层穿越/非法路径拒绝。
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestSanitizeAssetPath 表驱动：frontend/ 子树放行、目录回退、白名单外与穿越拒绝。
func TestSanitizeAssetPath(t *testing.T) {
	cases := []struct {
		name    string // 用例名
		in      string // 请求 filepath（gin 通配捕获，带前导斜杠）
		want    string // 期望清理后的安全相对路径
		wantOK  bool   // 期望是否放行
	}{
		{name: "正常前端资源", in: "/frontend/manifest.json", want: "frontend/manifest.json", wantOK: true},
		{name: "子目录资源", in: "/frontend/css/main.css", want: "frontend/css/main.css", wantOK: true},
		{name: "根访问回退首页", in: "/", want: "frontend/index.html", wantOK: true},
		{name: "空路径回退首页", in: "", want: "frontend/index.html", wantOK: true},
		{name: "frontend 目录回退首页", in: "/frontend", want: "frontend/index.html", wantOK: true},
		{name: "frontend 目录斜杠回退首页", in: "/frontend/", want: "frontend/index.html", wantOK: true},
		// P0 加固：包内敏感文件一律拒绝（此前 plugin.exe/pubkey.pem 可被匿名下载）
		{name: "二进制本体拒绝", in: "/plugin.exe", want: "", wantOK: false},
		{name: "许可证公钥拒绝", in: "/pubkey.pem", want: "", wantOK: false},
		{name: "包内清单拒绝", in: "/manifest.json", want: "", wantOK: false},
		{name: "校验清单拒绝", in: "/checksums.json", want: "", wantOK: false},
		// 穿越：Clean 消解后必须仍在 frontend/ 子树内
		{name: "穿越到包内文件", in: "/frontend/../plugin.exe", want: "", wantOK: false},
		{name: "穿越逃逸插件目录", in: "/../../secret.txt", want: "", wantOK: false},
		{name: "穿越系统路径", in: "/../etc/passwd", want: "", wantOK: false},
		{name: "双斜杠仍放行前端资源", in: "/frontend//index.js", want: "frontend/index.js", wantOK: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := sanitizeAssetPath(tc.in)
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("sanitizeAssetPath(%q) = (%q, %v)，期望 (%q, %v)", tc.in, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// TestAssetPathTraversal 路径穿越（../ 逃逸插件目录）→ 404 拒绝（白名单在 DB 查询前拦截）。
func TestAssetPathTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewPluginHandler(nil, nil)
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

// TestAssetSensitiveFile 白名单外文件（plugin.exe）→ 404 拒绝。
func TestAssetSensitiveFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewPluginHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/plugin-assets/test-plugin/plugin.exe", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{
		{Key: "id", Value: "test-plugin"},
		{Key: "filepath", Value: "/plugin.exe"},
	}
	h.Asset(c)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("白名单外文件应 404，实际 %d", rec.Code)
	}
}

// TestAssetInvalidID 非法插件 ID（含路径字符）→ 404。
func TestAssetInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewPluginHandler(nil, nil)
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

// contains 简单子串判断（测试辅助）。
func contains(s string, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
