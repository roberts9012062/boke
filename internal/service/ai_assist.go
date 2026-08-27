// internal/service/ai_assist.go
// 发帖 AI 辅助（MiniMax 多模态接入）：内容扩写 / 润色（chat 管道），
// 内容配图（image-01 文生图）、配乐（music-3.0 音乐生成）、图片识别（MiniMax-M3 视觉）。
//
// 路由约定：任务配置（ai_tasks）提供提示词模板与启停开关；
// 配图/配乐/识图按任务 model 字段路由到含该模型的供应商（通常为 minimax 渠道）。
// 生成物 URL 上游仅 24 小时有效——下载后转存本站媒体库（media_assets 登记）再返回。
package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/roberts9012062/boke/internal/ai"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 发帖 AI 辅助动作常量（与前端 compose 辅助面板约定）。
const (
	AssistExpand    = "expand"    // 内容扩写（post.expand 任务）
	AssistPolish    = "polish"    // 内容润色（post.polish 任务）
	AssistImage     = "image"     // 内容配图（post.image 任务 → image-01 文生图）
	AssistMusic     = "music"     // 内容配乐（post.music 任务 → music-3.0 音乐生成）
	AssistRecognize = "recognize" // 图片识别（image.recognize 任务 → MiniMax-M3 视觉）
)

// 对应任务名（迁移 023 种子；提示词在后台「AI 设置-任务配置」可改）。
var assistTaskNames = map[string]string{
	AssistExpand:    "post.expand",
	AssistPolish:    "post.polish",
	AssistImage:     "post.image",
	AssistMusic:     "post.music",
	AssistRecognize: "image.recognize",
}

// AssistResult 辅助执行结果（文本类填 Text；生成类填媒体字段）。
type AssistResult struct {
	Action    string `json:"action"`               // 动作名
	Text      string `json:"text,omitempty"`       // 文本结果（扩写/润色/识图描述）
	MediaURL  string `json:"media_url,omitempty"`  // 生成物本站地址（配图/配乐，已转存）
	MediaType string `json:"media_type,omitempty"` // 生成物类型（image / audio）
	MediaID   int64  `json:"media_id,omitempty"`   // 媒体库 ID（发帖关联 media_ids 用）
	MediaMime string `json:"media_mime,omitempty"` // MIME 类型
	MediaSize int64  `json:"media_size,omitempty"` // 文件大小（字节）
}

// AssistInput 辅助请求参数（按动作取用：content 正文；imageURL 待识别图片地址）。
type AssistInput struct {
	Content  string // 帖子正文（扩写/润色/配图/配乐的输入）
	ImageURL string // 待识别图片地址（recognize）
}

// Assist 发帖 AI 辅助统一入口（动作分派）。
func (s *AiService) Assist(ctx context.Context, action string, input AssistInput) (*AssistResult, error) {
	taskName, ok := assistTaskNames[action]
	if !ok {
		return nil, errs.New(errs.CodeBadRequest, "不支持的 AI 辅助动作："+action)
	}
	task, found, err := s.tasks.FindByName(ctx, taskName)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errs.New(errs.CodeNotFound, "AI 任务「"+taskName+"」未配置（迁移 023 种子缺失）")
	}
	if !task.Enabled {
		return nil, errs.New(errs.CodeStateConflict, "AI 任务「"+taskName+"」已停用，请在 AI 设置中启用")
	}

	switch action {
	case AssistExpand, AssistPolish:
		return s.assistChat(ctx, task, input.Content)
	case AssistRecognize:
		return s.assistRecognize(ctx, task, input.ImageURL)
	case AssistImage:
		return s.assistImage(ctx, task, input.Content)
	case AssistMusic:
		return s.assistMusic(ctx, task, input.Content)
	}
	return nil, errs.New(errs.CodeBadRequest, "不支持的 AI 辅助动作："+action)
}

