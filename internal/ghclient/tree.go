// internal/ghclient/tree.go
// GitHub 文件树（插件商城文件夹结构清单：git trees API recursive 枚举）。
// 说明（M5 文件夹结构）：商城不再读仓库根 plugins.json，改为枚举仓库文件夹——
//
//	每个插件一个文件夹，内含 plugin.json；经 FetchTree 一次拿到全量路径后由 service 层筛选。
package ghclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GitTree 仓库文件树响应。
type GitTree struct {
	SHA       string         `json:"sha"`       // 树对象 SHA
	Truncated bool           `json:"truncated"` // 是否截断（条目过多）
	Tree      []GitTreeEntry `json:"tree"`      // 条目列表
}

// GitTreeEntry 树条目。
type GitTreeEntry struct {
	Path string `json:"path"` // 相对路径（如 seo-optimizer/plugin.json）
	Type string `json:"type"` // blob / tree
}

// FetchTree 拉取仓库默认分支的完整文件树（recursive=1）。
// 返回：全量路径列表；仓库为空（409）或不存在（404）返回错误。
func (c *Client) FetchTree(ctx context.Context, owner string, repo string) ([]string, error) {
	if err := validateRepo(owner, repo); err != nil {
		return nil, err
	}
	reqURL := c.wrapURL(fmt.Sprintf("%s/repos/%s/%s/git/trees/HEAD?recursive=1", c.base(), owner, repo))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	c.setAuth(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("拉取插件源文件树失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		// 空仓库：GitHub 对无提交仓库的 trees 接口返回 409
		return nil, fmt.Errorf("插件源仓库为空，请按文件夹结构组织插件后重试")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("拉取插件源文件树失败（HTTP %d）：%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tree GitTree
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, fmt.Errorf("解析插件源文件树失败：%w", err)
	}
	if tree.Truncated {
		return nil, fmt.Errorf("插件源文件树过大（已被 GitHub 截断），请精简仓库结构")
	}
	paths := make([]string, 0, len(tree.Tree))
	for _, entry := range tree.Tree {
		paths = append(paths, entry.Path)
	}
	return paths, nil
}
