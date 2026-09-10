// 中继站对接服务：配置管理、连接测试（handshake）、申请流、发布出口。
// 拆分件：传输层 relaytransport.go / 纯文本化 relayplain.go / 删帖联动 relaydelete.go。
// 出站方向是唯一方向（ADR-1）：本服务只向中继站发起 HTTP 请求。
package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/config"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/repository"
)

// relayHTTPTimeout 出站请求超时（握手/发布；轮询在 relayclient 内独立设置）。
const relayHTTPTimeout = 15 * time.Second

// RelayService 中继站配置与发布出口。
type RelayService struct {
	relay    *repository.RelayRepo
	posts    *repository.PostRepo
	tags     *repository.TagRepo
	media    *repository.MediaRepo
	settings *repository.SettingRepo // 站点设置（握手上报站名取 site_name）
	cfg      config.Config
	log      *zap.Logger
	client   *http.Client
	nonces   *ProbeNonceStore // 申请质询随机数（v1.5，探测应答用）
	version  string           // 本站部署版本（探测应答回显；update.CurrentVersion）
}

// NewRelayService 构造中继站服务（version 为部署版本，探测质询应答回显用）。
func NewRelayService(relay *repository.RelayRepo, posts *repository.PostRepo, tags *repository.TagRepo,
	media *repository.MediaRepo, settings *repository.SettingRepo, cfg config.Config,
	log *zap.Logger, version string) *RelayService {
	return &RelayService{
		relay: relay, posts: posts, tags: tags, media: media, settings: settings, cfg: cfg, log: log,
		client: &http.Client{Timeout: relayHTTPTimeout},
		nonces: NewProbeNonceStore(), version: version,
	}
}

// GetConfig 读取对接配置（后台展示；key 明文仅后台管理员可见）。
func (s *RelayService) GetConfig(ctx context.Context) (model.RelayConfig, error) {
	return s.relay.Config(ctx)
}

// SaveConfigParams 配置保存参数。
type SaveConfigParams struct {
	Enabled            bool
	URL                string
	SiteKey            string
	Mode               string
	DefaultCategory    string
	LocalRetentionDays int
}

// SaveConfig 保存配置并重启订阅任务；从关闭切到开启时游标重置（首屏重新回填）。
// key 隐藏保管：SiteKey 传空表示沿用已保存的 key（申请制下发后前端不再持有明文）。
func (s *RelayService) SaveConfig(ctx context.Context, p SaveConfigParams) error {
	if p.Enabled {
		if !strings.HasPrefix(p.URL, "http") {
			return errs.New(errs.CodeValidation, "中继站 URL 不能为空")
		}
		if p.Mode != "public" && p.Mode != "bridged" {
			return errs.New(errs.CodeValidation, "站点模式必须为 public 或 bridged")
		}
		if p.LocalRetentionDays < 1 || p.LocalRetentionDays > 30 {
			return errs.New(errs.CodeValidation, "本地保存天数须在 1~30 之间")
		}
	}
	old, err := s.relay.Config(ctx)
	if err != nil {
		return err
	}
	if p.SiteKey == "" {
		p.SiteKey = old.SiteKey // 未显式提供时保留已隐藏保管的 key
	}
	if p.Enabled && p.SiteKey == "" {
		return errs.New(errs.CodeValidation, "尚未获得对接许可，请先申请")
	}
	if p.URL == "" {
		p.URL = old.URL
	}
	if p.DefaultCategory == "" {
		p.DefaultCategory = old.DefaultCategory
	}
	if p.LocalRetentionDays == 0 {
		p.LocalRetentionDays = old.LocalRetentionDays
		if p.LocalRetentionDays == 0 {
			p.LocalRetentionDays = 7
		}
	}
	if err := s.relay.SaveConfig(ctx, repository.SaveConfigParams{
		Enabled: p.Enabled, URL: strings.TrimRight(p.URL, "/"), SiteKey: p.SiteKey,
		Mode: p.Mode, DefaultCategory: p.DefaultCategory, LocalRetentionDays: p.LocalRetentionDays,
	}); err != nil {
		return err
	}
	// 关 → 开：游标清零触发首屏回填（协议 §4.6 backfill_seq）
	if p.Enabled && !old.Enabled {
		if err := s.relay.ResetCursor(ctx); err != nil {
			return err
		}
	}
	// 订阅管理器监视 updated_at，保存后 ≤5s 自动生效
	return nil
}


// TestConnection 连接测试：实时调中继站 handshake，返回元信息与配额回显（不落库）。
func (s *RelayService) TestConnection(ctx context.Context, url string, siteKey string, mode string) (model.RelayHandshakeResp, error) {
	if !strings.HasPrefix(url, "http") || siteKey == "" || (mode != "public" && mode != "bridged") {
		return model.RelayHandshakeResp{}, errs.New(errs.CodeValidation, "URL / key / 模式不完整")
	}
	name, avatar := s.siteBrief()
	reqBody := map[string]any{
		"proto_ver": 1, "mode": mode, "base_url": s.cfg.SiteBaseURL,
		"site_name": name, "avatar": avatar,
	}
	var resp model.RelayHandshakeResp
	if err := s.postJSON(ctx, strings.TrimRight(url, "/")+"/api/v1/handshake", siteKey, reqBody, &resp); err != nil {
		return model.RelayHandshakeResp{}, err
	}
	return resp, nil
}

// siteBrief 本站概要（握手上报用）：站名取站点设置的 site_name（缺省回退 host），头像留空。
func (s *RelayService) siteBrief() (string, string) {
	host := s.cfg.SiteBaseURL
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	name := host
	if s.settings != nil {
		if v, found, err := s.settings.Get(context.Background(), "site_name"); err == nil && found && v != "" {
			name = v
		}
	}
	return name, ""
}

