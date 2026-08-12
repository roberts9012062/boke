// internal/plugin/bpkg/pack.go
// .bpk 打包（M3.4，cmd/bp 使用）：zip 封装 manifest.json + 插件文件 + checksums.json。
// 对齐 docs/architecture.md 6.5.6 包结构；条目顺序稳定（可重复构建，哈希可复现）。
package bpkg

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// 包内固定条目名（与文档规格一致）。
const (
	ManifestName   = "manifest.json"   // 包内清单
	ChecksumsName  = "checksums.json"  // 各文件 SHA-256（不含自身）
	PluginBinName  = "plugin.bin"      // 插件二进制
	PubkeyName     = "pubkey.pem"      // 许可证公钥（付费插件，M3.5）
	FrontendDir    = "frontend/"       // 前端扩展资产目录（M3.6）
)

// Pack 打包 .bpk（返回 zip 内容）。
// 参数：manifest 包内清单；files 插件文件（相对路径 → 内容，如 plugin.bin/pubkey.pem/frontend/*）。
// 说明：checksums.json 由本函数生成（覆盖 manifest.json 与全部插件文件，按字典序稳定）。
func Pack(manifest Manifest, files map[string][]byte) ([]byte, error) {
	// 收集全部条目（manifest.json + 插件文件），字典序稳定
	names := make([]string, 0, len(files)+1)
	names = append(names, ManifestName)
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	// 先计算 checksums（内容哈希，不含 checksums.json 自身）
	checksums := make(map[string]string, len(names))
	for _, name := range names {
		content := files[name]
		if name == ManifestName {
			raw, err := json.Marshal(manifest)
			if err != nil {
				return nil, fmt.Errorf("manifest 序列化失败：%w", err)
			}
			content = raw
		}
		checksums[name] = hashBytes(content)
	}

	// 写 zip（manifest.json → 插件文件 → checksums.json）
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, name := range names {
		content := files[name]
		if name == ManifestName {
			raw, err := json.Marshal(manifest)
			if err != nil {
				return nil, fmt.Errorf("manifest 序列化失败：%w", err)
			}
			content = raw
		}
		entry, err := writer.Create(name)
		if err != nil {
			return nil, fmt.Errorf("zip 条目创建失败（%s）：%w", name, err)
		}
		if _, err := entry.Write(content); err != nil {
			return nil, fmt.Errorf("zip 条目写入失败（%s）：%w", name, err)
		}
	}
	// checksums.json 最后写入（不参与自身哈希）
	checksumRaw, err := json.Marshal(checksums)
	if err != nil {
		return nil, fmt.Errorf("checksums 序列化失败：%w", err)
	}
	entry, err := writer.Create(ChecksumsName)
	if err != nil {
		return nil, fmt.Errorf("zip 条目创建失败（%s）：%w", ChecksumsName, err)
	}
	if _, err := entry.Write(checksumRaw); err != nil {
		return nil, fmt.Errorf("zip 条目写入失败（%s）：%w", ChecksumsName, err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("zip 写入失败：%w", err)
	}
	return buf.Bytes(), nil
}

// hashBytes 计算内容 SHA-256（hex 小写）。
func hashBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