// assistChat 文本辅助（扩写/润色）：任务提示词 + 正文 → chat（自动路由已配置供应商）。
func (s *AiService) assistChat(ctx context.Context, task *repository.AiTask, content string) (*AssistResult, error) {
	if strings.TrimSpace(content) == "" {
		return nil, errs.New(errs.CodeBadRequest, "帖子内容为空，请先输入内容")
	}
	provider, err := s.resolveProvider(ctx, *task)
	if err != nil {
		return nil, err
	}
	result, err := s.chatProvider(ctx, provider, task.TaskName, ai.ChatRequest{
		Model: task.Model,
		Messages: []ai.Message{
			{Role: "system", Content: task.PromptTemplate},
			{Role: "user", Content: content},
		},
		MaxTokens: task.MaxTokens,
	})
	if err != nil {
		return nil, err
	}
	return &AssistResult{Action: task.TaskName, Text: result.Text}, nil
}

// assistRecognize 图片识别：任务 model 路由到视觉模型（MiniMax-M3）多模态对话。
// 图片地址支持两种形态：公网 URL 原样发送；本站路径（/media/...）读文件转
// base64 data URL 发送（不依赖上游服务器回源拉图，站内图片识别更稳定）。
func (s *AiService) assistRecognize(ctx context.Context, task *repository.AiTask, imageURL string) (*AssistResult, error) {
	if imageURL == "" {
		return nil, errs.New(errs.CodeBadRequest, "缺少待识别的图片地址")
	}
	resolvedURL, err := s.resolveImageInput(imageURL)
	if err != nil {
		return nil, err
	}
	provider, err := s.resolveProviderByModel(ctx, task.Model)
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, "图片识别需供应商配置模型 "+task.Model+"（如 MiniMax-M3）："+err.Error())
	}
	result, err := s.chatProvider(ctx, provider, task.TaskName, ai.ChatRequest{
		Model: task.Model,
		Messages: []ai.Message{
			{Role: "system", Content: task.PromptTemplate},
			{Role: "user", Content: "请识别这张图片", ImageURL: resolvedURL},
		},
		MaxTokens: task.MaxTokens,
	})
	if err != nil {
		return nil, err
	}
	return &AssistResult{Action: task.TaskName, Text: result.Text}, nil
}

// resolveImageInput 图片输入归一：本站 /media/ 路径 → base64 data URL；
// 公网 URL 与 data URL 原样返回。
func (s *AiService) resolveImageInput(imageURL string) (string, error) {
	if !strings.HasPrefix(imageURL, "/media/") || s.mediaStore == nil {
		return imageURL, nil
	}
	raw, err := os.ReadFile(filepath.Join(s.mediaStore.RootDir(), strings.TrimPrefix(imageURL, "/media/")))
	if err != nil {
		return "", errs.New(errs.CodeBadRequest, "读取站内图片失败："+err.Error())
	}
	mimeType := http.DetectContentType(raw)
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

// renderGenPrompt 组装生成类提示词（任务模板 {content} 占位替换；纯函数）。
func renderGenPrompt(template string, content string) string {
	prompt := strings.ReplaceAll(template, "{content}", content)
	return strings.TrimSpace(prompt)
}

// assistImage 内容配图：模板渲染提示词 → image-01 文生图 → 下载转存媒体库。
func (s *AiService) assistImage(ctx context.Context, task *repository.AiTask, content string) (*AssistResult, error) {
	if strings.TrimSpace(content) == "" {
		return nil, errs.New(errs.CodeBadRequest, "帖子内容为空，请先输入内容")
	}
	provider, apiKey, err := s.resolveGenerateProvider(ctx, task.Model)
	if err != nil {
		return nil, err
	}
	result, err := ai.GenerateImage(ctx, provider.BaseURL, apiKey, ai.ImageGenRequest{
		Model:  task.Model,
		Prompt: renderGenPrompt(task.PromptTemplate, content),
		N:      1,
	})
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, err.Error())
	}
	media, err := s.persistRemoteMedia(ctx, result.ImageURLs[0], "ai-image", "image")
	if err != nil {
		return nil, err
	}
	return &AssistResult{Action: task.TaskName, MediaURL: media.URL, MediaType: "image",
		MediaID: media.ID, MediaMime: media.MimeType, MediaSize: media.SizeBytes}, nil
}

