// internal/ai/provider.go
// AI 供应商实现（M4 扩展）：OpenAICompatProvider 实现 Provider 统一接口。
//
// 说明：当前所有供应商（deepseek/qwen/kimi/glm/openai）均为 OpenAI 兼容协议，
//       通过同一实现覆盖；未来新增非兼容供应商时，另写实现并注册即可。
package ai

import (
	"context"
	"time"
)

// OpenAICompatProvider OpenAI 兼容供应商（实现 Provider 接口，包装 Client）。
type OpenAICompatProvider struct {
	name   string  // 供应商名（路由键）
	client *Client // OpenAI 兼容客户端
}

// NewOpenAICompatProvider 创建 OpenAI 兼容供应商。
// 参数：name 供应商名；baseURL 接口地址；apiKey 明文 Key；defaultModel 默认模型；timeout 超时。
func NewOpenAICompatProvider(name string, baseURL string, apiKey string, defaultModel string, timeout time.Duration) *OpenAICompatProvider {
	return &OpenAICompatProvider{name: name, client: NewClient(baseURL, apiKey, defaultModel, timeout)}
}

// Name 供应商名。
func (p *OpenAICompatProvider) Name() string {
	return p.name
}

// Chat 非流式对话（委托客户端）。
func (p *OpenAICompatProvider) Chat(ctx context.Context, req ChatRequest) (*Result, error) {
	return p.client.ChatMessages(ctx, req)
}

// ChatStream 流式对话（委托客户端）。
func (p *OpenAICompatProvider) ChatStream(ctx context.Context, req ChatRequest) (ChatStream, error) {
	return p.client.ChatStream(ctx, req)
}

// Embedding 向量嵌入（委托客户端）。
func (p *OpenAICompatProvider) Embedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResult, error) {
	return p.client.Embedding(ctx, req)
}
