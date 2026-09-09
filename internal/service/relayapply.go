// 中继站申请流（从 relay.go 拆出）：自助申请（v1.5 质询制）与审批轮询。
// public 模式申请携带一次性 nonce（中继站据此回调本站探测端点验证域名控制权，协议 §4.12）。
package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// ApplyForJoinResult 申请结果（自动通过时 key 由后端隐藏保存；待审核时返回审核中状态）。
type ApplyForJoinResult struct {
	Status     string   `json:"status"`      // approved（已获许可）/ pending（审核中，v1.4）
	RelayName  string   `json:"relay_name"`  // 中继站名称（仪式与状态卡展示）
	Categories []string `json:"categories"`  // 中继站分类（默认分类预选第一个）
}

// ApplyForJoin 自助申请（协议 v1.5 质询制）：调中继站 POST /api/v1/apply。
// public 模式生成一次性 nonce 随请求发出——中继站将回调本站 /api/v1/relay/probe 验证；
// bridged 模式中继站不可达、不质询，一律人工审核。
// 自动通过：key 直接落库隐藏保管；手动审核：保存申请凭据，等待运营方通过后领取。
func (s *RelayService) ApplyForJoin(ctx context.Context, url string, mode string) (ApplyForJoinResult, error) {
	if !strings.HasPrefix(url, "http") || (mode != "public" && mode != "bridged") {
		return ApplyForJoinResult{}, errs.New(errs.CodeValidation, "请填写中继站地址并选择站点模式")
	}
	name, avatar := s.siteBrief()
	reqBody := map[string]any{
		"proto_ver": 1, "mode": mode, "base_url": s.baseURL(),
		"site_name": name, "avatar": avatar,
	}
	if mode == "public" {
		// 质询随机数：中继站回调探测端点核对（5 分钟内有效；站长全程零感知）
		reqBody["nonce"] = s.nonces.Issue()
	}
	var resp model.RelayApplyResp
	if err := s.postJSON(ctx, strings.TrimRight(url, "/")+"/api/v1/apply", "", reqBody, &resp); err != nil {
		return ApplyForJoinResult{}, err
	}
	old, err := s.relay.Config(ctx)
	if err != nil {
		return ApplyForJoinResult{}, err
	}
	retention := old.LocalRetentionDays
	if retention < 1 || retention > 30 {
		retention = 7
	}
	category := old.DefaultCategory
	if category == "" && len(resp.Categories) > 0 {
		category = resp.Categories[0]
	}
	if resp.Status == "pending" {
		// 手动审核：仅保存申请凭据，等待中继站通过（前端轮询 PollClaim）
		if err := s.relay.SaveClaim(ctx, strings.TrimRight(url, "/"), mode, resp.ClaimToken, category); err != nil {
			return ApplyForJoinResult{}, err
		}
		s.log.Info("申请已提交，等待中继站审核")
		return ApplyForJoinResult{Status: "pending", RelayName: resp.RelayName, Categories: resp.Categories}, nil
	}
	// 自动通过（或已通过站点找回）：key 直接隐藏落库
	if err := s.relay.SaveConfig(ctx, repository.SaveConfigParams{
		Enabled: false, URL: strings.TrimRight(url, "/"), SiteKey: resp.SiteKey,
		Mode: mode, DefaultCategory: category, LocalRetentionDays: retention,
	}); err != nil {
		return ApplyForJoinResult{}, err
	}
	return ApplyForJoinResult{Status: "approved", RelayName: resp.RelayName, Categories: resp.Categories}, nil
}

// PollClaim 轮询申请审批结果（前端每 5 秒调用）：通过则领取 key 隐藏保存并清空凭据。
func (s *RelayService) PollClaim(ctx context.Context) (ApplyForJoinResult, error) {
	rc, err := s.relay.Config(ctx)
	if err != nil {
		return ApplyForJoinResult{}, err
	}
	if rc.ClaimToken == "" {
		// 无待审凭据：按是否已持有 key 返回当前状态
		if rc.SiteKey != "" {
			return ApplyForJoinResult{Status: "approved", RelayName: metaName(rc.RelayMetaJSON)}, nil
		}
		return ApplyForJoinResult{Status: "idle"}, nil
	}
	var resp model.RelayClaimResp
	if err := s.postJSON(ctx, strings.TrimRight(rc.URL, "/")+"/api/v1/apply/claim", "",
		map[string]any{"claim_token": rc.ClaimToken}, &resp); err != nil {
		return ApplyForJoinResult{Status: "pending"}, nil // 网络抖动视为仍在审核
	}
	switch resp.Status {
	case "approved":
		if err := s.relay.SaveClaimedKey(ctx, resp.SiteKey); err != nil {
			return ApplyForJoinResult{}, err
		}
		s.log.Info("申请已通过，许可已隐藏保管")
		return ApplyForJoinResult{Status: "approved", RelayName: metaName(rc.RelayMetaJSON)}, nil
	case "rejected":
		_ = s.relay.SaveClaim(ctx, rc.URL, rc.Mode, "", rc.DefaultCategory) // 清空凭据
		return ApplyForJoinResult{Status: "rejected"}, nil
	default:
		return ApplyForJoinResult{Status: "pending"}, nil
	}
}

// metaName 从元信息 JSON 提取中继站名（容错）。
func metaName(metaJSON *string) string {
	if metaJSON == nil || *metaJSON == "" {
		return ""
	}
	var meta struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(*metaJSON), &meta); err != nil {
		return ""
	}
	return meta.Name
}
