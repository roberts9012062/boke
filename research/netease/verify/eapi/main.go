// 网易云 eapi 加密的最小验证程序：调用扫码登录 unikey 接口。
package main

import (
	"crypto/aes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AesEncryptECB AES-128-ECB 加密（PKCS7 填充）
func AesEncryptECB(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	pad := block.BlockSize() - len(plaintext)%block.BlockSize()
	padded := make([]byte, len(plaintext)+pad)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(pad)
	}
	out := make([]byte, len(padded))
	for bs := 0; bs < len(padded); bs += block.BlockSize() {
		block.Encrypt(out[bs:bs+block.BlockSize()], padded[bs:bs+block.BlockSize()])
	}
	return out, nil
}

// EapiEncrypt eapi 加密，返回大写 hex 的 params
func EapiEncrypt(url string, object any) (string, error) {
	text, err := json.Marshal(object)
	if err != nil {
		return "", err
	}
	url = strings.Replace(url, "eapi", "api", 1)
	message := fmt.Sprintf("nobody%suse%smd5forencrypt", url, string(text))
	digest := md5.Sum([]byte(message))
	data := fmt.Sprintf("%s-36cd479b6b5-%s-36cd479b6b5-%x", url, string(text), digest)
	cipherText, err := AesEncryptECB([]byte(data), []byte("e82ckenh8dichen8"))
	if err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(cipherText)), nil
}

func main() {
	params, err := EapiEncrypt("/api/login/qrcode/unikey", map[string]any{"type": "3"})
	if err != nil {
		fmt.Println("encrypt err:", err)
		return
	}
	form := url.Values{"params": {params}}
	req, err := http.NewRequest("POST", "https://music.163.com/eapi/login/qrcode/unikey",
		strings.NewReader(form.Encode()))
	if err != nil {
		fmt.Println("req err:", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "NeteaseMusic/9.3.40.1753206443(164);Dalvik/2.1.0 (Linux; U; Android 9; MIX 2 MIUI/V12.0.1.0.PDECNXM)")
	req.Header.Set("Cookie", "os=android; appver=9.3.40; deviceId=0B3A29B4D66C4E1D9F1A2B3C4D5E6F70")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("do err:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("HTTP %d\n%s\n", resp.StatusCode, string(body))
}
