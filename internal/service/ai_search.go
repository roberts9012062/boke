// internal/service/ai_search.go
// SearXNG 联网搜索（AI 可选增强）：聚合搜索引擎代理 + AI 对话联网注入。
//
// 说明：SearXNG 为开源自托管元搜索引擎（聚合 Google/Bing/DuckDuckGo 等），
//       本站在 Docker 编排内内置其实例（http://searxng:8080，不对外暴露）。
//       地址在后台「AI 设置-联网搜索」配置（settings 表 ai_search_url），
//       可选项：未配置时 AI 功能完全正常，仅无联网能力。
//       能力出口：① 后台/开放接口直接搜索（浏览器插件可凭 X-Api-Key 调用）
//                 ② AI 对话联网回答（web_search=true 时先检索后作答，附来源）。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/roberts9012062/boke/internal/ai"
	"github.com/roberts9012062/boke/pkg/errs"
)

// settingKeySearchURL SearXNG 地址的设置键（空 = 未配置，联网功能停用）。
const settingKeySearchURL = "ai_search_url"

// 联网搜索约束。
const (
	searchTimeout  = 15 * time.Second // 检索超时
	searchMaxLimit = 10               // 单次返回条数上限（防滥用）
	searchDefault  = 5                // 默认条数
)

// WebSearchResult 单条搜索结果。
type WebSearchResult struct {
	Title   string `json:"title"`   // 标题
	URL     string `json:"url"`     // 原文地址
	Snippet string `json:"snippet"` // 摘要片段
}

// SearchConfig 联网搜索配置。
type SearchConfig struct {
	URL string `json:"url"` // SearXNG 地址（http://searxng:8080；空=未启用）
}

// GetSearchConfig 读取联网搜索配置（未配置返回空 URL）。
func (s *AiService) GetSearchConfig(ctx context.Context) (*SearchConfig, error) {
	if s.settings == nil {
		return &SearchConfig{}, nil
	}
	raw, ok, err := s.settings.Get(ctx, settingKeySearchURL)
	if err != nil {
		return nil, err
	}
	if !ok {
		return &SearchConfig{}, nil
	}
	return &SearchConfig{URL: strings.TrimSpace(raw)}, nil
}

// SaveSearchConfig 保存联网搜索配置（空串=停用）。
func (s *AiService) SaveSearchConfig(ctx context.Context, cfg SearchConfig) error {
	if s.settings == nil {
		return errs.New(errs.CodeStateConflict, "设置存储未配置")
	}
	value := strings.TrimSpace(cfg.URL)
	if value != "" && !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return errs.New(errs.CodeBadRequest, "地址需以 http:// 或 https:// 开头")
	}
	return s.settings.SetMany(ctx, map[string]string{settingKeySearchURL: value})
}

// searxngResponse SearXNG JSON API 响应（仅解析用到的字段）。
type searxngResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}

// SearchWeb 执行联网搜索（SearXNG JSON API）。
// 参数：query 关键词；limit 返回条数（缺省 5，上限 10）。
// 未配置 SearXNG 时返回明确提示（调用方决定是否阻断）。
func (s *AiService) SearchWeb(ctx context.Context, query string, limit int) ([]WebSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errs.New(errs.CodeBadRequest, "搜索关键词不能为空")
	}
	cfg, err := s.GetSearchConfig(ctx)
	if err != nil {
		return nil, err
	}
	if cfg.URL == "" {
		return nil, errs.New(errs.CodeStateConflict, "联网搜索未配置（后台 AI 设置-联网搜索）")
	}
	if limit <= 0 {
		limit = searchDefault
	}
	if limit > searchMaxLimit {
		limit = searchMaxLimit
	}

	api := strings.TrimRight(cfg.URL, "/") + "/search?q=" + url.QueryEscape(query) + "&format=json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, "搜索请求构造失败："+err.Error())
	}
	client := &http.Client{Timeout: searchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, "搜索实例不可达（检查 SearXNG 是否运行）："+err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errs.New(errs.CodeUpstream, fmt.Sprintf("搜索实例返回 HTTP %d", resp.StatusCode))
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, "读取搜索结果失败："+err.Error())
	}
	var parsed searxngResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, errs.New(errs.CodeUpstream, "搜索结果解析失败（实例需开启 JSON format）")
	}
	results := make([]WebSearchResult, 0, len(parsed.Results))
	for _, item := range parsed.Results {
		if len(results) >= limit {
			break
		}
		if item.Title == "" && item.URL == "" {
			continue
		}
		results = append(results, WebSearchResult{Title: item.Title, URL: item.URL, Snippet: item.Content})
	}
	if len(results) == 0 {
		return nil, errs.New(errs.CodeUpstream, "搜索无结果（换个关键词或检查 SearXNG 聚合源）")
	}
	return results, nil
}

// ChatWithSearchResult 联网对话结果（回答 + 引用来源）。
type ChatWithSearchResult struct {
	Reply         string            `json:"reply"`          // 模型回答
	SearchResults []WebSearchResult `json:"search_results"` // 本次引用的搜索结果（透明可溯源）
}

// ChatWithSearch 联网对话：先按最后一条用户消息检索，把结果注入上下文后作答。
// 未配置搜索或检索失败时回退普通对话（联网是增强而非依赖，保证可用性）。
func (s *AiService) ChatWithSearch(ctx context.Context, model string, messages []ai.Message, maxTokens int) (*ChatWithSearchResult, error) {
	// 取最后一条用户消息作为检索词
	query := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			query = messages[i].Content
			break
		}
	}
	chatMessages := messages
	var cited []WebSearchResult
	// 联网场景注入检索上下文，推理模型思考段更长——输出额度不足会截空正文
	if maxTokens <= 0 {
		maxTokens = 2000
	}
	if query != "" {
		if results, err := s.SearchWeb(ctx, query, searchDefault); err == nil {
			cited = results
			// 注入检索上下文（置于消息列表最前，不破坏用户对话历史）
			var ctxBuilder strings.Builder
			ctxBuilder.WriteString("以下是关于用户问题的实时网络搜索结果（供参考，回答时可引用，注明来源）：\n")
			for i, item := range results {
				ctxBuilder.WriteString(fmt.Sprintf("%d. %s\n   %s\n   来源：%s\n", i+1, item.Title, item.Snippet, item.URL))
			}
			chatMessages = append([]ai.Message{{Role: "system", Content: ctxBuilder.String()}}, messages...)
		}
		// 检索失败静默回退普通对话（联网不可用不该让对话失败）
	}
	reply, err := s.GenerateChat(ctx, model, chatMessages, maxTokens)
	if err != nil {
		return nil, err
	}
	return &ChatWithSearchResult{Reply: reply, SearchResults: cited}, nil
}
