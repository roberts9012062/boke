// internal/ai/generate.go
// MiniMax 多模态生成调用器（OpenAI 兼容体系之外的专用端点，零第三方依赖）：
//   - 文生图  POST {base_url}/image_generation（model: image-01，同步返回图片 URL，24h 失效）
//   - 音乐生成 POST {base_url}/music_generation（model: music-3.0，URL 输出，24h 失效）
// 协议文档：https://platform.minimaxi.com/docs/api-reference/api-overview
// 说明：两类生成共用 MiniMax API Key（Bearer 认证），base_url 与 chat 渠道同源；
//       生成物 URL 有效期仅 24 小时，调用方须及时下载转存。
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// generateTimeout 生成类请求超时（图片/音乐生成为同步等待，耗时显著长于对话）。
const generateTimeout = 180 * time.Second

// ImageGenRequest 文生图请求（MiniMax image_generation）。
type ImageGenRequest struct {
	Model       string // 模型名（image-01 / image-01-live）
	Prompt      string // 图像描述（≤1500 字符）
	AspectRatio string // 宽高比（空=1:1；16:9 / 4:3 / 9:16 等）
	N           int    // 生成张数（1-9；0 视为 1）
}

// ImageGenResult 文生图结果。
type ImageGenResult struct {
	ImageURLs []string // 图片 URL 列表（24 小时有效，须转存）
	TaskID    string   // 任务 ID（事后查询用）
}

// MusicGenRequest 音乐生成请求（MiniMax music_generation）。
type MusicGenRequest struct {
	Model         string // 模型名（music-3.0 等）
	Prompt        string // 风格/情绪/场景描述（纯音乐时必填）
	Lyrics        string // 歌词（带人声时必填；空 + Instrumental=true 为纯音乐）
	Instrumental  bool   // 是否纯音乐（无人声）
	AudioFormat   string // 输出音频格式（mp3 / wav；空=mp3）
}

// MusicGenResult 音乐生成结果。
type MusicGenResult struct {
	AudioURL       string // 音频 URL（24 小时有效，须转存）
	DurationMillis int64  // 时长（毫秒）
}

// minimaxBaseResp MiniMax 统一状态块（两类生成接口共用）。
type minimaxBaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

// postGenerate MiniMax 生成类统一 POST（Bearer 认证 + base_resp 错误归一）。
func postGenerate(ctx context.Context, baseURL string, apiKey string, path string, body any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("请求序列化失败：%w", err)
	}
	url := strings.TrimRight(baseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: generateTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求 MiniMax 失败：%w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return fmt.Errorf("读取响应失败：%w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MiniMax 返回 HTTP %d：%s", resp.StatusCode, truncateForLog(string(data)))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("响应解析失败：%w", err)
	}
	return nil
}

// truncateForLog 错误信息截断（纯函数，防止超长响应刷屏）。
func truncateForLog(s string) string {
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}

// GenerateImage 执行文生图（同步返回图片 URL；上游错误含 MiniMax 状态码语义）。
func GenerateImage(ctx context.Context, baseURL string, apiKey string, req ImageGenRequest) (*ImageGenResult, error) {
	n := req.N
	if n <= 0 {
		n = 1
	}
	aspect := req.AspectRatio
	if aspect == "" {
		aspect = "1:1"
	}
	var resp struct {
		ID   string `json:"id"`
		Data struct {
			ImageURLs []string `json:"image_urls"`
		} `json:"data"`
		BaseResp minimaxBaseResp `json:"base_resp"`
	}
	body := map[string]any{
		"model":           req.Model,
		"prompt":          req.Prompt,
		"aspect_ratio":    aspect,
		"n":               n,
		"response_format": "url",
	}
	if err := postGenerate(ctx, baseURL, apiKey, "/image_generation", body, &resp); err != nil {
		return nil, err
	}
	if resp.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("文生图失败（%d）：%s", resp.BaseResp.StatusCode, resp.BaseResp.StatusMsg)
	}
	if len(resp.Data.ImageURLs) == 0 {
		return nil, fmt.Errorf("文生图未返回图片")
	}
	return &ImageGenResult{ImageURLs: resp.Data.ImageURLs, TaskID: resp.ID}, nil
}

// GenerateMusic 执行音乐生成（URL 输出；MiniMax 侧合成中状态按错误提示重试）。
func GenerateMusic(ctx context.Context, baseURL string, apiKey string, req MusicGenRequest) (*MusicGenResult, error) {
	format := req.AudioFormat
	if format == "" {
		format = "mp3"
	}
	var resp struct {
		Data struct {
			Status int    `json:"status"` // 1=合成中 2=已完成
			Audio  string `json:"audio"`  // 音频 URL
			ExtraInfo struct {
				MusicDuration int64 `json:"music_duration"` // 毫秒
			} `json:"extra_info"`
		} `json:"data"`
		BaseResp minimaxBaseResp `json:"base_resp"`
	}
	body := map[string]any{
		"model":          req.Model,
		"prompt":         req.Prompt,
		"is_instrumental": req.Instrumental,
		"output_format":  "url",
		"audio_setting": map[string]any{
			"format": format,
		},
	}
	if req.Lyrics != "" {
		body["lyrics"] = req.Lyrics
	}
	if err := postGenerate(ctx, baseURL, apiKey, "/music_generation", body, &resp); err != nil {
		return nil, err
	}
	if resp.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("音乐生成失败（%d）：%s", resp.BaseResp.StatusCode, resp.BaseResp.StatusMsg)
	}
	if resp.Data.Status == 1 {
		return nil, fmt.Errorf("音乐合成中，请稍后重试")
	}
	if resp.Data.Audio == "" {
		return nil, fmt.Errorf("音乐生成未返回音频")
	}
	return &MusicGenResult{AudioURL: resp.Data.Audio, DurationMillis: resp.Data.ExtraInfo.MusicDuration}, nil
}
