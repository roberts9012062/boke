// internal/plugin/bpkg/sign_test.go
// .bpk 包签名单元测试（P1 供应链加固）：签名 roundtrip、篡改拒绝、无签名策略、PEM 解析。
package bpkg

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testSignKey 生成测试用 Ed25519 密钥对。
func testSignKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("生成密钥失败：%v", err)
	}
	return pub, priv
}

// testSignedPack 打一个签名包并落盘，返回包路径与验签公钥。
func testSignedPack(t *testing.T, dir string) (string, ed25519.PublicKey) {
	t.Helper()
	pub, priv := testSignKey(t)
	content, err := PackSigned(testManifest, map[string][]byte{
		PluginBinName: []byte("signed-binary-content"),
	}, priv)
	if err != nil {
		t.Fatalf("签名打包失败：%v", err)
	}
	zipPath := filepath.Join(dir, "signed.bpk")
	if err := os.WriteFile(zipPath, content, 0o644); err != nil {
		t.Fatalf("写包失败：%v", err)
	}
	return zipPath, pub
}

// TestSignedPackRoundtrip 签名包：Read 通过 → 验签通过 → 落盘成功。
func TestSignedPackRoundtrip(t *testing.T) {
	zipPath, pub := testSignedPack(t, t.TempDir())
	_, entries, err := Read(zipPath)
	if err != nil {
		t.Fatalf("读取签名包失败：%v", err)
	}
	if err := VerifySignature(entries, []ed25519.PublicKey{pub}); err != nil {
		t.Fatalf("验签应通过：%v", err)
	}
	destDir := filepath.Join(t.TempDir(), "out")
	if err := WriteEntries(entries, destDir); err != nil {
		t.Fatalf("落盘失败：%v", err)
	}
	// 签名与 checksums 材料不落盘
	if _, err := os.Stat(filepath.Join(destDir, SignatureName)); err == nil {
		t.Fatal("signature.sig 不应落盘")
	}
	if _, err := os.Stat(filepath.Join(destDir, ChecksumsName)); err == nil {
		t.Fatal("checksums.json 不应落盘")
	}
}

// TestVerifyTamperedWithConsistentChecksums 核心攻击场景：投毒者重打包并重算 checksums
// （包内自证自洽）→ checksums 校验通过，但签名校验必须失败——这是签名体系存在的意义。
func TestVerifyTamperedWithConsistentChecksums(t *testing.T) {
	zipPath, pub := testSignedPack(t, t.TempDir())
	_, entries, err := Read(zipPath)
	if err != nil {
		t.Fatalf("读取签名包失败：%v", err)
	}
	// 攻击模拟：替换二进制并重算 checksums（保持自洽），保留原签名
	entries[PluginBinName] = []byte("tampered-binary")
	tampered := map[string]string{}
	for name, content := range entries {
		if name == ChecksumsName || name == SignatureName {
			continue
		}
		sum := sha256.Sum256(content)
		tampered[name] = hex.EncodeToString(sum[:])
	}
	raw, _ := json.Marshal(tampered)
	entries[ChecksumsName] = raw
	// 重写为 zip 再读（Read 过校验），然后验签必须失败
	newPath := filepath.Join(t.TempDir(), "tampered.bpk")
	if err := writeTestZip(newPath, entries); err != nil {
		t.Fatalf("重写包失败：%v", err)
	}
	_, entries2, err := Read(newPath)
	if err != nil {
		t.Fatalf("自洽篡改包应通过 checksums 校验（实际被拒：%v）——验签才是最后防线", err)
	}
	if err := VerifySignature(entries2, []ed25519.PublicKey{pub}); err == nil {
		t.Fatal("篡改包验签应失败，实际通过")
	}
}

// TestVerifyUnsigned 无签名包 + 已配置信任公钥 → 必须拒绝（强制签名策略）。
func TestVerifyUnsigned(t *testing.T) {
	pub, _ := testSignKey(t)
	content, err := Pack(testManifest, map[string][]byte{PluginBinName: []byte("plain")})
	if err != nil {
		t.Fatalf("打包失败：%v", err)
	}
	zipPath := filepath.Join(t.TempDir(), "plain.bpk")
	if err := os.WriteFile(zipPath, content, 0o644); err != nil {
		t.Fatalf("写包失败：%v", err)
	}
	_, entries, err := Read(zipPath)
	if err != nil {
		t.Fatalf("读取失败：%v", err)
	}
	if err := VerifySignature(entries, []ed25519.PublicKey{pub}); err == nil {
		t.Fatal("无签名包在配置信任公钥时应拒绝，实际通过")
	}
}

// TestVerifyWrongKey 签名包 + 非信任公钥 → 拒绝。
func TestVerifyWrongKey(t *testing.T) {
	zipPath, _ := testSignedPack(t, t.TempDir())
	otherPub, _ := testSignKey(t)
	_, entries, err := Read(zipPath)
	if err != nil {
		t.Fatalf("读取失败：%v", err)
	}
	if err := VerifySignature(entries, []ed25519.PublicKey{otherPub}); err == nil {
		t.Fatal("非信任公钥验签应失败，实际通过")
	}
}

// TestParsePublicKeysPEM PEM 解析：多块拼接合法、坏块拒绝。
func TestParsePublicKeysPEM(t *testing.T) {
	pub1, _ := testSignKey(t)
	pub2, _ := testSignKey(t)
	pem1, err := EncodePublicKeyPEM(pub1)
	if err != nil {
		t.Fatalf("编码公钥失败：%v", err)
	}
	pem2, err := EncodePublicKeyPEM(pub2)
	if err != nil {
		t.Fatalf("编码公钥失败：%v", err)
	}
	keys, err := ParsePublicKeysPEM(pem1 + pem2)
	if err != nil || len(keys) != 2 {
		t.Fatalf("多块解析失败：%v（%d 把）", err, len(keys))
	}
	if _, err := ParsePublicKeysPEM(""); err == nil {
		t.Fatal("空 PEM 应报错")
	}
	if _, err := ParsePublicKeysPEM("not a pem"); err == nil {
		t.Fatal("非 PEM 文本应报错")
	}
	// 私钥 PEM 当公钥配置 → 拒绝（防配置错位）
	_, priv := testSignKey(t)
	privPEM, _ := EncodePrivateKeyPEM(priv)
	if _, err := ParsePublicKeysPEM(privPEM); err == nil {
		t.Fatal("私钥 PEM 不应被当作公钥解析")
	}
	if strings.Contains(privPEM, "-----END") == false {
		t.Fatal("测试自检失败：私钥 PEM 编码异常")
	}
}

// writeTestZip 将条目写成 zip 文件（测试辅助）。
func writeTestZip(path string, entries map[string][]byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := zip.NewWriter(f)
	for name, content := range entries {
		entry, err := w.Create(name)
		if err != nil {
			return err
		}
		if _, err := entry.Write(content); err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return err
	}
	return f.Close()
}
