// internal/ai/types.go
// AI 内核共享类型（M4 扩展）：对话/流式/嵌入的请求与结果结构。
//
// 说明：对齐 docs/architecture.md 第 7.2 节「统一接口」——ChatRequest / ChatStream /
//       EmbeddingRequest 为供应商无关的统一契约，供应商差异收敛为 base_url + api_key + 模型映射。
package ai

import "context"

// Result AI 调用结果（文本 + token 用量）。
type Result struct {
	Text      string // 模型输出文本
	InTokens  int64  // 输入 token 数（ai_usage 落库）
	OutTokens int64  // 输出 token 数（ai_usage 落库）
}

// Message 对话消息（OpenAI 兼容格式）。
type Message struct {
	Role    string `json:"role"`    // system / user / assistant
	Content string `json:"content"` // 消息内容
	// ImageURL 可选图片输入（多模态识别用，MiniMax-M3 等视觉模型支持）：
	// 非空时本条消息按 [{type:text},{type:image_url}] 数组形式发出（OpenAI 兼容多模态协议）；
	// 序列化不直接参与（由客户端组装时转换），避免对纯文本供应商发出冗余字段。
	ImageURL string `json:"-"`
}

// ChatRequest 对话补全请求（统一契约，供应商无关）。
type ChatRequest struct {
	Model       string    // 模型名（空 = 供应商默认模型）
	Messages    []Message // 消息列表
	MaxTokens   int       // 最大输出 token
	Temperature float64   // 采样温度（0 = 客户端默认 0.2）
	Stream      bool      // 是否流式
}

// StreamChunk 流式增量（一次 SSE 事件的文本片段）。
type StreamChunk struct {
	Text string // 增量文本
}

// ChatStream 流式对话结果（逐 chunk 消费）。
// 约定：Recv 返回 io.EOF 表示流结束；Close 释放底层连接。
type ChatStream interface {
	Recv() (StreamChunk, error)
	Close() error
}

// EmbeddingRequest 向量嵌入请求（统一契约）。
type EmbeddingRequest struct {
	Model string // 模型名（空 = 供应商默认模型）
	Text  string // 待嵌入文本
}

// EmbeddingResult 向量嵌入结果。
type EmbeddingResult struct {
	Vector []float64 // 嵌入向量
}

// Provider AI 供应商统一接口（架构蓝图第 7.2 节）。
// 多态抽象：当前仅 OpenAICompatProvider 实现；未来新增非 OpenAI 兼容供应商时
// 只需新增实现并注册，调用方无需感知差异。
type Provider interface {
	Name() string                                                          // 供应商名（注册/路由键）
	Chat(ctx context.Context, req ChatRequest) (*Result, error)           // 非流式对话
	ChatStream(ctx context.Context, req ChatRequest) (ChatStream, error)  // 流式对话（SSE）
	Embedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResult, error) // 向量嵌入
}
