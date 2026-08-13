// internal/ai/client.go
// OpenAI 兼容 Chat Completions / Embeddings 客户端（M4）：零第三方依赖，net/http 直连。
//
// 覆盖供应商：deepseek / qwen / kimi / glm / openai（均为 OpenAI 兼容接口）。
// 调用形态：
//   - 对话 POST {base_url}/chat/completions，Bearer 认证，取 choices[0].message.content 与 usage
//   - 嵌入 POST {base_url}/embeddings，取 data[0].embedding
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

// defaultTemperature 采样温度默认值（偏确定性，适合结构化输出）。
const defaultTemperature = 0.2

// chatRequest Chat Completions 请求体（仅封装本包用到的字段）。
type chatRequest struct {
	Model       string    `json:"model"`       // 模型名
	Messages    []Message `json:"messages"`    // 消息列表
	MaxTokens   int       `json:"max_tokens"`  // 最大输出 token
	Temperature float64   `json:"temperature"` // 采样温度
	Stream      bool      `json:"stream"`      // 是否流式
}

// chatResponse Chat Completions 响应体（非流式，仅解析用到的字段）。
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

// embeddingRequest Embeddings 请求体。
type embeddingRequest struct {
	Model string `json:"model"` // 模型名
	Input string `json:"input"` // 待嵌入文本
}

// embeddingResponse Embeddings 响应体。
type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"` // 向量
	} `json:"data"`
}

// modelsResponse 模型列表响应体（OpenAI 兼容 /models）。
type modelsResponse struct {
	Data []struct {
		ID string `json:"id"` // 模型名
	} `json:"data"`
}

// Client OpenAI 兼容客户端（连接器类，OOP 仅用于外部系统接口）。
type Client struct {
	httpClient *http.Client // HTTP 客户端（共享连接池）
	baseURL    string       // 供应商接口地址（不含 /chat/completions）
	apiKey     string       // 解密后的 API Key
	model      string       // 默认模型名（ChatRequest.Model 为空时使用）
	timeout    time.Duration
}

// NewClient 创建 OpenAI 兼容客户端。
// 参数：baseURL 供应商接口地址；apiKey 明文 Key；model 默认模型；timeout 请求超时。
func NewClient(baseURL string, apiKey string, model string, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		timeout:    timeout,
	}
}

// Chat 执行一次对话补全（便捷入口：system=提示词 + user=输入）。
// 返回：输出文本与 token 用量；上游错误（HTTP 非 2xx/网络失败）返回带上下文错误。
func (c *Client) Chat(ctx context.Context, prompt string, input string, maxTokens int) (*Result, error) {
	return c.ChatMessages(ctx, ChatRequest{
		Model: c.model,
		Messages: []Message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: input},
		},
		MaxTokens: maxTokens,
	})
}

// ChatMessages 执行一次对话补全（统一契约 ChatRequest；model 空则用客户端默认模型）。
func (c *Client) ChatMessages(ctx context.Context, req ChatRequest) (*Result, error) {
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
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: temperature,
		Stream:      false,
	}
	raw, status, err := c.post(ctx, "/chat/completions", body)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, httpError(status, raw)
	}
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

// Embedding 执行一次向量嵌入（统一契约 EmbeddingRequest）。
func (c *Client) Embedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResult, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}
	body := embeddingRequest{Model: model, Input: req.Text}
	raw, status, err := c.post(ctx, "/embeddings", body)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, httpError(status, raw)
	}
	var parsed embeddingResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("解析嵌入响应失败: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, errors.New("AI 服务未返回嵌入向量（data 为空）")
	}
	return &EmbeddingResult{Vector: parsed.Data[0].Embedding}, nil
}

// Models 拉取供应商可用模型清单（GET {base_url}/models，OpenAI 兼容）。
// 返回：模型名列表（去空）；上游错误透出带上下文错误。
func (c *Client) Models(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpError(resp.StatusCode, raw)
	}
	var parsed modelsResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("解析模型列表响应失败: %w", err)
	}
	// 去空 + 去重（部分供应商会返回重复或空 id）
	seen := make(map[string]bool, len(parsed.Data))
	models := make([]string, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		models = append(models, id)
	}
	if len(models) == 0 {
		return nil, errors.New("AI 服务未返回模型列表（data 为空）")
	}
	return models, nil
}
// post 发送 POST 请求并读取响应体（通用：对话/嵌入/测试共用）。
// 返回：响应原始字节 + HTTP 状态码；网络失败返回带上下文错误。
func (c *Client) post(ctx context.Context, path string, payload any) ([]byte, int, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("构造请求体失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("AI 服务请求失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 响应上限 4MB
	if err != nil {
		return nil, 0, fmt.Errorf("读取 AI 响应失败: %w", err)
	}
	return raw, resp.StatusCode, nil
}

// httpError 构造上游 HTTP 错误（截断过长响应体，便于后台排查 API Key / 配额问题）。
func httpError(status int, raw []byte) error {
	msg := strings.TrimSpace(string(raw))
	if len(msg) > 300 {
		msg = msg[:300]
	}
	return fmt.Errorf("AI 服务返回 HTTP %d: %s", status, msg)
}
