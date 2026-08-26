// internal/ghclient/client.go
// GitHub 客户端（连接器类，外部系统接口）：拉取插件商城内容 + Release 资产下载（M3.4）。
// 说明（M3.1）：插件商城以 GitHub 仓库为内容载体——用户可自定义仓库地址（settings.plugin_source）。
// 说明（M5 文件夹结构）：商城改为文件夹结构——每个插件一个文件夹，内含 plugin.json（元数据）
//
//	与 README.md（介绍）；文件统一经 FetchFile 拉取（Contents API + raw 格式），
//	文件夹枚举经 tree.go FetchTree（git trees API）。
//
// 说明（M3.4）：安装链路按清单 assets.pattern 匹配 Release 资产下载 .bpk——元数据走 API（8s），
//
//	资产走 CDN 直链（长超时流式，公开仓库无需 token）。
package ghclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"strconv"
	"time"
)

// 请求超时（GitHub API 慢时快速失败）。
const requestTimeout = 8 * time.Second

// 资产下载超时（.bpk 包可达数十 MB，流式下载需更长时限）。
const downloadTimeout = 300 * time.Second // P1：慢网络（GitHub CDN）宽容超时；10MB 包在受限链路需 2 分钟以上

// Client GitHub 客户端（连接器类）。
type Client struct {
	mu      sync.RWMutex // 保护 token 与 proxy（运行期动态更新，并发安全）
	token   string       // GitHub Token（.env GITHUB_TOKEN 或 OAuth 连接）
	proxy   string       // GitHub 加速代理前缀（空 = 直连；国内网络拉取失败时经设置页配置）
	client  *http.Client
	baseURL string // API 基址（默认 https://api.github.com；测试注入 httptest）
}

// apiBaseURL GitHub API 默认基址。
const apiBaseURL = "https://api.github.com"

// NewClient 创建 GitHub 客户端。
// 参数：token GitHub Token（可为空，仅公开仓库可用）。
func NewClient(token string) *Client {
	return &Client{
		token:   token,
		client:  &http.Client{Timeout: requestTimeout},
		baseURL: apiBaseURL,
	}
}

// base 返回 API 基址（未注入时用默认值）。
func (c *Client) base() string {
	if c.baseURL != "" {
		return c.baseURL
	}
	return apiBaseURL
}

// SetToken 动态更新 GitHub Token（OAuth 连接/断开时调用，并发安全）。
func (c *Client) SetToken(token string) {
	c.mu.Lock()
	c.token = token
	c.mu.Unlock()
}

// getToken 读取当前 Token（并发安全）。
func (c *Client) getToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

// SetProxy 动态更新 GitHub 加速代理（并发安全；空串恢复直连）。
// 代理为**前缀拼接**模式：实际请求 URL = proxy + "/" + 原完整 URL
// （如 https://gh-proxy.com + https://api.github.com/repos/...，与公共 gh-proxy 服务约定一致）。
// 仅接受 https:// 前缀地址；非法输入按直连处理（不中断业务）。
// 安全：代理模式下不发送 Authorization 头（token 不应发给第三方代理服务器，仅支持公开仓库）。
func (c *Client) SetProxy(proxy string) {
	proxy = strings.TrimSpace(proxy)
	proxy = strings.TrimSuffix(proxy, "/")
	if proxy != "" && !strings.HasPrefix(proxy, "https://") {
		proxy = "" // 非 https 前缀视为无效：恢复直连（防明文代理与协议混用）
	}
	c.mu.Lock()
	c.proxy = proxy
	c.mu.Unlock()
}

// proxied 当前是否处于代理模式（并发安全）。
func (c *Client) proxied() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.proxy != ""
}

// wrapURL API 请求包装：恒直连（公共代理会剥离 Authorization 头，走代理的 API
// 请求按匿名 IP 限流——2026-08 实测 403；API 小请求直连本身稳定）。
func (c *Client) wrapURL(target string) string {
	return target
}

// wrapAssetURL 资产下载包装：代理模式把完整目标 URL 拼接在代理前缀之后
// （gh-proxy 类公共加速对大文件 CDN 下载有效且无需鉴权），直连模式原样返回。
func (c *Client) wrapAssetURL(target string) string {
	if !c.proxied() {
		return target
	}
	c.mu.RLock()
	proxy := c.proxy
	c.mu.RUnlock()
	return proxy + "/" + target
}

