// 网易云 weapi 加密与播放地址接口的最小验证程序（纯标准库）。
// 用途：验证调研报告中 weapi 加密算法的正确性（匿名获取播放地址）。
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// weapi 固定常量
const (
	presetKey = "0CoJUm6Qyw8W8jud"
	weapiIV   = "0102030405060708"
	base62    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	weapiPub  = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDgtQn2JZ34ZC28NWYpAUd98iZ37BUrX/aKzmFbt7clFSs6sXqHauqKWqdtLkF2KexO40H1YTX8z2lSgBBOAxLsvaklV8k4cBFK9snQXE9/DDaFt6Rr7iVZMldczhC0JNgTz+SHXT6CBHuX3e9SdB1Ua44oncaTWz7OBGLbCiK45wIDAQAB
-----END PUBLIC KEY-----`
)

// AesEncryptCBC AES-128-CBC 加密（PKCS7 填充）
func AesEncryptCBC(plaintext, key, iv []byte) ([]byte, error) {
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
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return out, nil
}

// RsaEncryptNoPadding RSA 无填充公钥加密（输入输出左补零至 128 字节）
func RsaEncryptNoPadding(data []byte, pub *rsa.PublicKey) ([]byte, error) {
	padded := make([]byte, pub.Size())
	copy(padded[pub.Size()-len(data):], data)
	c := new(big.Int).SetBytes(padded)
	m := new(big.Int).Exp(c, big.NewInt(int64(pub.E)), pub.N)
	out := m.Bytes()
	if len(out) < pub.Size() {
		buf := make([]byte, pub.Size())
		copy(buf[pub.Size()-len(out):], out)
		out = buf
	}
	return out, nil
}

func randomSecretKey() []byte {
	b := make([]byte, 16)
	rb := make([]byte, 16)
	_, _ = rand.Read(rb)
	for i := 0; i < 16; i++ {
		b[i] = base62[int(rb[i])%len(base62)]
	}
	return b
}

func reverseBytes(in []byte) []byte {
	out := make([]byte, len(in))
	for i, j := 0, len(in)-1; i < len(in); i, j = i+1, j-1 {
		out[i] = in[j]
	}
	return out
}

// WeapiEncrypt 返回 params 与 encSecKey
func WeapiEncrypt(object any) (string, string, error) {
	text, err := json.Marshal(object)
	if err != nil {
		return "", "", err
	}
	secretKey := randomSecretKey()

	first, err := AesEncryptCBC(text, []byte(presetKey), []byte(weapiIV))
	if err != nil {
		return "", "", err
	}
	firstB64 := base64.StdEncoding.EncodeToString(first)

	second, err := AesEncryptCBC([]byte(firstB64), secretKey, []byte(weapiIV))
	if err != nil {
		return "", "", err
	}
	params := base64.StdEncoding.EncodeToString(second)

	block, _ := pem.Decode([]byte(weapiPub))
	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", "", err
	}
	pub := pubAny.(*rsa.PublicKey)
	encrypted, err := RsaEncryptNoPadding(reverseBytes(secretKey), pub)
	if err != nil {
		return "", "", err
	}
	return params, hex.EncodeToString(encrypted), nil
}

func main() {
	// 构造参数：ids 为字符串形式 JSON 数组
	idsJSON, _ := json.Marshal([]int64{1295601353})
	params := map[string]any{
		"ids":        string(idsJSON),
		"level":      "higher",
		"encodeType": "mp3",
		"csrf_token": "",
	}
	paramsStr, encSecKey, err := WeapiEncrypt(params)
	if err != nil {
		fmt.Println("encrypt err:", err)
		return
	}
	fmt.Printf("params len=%d encSecKey len=%d\n", len(paramsStr), len(encSecKey))

	form := url.Values{"params": {paramsStr}, "encSecKey": {encSecKey}}
	req, err := http.NewRequest("POST", "https://music.163.com/weapi/song/enhance/player/url/v1",
		strings.NewReader(form.Encode()))
	if err != nil {
		fmt.Println("req err:", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")
	req.Header.Set("Referer", "https://music.163.com")
	req.Header.Set("Cookie", "os=pc; appver=9.3.40")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("do err:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("HTTP %d\n%s\n", resp.StatusCode, string(body))

	// 解析并打印关键字段
	var parsed struct {
		Code int `json:"code"`
		Data []struct {
			Id     int64  `json:"id"`
			Url    string `json:"url"`
			Br     int64  `json:"br"`
			Code   int64  `json:"code"`
			Expi   int64  `json:"expi"`
			Type   string `json:"type"`
			Level  string `json:"level"`
			Encode string `json:"encodeType"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &parsed)
	if parsed.Code == 200 && len(parsed.Data) > 0 {
		d := parsed.Data[0]
		fmt.Printf("=> id=%d code=%d br=%d expi=%d type=%s level=%s encodeType=%s\n  url=%s\n",
			d.Id, d.Code, d.Br, d.Expi, d.Type, d.Level, d.Encode, d.Url)
	}
}
