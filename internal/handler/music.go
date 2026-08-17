// internal/handler/music.go
// 音乐桥接控制器（E7 可 pluggable 化 + B2 seam 化）：宿主提供通用公开桥接，
// 音乐源经 music capability seam（plugin.MusicSource）解析——handler 为纯消费方，
// 不感知插件 ID 与 gRPC；发现/注册表/兜底逻辑收敛在 service.PluginService.MusicSource。
//
// 契约（见 docs/plugin-development.md 音乐源章节）：
//   - 插件市场清单声明 music_provider（provider 名，如 "qq"/"netease"）；
//   - 插件实现标准端点：POST /music/url（body {src} → {url}）、GET /music/bgm（可选）；
//   - 宿主端点：GET /api/v1/music/:provider/url?src=、GET /api/v1/music/:provider/bgm（公开）；
//   - 新增音乐源插件无需改宿主代码（注册表 + 清单声明即可）。
package handler

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// MusicHandler 音乐桥接控制器（连接器类）。
type MusicHandler struct {
	music     *service.QQMusicService // QQ 音乐解析服务（songmid→songid 转换，历史端点）
	pluginSvc *service.PluginService  // 插件服务（music seam 消费门面）
}

// NewMusicHandler 创建音乐解析控制器。
func NewMusicHandler(music *service.QQMusicService, pluginSvc *service.PluginService) *MusicHandler {
	return &MusicHandler{music: music, pluginSvc: pluginSvc}
}

// ResolveQQ 解析 QQ 音乐 songmid（GET /api/v1/music/qq-resolve?songmid=xxx，需登录）。
// 返回：songid 数字 ID / title 歌名 / artist 歌手（前端据此生成播放器 URL）。
func (h *MusicHandler) ResolveQQ(c *gin.Context) {
	songmid := c.Query("songmid")
	if songmid == "" {
		resp.FailFrom(c, errs.ErrBadRequest)
		return
	}
	songid, title, artist, err := h.music.ResolveSongID(c.Request.Context(), songmid)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{
		"songmid": songmid,
		"songid":  songid,
		"title":   title,
		"artist":  artist,
	})
}

// ProviderURL 通用播放地址桥接（GET /api/v1/music/:provider/url?src=xxx，公开）。
// src 为源特定标识（qq= songmid、netease= 歌曲 id），经 music seam 解析。
func (h *MusicHandler) ProviderURL(c *gin.Context) {
	h.providerURL(c, c.Param("provider"), c.Query("src"))
}

// providerURL 解析播放地址（provider/src 显式传参）。
// 说明：兼容端点（NeteaseURL/QqURL）经此复用而非改写 RawQuery 转发——
// gin 的 c.Query 有惰性 queryCache，handler 内改写 RawQuery 后再 Query 读到的
// 仍是旧缓存（历史 bug：qq-url/netease-url 恒返回参数错误，BGM 播放器点击无反应）。
func (h *MusicHandler) providerURL(c *gin.Context, provider string, src string) {
	if src == "" {
		resp.FailFrom(c, errs.ErrBadRequest)
		return
	}
	if h.pluginSvc == nil {
		resp.FailFrom(c, errs.New(errs.CodeUpstream, "插件服务未配置"))
		return
	}
	source, err := h.pluginSvc.MusicSource(c.Request.Context(), provider)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	status, data, err := source.ResolveURL(c.Request.Context(), src)
	if err != nil {
		resp.FailFrom(c, errs.New(errs.CodeUpstream, "音乐源插件调用失败"))
		return
	}
	// 插件返回自定义格式（非统一包装），直接透传 JSON
	c.Data(status, "application/json; charset=utf-8", data)
}

// ProviderBGM 通用背景音乐桥接（GET /api/v1/music/:provider/bgm，公开）。
// 经 music seam 解析（未实现该端点的源返回 404 透传为空配置）。
func (h *MusicHandler) ProviderBGM(c *gin.Context) {
	h.providerBGM(c, c.Param("provider"))
}

// providerBGM 解析背景音乐（provider 显式传参；兼容端点复用，同 providerURL 说明）。
func (h *MusicHandler) providerBGM(c *gin.Context, provider string) {
	if h.pluginSvc == nil {
		resp.FailFrom(c, errs.New(errs.CodeUpstream, "插件服务未配置"))
		return
	}
	source, err := h.pluginSvc.MusicSource(c.Request.Context(), provider)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	status, data, err := source.ResolveBGM(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, errs.New(errs.CodeUpstream, "音乐源插件调用失败"))
		return
	}
	// 插件未实现 BGM 契约（404）：返回空配置（前台悬浮播放器不展示）
	if status == 404 {
		resp.OK(c, gin.H{"enabled": false, "songs": []gin.H{}})
		return
	}
	var s struct {
		Enabled     bool             `json:"enabled"`
		PlaylistTid string           `json:"playlist_tid"`
		Songs       []map[string]any `json:"songs"`
	}
	_ = json.Unmarshal(data, &s)
	if s.Songs == nil {
		s.Songs = []map[string]any{}
	}
	resp.OK(c, gin.H{"enabled": s.Enabled, "playlist_tid": s.PlaylistTid, "songs": s.Songs})
}

// ---------- 兼容端点（前端存量调用；内部走通用桥接，显式传参规避 gin queryCache） ----------

// NeteaseURL 获取网易云歌曲播放地址（GET /api/v1/music/netease-url?song_id=xxx，公开）。
func (h *MusicHandler) NeteaseURL(c *gin.Context) {
	songID := strings.TrimSpace(c.Query("song_id"))
	if songID == "" {
		resp.FailFrom(c, errs.ErrBadRequest)
		return
	}
	h.providerURL(c, "netease", songID)
}

// QqURL 获取 QQ 音乐歌曲播放地址（GET /api/v1/music/qq-url?songmid=xxx，公开）。
func (h *MusicHandler) QqURL(c *gin.Context) {
	songmid := strings.TrimSpace(c.Query("songmid"))
	if songmid == "" {
		resp.FailFrom(c, errs.ErrBadRequest)
		return
	}
	h.providerURL(c, "qq", songmid)
}

// QqBGM 获取首页背景音乐配置与歌单歌曲（GET /api/v1/music/qq-bgm，公开）。
func (h *MusicHandler) QqBGM(c *gin.Context) {
	h.providerBGM(c, "qq")
}
