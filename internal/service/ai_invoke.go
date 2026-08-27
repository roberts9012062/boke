// internal/service/ai_invoke.go
// AI 统一推理核心（M4 扩展）：供应商解析 + 统一 Provider 调用 + 用量/费用落库。
//
// 设计：消除「内置场景 runTask」与「插件 Generate」两条重复的
//       「找供应商→解密→建客户端→调用→落库」路径，统一收敛为
//       resolveProviderByModel / chatProvider / chatStreamProvider / embedProvider。
package service

import (
	"context"
	"time"

	"github.com/roberts9012062/boke/internal/ai"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// resolveProviderByModel 按模型名精确匹配已启用供应商（通用接口/插件用）。
// 返回：命中的供应商；未命中返回业务错误。
func (s *AiService) resolveProviderByModel(ctx context.Context, model string) (*repository.AiProvider, error) {
	providers, err := s.providers.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched *repository.AiProvider
	for i := range providers {
		if !providers[i].Enabled {
			continue
		}
		for _, m := range providers[i].Models {
			if m == model {
				matched = &providers[i]
				break
			}
		}
		if matched != nil {
			break
		}
	}
	if matched == nil {
		return nil, errs.New(errs.CodeBadRequest, "模型「"+model+"」未在已启用供应商中找到")
	}
	return matched, nil
}

// buildProvider 由供应商实体构造统一 Provider（解密 Key + 兜底模型）。
// 返回：构造好的 Provider + 实际使用的模型名。
func (s *AiService) buildProvider(provider *repository.AiProvider, reqModel string) (ai.Provider, string, error) {
	apiKey, err := decryptAPIKey(provider.APIKeyEncrypted, s.keySecret)
	if err != nil {
		return nil, "", errs.New(errs.CodeUpstream, "API Key 解密失败，请重新保存")
	}
	if apiKey == "" {
		return nil, "", errs.New(errs.CodeUpstream, "供应商「"+provider.Name+"」未配置 API Key，请先在 AI 设置中填写")
	}
	model := reqModel
	if model == "" {
		model = firstModel(provider.Models)
	}
	if model == "" {
		return nil, "", errs.New(errs.CodeBadRequest, "供应商「"+provider.Name+"」未配置模型")
	}
	p := ai.NewOpenAICompatProvider(provider.Name, provider.BaseURL, apiKey, model, aiRequestTimeout*time.Second)
	return p, model, nil
}

// chatProvider 统一非流式对话：构造 Provider → 调用 → 用量/费用落库。
// 返回：模型输出与 token 用量（已落 ai_usage，含费用折算）。
func (s *AiService) chatProvider(ctx context.Context, provider *repository.AiProvider, taskName string, req ai.ChatRequest) (*ai.Result, error) {
	p, model, err := s.buildProvider(provider, req.Model)
	if err != nil {
		return nil, err
	}
	req.Model = model
	result, err := p.Chat(ctx, req)
	// 瞬时故障自动重试一次（上游网关偶发超时/断连——长输入推理请求
	// 撞 MiniMax 等网关 30s 限时会整单 500，重试可显著自愈；重试仍失败才报错）
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, errs.New(errs.CodeUpstream, "AI 服务不可用："+err.Error())
		case <-time.After(time.Second):
		}
		result, err = p.Chat(ctx, req)
	}
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, "AI 服务不可用："+err.Error())
	}
	// 用量 + 费用落库（失败静默：统计是观测数据，不影响场景结果）
	_ = s.usage.Record(ctx, repository.AiUsage{
		TaskName: taskName, ProviderID: provider.ID,
		TokensIn: result.InTokens, TokensOut: result.OutTokens,
		Cost: calcCost(provider, result.InTokens, result.OutTokens),
	})
	return result, nil
}

// chatStreamProvider 统一流式对话：构造 Provider → 返回流。
// 说明：流式长连接暂不落 ai_usage（token 用量需上游流式 usage 块，后续增强）。
func (s *AiService) chatStreamProvider(ctx context.Context, provider *repository.AiProvider, req ai.ChatRequest) (ai.ChatStream, error) {
	p, model, err := s.buildProvider(provider, req.Model)
	if err != nil {
		return nil, err
	}
	req.Model = model
	stream, err := p.ChatStream(ctx, req)
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, "AI 服务不可用："+err.Error())
	}
	return stream, nil
}

// embedProvider 统一向量嵌入：构造 Provider → 调用（不落 ai_usage，独立观测维度）。
func (s *AiService) embedProvider(ctx context.Context, provider *repository.AiProvider, req ai.EmbeddingRequest) (*ai.EmbeddingResult, error) {
	p, model, err := s.buildProvider(provider, req.Model)
	if err != nil {
		return nil, err
	}
	req.Model = model
	result, err := p.Embedding(ctx, req)
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, "AI 服务不可用："+err.Error())
	}
	return result, nil
}

// calcCost 按供应商单价折算费用（元）。
// 公式：tokens_in/1e6 * 输入单价 + tokens_out/1e6 * 输出单价；无单价时记 0。
func calcCost(provider *repository.AiProvider, tokensIn int64, tokensOut int64) float64 {
	return float64(tokensIn)/1e6*provider.PriceInput + float64(tokensOut)/1e6*provider.PriceOutput
}
