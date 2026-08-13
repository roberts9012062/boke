// internal/ai/manager.go
// AI 供应商管理器（M4 扩展）：Provider 注册表 + 统一推理入口。
//
// 说明：对齐 docs/architecture.md 第 7.2 节「Manager」——providers 注册表，
//       提供按供应商名路由的统一 Chat / ChatStream / Embedding 调用。
//       路由策略（优先级/任务绑定）在 service 层决定，Manager 只做「名 → Provider」映射与调用。
package ai

import (
	"context"
	"errors"
)

// ErrProviderNotFound 指定供应商未注册。
var ErrProviderNotFound = errors.New("AI 供应商未注册")

// Manager 供应商管理器（连接器类，OOP 仅用于外部系统接口聚合）。
type Manager struct {
	providers map[string]Provider // 供应商名 → Provider
}

// NewManager 创建供应商管理器。
func NewManager() *Manager {
	return &Manager{providers: make(map[string]Provider)}
}

// Register 注册供应商（同名覆盖，便于按最新配置重建）。
func (m *Manager) Register(p Provider) {
	m.providers[p.Name()] = p
}

// Get 按名获取供应商（不存在返回 false）。
func (m *Manager) Get(name string) (Provider, bool) {
	p, ok := m.providers[name]
	return p, ok
}

// Chat 统一非流式对话入口（按供应商名路由）。
func (m *Manager) Chat(ctx context.Context, name string, req ChatRequest) (*Result, error) {
	p, ok := m.Get(name)
	if !ok {
		return nil, ErrProviderNotFound
	}
	return p.Chat(ctx, req)
}

// ChatStream 统一流式对话入口（按供应商名路由）。
func (m *Manager) ChatStream(ctx context.Context, name string, req ChatRequest) (ChatStream, error) {
	p, ok := m.Get(name)
	if !ok {
		return nil, ErrProviderNotFound
	}
	return p.ChatStream(ctx, req)
}

// Embedding 统一向量嵌入入口（按供应商名路由）。
func (m *Manager) Embedding(ctx context.Context, name string, req EmbeddingRequest) (*EmbeddingResult, error) {
	p, ok := m.Get(name)
	if !ok {
		return nil, ErrProviderNotFound
	}
	return p.Embedding(ctx, req)
}