// setAuth 附加 GitHub Token（代理模式跳过——token 不发给第三方代理，公开仓库无需凭证）。
func (c *Client) setAuth(req *http.Request) {
	if c.proxied() {
		return
	}
	if token := c.getToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// validateRepo 仓库名合法性校验（防路径注入；纯函数）。
func validateRepo(owner string, repo string) error {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return errors.New("插件源仓库格式不正确（应为 owner/repo）")
	}
	// 拒绝 . / .. 完整名（点字符本身合法，但单独成段可做路径穿越）
	if owner == "." || owner == ".." || repo == "." || repo == ".." {
		return errors.New("插件源仓库格式不正确")
	}
	for _, ch := range owner + repo {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' || ch == '.') {
			return errors.New("插件源仓库格式不正确")
		}
	}
	return nil
}

// ErrFileNotFound 仓库文件不存在（HTTP 404，供上层区分"插件未提供 README"）。
var ErrFileNotFound = errors.New("仓库文件不存在")

// validateFilePath 校验仓库内文件路径合法性（防路径穿越；纯函数）。
// 允许：非空、相对路径（无前导 /）、无反斜杠、无空段、无 . / .. 段。
func validateFilePath(filePath string) error {
	if filePath == "" {
		return errors.New("文件路径不能为空")
	}
	if strings.HasPrefix(filePath, "/") || strings.Contains(filePath, "\\") {
		return errors.New("文件路径格式不正确")
	}
	for _, part := range strings.Split(filePath, "/") {
		if part == "" || part == "." || part == ".." {
			return errors.New("文件路径格式不正确")
		}
	}
	return nil
}

// encodeFilePath 逐段 URL 转义文件路径（保留 / 分隔；纯函数）。
func encodeFilePath(filePath string) string {
	parts := strings.Split(filePath, "/")
	encoded := make([]string, 0, len(parts))
	for _, part := range parts {
		encoded = append(encoded, url.PathEscape(part))
	}
	return strings.Join(encoded, "/")
}

