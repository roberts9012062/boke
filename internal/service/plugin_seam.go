// internal/service/plugin_seam.go
// capability seam 消费门面（B2 Cordis 对标）：业务侧经此查找 seam 服务，
// 不感知插件 ID 与 gRPC 细节。首个人工 seam：music（音乐源）。
//
// 懒注册策略：注册表未命中时走现有发现（市场清单 music_provider → 静态兜底表），
// 构造提供方适配器注册后返回——首次请求后直达注册表，行为向后完全兼容；
// 插件停用/卸载经 deactivate 统一 UnregisterAll 清理（注册可逆）。
//
// 新增 seam 流程（三角色检查单，见 docs/plugin-development.md seam 目录）：
//  1. 服务定义：internal/plugin/seam_{name}.go 声明接口 + 键构造；
//  2. 提供方：内置实现或 NewXxxAdapter（包装 CallAPI）注册进 ServiceRegistry；
//  3. 消费方：本文件加查找门面方法，业务 handler 只依赖接口。
package service

import (
	"context"

	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/pkg/errs"
	sdk "github.com/roberts9012062/boke/pkg/plugin-sdk"
)

// seamSystemCaller seam 桥接统一系统调用者身份（插件侧 TrustedCaller 放行；
// 公开消费是宿主的产品决策，与 handler 直连桥接语义一致）。
var seamSystemCaller = sdk.CallerIdentity{System: true}

// musicFallbackProviders 音乐源静态兜底注册表（provider → 插件 ID；
// 市场清单不可用时的发现兜底，自 handler/music.go 迁入——发现逻辑收敛到 service 层）。
var musicFallbackProviders = map[string]string{
	"netease": "netease-music",
	"qq":      "qq-music",
}

// seamRegistry 取 seam 服务注册表（懒初始化——构造签名零改动，装配零侵入）。
func (s *PluginService) seamRegistry() *plugin.ServiceRegistry {
	s.servicesOnce.Do(func() {
		s.services = plugin.NewServiceRegistry()
	})
	return s.services
}

// MusicSource 按音乐源 provider 名查找 seam 服务（消费方门面）。
// 流程：注册表命中直达 → 未命中发现（市场清单 → 静态兜底）→ 适配器懒注册 → 返回。
func (s *PluginService) MusicSource(ctx context.Context, provider string) (plugin.MusicSource, error) {
	if provider == "" {
		return nil, errs.New(errs.CodeBadRequest, "音乐源 provider 为空")
	}
	key := plugin.MusicSourceKey(provider)
	if src, ok := plugin.LookupService[plugin.MusicSource](s.seamRegistry(), key); ok {
		return src, nil
	}
	pluginID, err := s.discoverMusicPluginID(ctx, provider)
	if err != nil {
		return nil, err
	}
	if pluginID == "" {
		return nil, errs.New(errs.CodeNotFound, "未知音乐源："+provider)
	}
	adapter := plugin.NewMusicSourceAdapter(pluginID, s.CallAPI, seamSystemCaller)
	s.seamRegistry().Register(key, pluginID, adapter)
	return adapter, nil
}

// discoverMusicPluginID 发现 provider 对应的运行中音乐插件 ID。
// 优先市场清单（music_provider 声明 + 已安装 running），失败/未命中回退静态兜底表。
func (s *PluginService) discoverMusicPluginID(ctx context.Context, provider string) (string, error) {
	if pluginID, err := s.MusicProviderPlugin(ctx, provider); err == nil && pluginID != "" {
		return pluginID, nil
	}
	return musicFallbackProviders[provider], nil // 静态兜底（未命中为空串）
}
