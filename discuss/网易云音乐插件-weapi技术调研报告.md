# 网易云音乐插件 · weapi 技术调研报告（Go 实现）

> 状态：调研完成，可直接进入实现
> 日期：2026-08-13
> 目的：为「网易云音乐插件」补齐 weapi/eapi 加密、登录流程、播放地址接口、登录态持久化的准确技术细节，并给出可直接使用的 Go 代码与现成 Go 库。
> 关联文档：[网易云音乐插件-方案设计.md](./网易云音乐插件-方案设计.md)

---

## 〇、结论速览（TL;DR）

1. **登录方式选择**：
   - **扫码登录**最稳，推荐主推。接口：`/weapi/login/qrcode/unikey` → 轮询 `/weapi/login/qrcode/client/login`（状态 800/801/802/803）。
   - **手机号+密码登录**：经典 weapi 接口 `/weapi/login/cellphone` 目前在部分环境会触发 **8821 行为验证码**，活跃的 Go 项目 [chaunsin/netease-cloud-music](https://github.com/chaunsin/netease-cloud-music) 已改用 **eapi 接口** `/eapi/w/login/cellphone`（注释明确 "use weapi 出现 8821需要行为验证码验证"）。因此**手机号登录请走 eapi 接口**。
2. **现成 Go 库（可直接 import）**：
   - [XiaoMengXinX/Music163Api-Go](https://github.com/XiaoMengXinX/Music163Api-Go)：轻量，纯 eapi 实现，**扫码登录 + 播放地址 + 歌词 + 歌曲详情开箱即用**；缺点：**无手机号密码登录**。
   - [chaunsin/netease-cloud-music](https://github.com/chaunsin/netease-cloud-music)：全功能，weapi/eapi/linuxapi 三种加密全实现，手机号（eapi）、扫码、播放地址、下载全部覆盖；缺点：依赖较重（resty/cobra/badger 等），且部分为 CLI 结构。
   - 推荐：**自研轻量客户端（参考两库源码）+ 扫码登录**，见第六节完整代码。
3. **重要现状**：Node 参考实现 [Binaryify/NeteaseCloudMusicApi](https://github.com/Binaryify/NeteaseCloudMusicApi) **已于 2024 年因版权问题停止维护，仓库被清空**（README 仅剩「保护版权,此仓库不再维护」）。不要直接依赖该仓库，但大量 fork（如 [Huyongqiang/NeteaseCloudMusicApi](https://github.com/Huyongqiang/NeteaseCloudMusicApi)）仍保留完整代码，可作算法对照。
4. **播放地址接口**：推荐 `/weapi/song/enhance/player/url/v1`（level 音质）或同路径 eapi 版；URL 有效期约 **20 分钟**（`expi≈1200`，过期 403），插件侧需缓存 + 定时刷新。

### ✅ 实测验证（2026-08-13，本报告算法已跑通）

用下文 1.1 节的 weapi 加密代码匿名请求 `/weapi/song/enhance/player/url/v1`（歌曲 id=1295601353，level=higher）：

```
HTTP 200
{"code":200, "data":[{ "id":1295601353,
  "url":"http://m8.music.126.net/20260813232904/.../....mp3?vuutv=...",
  "br":192000, "size":5455038, "md5":"8a6bd4...", "code":200, "expi":1200,
  "type":"mp3", "level":"higher", "encodeType":"mp3", "freeTrialInfo":null }]}
```

- 直链 HEAD 验证：`200 / audio/mpeg`，Content-Length 与 `size` 字段完全一致（5455038 字节）✅
- **匿名音质观察**：请求 `level=higher`（期望 320k）实际返回 `br=192000`（192k）——**未登录匿名请求会被降级**，登录后可获取更高音质。插件建议**登录态取 URL**。
- eapi 加密同样验证通过：`/eapi/login/qrcode/unikey` 正常返回 `{"code":200,"unikey":"66a79aa5-..."}` ✅
- 验证源码保留在 `research/netease/verify/`（weapi 版与 eapi 版两个独立 main.go，纯标准库，`go run .` 即可复现）。

---

## 一、加密算法详解

网易云有三套加密：**weapi**（Web 端）、**eapi**（客户端/移动端，MAC/Win/Android/iOS）、**linuxapi**（Linux 客户端）。三套是独立算法，接口路径前缀对应 `/weapi/`、`/eapi/`、`/api/`（linuxapi 为 `/api/` 后缀 `eparams`）。登录态 cookie 三套通用。

### 1.1 weapi（核心，Web 接口加密）

**算法本质**：对请求体 JSON 做「双层 AES-128-CBC + RSA 无填充加密」。

**固定常量**（与官方 JS 一致，全网公开）：

| 常量 | 值 |
|---|---|
| 第一层 AES 密钥 `presetKey` | `0CoJUm6Qyw8W8jud` |
| AES IV | `0102030405060708` |
| 随机密钥字符集 `base62` | `abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789` |
| RSA 公钥（PEM，指数 65537） | 见下方代码 `publicKey` 常量 |

> 说明：经典 JS 实现中的 `modulus = 00e0b509...` 与上述 PEM 是**同一个 RSA 公钥**（PEM 的 DER 编码去掉了前导符号字节 `00`）。两个形式任选其一。

**加密步骤**（以明文 `text = JSON.stringify(请求参数)` 为例）：

1. 生成 16 字节随机密钥 `secretKey`（字符集 base62）。
2. `first = AES-128-CBC(明文=text, key=presetKey, iv)`，输出 **base64** 字符串。
3. `params = AES-128-CBC(明文=first(即上一步的 base64 字符串), key=secretKey, iv)`，输出 **base64** 字符串。
4. `encSecKey = RSA 无填充加密(字节序反转后的 secretKey)`，结果**左补零至 128 字节**，输出 **hex**（256 个小写十六进制字符）。
5. 请求体：`params=<base64>&encSecKey=<hex>`，POST `application/x-www-form-urlencoded`。

**关键易错点**：
- 第二步的明文是第一步的 **base64 字符串本身**（不是二进制密文）。
- `secretKey` 先**反转**再 RSA 加密。
- RSA 加密结果与输入都必须**左补零到 128 字节**（否则 encSecKey 可能不足 256 hex 字符导致校验失败）。

**Go 实现**（可直接使用，来自对 [chaunsin pkg/crypto](https://github.com/chaunsin/netease-cloud-music/blob/master/pkg/crypto/crypto.go) 的整理与修正）：

```go
package netease

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
	"math/big"
	"net/url"
)

// weapi 固定常量（与官方 JS 一致）
const (
	presetKey = "0CoJUm6Qyw8W8jud" // 第一层 AES 密钥
	weapiIV   = "0102030405060708" // AES IV
	base62    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	// RSA 公钥（e=65537），与经典 modulus 00e0b509... 为同一公钥
	weapiPublicKey = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDgtQn2JZ34ZC28NWYpAUd98iZ37BUrX/aKzmFbt7clFSs6sXqHauqKWqdtLkF2KexO40H1YTX8z2lSgBBOAxLsvaklV8k4cBFK9snQXE9/DDaFt6Rr7iVZMldczhC0JNgTz+SHXT6CBHuX3e9SdB1Ua44oncaTWz7OBGLbCiK45wIDAQAB
-----END PUBLIC KEY-----`
)

// AesEncryptCBC AES-128-CBC 加密，PKCS7 填充
func AesEncryptCBC(plaintext, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	// PKCS7 填充
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

// RsaEncryptNoPadding RSA 无填充公钥加密，输入输出均左补零至 128 字节
func RsaEncryptNoPadding(data []byte, pub *rsa.PublicKey) ([]byte, error) {
	if len(data) > pub.Size() {
		return nil, fmt.Errorf("data too long: %d > %d", len(data), pub.Size())
	}
	padded := make([]byte, pub.Size()) // 左补零到 128 字节
	copy(padded[pub.Size()-len(data):], data)
	c := new(big.Int).SetBytes(padded)
	m := new(big.Int).Exp(c, big.NewInt(int64(pub.E)), pub.N)
	out := m.Bytes()
	if len(out) < pub.Size() { // 结果同样左补零
		buf := make([]byte, pub.Size())
		copy(buf[pub.Size()-len(out):], out)
		out = buf
	}
	return out, nil
}

// parseWeapiPublicKey 解析内置 RSA 公钥
func parseWeapiPublicKey() (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(weapiPublicKey))
	if block == nil {
		return nil, fmt.Errorf("decode pem failed")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not rsa public key")
	}
	return rsaPub, nil
}

// randomSecretKey 生成 16 位随机密钥（base62 字符集）
func randomSecretKey() []byte {
	const n = 16
	b := make([]byte, n)
	randBytes := make([]byte, n)
	_, _ = rand.Read(randBytes) // crypto/rand
	for i := 0; i < n; i++ {
		b[i] = base62[int(randBytes[i])%len(base62)]
	}
	return b
}

// reverseBytes 字节序反转
func reverseBytes(in []byte) []byte {
	out := make([]byte, len(in))
	for i, j := 0, len(in)-1; i < len(in); i, j = i+1, j-1 {
		out[i] = in[j]
	}
	return out
}

// WeapiEncrypt weapi 加密：返回 params 与 encSecKey
func WeapiEncrypt(object any) (params, encSecKey string, err error) {
	text, err := json.Marshal(object)
	if err != nil {
		return "", "", err
	}
	secretKey := randomSecretKey() // 1. 随机密钥

	// 2. 第一层 AES：固定 presetKey 加密明文，输出 base64
	first, err := AesEncryptCBC(text, []byte(presetKey), []byte(weapiIV))
	if err != nil {
		return "", "", err
	}
	firstB64 := base64.StdEncoding.EncodeToString(first)

	// 3. 第二层 AES：随机密钥加密"第一层 base64 字符串"，输出 base64 即 params
	second, err := AesEncryptCBC([]byte(firstB64), secretKey, []byte(weapiIV))
	if err != nil {
		return "", "", err
	}
	params = base64.StdEncoding.EncodeToString(second)

	// 4. RSA 无填充加密反转后的随机密钥，hex 即 encSecKey
	pub, err := parseWeapiPublicKey()
	if err != nil {
		return "", "", err
	}
	encrypted, err := RsaEncryptNoPadding(reverseBytes(secretKey), pub)
	if err != nil {
		return "", "", err
	}
	encSecKey = hex.EncodeToString(encrypted)
	return params, encSecKey, nil
}

// WeapiPostForm 构造 weapi POST 表单体
func WeapiPostForm(object any) (url.Values, error) {
	params, encSecKey, err := WeapiEncrypt(object)
	if err != nil {
		return nil, err
	}
	return url.Values{"params": {params}, "encSecKey": {encSecKey}}, nil
}
```

> 说明：`randomSecretKey` 用 `crypto/rand` 生成随机字节后映射到 base62 字符集，也可改用 `math/rand`（chaunsin 原版即如此）。

### 1.2 eapi（客户端接口加密，手机号登录/部分播放地址用）

**算法本质**：对「路径 + 明文 JSON + md5 摘要」整体做 **AES-128-ECB** 加密，输出大写 hex。

**固定常量**：AES 密钥 `e82ckenh8dichen8`；盐串 `36cd479b6b5`。

**加密步骤**：

1. 请求路径中 `eapi` 替换为 `api`（如 `/eapi/w/login/cellphone` → `/api/w/login/cellphone`）。
2. `message = "nobody" + url + "use" + text + "md5forencrypt"`，其中 `text` 为明文 JSON。
3. `digest = md5hex(message)`。
4. `data = url + "-36cd479b6b5-" + text + "-36cd479b6b5-" + digest`。
5. `params = UPPER(hex(AES-128-ECB(data, key=e82ckenh8dichen8)))`。
6. 请求体：`params=<大写hex>`，POST `application/x-www-form-urlencoded`。

**Go 实现**（以下函数追加到 crypto.go；补充 import `crypto/md5`、`strings`）：

```go
// AesEncryptECB AES-128-ECB 加密，PKCS7 填充
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

// EapiEncrypt eapi 加密：返回 params（大写 hex）
func EapiEncrypt(url string, object any) (string, error) {
	text, err := json.Marshal(object)
	if err != nil {
		return "", err
	}
	url = strings.Replace(url, "eapi", "api", 1) // 路径中 eapi 需替换为 api
	message := fmt.Sprintf("nobody%suse%smd5forencrypt", url, string(text))
	digest := md5.Sum([]byte(message))
	data := fmt.Sprintf("%s-36cd479b6b5-%s-36cd479b6b5-%x", url, string(text), digest)
	cipherText, err := AesEncryptECB([]byte(data), []byte("e82ckenh8dichen8"))
	if err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(cipherText)), nil
}
```

> 参考实现：[Music163Api-Go utils/request.go](https://github.com/XiaoMengXinX/Music163Api-Go/blob/master/utils/request.go)（`SpliceStr`/`Format2Params`）、[chaunsin pkg/crypto EApiEncrypt](https://github.com/chaunsin/netease-cloud-music/blob/master/pkg/crypto/crypto.go)。

### 1.3 linuxapi（仅了解，一般用不到）

AES-128-ECB，密钥 `rFgB&h#%2?^eDg:Q`，输出 `eparams`（大写 hex），无 iv。适用于 Linux 客户端接口。

---

## 二、登录流程

### 2.1 手机号 + 密码登录

**⚠️ 关键结论**：weapi 接口 `/weapi/login/cellphone` 是历史方案，现环境可能返回 `code: 8821`（需要行为验证码）。**当前应使用 eapi 接口**：

| 项 | 值 |
|---|---|
| URL | `https://interface.music.163.com/eapi/w/login/cellphone` |
| 加密 | eapi（见 1.2），路径参数传 `/api/w/login/cellphone` |
| 请求体 | `params=<大写hex>` |
| 参数 | `phone`、`countrycode`（默认 86）、`password`（**md5 hex 小写**）或 `captcha`、`remember`（`"true"`）、`type`（`"1"` 手机号）、`https`（`"true"`） |
| 返回 | `{code:200, loginType, token(MUSIC_U), account:{id,...}, profile:{nickname,avatarUrl,...}, bindings:[...]}`，**Set-Cookie 带 MUSIC_U/__csrf 等** |
| 常见错误码 | 400（参数错）、509（密码错误）、8821（需行为验证）、-460（风控 Cheating） |

参数构造参考 [chaunsin api/weapi/login.go LoginCellphone](https://github.com/chaunsin/netease-cloud-music/blob/master/api/weapi/login.go)：

```go
// LoginCellphoneReq 手机号登录请求参数
type LoginCellphoneReq struct {
	Phone       string `json:"phone"`
	Countrycode int64  `json:"countrycode"`
	Password    string `json:"password,omitempty"` // 明文密码，发送前 md5
	Captcha     string `json:"captcha,omitempty"`  // 短信验证码（与密码二选一）
	Remember    bool   `json:"remember"`
}

// LoginCellphone 手机号登录（eapi 接口）
func LoginCellphone(client *http.Client, req LoginCellphoneReq) (*LoginResp, error) {
	params := map[string]any{
		"phone":       req.Phone,
		"countrycode": req.Countrycode,
		"remember":    fmt.Sprintf("%v", req.Remember),
		"type":        "1",  // 0: 邮箱 1: 手机号
		"https":       "true",
	}
	if req.Countrycode <= 0 {
		params["countrycode"] = 86
	}
	if req.Captcha != "" {
		params["captcha"] = req.Captcha
	} else if req.Password != "" {
		params["password"] = md5Hex(req.Password) // md5 小写 hex
	} else {
		return nil, errors.New("password or captcha required")
	}
	enc, err := EapiEncrypt("/api/w/login/cellphone", params)
	if err != nil {
		return nil, err
	}
	body, header, err := postForm(client, "https://interface.music.163.com/eapi/w/login/cellphone",
		url.Values{"params": {enc}})
	if err != nil {
		return nil, err
	}
	// 1. 从 resp.Header 的 Set-Cookie 提取并保存 MUSIC_U、__csrf 等（见第四节）
	// 2. 解析 body 为 LoginResp
	var resp LoginResp
	_ = json.Unmarshal(body, &resp)
	return &resp, nil
}
```

**短信验证码登录**（备选，参考 [chaunsin](https://github.com/chaunsin/netease-cloud-music/blob/master/internal/ncmctl/login_phone.go)）：

1. `POST /weapi/sms/captcha/sent`，参数 `cellphone`、`ctcode=86`、`secrete=music_middleuser_pclogin` → 发验证码（24h 限 5 次）。
2. `POST /weapi/sms/captcha/verify`，参数 `cellphone`、`captcha`、`ctcode=86` → 校验。
3. 调 `LoginCellphone` 时传 `captcha` 代替 `password`。

### 2.2 扫码登录（推荐主推）

**流程**（三步，weapi 版本；[chaunsin weapi](https://github.com/chaunsin/netease-cloud-music/blob/master/api/weapi/login.go) 与 [Music163Api-Go](https://github.com/XiaoMengXinX/Music163Api-Go/blob/master/api/qrUnikey.go) 均有实现，Binaryify fork 的 [login_qr_key.js](https://github.com/Huyongqiang/NeteaseCloudMusicApi/blob/master/module/login_qr_key.js) 作算法对照）：

1. **获取 unikey**：`POST https://music.163.com/weapi/login/qrcode/unikey`，body weapi 加密 `{"type":1}` → 返回 `{code:200, unikey:"xxx", qrurl:"https://music.163.com/login?codekey=xxx"}`。
2. **生成二维码**：内容为 `https://music.163.com/login?codekey={unikey}`（web 端可附加 `&chainId=...`，不传也可扫）。
3. **轮询登录状态**：`POST https://music.163.com/weapi/login/qrcode/client/login`，body weapi 加密 `{"key":unikey,"type":1}`，**每 2~3 秒**一次，直至：

| code | 含义 | 处理 |
|---|---|---|
| 800 | 二维码不存在/已过期/用户取消 | 终止，重新生成 |
| 801 | 等待扫码 | 继续轮询 |
| 802 | 已扫码，等待确认 | 继续轮询 |
| 803 | 授权成功 | **Set-Cookie 返回 MUSIC_U 等登录态**，保存后退出轮询 |

成功后调用 `/weapi/w/nuser/account/get` 校验并取用户信息。

**Go 实现**（weapi 版核心轮询）：

```go
// QrCreateKey 第一步：获取扫码登录 unikey
func QrCreateKey(client *http.Client) (string, error) {
	form, err := WeapiPostForm(map[string]any{"type": 1})
	if err != nil {
		return "", err
	}
	body, _, err := postForm(client, "https://music.163.com/weapi/login/qrcode/unikey", form)
	if err != nil {
		return "", err
	}
	var resp struct {
		Code   int    `json:"code"`
		UniKey string `json:"unikey"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if resp.Code != 200 || resp.UniKey == "" {
		return "", fmt.Errorf("create qr key failed: %s", string(body))
	}
	return resp.UniKey, nil
}

// QrCheck 第三步：查询扫码状态；803 表示登录成功
func QrCheck(client *http.Client, key string) (code int, err error) {
	form, err := WeapiPostForm(map[string]any{"key": key, "type": 1})
	if err != nil {
		return 0, err
	}
	body, _, err := postForm(client, "https://music.163.com/weapi/login/qrcode/client/login", form)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}
	return resp.Code, nil
}

// QrLogin 完整扫码登录：轮询直到 803，返回最新 cookie 集合
func QrLogin(client *http.Client, onQr func(qrContent string)) (*http.Response, error) {
	key, err := QrCreateKey(client)
	if err != nil {
		return nil, err
	}
	if onQr != nil {
		onQr("https://music.163.com/login?codekey=" + key) // 前端据此生成二维码
	}
	for i := 0; i < 120; i++ { // 最长约 6 分钟
		code, err := QrCheck(client, key)
		if err != nil {
			return nil, err
		}
		switch code {
		case 803: // 登录成功，此时 client 的 cookie jar 已收到 Set-Cookie
			return nil, nil
		case 800:
			return nil, errors.New("qr expired or canceled")
		case 801, 802:
			time.Sleep(3 * time.Second)
		default:
			return nil, fmt.Errorf("qr login unexpected code: %d", code)
		}
	}
	return nil, errors.New("qr login timeout")
}
```

> 扫码登录的 eapi 版本路径为 `/eapi/login/qrcode/unikey`、`/eapi/login/qrcode/client/login`（[Music163Api-Go](https://github.com/XiaoMengXinX/Music163Api-Go/blob/master/api/qrUnikey.go) 用此版，body `{"type":"3"}`），两种都可正常工作。

---

## 三、获取播放地址

### 3.1 接口选择

| 接口 | URL | 音质参数 | 说明 |
|---|---|---|---|
| 老接口（br） | `https://interface.music.163.com/weapi/song/enhance/player/url` | `br`（128000/192000/320000/999000） | 历史接口，[chaunsin SongPlayer](https://github.com/chaunsin/netease-cloud-music/blob/master/api/weapi/song.go) 在用，需 `csrf_token` |
| **新接口 v1（推荐）** | `https://music.163.com/weapi/song/enhance/player/url/v1` | `level`（standard/exhigh/higher/lossless/hires/sky/jyeffect/jymaster）+ `encodeType`（mp3/aac/flac） | 现行主流，Binaryify fork 与 chaunsin 均支持 |
| eapi 版 v1 | `https://interface.music.163.com/eapi/song/enhance/player/url/v1` | 同上 | [Music163Api-Go GetSongURL](https://github.com/XiaoMengXinX/Music163Api-Go/blob/master/api/songURL.go) 在用，路径参数 `/api/song/enhance/player/url/v1` |

**推荐**：weapi 版 v1（`/weapi/song/enhance/player/url/v1`），参数 `ids`、`level`、`encodeType`、`csrf_token`。

### 3.2 参数细节

- `ids`：**字符串形式的 JSON 数组**，如 `"[123,456]"`（元素数字或字符串均可；Binaryify 用 `'[' + id + ']'`，chaunsin 用自定义 MarshalJSON 输出 `"[1,2]"`）。
- `level` 与 `br` 对应关系：`standard`≈128k、`exhigh`≈192k、`higher`≈320k、`lossless`≈无损 FLAC、`hires`≈Hi-Res FLAC。
- `encodeType`：`mp3` / `aac` / `flac`。**要 mp3 直链：`level=higher&encodeType=mp3`（320k）或 `level=exhigh&encodeType=mp3`（192k）**。
- `csrf_token`：登录后 cookie `__csrf` 的值（weapi 接口传入更稳；[chaunsin](https://github.com/chaunsin/netease-cloud-music/blob/master/api/api.go) 同时放 URL query 与 body）。
- 无需登录（匿名）也能拿到免费歌曲 URL，但**非 VIP 有音质限制**：非 VIP 账号经 v1 接口通常最高 `higher`（320k mp3）；VIP 歌曲仅返回**试听片段**（`freeTrialInfo` 非空，url 为 1 分钟片段地址）。

### 3.3 返回结构

```json
{
  "code": 200,
  "data": [
    {
      "id": 1295601353,
      "url": "http://m8.music.126.net/20211031014702/3ace.../ymusic/....mp3",
      "br": 320000,
      "size": 12345678,
      "md5": "d2db5cbbef195ff34812eb8c82c83d67",
      "code": 200,
      "expi": 1200,
      "type": "mp3",
      "gain": 0.0,
      "fee": 0,
      "payed": 0,
      "flag": 0,
      "freeTrialInfo": null,
      "level": "higher",
      "encodeType": "mp3"
    }
  ]
}
```

**字段与状态码含义**（参考 [chaunsin SongPlayerRespData](https://github.com/chaunsin/netease-cloud-music/blob/master/api/weapi/song.go)）：

- 顶层 `code`：`200` 成功；`-460` Cheating（风控，未登录高频请求常见）；`301` 等。
- `data[].code`：`200` 正常；`404` 歌曲下架（变灰，url 为 null）；`-110` 无版权/不可播（url 为 null）。
- `url`：真实播放地址（m8/m801/m804.music.126.net CDN 或 http 直链），**有效期约 20 分钟**（`expi` 单位秒，实测 1200），过期访问返回 403，需重新获取。
- `freeTrialInfo`：非空表示 VIP 歌曲试听片段（含 `endTime`/`startTime`/`remainTime`），完整播放需 VIP 账号或购买。
- `size`/`md5`：文件大小与 md5，可校验完整性。

### 3.4 Go 实现

```go
// SongURLReq 播放地址请求参数
type SongURLReq struct {
	Ids        []int64 `json:"-"`          // 歌曲 id 列表
	Level      string  `json:"level"`      // standard/exhigh/higher/lossless/hires
	EncodeType string  `json:"encodeType"` // mp3/aac/flac
	CSRFToken  string  `json:"csrf_token,omitempty"`
}

// SongURLData 播放地址返回项
type SongURLData struct {
	Id            int64  `json:"id"`
	Url           string `json:"url"`
	Br            int64  `json:"br"`
	Size          int64  `json:"size"`
	Md5           string `json:"md5"`
	Code          int64  `json:"code"` // 200 正常 / 404 下架 / -110 无版权
	Expi          int64  `json:"expi"` // 有效期秒，约 1200
	Type          string `json:"type"`
	Fee           int64  `json:"fee"`
	Payed         int64  `json:"payed"`
	FreeTrialInfo any    `json:"freeTrialInfo"`
	Level         string `json:"level"`
	EncodeType    string `json:"encodeType"`
}

// GetSongURL 获取歌曲播放地址（weapi v1 接口）
func GetSongURL(client *http.Client, req SongURLReq) ([]SongURLData, error) {
	idsJSON, _ := json.Marshal(req.Ids) // 输出 "[1,2]" 字符串形式
	params := map[string]any{
		"ids":       string(idsJSON),
		"level":     req.Level,
		"encodeType": req.EncodeType,
	}
	if req.CSRFToken != "" {
		params["csrf_token"] = req.CSRFToken
	}
	form, err := WeapiPostForm(params)
	if err != nil {
		return nil, err
	}
	body, _, err := postForm(client, "https://music.163.com/weapi/song/enhance/player/url/v1", form)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Code int           `json:"code"`
		Data []SongURLData `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("song url api code=%d body=%s", resp.Code, string(body))
	}
	return resp.Data, nil
}
```

---

## 四、登录态持久化

### 4.1 需要保存的 cookie 字段

| Cookie | 是否必须 | 说明 |
|---|---|---|
| `MUSIC_U` | **必须** | 登录凭证本体，等价于登录态；丢失即掉线 |
| `__csrf` / `__csrf_token` | 建议 | weapi 接口的 `csrf_token` 参数来源（[chaunsin GetCSRF](https://github.com/chaunsin/netease-cloud-music/blob/master/api/api.go) 二者都读） |
| `MUSIC_A` | 可选 | 匿名用户凭证（未登录时也有）；登录后一般会被覆盖 |
| `os` / `appver` / `deviceId` / `NMTID` | 建议 | 模拟客户端环境，降低风控；Music163Api-Go 自动补充 `os=Android/appver/buildver/deviceId`（[request.go](https://github.com/XiaoMengXinX/Music163Api-Go/blob/master/utils/request.go)） |
| `__remember_me` | 可选 | `true`，chaunsin 默认附加 |

### 4.2 持久化方案

**方案 A（推荐，插件化最简）**：从登录响应 `Set-Cookie` 中提取关键 cookie，AES 加密后存 `data/plugins/netease-music/state.json`（与[方案设计](./网易云音乐插件-方案设计.md) 3.3 一致）：

```json
{ "MUSIC_U": "...", "__csrf": "...", "os": "pc", "appver": "9.3.40", "deviceId": "..." }
```

每次请求前将 state 拼成 `Cookie` 头；`deviceId` 首次登录时生成并固定（52 位 hex 或 UUID 拼接，见 [chaunsin GenerateDeviceId](https://github.com/chaunsin/netease-cloud-music/blob/master/pkg/utils/utils.go)）。

**方案 B**：使用 `net/http/cookiejar`（[Music163Api-Go](https://github.com/XiaoMengXinX/Music163Api-Go/blob/master/utils/request.go) 的做法）把全部 cookie 交给 jar 自动管理，jar 序列化落盘（chaunsin 用自研 [pkg/cookie](https://github.com/chaunsin/netease-cloud-music/tree/master/pkg/cookie) 持久化到 badger/文件）。

### 4.3 登录态校验与过期

- 校验：`POST https://music.163.com/weapi/w/nuser/account/get`，返回 `code=200` 且 `account`/`profile` 非空即有效（[chaunsin NeedLogin](https://github.com/chaunsin/netease-cloud-music/blob/master/api/weapi/weapi.go) 的判断逻辑）。
- 过期/失效：MUSIC_U 被风控或失效后接口返回 `301`/`-462`（登录状态失效）等；此时应清理 state 并引导站长重新登录。存在 `login/token/refresh` 接口可刷新，但 chaunsin 实测有 400 问题，**不建议依赖**，直接重登更稳。

---

## 五、现成 Go 库对比

| 维度 | [XiaoMengXinX/Music163Api-Go](https://github.com/XiaoMengXinX/Music163Api-Go) | [chaunsin/netease-cloud-music](https://github.com/chaunsin/netease-cloud-music) |
|---|---|---|
| 加密模式 | 仅 eapi | weapi + eapi + linuxapi |
| 手机号密码登录 | ❌ 无 | ✅（eapi 接口，绕开 8821） |
| 扫码登录 | ✅（eapi 版） | ✅（weapi 版 + eapi 版） |
| 播放地址 | ✅ `GetSongURL`（v1/level） | ✅ `SongPlayer`/`SongPlayerV1`（br/level） |
| 歌词/详情/搜索/歌单等 | 有（覆盖约 40 个 API） | 有（覆盖更广，含云盘/签到/VIP） |
| 依赖 | 极轻（仅 google/uuid，标准库加密） | 重（resty/cobra/badger/brotli 等） |
| Go 版本 | go 1.17+ | go 1.25 |
| 维护活跃度 | 一般（2024 后有提交） | 高（2025 仍在更新，含 CLI 工具 ncmctl） |
| 适用场景 | **插件轻量集成首选**（若接受无密码登录） | 全功能参考/直接复用 |

**建议**：插件以**自研轻量客户端**为主（参考两库源码，加密与登录逻辑约 300 行），扫码登录作为主要登录方式；如必须支持手机号密码登录，按第二节的 eapi 方案实现。若想最快跑通，可先直接 import `Music163Api-Go`（扫码 + 播放地址开箱即用），后续再替换为自研。

---

## 六、推荐落地架构（自研最小方案）

参考[方案设计](./网易云音乐插件-方案设计.md) 3.1，`netease/` 子包按职责拆分（遵守 AGENTS.md：每文件 ≤400 行、每目录 ≤8 文件、中文注释、函数式优先）：

```
cmd/netease-music-plugin/netease/
├── crypto.go      # weapi/eapi/linuxapi 加密（纯函数，见第一节代码）
├── client.go      # HTTP 客户端：Cookie 管理、postForm、UA/Header 注入
├── login.go       # 手机号登录(eapi) + 扫码登录(weapi) + 登出
├── song.go        # 播放地址 + 歌曲详情 + 搜索 + 歌词
├── state.go       # 登录态 AES 加密持久化（state.json 读写）
└── types.go       # 请求/响应强类型定义
```

插件 API 层复用[方案设计](./网易云音乐插件-方案设计.md) 3.2 的 `/login`、`/status`、`/song/{id}/url` 等路由。播放地址接口需要**缓存 + 提前刷新**（URL 有效期 20 分钟）：存 `(songId → {url, expireAt})`，expireAt 前 2 分钟重新拉取。

---

## 七、风险与注意事项

1. **版权风险（最高优先）**：网易云未开放官方 API；2024 年 Binaryify/NeteaseCloudMusicApi 因版权被要求停更、仓库清空（[相关新闻](https://www.landiannews.com/archives/101953.html)、[ithome](https://www.ithome.com/0/746/942.htm)）。插件应低调、限频、仅供站长自用，避免大流量抓取。
2. **风控**：高频请求触发 `-460 Cheating`（[Binaryify issue #289](https://github.com/Binaryify/NeteaseCloudMusicApi/issues/289) 有记录）；手机号密码登录可能触发 `8821` 行为验证码 → 走 eapi 接口或提示站长扫码。
3. **URL 时效**：播放地址 20 分钟过期（403），必须缓存 + 刷新；不要直接把 URL 长期存库。
4. **音质/版权限制**：非 VIP 最高约 320k（higher）；VIP 歌曲仅试听片段；无版权歌曲（变灰）url 为空。
5. **IP 要求**：部分接口对境外 IP 限制（如 QQ 音乐需国内 IP；网易云相对宽松但建议国内服务器部署）。
6. 加密常量若失效（网易改算法），需重新逆向；目前（2026-08）weapi/eapi 常量经 chaunsin（2025 年活跃维护）验证仍然有效。

---

## 八、参考资料

**Go 实现（主要参考）**
- [XiaoMengXinX/Music163Api-Go](https://github.com/XiaoMengXinX/Music163Api-Go) — 轻量 eapi Go 实现（crypto.go / request.go / qrUnikey.go / qrCheck.go / songURL.go）
- [chaunsin/netease-cloud-music](https://github.com/chaunsin/netease-cloud-music) — 全功能 weapi+eapi Go 实现（pkg/crypto/crypto.go、api/weapi/login.go、api/weapi/song.go、api/api.go）
- [pkg.go.dev: Music163Api-Go](https://pkg.go.dev/github.com/XiaoMengXinX/Music163Api-Go@v0.1.18)

**Node 参考（算法对照；原仓库已停更）**
- [Binaryify/NeteaseCloudMusicApi](https://github.com/Binaryify/NeteaseCloudMusicApi)（已停更清空）
- [Huyongqiang/NeteaseCloudMusicApi（fork）](https://github.com/Huyongqiang/NeteaseCloudMusicApi) — util/crypto.js、module/login_cellphone.js、module/song_url_v1.js、module/login_qr_key.js、module/login_qr_check.js

**逆向资料**
- [网易云音乐 params/encSecKey 生成原理](https://blog.csdn.net/weixin_39643061/article/details/119950551)
- [JS 逆向：网易云 AES+RSA 双层加密分析](http://www.chinadongda.com/j/?2402_88323260/article/details/161540263)
- [js 逆向——网易云音乐爬虫](https://www.cnblogs.com/wxd501/p/17029087.html)
- [调用网易云二维码登录 API 流程](https://juejin.cn/post/7232947178691723322)

**其他**
- [Binaryify #289：-460 Cheating 讨论](https://github.com/Binaryify/NeteaseCloudMusicApi/issues/289)
- [网易云音乐 NodeJS 开源 API 停更新闻（landiannews）](https://www.landiannews.com/archives/101953.html)
