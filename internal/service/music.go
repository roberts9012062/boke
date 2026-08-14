// internal/service/music.go
// QQ 音乐歌曲解析服务：把 songmid 转换为数字 songid，供前端生成可播放的外链播放器 URL。
//
// 背景（2026-08 实测）：QQ 音乐外链播放器（i.y.qq.com/n2/m/outchain/player/index.html）
// 参数格式已变更——只支持 songid（数字 ID）与 shorttag（短链码），不再支持 songmid，
// 旧格式生成的 iframe 一律提示「请粘贴正确链接」。而用户粘贴的分享链接只含 songmid，
// 故需先做 songmid→songid 转换。
//
// 转换链路（两步，均为腾讯公开接口，实测可用）：
//   1. 歌词接口 fcg_query_lyric_new.fcg?songmid=xxx → LRC 中的 [ti:歌名] [ar:歌手]
//   2. 搜索接口 client_search_cp?w=歌名+歌手 → 结果列表按 songmid 精确匹配 → songid
// 结果内存缓存 24 小时，避免同一歌曲重复请求外部接口。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/roberts9012062/boke/pkg/errs"
)

// qqMusicTimeout 腾讯接口请求超时。
const qqMusicTimeout = 8 * time.Second

// qqLyricTitle 歌词标题行匹配（LRC 头部 [ti:xxx]）。
var qqLyricTitle = regexp.MustCompile(`\[ti:([^\]]+)\]`)

// qqLyricArtist 歌词歌手行匹配（LRC 头部 [ar:xxx]）。
var qqLyricArtist = regexp.MustCompile(`\[ar:([^\]]+)\]`)

// QQMusicService QQ 音乐 songmid→songid 解析服务（连接器类，访问外部系统）。
type QQMusicService struct {
	httpClient *http.Client            // HTTP 客户端（共享连接池）
	cache      map[string]qqMusicEntry // 解析结果缓存（key=songmid）
	mu         sync.Mutex              // 缓存并发保护
}

// qqMusicEntry 缓存条目。
type qqMusicEntry struct {
	songid  int64     // 数字歌曲 ID（播放器参数）
	title   string    // 歌名
	artist  string    // 歌手
	expires time.Time // 过期时间
}

// qqLyricResp 歌词接口响应体（仅取用到的字段）。
type qqLyricResp struct {
	Lyric string `json:"lyric"` // LRC 歌词文本
}

// qqSearchResp 搜索接口响应体（仅取用到的字段）。
type qqSearchResp struct {
	Data struct {
		Song struct {
			List []struct {
				SongID   int64  `json:"songid"`  // 数字歌曲 ID
				SongMID  string `json:"songmid"` // 字符串歌曲 MID
				SongName string `json:"songname"` // 歌名
			} `json:"list"`
		} `json:"song"`
	} `json:"data"`
}

// NewQQMusicService 创建 QQ 音乐解析服务。
func NewQQMusicService() *QQMusicService {
	return &QQMusicService{
		httpClient: &http.Client{Timeout: qqMusicTimeout},
		cache:      make(map[string]qqMusicEntry),
	}
}

// ResolveSongID 解析 songmid → songid（含歌名/歌手）。
// 参数：ctx 上下文；songmid QQ 音乐歌曲 MID（如 0039MnYb0qxYhV）。
// 返回：songid 数字 ID；title 歌名；artist 歌手；err 解析失败（外部接口异常/歌曲不存在）。
func (s *QQMusicService) ResolveSongID(ctx context.Context, songmid string) (int64, string, string, error) {
	// ---------- 缓存命中直接返回（24 小时 TTL） ----------
	s.mu.Lock()
	if entry, ok := s.cache[songmid]; ok && time.Now().Before(entry.expires) {
		s.mu.Unlock()
		return entry.songid, entry.title, entry.artist, nil
	}
	s.mu.Unlock()

	// ---------- 步骤 1：歌词接口拿歌名与歌手 ----------
	title, artist, err := s.fetchLyricMeta(ctx, songmid)
	if err != nil {
		return 0, "", "", err
	}

	// ---------- 步骤 2：搜索接口按歌名+歌手检索，songmid 精确匹配 ----------
	songid, err := s.searchSongID(ctx, songmid, title, artist)
	if err != nil {
		return 0, "", "", err
	}

	// 写缓存（解析成功才缓存）
	s.mu.Lock()
	s.cache[songmid] = qqMusicEntry{songid: songid, title: title, artist: artist, expires: time.Now().Add(24 * time.Hour)}
	s.mu.Unlock()
	return songid, title, artist, nil
}

// fetchLyricMeta 调歌词接口获取歌名/歌手（纯函数，不修改接收者状态）。
func (s *QQMusicService) fetchLyricMeta(ctx context.Context, songmid string) (string, string, error) {
	lyricURL := fmt.Sprintf(
		"https://c.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg?songmid=%s&format=json&nobase64=1",
		url.QueryEscape(songmid),
	)
	body, err := s.getJSON(ctx, lyricURL)
	if err != nil {
		return "", "", err
	}
	var lyricResp qqLyricResp
	if err := json.Unmarshal(body, &lyricResp); err != nil {
		return "", "", errs.New(errs.CodeUpstream, "QQ音乐服务暂时不可用，请稍后再试")
	}
	title := strings.TrimSpace(firstMatch(qqLyricTitle, lyricResp.Lyric))
	artist := strings.TrimSpace(firstMatch(qqLyricArtist, lyricResp.Lyric))
	if title == "" {
		// 歌词为空说明歌曲不存在或已下架
		return "", "", errs.New(errs.CodeNotFound, "QQ音乐未找到该歌曲，请检查链接")
	}
	return title, artist, nil
}

// searchSongID 调搜索接口按歌名+歌手检索，songmid 精确匹配取 songid（纯函数）。
func (s *QQMusicService) searchSongID(ctx context.Context, songmid string, title string, artist string) (int64, error) {
	query := url.QueryEscape(strings.TrimSpace(title + " " + artist))
	searchURL := fmt.Sprintf(
		"https://c.y.qq.com/soso/fcgi-bin/client_search_cp?format=json&w=%s&n=5&cr=1",
		query,
	)
	body, err := s.getJSON(ctx, searchURL)
	if err != nil {
		return 0, err
	}
	var searchResp qqSearchResp
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return 0, errs.New(errs.CodeUpstream, "QQ音乐服务暂时不可用，请稍后再试")
	}
	for _, item := range searchResp.Data.Song.List {
		if item.SongMID == songmid {
			return item.SongID, nil
		}
	}
	return 0, errs.New(errs.CodeNotFound, "QQ音乐未找到该歌曲，请检查链接")
}

// getJSON 发起 GET 请求并返回响应体（携带 QQ 音乐要求的 Referer/UA 头）。
func (s *QQMusicService) getJSON(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	// 腾讯接口要求来源头，缺失会被拒（HTTP 403 或空数据）
	req.Header.Set("Referer", "https://y.qq.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0 Safari/537.36")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, "QQ音乐服务暂时不可用，请稍后再试")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errs.New(errs.CodeUpstream, "QQ音乐服务暂时不可用，请稍后再试")
	}
	return io.ReadAll(io.LimitReader(resp.Body, 2<<20))
}

// firstMatch 取正则首个捕获组（无匹配返回空串；纯函数）。
func firstMatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}
