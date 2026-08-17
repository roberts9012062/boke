// internal/plugin/bpkg/sign.go
// .bpk 包签名（P1 供应链加固）：对 checksums.json 内容的 Ed25519 签名。
// 由于 checksums 覆盖包内全部条目哈希，对它签名等价于「签名整个包」——
// 打包方无法在签名后替换任何文件（含 checksums 自身，改任意条目即验签失败）。
//
// 信任模型：市场根公钥配置在主站 settings（plugin_pkg_pubkeys，PEM 文本可含多个块）。
// 策略（见 service/plugin_bpk.go 接入处）：
//   - 已配置信任公钥 → 包必须携带签名且任一公钥验签通过，否则拒绝安装/升级；
//   - 未配置 → 放行无签名包（兼容官方存量包与本地开发）。
package bpkg

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// SignatureName 包签名条目名（Ed25519 签名，64 字节原始值；不参与 checksums）。
const SignatureName = "signature.sig"

// SignChecksums 用 Ed25519 私钥对 checksums 内容签名（打包侧使用）。
func SignChecksums(checksums []byte, priv ed25519.PrivateKey) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("私钥格式不正确（期望 Ed25519）")
	}
	return ed25519.Sign(priv, checksums), nil
}

// VerifySignature 用信任公钥列表验包签名（安装侧使用；任一公钥通过即接受）。
// 返回：nil=验签通过；非 nil=缺少签名或全部公钥验签失败（调用方应拒绝安装）。
func VerifySignature(entries map[string][]byte, trustedKeys []ed25519.PublicKey) error {
	if len(trustedKeys) == 0 {
		return errors.New("未配置信任公钥")
	}
	sig, ok := entries[SignatureName]
	if !ok || len(sig) != ed25519.SignatureSize {
		return errors.New("包缺少有效签名（配置了信任公钥时必须分发签名包）")
	}
	checksums, ok := entries[ChecksumsName]
	if !ok {
		return errors.New("包缺少 checksums.json")
	}
	for _, key := range trustedKeys {
		if ed25519.Verify(key, checksums, sig) {
			return nil
		}
	}
	return errors.New("包签名不匹配任何信任公钥")
}

// ParsePublicKeysPEM 解析 PEM 文本中的全部 Ed25519 公钥（可含多个 PUBLIC KEY 块拼接）。
// 严格模式：任一块无法解析为 Ed25519 公钥即报错——防坏钥配置静默导致验签失效。
func ParsePublicKeysPEM(pemText string) ([]ed25519.PublicKey, error) {
	keys := make([]ed25519.PublicKey, 0)
	rest := []byte(pemText)
	for {
		block, next := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = next
		if block.Type != "PUBLIC KEY" {
			return nil, fmt.Errorf("PEM 块类型不正确（%s，期望 PUBLIC KEY）", block.Type)
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("公钥解析失败：%w", err)
		}
		key, ok := parsed.(ed25519.PublicKey)
		if !ok {
			return nil, errors.New("公钥类型不正确（期望 Ed25519）")
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, errors.New("PEM 中未找到公钥")
	}
	return keys, nil
}

// ParsePrivateKeyPEM 解析 PKCS#8 PEM 私钥（打包工具使用）。
func ParsePrivateKeyPEM(pemText string) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("私钥 PEM 格式不正确（期望 PRIVATE KEY 块）")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("私钥解析失败：%w", err)
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("私钥类型不正确（期望 Ed25519）")
	}
	return key, nil
}

// EncodePrivateKeyPEM 私钥编码为 PKCS#8 PEM（keygen 工具使用）。
func EncodePrivateKeyPEM(priv ed25519.PrivateKey) (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", fmt.Errorf("私钥序列化失败：%w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), nil
}

// EncodePublicKeyPEM 公钥编码为 PKIX PEM（keygen 工具使用）。
func EncodePublicKeyPEM(pub ed25519.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("公钥序列化失败：%w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}
