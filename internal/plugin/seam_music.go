// internal/plugin/seam_music.go
// music 服务接缝（B2 首个 capability seam，三角色完整落地）：
//   - 服务定义：MusicSource 接口 + MusicSourceKey 键构造（本文件）
//   - 提供方：NewMusicSourceAdapter——进程外音乐插件（netease/qq）经此包装注册进 ServiceRegistry；
//     未来内置实现或新源插件按同键并列注册即可，消费方零改动
//   - 消费方：internal/handler/music.go——LookupService 查找后调用，不感知插件 ID 与 gRPC
//
// 设计说明（对齐 dsh ctx.music 思想，裁剪为博客场景）：
//   - 返回透传 (status, data)：各音乐源响应格式不同（前端按源解析），
//     seam 契约统一为「解析播放地址」而非统一响应结构——避免为对齐而对齐的过度抽象；
//   - 键形如 "music.netease"——命名空间 + 提供方名，新增源即新增键，宿主零代码。
package plugin

import (
	"context"
	"encoding/json"

	sdk "github.com/roberts9012062/boke/pkg/plugin-sdk"
)

// musicNamespace music 服务键命名空间（seam 目录：唯一已落地 seam；ai/search 预留）。
const musicNamespace = "music"

// MusicSource 音乐源服务（seam 服务定义：宿主消费方依赖此接口，不依赖具体插件）。
type MusicSource interface {
	// ResolveURL 解析单曲播放地址（src 为源特定标识：qq=songmid、netease=歌曲 id）。
	// 返回：status/data 为插件端点原始响应透传（各源格式不同，前端按源解析）。
	ResolveURL(ctx context.Context, src string) (status int, data []byte, err error)
	// ResolveBGM 解析背景音乐配置与歌单（未实现 BGM 的源返回 404 空配置由消费方兜底）。
	ResolveBGM(ctx context.Context) (status int, data []byte, err error)
}

// MusicSourceKey 构造 music 服务键（"music.{provider}"；provider 如 "netease"/"qq"）。
func MusicSourceKey(provider string) string {
	return musicNamespace + "." + provider
}

// PluginAPICaller 插件自定义 API 调用函数（提供方适配器依赖；由 service 层注入，
// 避免 plugin 包反向依赖 service——依赖方向保持 service → plugin 单向）。
// 语义与 PluginService.CallAPI 一致：method+path 转发到运行中插件进程。
type PluginAPICaller func(ctx context.Context, pluginID string, method string, path string, body []byte, caller sdk.CallerIdentity) (int, []byte, error)

// MusicSourceAdapter 进程外音乐插件 → MusicSource 提供方适配器（连接器类）。
// 契约端点（见 docs/plugin-development.md 音乐源章节）：
// POST /music/url（body {src}）与 GET /music/bgm，经宿主系统调用者身份转发。
type MusicSourceAdapter struct {
	pluginID  string          // 提供方插件 ID（CallAPI 目标）
	callAPI   PluginAPICaller // 插件 API 调用（注入，含调用者身份）
	caller    sdk.CallerIdentity // 系统调用者身份（桥接为宿主产品决策，插件侧 TrustedCaller 放行）
}

// NewMusicSourceAdapter 创建适配器。
// 参数：pluginID 音乐插件 ID；callAPI 插件 API 调用函数；caller 调用者身份（宿主系统身份）。
func NewMusicSourceAdapter(pluginID string, callAPI PluginAPICaller, caller sdk.CallerIdentity) *MusicSourceAdapter {
	return &MusicSourceAdapter{pluginID: pluginID, callAPI: callAPI, caller: caller}
}

// ResolveURL 解析单曲播放地址（转发插件 POST /music/url 契约端点）。
func (a *MusicSourceAdapter) ResolveURL(ctx context.Context, src string) (int, []byte, error) {
	body, err := json.Marshal(map[string]string{"src": src})
	if err != nil {
		return 0, nil, err
	}
	return a.callAPI(ctx, a.pluginID, "POST", "/music/url", body, a.caller)
}

// ResolveBGM 解析背景音乐（转发插件 GET /music/bgm 契约端点）。
func (a *MusicSourceAdapter) ResolveBGM(ctx context.Context) (int, []byte, error) {
	return a.callAPI(ctx, a.pluginID, "GET", "/music/bgm", nil, a.caller)
}
