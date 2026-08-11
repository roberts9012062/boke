// internal/ai/client.go
// OpenAI 兼容 Chat Completions 客户端（M4）：零第三方依赖，net/http 直连。
//
// 覆盖供应商：deepseek / qwen / kimi / glm / openai（均为 OpenAI 兼容接口）。
// 调用形态：POST {base_url}/chat/completions，Bearer 认证，
//           响应取 choices[0].message.content 与 usage（token 计数，供 ai_usage 落库）。
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// chatRequest Chat Completions 请求体（仅封装本包用到的字段）。
type chatRequest struct {
	Model       string    `json:"model"`        // 模型名
	Messages    []message `json:"messages"`     // 消息列表（system 提示词 + user 输入）
	MaxTokens   int       `json:"max_tokens"`   // 最大输出 token
	Temperature float64   `json:"temperature"`  // 采样温度（0.2 偏确定性，适合结构化输出）
	Stream      bool      `json:"stream"`       // 非流式
}

// message 对话消息（OpenAI 兼容格式）。
type message struct {
	Role    string `json:"role"`    // system / user
	Content string `json:"content"` // 消息内容
}

// chatResponse Chat Completions 响应体（仅解析用到的字段）。
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"` // 模型输出文本
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`     // 输入 token
		CompletionTokens int64 `json:"completion_tokens"` // 输出 token
	} `json:"usage"`
}

// Result AI 调用结果（文本 + token 用量）。
type Result struct {
	Text      string // 模型输出文本
	InTokens  int64  // 输入 token 数（ai_usage 落库）
	OutTokens int64  // 输出 token 数（ai_usage 落库）
}

// Client OpenAI 兼容客户端（连接器类，OOP 仅用于外部系统接口）。
type Client struct {
	httpClient *http.Client // HTTP 客户端（共享连接池）
	baseURL    string       // 供应商接口地址（不含 /chat/completions）
	apiKey     string       // 解密后的 API Key
	model      string       // 模型名
	timeout    time.Duration
}

// NewClient 创建 OpenAI 兼容客户端。
// 参数：baseURL 供应商接口地址；apiKey 明文 Key；model 模型名；timeout 请求超时。
func NewClient(baseURL string, apiKey string, model string, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		timeout:    timeout,
	}
}

// Chat 执行一次对话补全（system=提示词 + user=输入）。
// 返回：输出文本与 token 用量；上游错误（HTTP 非 2xx/网络失败）返回带上下文错误。
func (c *Client) Chat(ctx context.Context, prompt string, input string, maxTokens int) (*Result, error) {
	body := chatRequest{
		Model: c.model,
		Messages: []message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: input},
		},
		MaxTokens:   maxTokens,
		Temperature: 0.2,
		Stream:      false,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("构造请求体失败: %w", err)
	}

	// 请求（带超时上下文）
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AI 服务请求失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 响应上限 4MB
	if err != nil {
		return nil, fmt.Errorf("读取 AI 响应失败: %w", err)
	}
	// 非 2xx：透出上游错误信息（便于后台排查 API Key / 配额问题）
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return nil, fmt.Errorf("AI 服务返回 HTTP %d: %s", resp.StatusCode, msg)
	}

	// 解析响应（choices 为空视为上游异常）
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("解析 AI 响应失败: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, errors.New("AI 服务未返回内容（choices 为空）")
	}
	return &Result{
		Text:      strings.TrimSpace(parsed.Choices[0].Message.Content),
		InTokens:  parsed.Usage.PromptTokens,
		OutTokens: parsed.Usage.CompletionTokens,
	}, nil
}
