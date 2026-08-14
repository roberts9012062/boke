// internal/handler/music.go
// 音乐解析控制器：QQ 音乐 songmid→songid 转换 + 网易云播放地址公开代理。
package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// MusicHandler 音乐解析控制器（连接器类）。
type MusicHandler struct {
	music     *service.QQMusicService // QQ 音乐解析服务
	pluginSvc *service.PluginService  // 插件服务（网易云播放地址代理）
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

// NeteaseURL 获取网易云歌曲播放地址（GET /api/v1/music/netease-url?song_id=xxx，公开）。
// 说明：访客无需登录即可播放，故本端点挂公开组（OptionalAuth）；
//       内部转发到 netease-music 插件的 /song-url（插件用站长登录态取地址）。
func (h *MusicHandler) NeteaseURL(c *gin.Context) {
	songID := c.Query("song_id")
	if songID == "" {
		resp.FailFrom(c, errs.ErrBadRequest)
		return
	}
	if h.pluginSvc == nil {
		resp.FailFrom(c, errs.New(errs.CodeUpstream, "网易云音乐插件未安装或未启用"))
		return
	}
	body, _ := json.Marshal(map[string]string{"id": songID})
	status, data, err := h.pluginSvc.CallAPI(c.Request.Context(), "netease-music", "POST", "/song-url", body)
	if err != nil {
		resp.FailFrom(c, errs.New(errs.CodeUpstream, "网易云音乐插件调用失败"))
		return
	}
	// 插件返回自定义格式（非统一包装），直接透传 JSON
	c.Data(status, "application/json; charset=utf-8", data)
}

// QqURL 获取 QQ 音乐歌曲播放地址（GET /api/v1/music/qq-url?songmid=xxx，公开）。
// 说明：访客无需登录即可播放，故本端点挂公开组；
//       内部转发到 qq-music 插件的 /song-url（插件用站长登录态取 vkey 地址）。
func (h *MusicHandler) QqURL(c *gin.Context) {
	songmid := c.Query("songmid")
	if songmid == "" {
		resp.FailFrom(c, errs.ErrBadRequest)
		return
	}
	if h.pluginSvc == nil {
		resp.FailFrom(c, errs.New(errs.CodeUpstream, "QQ 音乐插件未安装或未启用"))
		return
	}
	body, _ := json.Marshal(map[string]string{"songmid": songmid})
	status, data, err := h.pluginSvc.CallAPI(c.Request.Context(), "qq-music", "POST", "/song-url", body)
	if err != nil {
		resp.FailFrom(c, errs.New(errs.CodeUpstream, "QQ 音乐插件调用失败"))
		return
	}
	c.Data(status, "application/json; charset=utf-8", data)
}

// QqBGM 获取首页背景音乐配置与歌单歌曲（GET /api/v1/music/qq-bgm，公开）。
// 说明：访客无需登录即可看到悬浮播放器；内部转发到 qq-music 插件的
//       /bgm-settings（开关 + 歌单 ID）与 /playlist-songs（歌曲列表）。
func (h *MusicHandler) QqBGM(c *gin.Context) {
	if h.pluginSvc == nil {
		resp.FailFrom(c, errs.New(errs.CodeUpstream, "QQ 音乐插件未安装或未启用"))
		return
	}
	status, data, err := h.pluginSvc.CallAPI(c.Request.Context(), "qq-music", "GET", "/bgm-settings", nil)
	if err != nil {
		resp.FailFrom(c, errs.New(errs.CodeUpstream, "QQ 音乐插件调用失败"))
		return
	}
	var s struct {
		Enabled     bool   `json:"enabled"`
		PlaylistTid string `json:"playlist_tid"`
	}
	_ = json.Unmarshal(data, &s)
	out := gin.H{"enabled": s.Enabled, "playlist_tid": s.PlaylistTid, "songs": []gin.H{}}
	if s.Enabled && s.PlaylistTid != "" {
		reqBody, _ := json.Marshal(map[string]string{"tid": s.PlaylistTid})
		if _, songsData, err2 := h.pluginSvc.CallAPI(c.Request.Context(), "qq-music", "POST", "/playlist-songs", reqBody); err2 == nil {
			var pr struct {
				Songs []map[string]any `json:"songs"`
			}
			if json.Unmarshal(songsData, &pr) == nil && pr.Songs != nil {
				out["songs"] = pr.Songs
			}
		}
	}
	_ = status
	resp.OK(c, out)
}
