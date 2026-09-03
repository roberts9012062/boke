// internal/service/openapi_test.go
// 接口开放服务单测：授权端点校验聚合插件开放目录（回归：勾选插件端点保存被拒的 bug）。
package service

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNormalizeEndpointsWithPluginCatalog 验证授权校验接受插件贡献端点
// （静态目录 ∪ 插件目录；插件目录缺失该端点或无聚合器时仍拒绝）。
func TestNormalizeEndpointsWithPluginCatalog(t *testing.T) {
	dataDir := t.TempDir()
	plugDir := filepath.Join(dataDir, "plugins", "tg-image-bed")
	if err := os.MkdirAll(plugDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"tg-image-bed","name":"TG图床","version":"0.3.1",` +
		`"open_endpoints":[{"endpoint":"tg-image-bed.upload","method":"POST",` +
		`"path":"/api/v1/open/plugins/tg-image-bed/upload","name":"上传图片"}]}`
	if err := os.WriteFile(filepath.Join(plugDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog := NewPluginOpenCatalog(dataDir, nil)
	svc := NewOpenAPIService(nil, catalog)

	// 插件端点 + 静态端点混合：全部接受（bug 场景——此前插件端点被拒）
	got, err := svc.normalizeEndpoints([]string{"tg-image-bed.upload", "posts.list"})
	if err != nil {
		t.Fatalf("插件端点应被接受：%v", err)
	}
	if len(got) != 2 {
		t.Fatalf("结果数不符：%v", got)
	}

	// 未声明端点仍拒绝（防绕过目录授权）
	if _, err := svc.normalizeEndpoints([]string{"tg-image-bed.list"}); err == nil {
		t.Fatal("未声明的插件端点应被拒绝")
	}

	// 聚合器为空（降级）：插件端点不在静态目录，拒绝——兼容旧装配语义
	fallback := NewOpenAPIService(nil, nil)
	if _, err := fallback.normalizeEndpoints([]string{"tg-image-bed.upload"}); err == nil {
		t.Fatal("无聚合器时插件端点应被拒绝（仅静态目录）")
	}
}