// assistMusic 内容配乐：模板渲染提示词 → music-3.0 纯音乐生成 → 下载转存媒体库。
func (s *AiService) assistMusic(ctx context.Context, task *repository.AiTask, content string) (*AssistResult, error) {
	if strings.TrimSpace(content) == "" {
		return nil, errs.New(errs.CodeBadRequest, "帖子内容为空，请先输入内容")
	}
	provider, apiKey, err := s.resolveGenerateProvider(ctx, task.Model)
	if err != nil {
		return nil, err
	}
	result, err := ai.GenerateMusic(ctx, provider.BaseURL, apiKey, ai.MusicGenRequest{
		Model:        task.Model,
		Prompt:       renderGenPrompt(task.PromptTemplate, content),
		Instrumental: true, // 博客配乐场景：纯音乐（无人声）
		AudioFormat:  "mp3",
	})
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, err.Error())
	}
	media, err := s.persistRemoteMedia(ctx, result.AudioURL, "ai-music", "audio")
	if err != nil {
		return nil, err
	}
	return &AssistResult{Action: task.TaskName, MediaURL: media.URL, MediaType: "audio",
		MediaID: media.ID, MediaMime: media.MimeType, MediaSize: media.SizeBytes}, nil
}

// resolveGenerateProvider 解析生成类供应商（按任务 model 找到含该模型的供应商并解密 Key）。
func (s *AiService) resolveGenerateProvider(ctx context.Context, model string) (*repository.AiProvider, string, error) {
	provider, err := s.resolveProviderByModel(ctx, model)
	if err != nil {
		return nil, "", errs.New(errs.CodeUpstream,
			"生成类任务需供应商配置模型 "+model+"（在 AI 设置的 minimax 渠道模型清单中）："+err.Error())
	}
	apiKey, err := decryptAPIKey(provider.APIKeyEncrypted, s.keySecret)
	if err != nil || apiKey == "" {
		return nil, "", errs.New(errs.CodeUpstream, "供应商「"+provider.Name+"」未配置 API Key，请先在 AI 设置中填写")
	}
	return provider, apiKey, nil
}

// downloadLimit 生成物下载大小上限（64MB，与媒体单文件上限对齐）。
const downloadLimit = 64 << 20

// persistedMedia 转存结果（本站地址 + 媒体库登记信息）。
type persistedMedia struct {
	URL       string // 本站可持久访问地址（/media/...）
	ID        int64  // media_assets ID（0=登记失败，仅落盘）
	MimeType  string
	SizeBytes int64
}

// persistRemoteMedia 下载上游生成物（24h 失效 URL）→ 转存本站媒体库并登记。
func (s *AiService) persistRemoteMedia(ctx context.Context, remoteURL string, namePrefix string, kind string) (*persistedMedia, error) {
	if s.mediaStore == nil {
		return nil, errs.New(errs.CodeStateConflict, "媒体存储未配置，无法转存生成物")
	}
	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, "生成物下载请求构造失败："+err.Error())
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, "生成物下载失败（上游 URL 有效期仅 24 小时）："+err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errs.New(errs.CodeUpstream, fmt.Sprintf("生成物下载失败：HTTP %d", resp.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, downloadLimit))
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, "生成物读取失败："+err.Error())
	}
	// 文件名：前缀 + 扩展名（按类型；image-01 输出 png，音乐输出 mp3）
	ext := ".png"
	mimeType := "image/png"
	if kind == "audio" {
		ext, mimeType = ".mp3", "audio/mpeg"
	}
	stored, err := s.mediaStore.SaveBytes(namePrefix+"-"+time.Now().Format("20060102-150405")+ext, mimeType, data)
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, "生成物转存媒体库失败："+err.Error())
	}
	out := &persistedMedia{URL: stored.URL, MimeType: stored.MimeType, SizeBytes: stored.SizeBytes}
	// 媒体登记（失败不阻断——文件已落盘可访问，仅列表缺记录）
	if s.mediaRepo != nil {
		if id, err := s.mediaRepo.Create(ctx, repository.MediaAsset{
			OwnerID:    0, // 0=系统生成（AI 辅助产物）
			Type:       stored.Type,
			StorageKey: stored.StorageKey,
			URL:        stored.URL,
			MimeType:   stored.MimeType,
			SizeBytes:  stored.SizeBytes,
			Status:     "ready",
		}); err == nil {
			out.ID = id
		}
	}
	return out, nil
}
