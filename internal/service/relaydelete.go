// 中继站删帖联动下架（从 relay.go 拆出）：删帖后异步调中继站 DELETE 同步下架大世界内容。
// 出站方向 + 失败仅日志（本地删除已完成，中继内容随 TTL 兜底清理）。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// DeleteOnRelayAsync 删帖下架：异步调中继站 DELETE /contents/:post:{postID}（己站 origin 形式，
// 中继站按请求方站点校验归属）；失败仅日志（本地删除已完成，中继内容随 TTL 兜底清理）。
func (s *RelayService) DeleteOnRelayAsync(postID int64) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.deleteOnRelay(ctx, postID); err != nil {
			s.log.Warn("中继站内容下架失败", zap.Int64("post_id", postID), zap.Error(err))
		}
	}()
}

// deleteOnRelay 删除单条己站内容（协议 §4.3 的 origin 简写形式）。
func (s *RelayService) deleteOnRelay(ctx context.Context, postID int64) error {
	rc, err := s.relay.Config(ctx)
	if err != nil {
		return err
	}
	if !rc.Enabled || rc.URL == "" || rc.SiteKey == "" {
		return nil // 未启用：静默跳过
	}
	contentID := fmt.Sprintf(":post:%d", postID) // 空站点段：由中继站按请求方填充
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, rc.URL+"/api/v1/contents/"+contentID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+rc.SiteKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("中继站不可达: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var envelope struct {
		Code    any    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("中继站响应异常（HTTP %d）", resp.StatusCode)
	}
	if resp.StatusCode == 404 || (resp.StatusCode == 400 && envelope.Code == "CONTENT_NOT_FOUND") {
		return nil // 中继侧本就不存在（未推送过/已过期）：视为成功
	}
	if resp.StatusCode != 200 || !relayCodeOK(envelope.Code) {
		return fmt.Errorf("中继站错误 [%v] %s", envelope.Code, envelope.Message)
	}
	s.log.Info("中继站内容已下架", zap.Int64("post_id", postID))
	return nil
}
