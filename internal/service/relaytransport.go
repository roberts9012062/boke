// 中继站出站 HTTP 传输层（从 relay.go 拆出）：
// 统一 GET/POST 封装（Bearer key 认证、JSON 编解码、协议错误码透传）与响应码判定。
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// relayCodeOK 判定协议响应包的业务码是否成功（JSON any 数字是 float64(0)，与 int 比较恒不等）。
func relayCodeOK(code any) bool {
	if n, ok := code.(float64); ok {
		return n == 0
	}
	return code == nil
}

// getJSON 统一出站 GET：Bearer key 认证、JSON 解码、协议错误码透传（轮询用，限读 8MB）。
func (s *RelayService) getJSON(ctx context.Context, url string, siteKey string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+siteKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("中继站不可达: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	var envelope struct {
		Code    any             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("中继站响应异常（HTTP %d）", resp.StatusCode)
	}
	if resp.StatusCode != 200 || !relayCodeOK(envelope.Code) {
		return fmt.Errorf("中继站错误 [%v] %s", envelope.Code, envelope.Message)
	}
	if out != nil && len(envelope.Data) > 0 {
		return json.Unmarshal(envelope.Data, out)
	}
	return nil
}

// postJSON 统一出站 POST：Bearer key 认证、JSON 编解码、协议错误码透传。
func (s *RelayService) postJSON(ctx context.Context, url string, siteKey string, reqBody any, out any) error {
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+siteKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("中继站不可达: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var envelope struct {
		Code    any             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("中继站响应异常（HTTP %d）", resp.StatusCode)
	}
	if resp.StatusCode != 200 || !relayCodeOK(envelope.Code) {
		return fmt.Errorf("中继站错误 [%v] %s", envelope.Code, envelope.Message)
	}
	if out != nil && len(envelope.Data) > 0 {
		return json.Unmarshal(envelope.Data, out)
	}
	return nil
}
