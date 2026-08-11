// internal/media/store.go
// 媒体存储：本地磁盘（data/media/）保存上传文件。
//
// 约束（需求 3.4 + 6 安全）：
//   - 图片：jpg/png/gif/webp，≤10MB
//   - 音频：mp3/m4a/wav，≤20MB（"收声音"产品特色，支持录音）
//   - 随机文件名（防路径穿越与重名覆盖）
//   - 按年月分子目录（data/media/202608/）
package media

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 媒体类型（与 model.MediaAsset.Type 对应）。
const (
	TypeImage = "image"
	TypeAudio = "audio"
	TypeVideo = "video" // M2：视频发帖
)

// 大小限制（字节）。
const (
	MaxImageSize = 10 << 20  // 10MB
	MaxAudioSize = 20 << 20  // 20MB
	MaxVideoSize = 200 << 20 // 200MB（设计稿示例 18.4MB 视频，200MB 覆盖常见场景）
)

// 允许的扩展名 → 类型映射（webm 双格式由 detectType 按 MIME 区分，不在此表）。
var extTypeMap = map[string]string{
	".jpg": TypeImage, ".jpeg": TypeImage, ".png": TypeImage,
	".gif": TypeImage, ".webp": TypeImage,
	".mp3": TypeAudio, ".m4a": TypeAudio, ".wav": TypeAudio,
	".mp4": TypeVideo, ".mov": TypeVideo, // M2：视频发帖
}

// ErrUnsupportedType 不支持的文件类型。
var ErrUnsupportedType = errors.New("不支持的文件类型")

// ErrFileTooLarge 文件超出大小限制。
var ErrFileTooLarge = errors.New("文件超出大小限制")

// Store 媒体存储（连接器类，持有根目录）。
type Store struct {
	rootDir string // 存储根目录（data/media）
}

// NewStore 创建媒体存储（自动创建根目录）。
func NewStore(rootDir string) (*Store, error) {
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建媒体目录失败：%w", err)
	}
	return &Store{rootDir: rootDir}, nil
}

// detectType 根据扩展名识别媒体类型（webm 按 MIME 区分音频/视频）。
func detectType(filename string, mime string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	// webm 双格式：audio/webm = 录音（MediaRecorder），video/webm = 视频
	if ext == ".webm" {
		if strings.HasPrefix(mime, "video/") {
			return TypeVideo, nil
		}
		return TypeAudio, nil
	}
	mediaType, ok := extTypeMap[ext]
	if !ok {
		return "", ErrUnsupportedType
	}
	return mediaType, nil
}

// maxSizeFor 返回指定类型的最大允许大小。
func maxSizeFor(mediaType string) int64 {
	switch mediaType {
	case TypeAudio:
		return MaxAudioSize
	case TypeVideo:
		return MaxVideoSize
	default:
		return MaxImageSize
	}
}

// randomName 生成随机文件名（16 位 hex + 扩展名）。
func randomName(ext string) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("随机文件名生成失败：%w", err)
	}
	return hex.EncodeToString(buf) + ext, nil
}

// StorageResult 保存结果（供 repository 写入 media_assets）。
type StorageResult struct {
	Type      string // 媒体类型
	StorageKey string // 相对存储键（data/media 下路径，如 202608/xxx.jpg）
	URL       string // 访问地址（/media/xxx.jpg 静态路径）
	MimeType  string // MIME 类型
	SizeBytes int64  // 文件大小
}

// Save 保存上传文件到本地磁盘。
// 参数：header 上传文件头；reader 文件内容。
// 返回：保存结果；类型不支持/超限等错误。
func (s *Store) Save(header *multipart.FileHeader, reader io.Reader) (StorageResult, error) {
	// ---------- 类型与大小校验 ----------
	mediaType, err := detectType(header.Filename, header.Header.Get("Content-Type"))
	if err != nil {
		return StorageResult{}, err
	}
	if header.Size > maxSizeFor(mediaType) {
		return StorageResult{}, ErrFileTooLarge
	}

	// ---------- 生成存储路径（年月子目录 + 随机文件名） ----------
	monthDir := time.Now().Format("200601")
	dir := filepath.Join(s.rootDir, monthDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return StorageResult{}, fmt.Errorf("创建媒体子目录失败：%w", err)
	}
	name, err := randomName(filepath.Ext(header.Filename))
	if err != nil {
		return StorageResult{}, err
	}

	// ---------- 写入文件 ----------
	storageKey := filepath.ToSlash(filepath.Join(monthDir, name))
	fullPath := filepath.Join(dir, name)
	out, err := os.Create(fullPath)
	if err != nil {
		return StorageResult{}, fmt.Errorf("创建媒体文件失败：%w", err)
	}
	defer out.Close()

	size, err := io.Copy(out, reader)
	if err != nil {
		// 写入失败时清理半成品文件
		_ = os.Remove(fullPath)
		return StorageResult{}, fmt.Errorf("写入媒体文件失败：%w", err)
	}

	// MIME 类型（按扩展名推断，通用映射）
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = mimeByExt(filepath.Ext(header.Filename))
	}

	return StorageResult{
		Type:       mediaType,
		StorageKey: storageKey,
		URL:        "/media/" + storageKey,
		MimeType:   mimeType,
		SizeBytes:  size,
	}, nil
}

// RootDir 返回存储根目录（静态服务挂载用）。
func (s *Store) RootDir() string {
	return s.rootDir
}

// Remove 删除本地文件（后台媒体库删除；文件不存在视为成功）。
// 参数：storageKey 相对存储键（Save 返回的 StorageKey，如 202608/xxx.jpg）。
// 说明：校验路径不越界（仅允许 rootDir 内相对路径，防路径穿越）。
func (s *Store) Remove(storageKey string) error {
	if storageKey == "" {
		return nil
	}
	path := filepath.Join(s.rootDir, storageKey)
	// 防路径穿越：拼接后必须仍在根目录内
	if !strings.HasPrefix(path, s.rootDir+string(os.PathSeparator)) {
		return fmt.Errorf("非法存储键：%s", storageKey)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// mimeByExt 按扩展名推断 MIME（Content-Type 缺失时兜底）。
func mimeByExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	case ".webm":
		return "video/webm"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	default:
		return "application/octet-stream"
	}
}
