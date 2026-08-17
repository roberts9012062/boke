// internal/plugin/bpkg/pack.go
// .bpk 打包（M3.4，cmd/bp 使用）：zip 封装 manifest.json + 插件文件 + checksums.json。
// 对齐 docs/architecture.md 6.5.6 包结构；条目顺序稳定（可重复构建，哈希可复现）。
// P1 加固：PackSigned 额外写入 signature.sig（对 checksums.json 的 Ed25519 签名）。
package bpkg

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
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

// Pack 打包 .bpk（不签名——本地开发/兼容模式；正式分发走 PackSigned）。
// 参数：manifest 包内清单；files 插件文件（相对路径 → 内容，如 plugin.bin/pubkey.pem/frontend/*）。
// 说明：checksums.json 由本函数生成（覆盖 manifest.json 与全部插件文件，按字典序稳定）。
func Pack(manifest Manifest, files map[string][]byte) ([]byte, error) {
	return packArchive(manifest, files, nil)
}

// PackSigned 打包并签名（市场正式分发）：签名条目 signature.sig 最后写入，不参与 checksums。
// 参数：priv 市场根私钥（Ed25519；签 checksums 内容 = 签整个包，见 sign.go 信任模型）。
func PackSigned(manifest Manifest, files map[string][]byte, priv ed25519.PrivateKey) ([]byte, error) {
	return packArchive(manifest, files, priv)
}

// packArchive 打包实现（signer 为空=不写签名条目）。
func packArchive(manifest Manifest, files map[string][]byte, signer ed25519.PrivateKey) ([]byte, error) {
	// 收集全部条目（manifest.json + 插件文件），字典序稳定
	names := make([]string, 0, len(files)+1)
	names = append(names, ManifestName)
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	// 先计算 checksums（内容哈希，不含 checksums.json 与 signature.sig 自身）
	checksums := make(map[string]string, len(names))
	for _, name := range names {
		content, err := entryContent(manifest, files, name)
		if err != nil {
			return nil, err
		}
		checksums[name] = hashBytes(content)
	}
	checksumRaw, err := json.Marshal(checksums)
	if err != nil {
		return nil, fmt.Errorf("checksums 序列化失败：%w", err)
	}

	// 包签名（可选）：对 checksums 字节签名——覆盖全部条目哈希，签名后任何篡改均验签失败
	var signature []byte
	if signer != nil {
		signature, err = SignChecksums(checksumRaw, signer)
		if err != nil {
			return nil, fmt.Errorf("包签名失败：%w", err)
		}
	}

	// 写 zip（manifest.json → 插件文件 → [signature.sig] → checksums.json）
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, name := range names {
		content, err := entryContent(manifest, files, name)
		if err != nil {
			return nil, err
		}
		entry, err := writer.Create(name)
		if err != nil {
			return nil, fmt.Errorf("zip 条目创建失败（%s）：%w", name, err)
		}
		if _, err := entry.Write(content); err != nil {
			return nil, fmt.Errorf("zip 条目写入失败（%s）：%w", name, err)
		}
	}
	if signature != nil {
		entry, err := writer.Create(SignatureName)
		if err != nil {
			return nil, fmt.Errorf("zip 条目创建失败（%s）：%w", SignatureName, err)
		}
		if _, err := entry.Write(signature); err != nil {
			return nil, fmt.Errorf("zip 条目写入失败（%s）：%w", SignatureName, err)
		}
	}
	// checksums.json 最后写入（不参与自身哈希）
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

// entryContent 取条目内容（manifest.json 由结构体序列化，保证 checksums 计算与写入一致）。
func entryContent(manifest Manifest, files map[string][]byte, name string) ([]byte, error) {
	if name != ManifestName {
		return files[name], nil
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("manifest 序列化失败：%w", err)
	}
	return raw, nil
}

// hashBytes 计算内容 SHA-256（hex 小写）。
func hashBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
