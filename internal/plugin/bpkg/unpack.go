// internal/plugin/bpkg/unpack.go
// .bpk 安全读取与解包（M3.4）：校验 checksums.json（包内文件逐项 SHA-256）+ zip-slip 防护。
// 对齐 docs/architecture.md 6.5.5：解包到 plugins/{id}/ → 读 manifest.json → 校验。
// P1 加固：拆分为 Read（校验不落盘）+ WriteEntries（落盘）两段——安装方在两段之间
// 完成包签名验签（VerifySignature），验签失败不落盘。
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

// Read 读取并校验 .bpk（不落盘）：返回包内清单与全部条目内容（含 checksums 与签名材料）。
// 校验：条目大小上限（zip 头声明 + 实际读取长度双重检查）、zip-slip 防护、
// checksums.json 逐条目哈希比对、manifest 必填字段。
func Read(srcPath string) (*Manifest, map[string][]byte, error) {
	reader, err := zip.OpenReader(srcPath)
	if err != nil {
		return nil, nil, fmt.Errorf("打开 .bpk 失败：%w", err)
	}
	defer reader.Close()

	// 预读全部条目内容（先校验再落盘；受大小上限约束）
	entries := make(map[string][]byte, len(reader.File))
	var total int64
	for _, f := range reader.File {
		name := f.Name
		if name == "" {
			return nil, nil, fmt.Errorf("存在空条目名")
		}
		// 大小上限（zip 头声明的解压大小——预检）
		if f.UncompressedSize64 > MaxFileBytes {
			return nil, nil, fmt.Errorf("条目 %s 超过单文件上限（%dMB）", name, MaxFileBytes>>20)
		}
		// zip-slip 防护：Clean 后必须仍是相对路径且不越出目标目录
		clean := filepath.Clean(name)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return nil, nil, fmt.Errorf("条目路径不合法（%s）", name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, nil, fmt.Errorf("读取条目失败（%s）：%w", name, err)
		}
		content, err := io.ReadAll(io.LimitReader(rc, MaxFileBytes+1))
		_ = rc.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("读取条目失败（%s）：%w", name, err)
		}
		// 实际读取长度检查（P2 加固）：zip 头可谎报解压大小绕过预检——
		// 超限拒绝而非静默截断（截断落盘会产出与哈希不符的损坏文件）
		if int64(len(content)) > MaxFileBytes {
			return nil, nil, fmt.Errorf("条目 %s 实际解压大小超过单文件上限（%dMB）", name, MaxFileBytes>>20)
		}
		total += int64(len(content))
		if total > MaxTotalBytes {
			return nil, nil, fmt.Errorf("解包总大小超过上限（%dMB）", MaxTotalBytes>>20)
		}
		entries[name] = content
	}

	// 必须包含 manifest.json 与 checksums.json
	if _, ok := entries[ManifestName]; !ok {
		return nil, nil, fmt.Errorf(".bpk 缺少 manifest.json")
	}
	checksumRaw, ok := entries[ChecksumsName]
	if !ok {
		return nil, nil, fmt.Errorf(".bpk 缺少 checksums.json")
	}

	// checksums.json 校验：覆盖全部条目（除自身与签名）逐项 SHA-256 比对
	var checksums map[string]string
	if err := json.Unmarshal(checksumRaw, &checksums); err != nil {
		return nil, nil, fmt.Errorf("checksums.json 解析失败：%w", err)
	}
	for name, content := range entries {
		if name == ChecksumsName || name == SignatureName {
			continue // 自身与签名条目不参与（签名是对 checksums 的签名）
		}
		expect, ok := checksums[name]
		if !ok {
			return nil, nil, fmt.Errorf("checksums.json 未声明条目 %s", name)
		}
		actual := sha256Hex(content)
		if !strings.EqualFold(actual, expect) {
			return nil, nil, fmt.Errorf("条目 %s 校验失败（SHA-256 不匹配）", name)
		}
	}

	// 解析包内清单（必填字段校验）
	manifest, err := ParseManifest(entries[ManifestName])
	if err != nil {
		return nil, nil, err
	}
	return manifest, entries, nil
}

// WriteEntries 落盘条目到目标目录（目录条目创建；frontend/ 子目录自动创建）。
// checksums.json 与 signature.sig 为校验材料，不落盘。
func WriteEntries(entries map[string][]byte, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("创建解包目录失败：%w", err)
	}
	for name, content := range entries {
		if name == ChecksumsName || name == SignatureName {
			continue
		}
		// 目录条目（zip 常见 frontend/）直接创建
		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(filepath.Join(destDir, name), 0o755); err != nil {
				return fmt.Errorf("创建目录失败（%s）：%w", name, err)
			}
			continue
		}
		target := filepath.Join(destDir, filepath.Clean(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("创建目录失败（%s）：%w", name, err)
		}
		if err := os.WriteFile(target, content, 0o755); err != nil {
			return fmt.Errorf("写入文件失败（%s）：%w", name, err)
		}
	}
	return nil
}

// Unpack 安全解包 .bpk 到 destDir（Read + WriteEntries 组合；兼容旧调用入口）。
// 需要落盘前验签的安装路径应分别调用 Read → VerifySignature → WriteEntries。
func Unpack(srcPath string, destDir string) (*Manifest, error) {
	manifest, entries, err := Read(srcPath)
	if err != nil {
		return nil, err
	}
	if err := WriteEntries(entries, destDir); err != nil {
		return nil, err
	}
	return manifest, nil
}

// sha256Hex 计算内容 SHA-256（hex 小写；与 pack 侧一致）。
func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
