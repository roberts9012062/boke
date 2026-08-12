// internal/service/plugin_order.go
// 插件购买订单服务（M3.9 支付渠道——可插拔设计）：
//   创建订单 → 支付（真实渠道预留：微信/支付宝；开发环境模拟直接成功）
//   → 服务端持私钥签发 license.jwt（私钥 AES 加密存 settings）→ 自动激活。
//   安全：签发私钥加密存储 + 订单状态机（pending → paid/failed）+ 支付幂等。
package service

import (
	"context"

	"github.com/roberts9012062/boke/internal/ai"
	"github.com/roberts9012062/boke/internal/plugin/license"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 服务端签发私钥的 settings 键（PEM 经 AES 加密存储）。
const settingIssuerKey = "plugin_license_private_key"

// 签发许可证的授权功能（付费插件统一 demo_pro——与 demo 插件契约一致）。
const issuedFeature = "demo_pro"

// SetIssuerKey 配置服务端签发私钥（PEM 加密落库；支付签发前必须配置）。
func (s *PluginService) SetIssuerKey(ctx context.Context, privateKeyPEM string) error {
	if _, err := license.ParsePrivateKeyPEM(privateKeyPEM); err != nil {
		return errs.New(errs.CodeBadRequest, "私钥格式不正确："+err.Error())
	}
	encrypted, err := ai.EncryptSecret(privateKeyPEM, s.keySecret)
	if err != nil {
		return errs.New(errs.CodeUpstream, "私钥加密失败："+err.Error())
	}
	if err := s.settings.SetMany(ctx, map[string]string{settingIssuerKey: encrypted}); err != nil {
		return err
	}
	return nil
}

// CreateOrder 创建购买订单（pending；价格由前端传入——dev 模拟场景不核价，
// 真实渠道接入后需服务端定价校验）。
func (s *PluginService) CreateOrder(ctx context.Context, instanceID int64, price int) (int64, error) {
	inst, err := s.plugs.FindByID(ctx, instanceID)
	if err != nil {
		return 0, errs.ErrNotFound
	}
	return s.orders.Create(ctx, inst.PluginID, instanceID, price)
}

// PayOrder 支付订单并签发许可证（幂等：已 paid 直接返回）。
// 流程：读签发私钥 → 签发 license.jwt（sub=插件 ID，pro + demo_pro）→ 落库订单
//       → 自动激活（复用 ActivateLicense 验签链路）。
// 说明：真实支付渠道（微信/支付宝）接入点即本方法前置——回调验签通过后调用签发。
func (s *PluginService) PayOrder(ctx context.Context, orderID int64) (string, error) {
	order, err := s.orders.FindByID(ctx, orderID)
	if err != nil {
		return "", errs.ErrNotFound
	}
	if order.State == repository.OrderPaid {
		return order.LicenseJWT, nil // 幂等：已签发直接返回
	}

	// 读取签发私钥（settings 加密存储）
	encrypted, ok, err := s.settings.Get(ctx, settingIssuerKey)
	if err != nil {
		return "", err
	}
	if !ok || encrypted == "" {
		return "", errs.New(errs.CodeStateConflict, "服务端未配置许可证签发私钥，请先在插件设置中配置")
	}
	pemText, err := ai.DecryptSecret(encrypted, s.keySecret)
	if err != nil {
		return "", errs.New(errs.CodeUpstream, "签发私钥解密失败："+err.Error())
	}
	priv, err := license.ParsePrivateKeyPEM(pemText)
	if err != nil {
		return "", errs.New(errs.CodeUpstream, "签发私钥解析失败："+err.Error())
	}

	// 签发（pro + demo_pro 功能，永久授权）
	raw, err := license.Sign(priv, &license.License{
		Sub: order.PluginID, Licensee: "站点购买",
		Edition: "pro", Features: []string{issuedFeature}, ExpiresAt: 0,
	})
	if err != nil {
		return "", errs.New(errs.CodeUpstream, "许可证签发失败："+err.Error())
	}
	jwt := string(raw)

	// 落库订单 + 自动激活（激活失败不阻断订单——可后续手动激活）
	if err := s.orders.MarkPaid(ctx, orderID, jwt); err != nil {
		return "", err
	}
	_ = s.ActivateLicense(ctx, order.PluginID, jwt)
	return jwt, nil
}
