// internal/plugin/license/license_test.go
// 许可证单元测试：签发→验签 roundtrip、篡改拒绝、过期/宽限期判断、PEM 读写。
package license

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestSignVerifyRoundtrip 签发 → 验签 roundtrip：字段一致。
func TestSignVerifyRoundtrip(t *testing.T) {
	priv, pub := keyPair(t)
	raw, err := Sign(priv, &License{
		Sub: "plugin:demo-plugin", Licensee: "站点A", Edition: "pro",
		Features: []string{"demo_pro"}, ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("签发失败：%v", err)
	}
	lic, err := Verify(pub, raw)
	if err != nil {
		t.Fatalf("验签失败：%v", err)
	}
	if lic.Edition != "pro" || len(lic.Features) != 1 || lic.Features[0] != "demo_pro" {
		t.Fatalf("字段不符：%+v", lic)
	}
	if lic.Signature == "" {
		t.Fatal("签名缺失")
	}
}

// TestVerifyTampered 篡改许可证内容（edition）→ 验签拒绝。
func TestVerifyTampered(t *testing.T) {
	priv, pub := keyPair(t)
	raw, err := Sign(priv, &License{
		Sub: "plugin:demo-plugin", Licensee: "站点A", Edition: "free", Features: nil,
	})
	if err != nil {
		t.Fatalf("签发失败：%v", err)
	}
	// 篡改 edition 为 pro
	var lic License
	if err := json.Unmarshal(raw, &lic); err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	lic.Edition = "pro"
	tampered, _ := json.Marshal(lic)
	if _, err := Verify(pub, tampered); err == nil {
		t.Fatal("篡改许可证应验签失败，实际成功")
	}
	// 错误公钥也应拒绝
	_, otherPub := keyPair(t)
	if _, err := Verify(otherPub, raw); err == nil {
		t.Fatal("错误公钥应验签失败，实际成功")
	}
}

// TestExpiryAndGrace 过期与宽限期判断。
func TestExpiryAndGrace(t *testing.T) {
	now := time.Now()
	// 未过期
	l := &License{ExpiresAt: now.Add(time.Hour).Unix()}
	if l.IsExpired(now) || l.IsDegraded(now) {
		t.Fatal("未过期不应判定过期/降级")
	}
	// 已过期但在宽限期内（exp 后 7 天内）：过期但未降级
	expired := now.Add(-24 * time.Hour).Unix()
	l2 := &License{ExpiresAt: expired}
	if !l2.IsExpired(now) {
		t.Fatal("应判定过期")
	}
	if l2.IsDegraded(now) {
		t.Fatal("宽限期内不应降级")
	}
	// 超过宽限期（exp 后 8 天）：降级
	l3 := &License{ExpiresAt: now.Add(-8 * 24 * time.Hour).Unix()}
	if !l3.IsDegraded(now) {
		t.Fatal("超宽限期应降级")
	}
	// 永久许可（exp=0）
	l4 := &License{ExpiresAt: 0}
	if l4.IsExpired(now) || l4.IsDegraded(now) {
		t.Fatal("永久许可不应过期/降级")
	}
}

// TestPEMRoundtrip PEM 私钥/公钥读写 roundtrip。
func TestPEMRoundtrip(t *testing.T) {
	dir := t.TempDir()
	privPath := filepath.Join(dir, "priv.pem")
	pubPath := filepath.Join(dir, "pub.pem")
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥失败：%v", err)
	}
	if err := SavePrivateKey(privPath, priv); err != nil {
		t.Fatalf("保存私钥失败：%v", err)
	}
	if err := SavePublicKey(pubPath, pub); err != nil {
		t.Fatalf("保存公钥失败：%v", err)
	}
	loadedPriv, err := LoadPrivateKey(privPath)
	if err != nil {
		t.Fatalf("加载私钥失败：%v", err)
	}
	loadedPub, err := LoadPublicKey(pubPath)
	if err != nil {
		t.Fatalf("加载公钥失败：%v", err)
	}
	// 用加载的密钥对完成签发验签（验证 PEM 内容完整）
	raw, err := Sign(loadedPriv, &License{Sub: "plugin:x", Licensee: "s", Edition: "pro"})
	if err != nil {
		t.Fatalf("签发失败：%v", err)
	}
	if _, err := Verify(loadedPub, raw); err != nil {
		t.Fatalf("验签失败：%v", err)
	}
	// 文件权限：私钥 0600（仅 POSIX；Windows 无权限位语义）
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(privPath)
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("私钥权限应为 0600，实际 %o", info.Mode().Perm())
		}
	}
}

// keyPair 生成测试密钥对（失败即终止）。
func keyPair(t *testing.T) (ed25519.PrivateKey, ed25519.PublicKey) {
	t.Helper()
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥失败：%v", err)
	}
	return priv, pub
}
