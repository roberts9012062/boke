// internal/ghclient/client.go
// GitHub 客户端（连接器类，外部系统接口）：拉取插件商城清单（plugins.json）。
// 说明（M3.1）：插件商城以 GitHub 仓库为内容载体——用户可自定义仓库地址（settings.plugin_source），
//   清单文件约定为仓库根目录 plugins.json（GitHub Contents API + raw 格式返回）。
package ghclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// 请求超时（GitHub API 慢时快速失败）。
const requestTimeout = 8 * time.Second

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

// FetchManifest 拉取仓库根目录的 plugins.json 清单。
// 参数：ctx 上下文；owner 仓库属主；repo 仓库名。
// 返回：清单文件原始内容（UTF-8）；仓库或文件不存在返回错误。
// 说明：优先 Contents API（带 token 可访问私有仓库），响应为 base64 JSON 或 raw 文本（Accept 头指定）。
func (c *Client) FetchManifest(ctx context.Context, owner string, repo string) ([]byte, error) {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return nil, errors.New("插件源仓库格式不正确（应为 owner/repo）")
	}

	// 防路径注入：仓库名仅允许常规字符
	for _, ch := range owner + repo {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' || ch == '.') {
			return nil, errors.New("插件源仓库格式不正确")
		}
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
