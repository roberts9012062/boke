// internal/plugin/bpkg/unpack.go
// .bpk 安全解包（M3.4）：校验 checksums.json（包内文件逐项 SHA-256）+ zip-slip 防护。
// 对齐 docs/architecture.md 6.5.5：解包到 plugins/{id}/ → 读 manifest.json → 校验。
package bpkg

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// 解包安全限制（防 zip bomb 与超大文件拖垮磁盘）。
const (
	MaxFileBytes  = 64 << 20  // 单文件解压上限（64MB，插件二进制一般 ≤50MB）
	MaxTotalBytes = 256 << 20 // 解包总大小上限（256MB）
)

// Unpack 安全解包 .bpk 到 destDir，返回包内清单。
// 流程：zip 打开 → checksums.json 校验（全部条目哈希比对）→ manifest.json 解析 → 逐文件解压。
// 安全：条目名 Clean 后校验（拒绝绝对路径/../ 穿越，zip-slip 防护）；大小上限防 zip bomb。
func Unpack(srcPath string, destDir string) (*Manifest, error) {
	reader, err := zip.OpenReader(srcPath)
	if err != nil {
		return nil, fmt.Errorf("打开 .bpk 失败：%w", err)
	}
	defer reader.Close()

	// 预读全部条目内容（先校验再落盘；受大小上限约束）
	entries := make(map[string][]byte, len(reader.File))
	var total int64
	for _, f := range reader.File {
		name := f.Name
		if name == "" {
			return nil, fmt.Errorf("存在空条目名")
		}
		// 大小上限（zip 头声明的解压大小）
		if f.UncompressedSize64 > MaxFileBytes {
			return nil, fmt.Errorf("条目 %s 超过单文件上限（%dMB）", name, MaxFileBytes>>20)
		}
		// zip-slip 防护：Clean 后必须仍是相对路径且不越出目标目录
		clean := filepath.Clean(name)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return nil, fmt.Errorf("条目路径不合法（%s）", name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("读取条目失败（%s）：%w", name, err)
		}
		content, err := io.ReadAll(io.LimitReader(rc, MaxFileBytes+1))
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("读取条目失败（%s）：%w", name, err)
		}
		total += int64(len(content))
		if total > MaxTotalBytes {
			return nil, fmt.Errorf("解包总大小超过上限（%dMB）", MaxTotalBytes>>20)
		}
		entries[name] = content
	}

	// 必须包含 manifest.json 与 checksums.json
	if _, ok := entries[ManifestName]; !ok {
		return nil, fmt.Errorf(".bpk 缺少 manifest.json")
	}
	checksumRaw, ok := entries[ChecksumsName]
	if !ok {
		return nil, fmt.Errorf(".bpk 缺少 checksums.json")
	}

	// checksums.json 校验：覆盖全部条目（除自身）逐项 SHA-256 比对
	var checksums map[string]string
	if err := json.Unmarshal(checksumRaw, &checksums); err != nil {
		return nil, fmt.Errorf("checksums.json 解析失败：%w", err)
	}
	for name, content := range entries {
		if name == ChecksumsName {
			continue
		}
		expect, ok := checksums[name]
		if !ok {
			return nil, fmt.Errorf("checksums.json 未声明条目 %s", name)
		}
		actual := sha256Hex(content)
		if !strings.EqualFold(actual, expect) {
			return nil, fmt.Errorf("条目 %s 校验失败（SHA-256 不匹配）", name)
		}
	}

	// 解析包内清单（必填字段校验）
	manifest, err := ParseManifest(entries[ManifestName])
	if err != nil {
		return nil, err
	}

	// 落盘（目录条目跳过；frontend/ 子目录自动创建）
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建解包目录失败：%w", err)
	}
	for name, content := range entries {
		if name == ChecksumsName {
			continue
		}
		// 目录条目（zip 常见 frontend/）直接创建
		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(filepath.Join(destDir, name), 0o755); err != nil {
				return nil, fmt.Errorf("创建目录失败（%s）：%w", name, err)
			}
			continue
		}
		target := filepath.Join(destDir, filepath.Clean(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, fmt.Errorf("创建目录失败（%s）：%w", name, err)
		}
		if err := os.WriteFile(target, content, 0o755); err != nil {
			return nil, fmt.Errorf("写入文件失败（%s）：%w", name, err)
		}
	}
	return manifest, nil
}

// sha256Hex 计算内容 SHA-256（hex 小写；与 pack 侧一致）。
func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
