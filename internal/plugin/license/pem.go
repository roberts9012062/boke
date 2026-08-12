// internal/plugin/license/pem.go
// Ed25519 密钥对 PEM 读写（M3.5）：作者侧生成/保存，主站登记公钥。
// 格式：私钥 PKCS8（x509.MarshalPKCS8PrivateKey）、公钥 PKIX（x509.MarshalPKIXPublicKey）。
package license

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// GenerateKeyPair 生成 Ed25519 密钥对。
func GenerateKeyPair() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("生成密钥失败：%w", err)
	}
	return priv, pub, nil
}

// SavePrivateKey 私钥写 PEM 文件（PKCS8）。
func SavePrivateKey(path string, priv ed25519.PrivateKey) error {
	raw, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("私钥编码失败：%w", err)
	}
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw}), 0o600)
}

// SavePublicKey 公钥写 PEM 文件（PKIX）。
func SavePublicKey(path string, pub ed25519.PublicKey) error {
	raw, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return fmt.Errorf("公钥编码失败：%w", err)
	}
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: raw}), 0o644)
}

// LoadPrivateKey 读 PEM 私钥（PKCS8 Ed25519）。
func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取私钥失败：%w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("私钥 PEM 解析失败")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("私钥解析失败：%w", err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("私钥类型错误（非 Ed25519）")
	}
	return priv, nil
}

// LoadPublicKey 读 PEM 公钥（PKIX Ed25519）。
func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取公钥失败：%w", err)
	}
	return ParsePublicKey(raw)
}

// LoadPublicKeyFromPEM 从 PEM 字符串解析公钥（M3.5：安装时登记的 pubkey_pem 字段）。
func LoadPublicKeyFromPEM(pemContent string) (ed25519.PublicKey, error) {
	return ParsePublicKey([]byte(pemContent))
}

// ParsePublicKey 解析 PEM 公钥内容（PKIX Ed25519）。
func ParsePublicKey(raw []byte) (ed25519.PublicKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("公钥 PEM 解析失败")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("公钥解析失败：%w", err)
	}
	pub, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("公钥类型错误（非 Ed25519）")
	}
	return pub, nil
}
