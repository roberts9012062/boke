// internal/service/plugin_manifest_local.go
// 清单本地镜像兜底（2026-08-19 修复）：nav 缺失时从本地 marketplace-repo 镜像读取。
//
// 背景：「我的插件」侧栏入口（nav）原先完全依赖远程 GitHub 清单拉取——网络故障、
// 限流或插件不在清单（本地上传 .bpk 安装）时 nav 丢失，插件明明运行中入口却消失。
// marketplace-repo 目录与插件源仓库同源（scripts/push-marketplace-repo.sh 的同步源），
// 作为最后一道兜底：远程清单补充后仍缺 nav 的项，读本地镜像同名 plugin.json 补齐。
package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// localRepoDir 本地清单镜像目录（相对服务工作目录，随项目部署）。
const localRepoDir = "marketplace-repo"

// localPluginManifest 本地 plugin.json 的最小解析结构（仅取 nav，其余字段不关心）。
type localPluginManifest struct {
	Nav *PluginNav `json:"nav"` // 侧栏入口声明
}

// readNavFromLocalRepo 读单个插件的 nav 声明（文件缺失/ID 非法/解析失败一律返回 nil，
// 调用方静默兜底——本地镜像缺失不应影响列表主流程）。
func readNavFromLocalRepo(pluginID string) *PluginNav {
	// 插件 ID 来自安装记录，防御路径穿越（拒绝分隔符与目录回溯）
	if pluginID == "" || strings.ContainsAny(pluginID, `/\`) || pluginID == "." || pluginID == ".." {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(localRepoDir, pluginID, "plugin.json"))
	if err != nil {
		return nil
	}
	var manifest localPluginManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil
	}
	return manifest.Nav
}

// fillNavFromLocalRepo 为已装列表中 nav 仍为空的项做本地镜像兜底（仅补空——
// 远程清单已补到的优先，本地镜像只在缺失时生效；风格与 ListInstalled 清单补充一致）。
func fillNavFromLocalRepo(items []InstalledPluginDTO) {
	for i := range items {
		if items[i].Nav == nil {
			items[i].Nav = readNavFromLocalRepo(items[i].PluginID)
		}
	}
}
