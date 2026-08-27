// internal/ai/stream.go
// OpenAI 兼容流式对话（SSE）解析（M4 扩展）。
//
// 说明：Chat Completions 流式返回 data: 前缀的 SSE 事件，逐事件携带 choices[0].delta.content
//       增量文本；收到 data: [DONE] 或连接结束即视为流结束。
package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// chatStreamChunk SSE 流式事件体（仅解析 delta 文本）。
type chatStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"` // 增量文本
		} `json:"delta"`
	} `json:"choices"`
}

// chatStream 流式结果实现（逐行解析 SSE data 事件）。
type chatStream struct {
	body    io.ReadCloser // HTTP 响应体（Close 释放连接）
	scanner *bufio.Scanner // 逐行读取
}

// Recv 读取下一个增量文本（无更多内容时返回 io.EOF）。
func (s *chatStream) Recv() (StreamChunk, error) {
	for s.scanner.Scan() {
		line := strings.TrimSpace(s.scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue // 跳过空行/事件注释（如 "event: ..."）
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return StreamChunk{}, io.EOF
		}
		var parsed chatStreamChunk
		if err := json.Unmarshal([]byte(data), &parsed); err != nil {
			continue // 忽略无法解析的事件（容错：不中断流）
		}
		if len(parsed.Choices) > 0 && parsed.Choices[0].Delta.Content != "" {
			return StreamChunk{Text: parsed.Choices[0].Delta.Content}, nil
		}
	}
	// 连接正常结束或读取错误：均视为流结束
	return StreamChunk{}, io.EOF
}

// Close 释放底层 HTTP 响应体。
func (s *chatStream) Close() error {
	return s.body.Close()
}

// ChatStream 执行一次流式对话（统一契约 ChatRequest；SSE 逐增量返回）。
func (c *Client) ChatStream(ctx context.Context, req ChatRequest) (ChatStream, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}
	temperature := req.Temperature
	if temperature <= 0 {
		temperature = defaultTemperature
	}
	body := chatRequest{
		Model:       model,
		Messages:    toWireMessages(req.Messages),
		MaxTokens:   req.MaxTokens,
		Temperature: temperature,
		Stream:      true,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("构造请求体失败: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	// 明确接受流式响应（部分供应商据此调整行为）
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("AI 服务请求失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		return nil, httpError(resp.StatusCode, raw)
	}
	return &chatStream{body: resp.Body, scanner: bufio.NewScanner(resp.Body)}, nil
}
