// internal/plugin/bpkg/bpkg_test.go
// .bpk 打包/解包单元测试：roundtrip、checksums 篡改拒绝、zip-slip 防护、缺件校验。
package bpkg

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// 测试用清单。
var testManifest = Manifest{ID: "demo-plugin", Name: "演示插件", Version: "0.1.0", Author: "测试", Description: "单测"}

// TestPackUnpackRoundtrip 打包 → 解包 roundtrip：清单与文件内容一致。
func TestPackUnpackRoundtrip(t *testing.T) {
	content, err := Pack(testManifest, map[string][]byte{
		PluginBinName: []byte("binary-content-001"),
	})
	if err != nil {
		t.Fatalf("打包失败：%v", err)
	}

	// 落盘后解包到临时目录
	zipPath := filepath.Join(t.TempDir(), "demo.bpk")
	if err := os.WriteFile(zipPath, content, 0o644); err != nil {
		t.Fatalf("写包失败：%v", err)
	}
	destDir := filepath.Join(t.TempDir(), "out")
	manifest, err := Unpack(zipPath, destDir)
	if err != nil {
		t.Fatalf("解包失败：%v", err)
	}
	if manifest.ID != testManifest.ID || manifest.Version != testManifest.Version {
		t.Fatalf("清单不符：%+v", manifest)
	}
	bin, err := os.ReadFile(filepath.Join(destDir, PluginBinName))
	if err != nil || string(bin) != "binary-content-001" {
		t.Fatalf("二进制内容不符：%v", err)
	}
}

// TestUnpackChecksumMismatch 篡改 plugin.bin 后解包必须拒绝（checksums 校验）。
func TestUnpackChecksumMismatch(t *testing.T) {
	content, err := Pack(testManifest, map[string][]byte{
		PluginBinName: []byte("original"),
	})
	if err != nil {
		t.Fatalf("打包失败：%v", err)
	}
	// 篡改：重写 zip 中 plugin.bin（保留 checksums.json 不变）
	zipPath := filepath.Join(t.TempDir(), "demo.bpk")
	if err := os.WriteFile(zipPath, content, 0o644); err != nil {
		t.Fatalf("写包失败：%v", err)
	}
	if err := rewriteZipEntry(zipPath, PluginBinName, []byte("tampered")); err != nil {
		t.Fatalf("篡改失败：%v", err)
	}
	if _, err := Unpack(zipPath, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("篡改包应解包失败，实际成功")
	}
}

// TestUnpackZipSlip 含 ../ 穿越条目的包必须拒绝（zip-slip 防护）。
func TestUnpackZipSlip(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "evil.bpk")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("创建包失败：%v", err)
	}
	w := zip.NewWriter(f)
	// 恶意条目：../evil.txt（穿越解包目录）
	entry, err := w.Create("../evil.txt")
	if err != nil {
		t.Fatalf("创建条目失败：%v", err)
	}
	if _, err := entry.Write([]byte("evil")); err != nil {
		t.Fatalf("写入条目失败：%v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("关闭 zip 失败：%v", err)
	}
	_ = f.Close()
	if _, err := Unpack(zipPath, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("zip-slip 包应解包失败，实际成功")
	}
}

// TestUnpackMissingChecksums 缺 checksums.json 必须拒绝。
func TestUnpackMissingChecksums(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "nocheck.bpk")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("创建包失败：%v", err)
	}
	w := zip.NewWriter(f)
	entry, err := w.Create(ManifestName)
	if err != nil {
		t.Fatalf("创建条目失败：%v", err)
	}
	if _, err := entry.Write([]byte(`{"id":"x","name":"x","version":"1.0.0"}`)); err != nil {
		t.Fatalf("写入条目失败：%v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("关闭 zip 失败：%v", err)
	}
	_ = f.Close()
	if _, err := Unpack(zipPath, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("缺 checksums.json 应解包失败，实际成功")
	}
}

// rewriteZipEntry 重写 zip 中指定条目内容（篡改用；保留其他条目）。
func rewriteZipEntry(zipPath string, target string, newContent []byte) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	entries := make(map[string][]byte)
	for _, f := range reader.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		buf := make([]byte, 0, f.UncompressedSize64)
		tmp := make([]byte, 4096)
		for {
			n, err := rc.Read(tmp)
			buf = append(buf, tmp[:n]...)
			if err != nil {
				break
			}
		}
		_ = rc.Close()
		entries[f.Name] = buf
	}
	_ = reader.Close()

	entries[target] = newContent
	// 重建 zip
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	w := zip.NewWriter(f)
	for _, name := range names {
		entry, err := w.Create(name)
		if err != nil {
			return err
		}
		if _, err := entry.Write(entries[name]); err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return err
	}
	return f.Close()
}