// PublishPostAsync 发布出口：帖子发布成功后异步推送中继站（失败仅日志，不打断发帖；
// 幂等键 origin_id=帖子 ID，重试安全）。
func (s *RelayService) PublishPostAsync(postID int64) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := s.publishPost(ctx, postID); err != nil {
			s.log.Warn("推送中继站失败", zap.Int64("post_id", postID), zap.Error(err))
		}
	}()
}

// publishPost 打包并推送单条帖子（说说 / 文章）。
func (s *RelayService) publishPost(ctx context.Context, postID int64) error {
	rc, err := s.relay.Config(ctx)
	if err != nil {
		return err
	}
	if !rc.Enabled || rc.URL == "" || rc.SiteKey == "" || rc.DefaultCategory == "" {
		return nil // 未启用或未选分类：静默跳过
	}
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return err
	}
	if post.Status != "published" || post.Visibility != "public" {
		return nil // 仅公开已发布内容进大世界
	}
	body, err := s.buildPublishBody(ctx, rc, post)
	if err != nil {
		return err
	}
	var resp struct {
		ContentID string `json:"content_id"`
		Seq       int64  `json:"seq"`
	}
	if err := s.postJSON(ctx, rc.URL+"/api/v1/contents", rc.SiteKey, body, &resp); err != nil {
		return err
	}
	s.log.Info("已推送中继站", zap.Int64("post_id", postID),
		zap.String("content_id", resp.ContentID), zap.Int64("seq", resp.Seq))
	return nil
}

// buildPublishBody 组装协议 §4.2 请求体（按站点模式处理图片与全文）。
func (s *RelayService) buildPublishBody(ctx context.Context, rc model.RelayConfig, post model.Post) (map[string]any, error) {
	tags := s.postTags(ctx, post.ID)
	kind := post.PostKind
	if kind == "" {
		kind = "moment"
	}
	body := map[string]any{
		"kind": kind, "category": rc.DefaultCategory, "tags": tags,
		"origin_id":    fmt.Sprintf("%d", post.ID),
		"published_at": post.CreatedAt.Unix(),
	}
	images, err := s.resolveImages(ctx, rc, post)
	if err != nil {
		return nil, err
	}
	if kind == "article" {
		article := map[string]any{
			"title": post.Title, "summary": plainForWorld(post.Summary),
			// 前端文章详情路由为 /posts/{id}（复数，协议 §4.2 示例同形），单数拼法曾致 404
			"origin_url": fmt.Sprintf("%s/posts/%d", s.cfg.SiteBaseURL, post.ID),
		}
		if len(images) > 0 {
			article["cover"] = images[0]
		}
		body["article"] = article
		if rc.Mode == "bridged" {
			full := plainForWorld(post.Content)
			if len([]rune(full)) > 8000 {
				full = string([]rune(full)[:8000])
			}
			body["article_full"] = map[string]any{"format": "markdown", "body": full}
		}
		return body, nil
	}
	moment := map[string]any{"text": plainForWorld(post.Content), "images": images}
	if post.ContentType == "audio" || post.ContentType == "video" {
		// 音视频不托管：以本站可播 URL 直传信封（协议 §3.2；bridged 站外站不可达为已知边界）
		mediaList, _ := s.media.FindByIDs(ctx, post.MediaIDs)
		urls := make([]string, 0, len(mediaList))
		for _, m := range mediaList {
			if m.Type == "video" || m.Type == "audio" {
				urls = append(urls, s.cfg.SiteBaseURL+m.URL)
			}
		}
		if post.ContentType == "video" {
			moment["videos"] = urls
		} else {
			moment["audios"] = urls
		}
	}
	body["moment"] = moment
	return body, nil
}

// resolveImages 图片 URL 解析：public 直接外链本站；bridged 上传中继站托管（协议红线：防裂图）。
func (s *RelayService) resolveImages(ctx context.Context, rc model.RelayConfig, post model.Post) ([]string, error) {
	if len(post.MediaIDs) == 0 {
		return []string{}, nil
	}
	mediaList, err := s.media.FindByIDs(ctx, post.MediaIDs)
	if err != nil {
		return nil, err
	}
	images := make([]string, 0, len(mediaList))
	for _, m := range mediaList {
		if m.Type != "image" {
			continue
		}
		if rc.Mode == "public" {
			images = append(images, s.cfg.SiteBaseURL+m.URL)
			continue
		}
		uploaded, err := s.uploadMediaToRelay(ctx, rc, m)
		if err != nil {
			s.log.Warn("bridged 图片上传中继站失败，跳过该图", zap.Int64("media_id", m.ID), zap.Error(err))
			continue
		}
		images = append(images, uploaded)
	}
	return images, nil
}

// postTags 读取帖子标签名（≤5）。
func (s *RelayService) postTags(ctx context.Context, postID int64) []string {
	rows, err := s.tags.ListByPost(ctx, postID)
	if err != nil {
		return []string{}
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		if len(names) >= 5 {
			break
		}
		names = append(names, row.Name)
	}
	return names
}

// baseURL 本站对外基础 URL（握手上报 base_url 用）。
func (s *RelayService) baseURL() string { return s.cfg.SiteBaseURL }

// getJSON 统一出站 GET：Bearer key 认证、JSON 解码、协议错误码透传（轮询用，限读 8MB）。
func (s *RelayService) ListWorld(ctx context.Context, category string, before time.Time, limit int) ([]model.RelayCacheItem, error) {
	return s.relay.ListCache(ctx, category, before, limit)
}

// CacheCount 本地缓存条数（后台展示）。
func (s *RelayService) CacheCount(ctx context.Context) (int, error) {
	return s.relay.CacheCount(ctx)
}

