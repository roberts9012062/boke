// internal/service/license.go
// 插件许可证服务（M3.5）：激活（Ed25519 验签）/ 状态查询 / 进程激活时下发许可。
// 对齐 docs/architecture.md 6.5.6：主站只存公钥（安装时登记 pubkey.pem），
//   离线宽限期 7 天（exp 后 7 天内仍可用，超期降级 demo——功能由 SDK FeatureEnabled 锁定）。
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/roberts9012062/boke/internal/plugin/license"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/plugin-sdk/proto"
)

// 宽限期（与 license 包一致：到期后 7 天）。
const licenseGrace = 7 * 24 * time.Hour

// LicenseStatusDTO 许可证状态（我的插件页/激活弹层展示）。
type LicenseStatusDTO struct {
	PluginID  string   `json:"plugin_id"`           // 插件 ID
	Activated bool     `json:"activated"`           // 是否已激活
	Edition   string   `json:"edition"`             // free（demo）/ pro
	Features  []string `json:"features,omitempty"`  // 授权功能（降级后为空）
	ExpiresAt *int64   `json:"expires_at,omitempty"` // 到期时间戳（空=永久）
	Degraded  bool     `json:"degraded"`            // 已降级（超宽限期）
}

// ActivateLicense 激活许可证（后台输入 license.jwt → 验签 → 覆盖写入）。
// 流程：插件存在 + 公钥已登记 → Verify（Ed25519）→ 主体校验（sub=plugin:{id}）→ 落库。
func (s *PluginService) ActivateLicense(ctx context.Context, pluginID string, licenseJWT string) error {
	inst, err := s.plugs.FindByPluginID(ctx, pluginID)
	if err != nil {
		return errs.ErrNotFound
	}
	if inst.Pubkey == "" {
		return errs.New(errs.CodeLicenseInvalid, "插件未登记公钥（免费插件无需激活）")
	}
	pub, err := license.LoadPublicKeyFromPEM(inst.Pubkey)
	if err != nil {
		return errs.New(errs.CodeLicenseInvalid, "插件公钥格式错误")
	}
	lic, err := license.Verify(pub, []byte(licenseJWT))
	if err != nil {
		return errs.New(errs.CodeLicenseInvalid, err.Error())
	}
	// 主体校验：许可证 sub 必须匹配插件（防跨插件许可证复用）
	if lic.Sub != "plugin:"+pluginID {
		return errs.New(errs.CodeLicenseInvalid, "许可证主体与插件不符")
	}
	// 落库（覆盖写入；expires_at 空=永久）
	var expires *time.Time
	if lic.ExpiresAt > 0 {
		t := time.Unix(lic.ExpiresAt, 0)
		expires = &t
	}
	if err := s.licenses.Save(ctx, repository.PluginLicense{
		PluginID: pluginID, Licensee: lic.Licensee, Edition: lic.Edition,
		Features: lic.Features, LicenseJWT: licenseJWT, ExpiresAt: expires,
	}); err != nil {
		return fmt.Errorf("许可证保存失败：%w", err)
	}
	return nil
}

// LicenseStatus 查询许可证状态（未激活返回 demo 态；超宽限期标记降级）。
func (s *PluginService) LicenseStatus(ctx context.Context, pluginID string) (*LicenseStatusDTO, error) {
	lic, err := s.licenses.FindByPluginID(ctx, pluginID)
	if errors.Is(err, repository.ErrNotFound) {
		return &LicenseStatusDTO{PluginID: pluginID, Activated: false, Edition: "free"}, nil
	}
	if err != nil {
		return nil, err
	}
	status := &LicenseStatusDTO{
		PluginID: pluginID, Activated: true, Edition: lic.Edition, Features: lic.Features,
	}
	if lic.ExpiresAt != nil {
		ts := lic.ExpiresAt.Unix()
		status.ExpiresAt = &ts
		// 超宽限期 → 降级（功能锁定，SDK FeatureEnabled 全 false）
		if time.Now().After(lic.ExpiresAt.Add(licenseGrace)) {
			status.Degraded = true
			status.Features = nil
		}
	}
	return status, nil
}

// licenseInfo 进程激活时下发的许可信息（PluginManager.LicenseProvider 装配）。
// 说明：无记录=demo（free）；降级时 features 清空（功能锁定）。
func (s *PluginService) LicenseInfoProvider(ctx context.Context, pluginID string) (*proto.LicenseInfo, error) {
	status, err := s.LicenseStatus(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	info := &proto.LicenseInfo{Edition: status.Edition, Features: status.Features, Degraded: status.Degraded}
	if status.ExpiresAt != nil {
		info.ExpiresAt = *status.ExpiresAt
	}
	return info, nil
}
