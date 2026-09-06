// 中继站媒体上传（bridged 模式）：读取本地媒体文件 → 超 1MB 时 JPEG 重编码压缩 → multipart 上传。
// 中继站只做上限校验不做转码（协议 §4.4），压缩责任在本站。
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/repository"
)

// relayMaxImageBytes 中继站出厂单张上限（1MB）；实际以握手 quota 为准，压缩目标取更小值。
const relayMaxImageBytes = 1048576

// relayJPEGQualities 压缩质量递降序列（仍超限则放弃该图并记日志）。
var relayJPEGQualities = []int{85, 75, 65, 55, 45}

// uploadMediaToRelay bridged 模式上传图片：返回中继站媒体 URL。
func (s *RelayService) uploadMediaToRelay(ctx context.Context, rc model.RelayConfig, m repository.MediaAsset) (string, error) {
	filePath := filepath.Join(s.cfg.DataDir, "media", m.StorageKey)
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("读取本地媒体失败: %w", err)
	}
	if int64(len(raw)) > relayMaxImageBytes {
		compressed, compErr := compressJPEG(raw, relayMaxImageBytes)
		if compErr != nil {
			return "", fmt.Errorf("图片超过 1MB 且压缩失败: %w", compErr)
		}
		raw = compressed
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", err
	}
	if _, err := part.Write(raw); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rc.URL+"/api/v1/media", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+rc.SiteKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("中继站不可达: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var envelope struct {
		Code    any    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			MediaURL string `json:"media_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return "", fmt.Errorf("中继站响应异常（HTTP %d）", resp.StatusCode)
	}
	if resp.StatusCode != 200 || !relayCodeOK(envelope.Code) {
		return "", fmt.Errorf("中继站错误 [%v] %s", envelope.Code, envelope.Message)
	}
	return envelope.Data.MediaURL, nil
}

// compressJPEG 将图片字节重编码为 JPEG 并按质量递降直到不超过上限。
// 仅使用标准库（decode 支持 jpg/png/gif；webp 不支持时返回错误）。
func compressJPEG(raw []byte, maxBytes int64) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}
	for _, quality := range relayJPEGQualities {
		var out bytes.Buffer
		if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, err
		}
		if int64(out.Len()) <= maxBytes {
			return out.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("质量降至 %d 仍超过 %d 字节", relayJPEGQualities[len(relayJPEGQualities)-1], maxBytes)
}
