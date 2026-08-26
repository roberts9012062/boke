// internal/plugin/seam_media.go
// media.storage 服务接缝（capability seam，第二个落地 seam——首个见 seam_music.go）：
// 图床类插件注册后接管媒体上传存储（上传直达外部对象存储如 Cloudflare R2），
// 未注册时宿主走本地磁盘（data/media），行为向后完全兼容。
//
// 三角色：
//   - 服务定义：本文件 MediaStorage 接口 + MediaStorageKey 键构造
//   - 提供方：图床插件的 MediaStorageAdapter（经 CallAPI 转发插件契约端点）
//   - 消费方：service 层门面（plugin_seam.go）→ PostService.UploadMedia 优先 seam
//
// 插件契约端点（宿主系统调用者身份调用，插件侧放行）：
//
//	GET  /storage/health   配对探测 → {"ok":true,...}（未配置/未配对返回 error）
//	POST /storage/upload   转存 {filename, mime, content_b64} → {type,storage_key,url,mime,size}
package plugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	sdk "github.com/roberts9012062/boke/pkg/plugin-sdk"

	"github.com/roberts9012062/boke/internal/media"
)

// mediaStorageNamespace media 服务键命名空间。
const mediaStorageNamespace = "media.storage"

// MediaStorageKey media.storage 服务键（单实例——图床插件唯一，无 provider 维度）。
func MediaStorageKey() string {
	return mediaStorageNamespace
}

// MediaStorage 媒体存储 seam 服务（宿主消费方依赖此接口，不依赖具体插件）。
// Save 语义与 media.Store.Save 一致：类型/大小校验由调用方（PostService）前置完成，
// 实现负责把内容存入外部对象存储并返回可公开访问的 URL。
type MediaStorage interface {
	// Save 保存媒体内容。
	// 参数：filename 原始文件名（扩展名识别用）；mimeType 客户端声明 MIME；content 文件全量字节。
	// 返回：StorageResult（Type/StorageKey/URL/MimeType/SizeBytes，与本地存储同构）。
	Save(ctx context.Context, filename string, mimeType string, content []byte) (media.StorageResult, error)
}

// MediaStorageAdapter 进程外图床插件 → MediaStorage 提供方适配器（连接器类）。
type MediaStorageAdapter struct {
	pluginID string             // 提供方插件 ID（CallAPI 目标）
	callAPI  PluginAPICaller    // 插件 API 调用（注入，含调用者身份）
	caller   sdk.CallerIdentity // 宿主系统调用者身份（插件侧 TrustedCaller 放行）
}

// NewMediaStorageAdapter 创建媒体存储适配器。
// 参数：pluginID 图床插件 ID；callAPI 插件 API 调用函数；caller 调用者身份。
func NewMediaStorageAdapter(pluginID string, callAPI PluginAPICaller, caller sdk.CallerIdentity) *MediaStorageAdapter {
	return &MediaStorageAdapter{pluginID: pluginID, callAPI: callAPI, caller: caller}
}

// mediaUploadRequest 契约端点 /storage/upload 请求体。
type mediaUploadRequest struct {
	Filename  string `json:"filename"`    // 原始文件名（扩展名）
	Mime      string `json:"mime"`        // MIME 类型
	Content64 string `json:"content_b64"` // 内容 base64（CallAPI body 为 JSON 通道）
}

// mediaUploadResponse 契约端点 /storage/upload 响应体。
type mediaUploadResponse struct {
	Error      string `json:"error,omitempty"` // 非空=失败原因
	Type       string `json:"type"`            // 媒体类型（image/...）
	StorageKey string `json:"storage_key"`     // 外部对象键
	URL        string `json:"url"`             // 公开访问 URL
	Mime       string `json:"mime"`            // MIME
	Size       int64  `json:"size"`            // 字节数
}

// Save 转发插件 POST /storage/upload 契约端点（响应映射 StorageResult）。
func (a *MediaStorageAdapter) Save(ctx context.Context, filename string, mimeType string, content []byte) (media.StorageResult, error) {
	body, err := json.Marshal(mediaUploadRequest{
		Filename: filename, Mime: mimeType,
		Content64: base64StdEncode(content),
	})
	if err != nil {
		return media.StorageResult{}, err
	}
	status, raw, err := a.callAPI(ctx, a.pluginID, "POST", "/storage/upload", body, a.caller)
	if err != nil {
		return media.StorageResult{}, fmt.Errorf("图床插件不可达：%w", err)
	}
	var resp mediaUploadResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return media.StorageResult{}, fmt.Errorf("图床插件响应解析失败（HTTP %d）", status)
	}
	if status != 200 || resp.Error != "" {
		reason := resp.Error
		if reason == "" {
			reason = fmt.Sprintf("HTTP %d", status)
		}
		return media.StorageResult{}, fmt.Errorf("图床上传失败：%s", reason)
	}
	return media.StorageResult{
		Type: resp.Type, StorageKey: resp.StorageKey, URL: resp.URL,
		MimeType: resp.Mime, SizeBytes: resp.Size,
	}, nil
}

// base64StdEncode 标准base64 编码薄封装（统一导入处）。
func base64StdEncode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
