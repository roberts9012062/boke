// internal/plugin/license/license.go
// 插件许可证（M3.5）：Ed25519 签发/验签 + 有效期判断。
// 对齐 docs/architecture.md 6.5.6 + plugin-dev-guide.md 9.3：
//   license.jwt = {sub, licensee, edition, features, exp, signature: base64(ed25519)}
//   签名消息 = 去掉 signature 字段的规范化 JSON（签发/验签同一结构序列化，防字段序漂移）。
//   主站只存公钥（安装时登记 pubkey.pem），私钥由作者自持（cmd/license-issue）。
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// 离线宽限期（文档 6.5.6：到期后 7 天内仍可用，超期自动降级 demo 模式）。
const GraceDays = 7 * 24 * time.Hour

// License 许可证（license.jwt 内容；signature 为 base64 编码的 Ed25519 签名）。
type License struct {
	Sub       string   `json:"sub"`       // 主体（如 "plugin:seo-helper"）
	Licensee  string   `json:"licensee"`  // 被许可方（站点 ID / 用户）
	Edition   string   `json:"edition"`   // 版本：free / pro
	Features  []string `json:"features"`  // 授权功能列表
	ExpiresAt int64    `json:"exp"`       // 到期时间戳（Unix 秒；0=永久）
	Signature string   `json:"signature"` // base64(ed25519 签名)，不参与签名消息
}

// Sign 签发许可证（作者侧 cmd/license-issue 调用）。
// 参数：priv 作者私钥；lic 待签名内容（Signature 字段忽略，由本函数填充）。
func Sign(priv ed25519.PrivateKey, lic *License) ([]byte, error) {
	if lic.Sub == "" || lic.Edition == "" {
		return nil, fmt.Errorf("许可证缺少必填字段（sub/edition）")
	}
	// 签名消息 = 去掉 signature 的规范化 JSON（字段序固定，见结构体定义）
	lic.Signature = "" // 置空确保签名消息一致
	message, err := json.Marshal(lic)
	if err != nil {
		return nil, fmt.Errorf("许可证序列化失败：%w", err)
	}
	lic.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, message))
	return json.Marshal(lic)
}

// Verify 验签许可证（主站激活接口调用）。
// 参数：pub 插件公钥（安装时登记）；raw license.jwt 内容。
// 返回：解析后的许可证；验签失败或格式错误返回错误。
func Verify(pub ed25519.PublicKey, raw []byte) (*License, error) {
	var lic License
	if err := json.Unmarshal(raw, &lic); err != nil {
		return nil, fmt.Errorf("许可证解析失败：%w", err)
	}
	if lic.Signature == "" {
		return nil, fmt.Errorf("许可证缺少签名")
	}
	// 先取签名（随后置空重建签名消息——与签发时一致），再解码比对
	signature := lic.Signature
	lic.Signature = ""
	message, err := json.Marshal(lic)
	if err != nil {
		return nil, fmt.Errorf("许可证序列化失败：%w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return nil, fmt.Errorf("许可证签名解码失败")
	}
	if !ed25519.Verify(pub, message, sig) {
		return nil, fmt.Errorf("许可证签名校验失败")
	}
	lic.Signature = signature // 恢复签名（调用方可读取原始值）
	return &lic, nil
}

// IsExpired 判断许可证是否已过期（now 为当前时间）。
// 说明：exp=0 视为永久有效。
func (l *License) IsExpired(now time.Time) bool {
	return l.ExpiresAt > 0 && now.Unix() >= l.ExpiresAt
}

// IsDegraded 判断是否已过宽限期（exp 到期超过 7 天 → 降级 demo）。
// 说明：宽限期内（exp 后 7 天内）仍保持全功能，超期降级。
func (l *License) IsDegraded(now time.Time) bool {
	if l.ExpiresAt <= 0 {
		return false // 永久许可
	}
	graceEnd := time.Unix(l.ExpiresAt, 0).Add(GraceDays)
	return now.After(graceEnd)
}
