// internal/ai/crypto.go
// AI 供应商 API Key 加解密（M4）：AES-256-GCM 对称加密。
//
// 用途：ai_providers.api_key_encrypted 字段 —— 数据库中不落明文 API Key，
//       后台保存时加密、调用时解密（密钥由 config.AIKeySecret 派生）。
// 说明：密钥经 sha256 定长派生为 32 字节（AES-256），输出 base64 便于存储。
package ai

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// EncryptSecret 加密明文密钥（AES-256-GCM，随机 nonce 前置）。
// 返回：base64( nonce(12) + ciphertext )；纯函数，不修改入参。
func EncryptSecret(secret string, keySecret string) (string, error) {
	block, err := aes.NewCipher(deriveKey(keySecret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	// 随机 nonce（GCM 标准 12 字节）
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	// 加密输出 = nonce + ciphertext（gcm.Seal 追加密文到 nonce 后）
	sealed := gcm.Seal(nonce, nonce, []byte(secret), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptSecret 解密 API Key（EncryptSecret 的逆操作）。
// 返回：明文；密文格式非法时返回错误。
func DecryptSecret(encrypted string, keySecret string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(deriveKey(keySecret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("密文长度不足")
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// deriveKey 将任意长度的密钥种子派生为 AES-256 定长密钥（sha256，32 字节）。
// 纯函数：相同输入恒有相同输出。
func deriveKey(keySecret string) []byte {
	sum := sha256.Sum256([]byte(keySecret))
	return sum[:]
}
