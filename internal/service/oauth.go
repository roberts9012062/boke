// internal/service/oauth.go
// GitHub OAuth 连接（M3.5）：授权跳转 / 回调换 token / 状态查询 / 断开。
// 对齐 docs/architecture.md 6.5.1：access_token AES 加密存 settings（gh_oauth_token），
//   匿名模式保留（未连接时清单/资产走公开 raw/CDN，不受影响）。
// 说明：OAuth App 凭证（client_id/secret）未配置时入口隐藏，配置后即用。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/roberts9012062/boke/internal/ai"
	"github.com/roberts9012062/boke/internal/ghclient"
	"github.com/roberts9012062/boke/internal/repository"
)

// OAuth 相关 settings 键（token 加密存储）。
const (
	settingGHToken = "gh_oauth_token" // GitHub access_token（AES 加密）
	settingGHUser  = "gh_oauth_user"  // 已连接 GitHub 用户名
)

// OAuthStatusDTO OAuth 连接状态。
type OAuthStatusDTO struct {
	Connected bool   `json:"connected"`            // 是否已连接
	Username  string `json:"username,omitempty"`   // 已连接用户名
	Enabled   bool   `json:"enabled"`              // OAuth 是否启用（凭证已配置）
}

// OAuthService GitHub OAuth 连接服务（连接器类）。
type OAuthService struct {
	clientID      string                 // OAuth App Client ID（空=未启用）
	clientSecret  string                 // OAuth App Client Secret
	keySecret     string                 // AES 加密密钥种子（token 加密存储）
	fallbackToken string                 // .env 静态 token（断开后回退）
	settings      *repository.SettingRepo // settings 读写
	gh            *ghclient.Client       // GitHub 客户端（回调后更新 token）
}

// NewOAuthService 创建 OAuth 服务。
// 参数：clientID/secret OAuth App 凭证（空=未启用）；keySecret 加密种子；
//      fallbackToken .env 静态 token（断开后回退，可空）；gh 客户端（SetToken 更新）。
func NewOAuthService(clientID string, clientSecret string, keySecret string, fallbackToken string, settings *repository.SettingRepo, gh *ghclient.Client) *OAuthService {
	return &OAuthService{clientID: clientID, clientSecret: clientSecret, keySecret: keySecret, fallbackToken: fallbackToken, settings: settings, gh: gh}
}

// FallbackToken .env 静态 token（断开 OAuth 后回退）。
func (s *OAuthService) FallbackToken() string {
	return s.fallbackToken
}

// Enabled OAuth 是否启用（凭证已配置）。
func (s *OAuthService) Enabled() bool {
	return s.clientID != "" && s.clientSecret != ""
}

// AuthorizeURL 生成 GitHub 授权跳转 URL（scope：read:user + 公开仓库读取）。
// 说明：公开仓库拉取/下载无需额外 scope；仅需用户身份（连接状态展示）。
func (s *OAuthService) AuthorizeURL() (string, error) {
	if !s.Enabled() {
		return "", fmt.Errorf("GitHub OAuth 未配置（缺少 GITHUB_OAUTH_CLIENT_ID/SECRET）")
	}
	params := url.Values{
		"client_id":    {s.clientID},
		"scope":        {"read:user"},
		"response_type": {"code"},
	}
	return "https://github.com/login/oauth/authorize?" + params.Encode(), nil
}

// Callback OAuth 回调：code 换 access_token → 查询用户名 → 加密存 settings + 更新 ghclient。
// 返回：连接状态（含用户名）。
func (s *OAuthService) Callback(ctx context.Context, code string) (*OAuthStatusDTO, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("GitHub OAuth 未配置")
	}
	if code == "" {
		return nil, fmt.Errorf("缺少授权码")
	}
	// 1. 换 token（GitHub OAuth access_token 端点）
	form := url.Values{
		"client_id":     {s.clientID},
		"client_secret": {s.clientSecret},
		"code":          {code},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("换取 access_token 失败：%w", err)
	}
	defer resp.Body.Close()
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析 access_token 响应失败")
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("授权失败（%s）", tokenResp.Error)
	}

	// 2. 查询用户名（GET /user）
	username, err := s.fetchUsername(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, err
	}

	// 3. 加密存储 + 更新 ghclient（后续清单/资产请求走 OAuth token）
	encrypted, err := ai.EncryptSecret(tokenResp.AccessToken, s.keySecret)
	if err != nil {
		return nil, fmt.Errorf("token 加密失败：%w", err)
	}
	if err := s.settings.SetMany(ctx, map[string]string{
		settingGHToken: encrypted,
		settingGHUser:  username,
	}); err != nil {
		return nil, fmt.Errorf("连接状态保存失败：%w", err)
	}
	s.gh.SetToken(tokenResp.AccessToken)

	return &OAuthStatusDTO{Connected: true, Username: username, Enabled: true}, nil
}

// Status 查询连接状态（settings 有加密 token 且可解密即视为已连接）。
func (s *OAuthService) Status(ctx context.Context) (*OAuthStatusDTO, error) {
	if !s.Enabled() {
		return &OAuthStatusDTO{Enabled: false}, nil
	}
	encrypted, ok, err := s.settings.Get(ctx, settingGHToken)
	if err != nil || !ok || encrypted == "" {
		return &OAuthStatusDTO{Enabled: true}, nil
	}
	username, _, _ := s.settings.Get(ctx, settingGHUser)
	return &OAuthStatusDTO{Connected: true, Username: username, Enabled: true}, nil
}

// Disconnect 断开连接（清除 settings + ghclient 重置为 .env 静态 token）。
// 参数：fallbackToken .env 静态 token（清除 OAuth token 后回退，可空）。
func (s *OAuthService) Disconnect(ctx context.Context, fallbackToken string) error {
	if err := s.settings.SetMany(ctx, map[string]string{
		settingGHToken: "",
		settingGHUser:  "",
	}); err != nil {
		return fmt.Errorf("断开连接失败：%w", err)
	}
	s.gh.SetToken(fallbackToken)
	return nil
}

// fetchUsername 查询当前 token 对应用户名（GET /user）。
func (s *OAuthService) fetchUsername(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("查询 GitHub 用户失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("查询 GitHub 用户失败（HTTP %d）", resp.StatusCode)
	}
	var user struct {
		Login string `json:"login"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err := json.Unmarshal(body, &user); err != nil || user.Login == "" {
		return "", fmt.Errorf("解析 GitHub 用户失败")
	}
	return user.Login, nil
}

// RestoreToken 启动时恢复 OAuth token（server 装配调用：解密 → ghclient.SetToken）。
func (s *OAuthService) RestoreToken(ctx context.Context) {
	encrypted, ok, err := s.settings.Get(ctx, settingGHToken)
	if err != nil || !ok || encrypted == "" {
		return
	}
	if token, err := ai.DecryptSecret(encrypted, s.keySecret); err == nil && token != "" {
		s.gh.SetToken(token)
	}
}