// FetchFile 拉取仓库内指定路径的文件原始内容（GitHub Contents API + raw 格式）。
// 参数：filePath 相对仓库根的文件路径（如 seo-optimizer/README.md）；sizeLimit 大小上限字节（0=不限制）。
// 返回：文件内容；文件不存在返回 ErrFileNotFound。
// 说明：优先 Contents API（带 token 可访问私有仓库），响应为 base64 JSON 或 raw 文本（Accept 头指定）。
func (c *Client) FetchFile(ctx context.Context, owner string, repo string, filePath string, sizeLimit int64) ([]byte, error) {
	if err := validateRepo(owner, repo); err != nil {
		return nil, err
	}
	if err := validateFilePath(filePath); err != nil {
		return nil, err
	}

	reqURL := c.wrapURL(fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.base(), owner, repo, encodeFilePath(filePath)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.raw+json") // 直接返回文件内容（非 base64）
	c.setAuth(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("拉取仓库文件失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrFileNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("拉取仓库文件失败（HTTP %d）：%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	// 大小上限（Content-Length 声明 + 流式读取双保险）
	if sizeLimit > 0 && resp.ContentLength > sizeLimit {
		return nil, fmt.Errorf("仓库文件超过大小上限（%dKB）", sizeLimit>>10)
	}
	reader := io.Reader(resp.Body)
	if sizeLimit > 0 {
		reader = io.LimitReader(resp.Body, sizeLimit+1)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if sizeLimit > 0 && int64(len(raw)) > sizeLimit {
		return nil, fmt.Errorf("仓库文件超过大小上限（%dKB）", sizeLimit>>10)
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
	Name string `json:"name"`                 // 资产文件名（如 demo-plugin-0.1.0-windows-amd64.bpk）
	URL  string `json:"browser_download_url"` // 下载直链（CDN，公开仓库无需 token）
	Size int64  `json:"size"`                 // 资产大小（字节）
}

// LatestRelease 最新 Release 信息。
type LatestRelease struct {
	TagName string         `json:"tag_name"` // 版本 tag（如 v0.1.0）
	Body    string         `json:"body"`     // Release 说明（更新日志，站点更新弹窗展示用）
	Assets  []ReleaseAsset `json:"assets"`   // 资产列表
}

// FetchLatestRelease 拉取仓库最新 Release（插件更新对比/资产匹配用）。
// 返回：tag 与资产列表；无 Release 或仓库不存在返回错误。
func (c *Client) FetchLatestRelease(ctx context.Context, owner string, repo string) (*LatestRelease, error) {
	if err := validateRepo(owner, repo); err != nil {
		return nil, err
	}
	return c.fetchRelease(ctx, fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.base(), owner, repo))
}

// FetchReleaseByTag 按版本 tag 拉取指定 Release（P1 升级版本钉扎：装清单声明版本对应的包，
// 而非永远追 latest——防清单过期时 latest 被替换绕过审核）。
func (c *Client) FetchReleaseByTag(ctx context.Context, owner string, repo string, tag string) (*LatestRelease, error) {
	if err := validateRepo(owner, repo); err != nil {
		return nil, err
	}
	if strings.TrimSpace(tag) == "" {
		return nil, fmt.Errorf("版本 tag 不能为空")
	}
	return c.fetchRelease(ctx, fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", c.base(), owner, repo, url.PathEscape(tag)))
}

// fetchRelease Release 信息拉取与解析（latest 与 by-tag 共用实现）。
func (c *Client) fetchRelease(ctx context.Context, reqURL string) (*LatestRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.wrapURL(reqURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	c.setAuth(req)
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
// 说明：资产走 CDN（objects.githubusercontent.com），公开仓库无需 token；长超时独立客户端；
// 代理模式下直链同样经前缀拼接加速（gh-proxy 类服务会改写重定向保持代理链路）。
func (c *Client) DownloadAsset(ctx context.Context, url string, destPath string, sizeLimit int64) error {
	// 断点续传重试：GitHub CDN 直连链路偶发中断（实测 10MB 包 3 次约 1 次断流）——
	// 每轮从已落盘偏移继续（Range），断点累积直至完整；服务器不支持续传时整轮重下
	const maxRounds = 6
	dlClient := &http.Client{Timeout: downloadTimeout}
	var offset int64
	var lastErr error
	for round := 0; round < maxRounds; round++ {
		if round > 0 {
			time.Sleep(time.Duration(round) * 500 * time.Millisecond)
		}
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, c.wrapAssetURL(url), nil)
		if reqErr != nil {
			return reqErr
		}
		c.setAuth(req)
		if offset > 0 {
			req.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
		}
		resp, doErr := dlClient.Do(req)
		if doErr != nil {
			lastErr = fmt.Errorf("下载插件包失败：%w", doErr)
			continue
		}
		// Range 生效=206 追加；服务器忽略 Range 返回 200=从头重下（重置偏移）
		appendMode := resp.StatusCode == http.StatusPartialContent
		if resp.StatusCode != http.StatusOK && !appendMode {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
			resp.Body.Close()
			lastErr = fmt.Errorf("下载插件包失败（HTTP %d）：%s", resp.StatusCode, strings.TrimSpace(string(body)))
			continue
		}
		if !appendMode {
			offset = 0
		}
		// 大小上限校验（当前偏移 + 本轮 Content-Length 双保险）
		if sizeLimit > 0 && resp.ContentLength > 0 && offset+resp.ContentLength > sizeLimit {
			resp.Body.Close()
			return fmt.Errorf("插件包超过大小上限（%dMB）", sizeLimit>>20)
		}
		flag := os.O_WRONLY | os.O_CREATE
		if appendMode {
			flag |= os.O_APPEND
		} else {
			flag |= os.O_TRUNC
		}
		out, openErr := os.OpenFile(destPath, flag, 0o644)
		if openErr != nil {
			resp.Body.Close()
			return fmt.Errorf("创建下载文件失败：%w", openErr)
		}
		n, copyErr := io.Copy(out, io.LimitReader(resp.Body, sizeLimit-offset+1))
		out.Close()
		resp.Body.Close()
		if sizeLimit > 0 && n > sizeLimit-offset {
			return fmt.Errorf("插件包超过大小上限（%dMB）", sizeLimit>>20)
		}
		offset += n
		if copyErr == nil {
			return nil // 完整落盘
		}
		lastErr = fmt.Errorf("下载插件包失败：%w", copyErr)
	}
	return lastErr
}
