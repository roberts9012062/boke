// internal/ai/router.go
// AI 任务路由（M4）：任务未绑定指定供应商时，按优先级自动选择供应商。
//
// 路由规则：enabled=true 的供应商中，priority 最小者优先（种子数据 1-5）。
// 纯函数：不修改入参、不访问外部状态。
package ai

import (
	"errors"
)

// ProviderCandidate 路由输入所需的供应商字段（与 repository 层解耦的最小结构）。
type ProviderCandidate struct {
	ID       int64 // 供应商 ID
	Enabled  bool  // 是否启用
	Priority int   // 路由优先级（小先选）
}

// ErrNoProvider 无可用供应商（全部未启用/未配置）。
var ErrNoProvider = errors.New("没有可用的 AI 供应商，请先在「AI 设置」中启用供应商并配置 API Key")

// RouteProvider 从供应商列表中选择最优路由目标（enabled 且 priority 最小）。
// 返回：选中供应商；无可用时返回 ErrNoProvider。
func RouteProvider(providers []ProviderCandidate) (ProviderCandidate, error) {
	var best ProviderCandidate
	found := false
	for _, p := range providers {
		if !p.Enabled {
			continue
		}
		// 优先选中第一个，之后仅当 priority 更小时替换
		if !found || p.Priority < best.Priority {
			best = p
			found = true
		}
	}
	if !found {
		return ProviderCandidate{}, ErrNoProvider
	}
	return best, nil
}
