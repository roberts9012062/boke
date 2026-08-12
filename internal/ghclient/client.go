// internal/ghclient/client.go
// GitHub 客户端（连接器类，外部系统接口）：拉取插件商城清单（plugins.json）+ Release 资产下载（M3.4）。
// 说明（M3.1）：插件商城以 GitHub 仓库为内容载体——用户可自定义仓库地址（settings.plugin_source），
//   清单文件约定为仓库根目录 plugins.json（GitHub Contents API + raw 格式返回）。
// 说明（M3.4）：安装链路按清单 assets.pattern 匹配 Release 资产下载 .bpk——元数据走 API（8s），
//   资产走 CDN 直链（长超时流式，公开仓库无需 token）。
package ghclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// 请求超时（GitHub API 慢时快速失败）。
const requestTimeout = 8 * time.Second

// 资产下载超时（.bpk 包可达数十 MB，流式下载需更长时限）。
const downloadTimeout = 120 * time.Second

// Client GitHub 客户端（连接器类）。
type Client struct {
	token  string // GitHub Token（.env GITHUB_TOKEN）
	client *http.Client
}

// NewClient 创建 GitHub 客户端。
// 参数：token GitHub Token（可为空，仅公开仓库可用）。
func NewClient(token string) *Client {
	return &Client{
		token: token,
		client: &http.Client{Timeout: requestTimeout},
	}
}

// validateRepo 仓库名合法性校验（防路径注入；纯函数）。
func validateRepo(owner string, repo string) error {
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(repo) == "" {
		return errors.New("插件源仓库格式不正确（应为 owner/repo）")
	}
	for _, ch := range owner + repo {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' || ch == '.') {
			return errors.New("插件源仓库格式不正确")
		}
	}
	return nil
}

// FetchManifest 拉取仓库根目录的 plugins.json 清单。
// 参数：ctx 上下文；owner 仓库属主；repo 仓库名。
// 返回：清单文件原始内容（UTF-8）；仓库或文件不存在返回错误。
// 说明：优先 Contents API（带 token 可访问私有仓库），响应为 base64 JSON 或 raw 文本（Accept 头指定）。
func (c *Client) FetchManifest(ctx context.Context, owner string, repo string) ([]byte, error) {
	if err := validateRepo(owner, repo); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/plugins.json", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.raw+json") // 直接返回文件内容（非 base64）
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("拉取插件清单失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("拉取插件清单失败（HTTP %d）：%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// 兜底：部分响应为 base64 JSON（Accept 未生效时）
	if content := decodeBase64IfJSON(raw); content != nil {
		return content, nil
	}
	return raw, nil
}

// decodeBase64IfJSON 若响应为 base64 包裹的 JSON（Contents API 默认格式）则解码。
func decodeBase64IfJSON(raw []byte) []byte {
	trimmed := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(trimmed, "{") {
		return nil
	}
	var wrapper struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(trimmed), &wrapper); err != nil || wrapper.Content == "" {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(wrapper.Content)
	if err != nil {
		return nil
	}
	return decoded
}

// ---------- Release 资产（M3.4：.bpk 下载安装） ----------

// ReleaseAsset Release 资产信息。
type ReleaseAsset struct {
	Name string `json:"name"` // 资产文件名（如 demo-plugin-0.1.0-windows-amd64.bpk）
	URL  string `json:"browser_download_url"` // 下载直链（CDN，公开仓库无需 token）
	Size int64  `json:"size"` // 资产大小（字节）
}

// LatestRelease 最新 Release 信息。
type LatestRelease struct {
	TagName string         `json:"tag_name"` // 版本 tag（如 v0.1.0）
	Assets  []ReleaseAsset `json:"assets"`   // 资产列表
}

// FetchLatestRelease 拉取仓库最新 Release（插件更新对比/资产匹配用）。
// 返回：tag 与资产列表；无 Release 或仓库不存在返回错误。
func (c *Client) FetchLatestRelease(ctx context.Context, owner string, repo string) (*LatestRelease, error) {
	if err := validateRepo(owner, repo); err != nil {
		return nil, err
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("拉取 Release 信息失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("拉取 Release 信息失败（HTTP %d）：%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var release LatestRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析 Release 信息失败：%w", err)
	}
	return &release, nil
}

// DownloadAsset 流式下载资产到 destPath（.bpk 安装包）。
// 参数：url 下载直链；destPath 落盘路径；sizeLimit 大小上限（0=不限制）。
// 说明：资产走 CDN（objects.githubusercontent.com），公开仓库无需 token；长超时独立客户端。
func (c *Client) DownloadAsset(ctx context.Context, url string, destPath string, sizeLimit int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	// 私有仓库资产下载需带 token（公开直链可不带）
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	dlClient := &http.Client{Timeout: downloadTimeout}
	resp, err := dlClient.Do(req)
	if err != nil {
		return fmt.Errorf("下载插件包失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("下载插件包失败（HTTP %d）：%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	// 大小上限校验（Content-Length 声明 + 流式读取双保险）
	if sizeLimit > 0 && resp.ContentLength > sizeLimit {
		return fmt.Errorf("插件包超过大小上限（%dMB）", sizeLimit>>20)
	}
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("创建下载文件失败：%w", err)
	}
	defer out.Close()
	n, err := io.Copy(out, io.LimitReader(resp.Body, sizeLimit+1))
	if err != nil {
		return fmt.Errorf("下载插件包失败：%w", err)
	}
	if sizeLimit > 0 && n > sizeLimit {
		return fmt.Errorf("插件包超过大小上限（%dMB）", sizeLimit>>20)
	}
	return nil
}
