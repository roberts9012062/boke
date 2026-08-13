// internal/ai/ai_test.go
// AI 内核单元测试（M4）：供应商路由、API Key 加解密、OpenAI 兼容客户端（httptest mock）。
package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------- 路由 ----------

// TestRouteProvider 路由选择：按 enabled + priority 选择最优供应商。
func TestRouteProvider(t *testing.T) {
	// 全启用：选 priority 最小（1）
	providers := []ProviderCandidate{
		{ID: 1, Enabled: true, Priority: 3},
		{ID: 2, Enabled: true, Priority: 1},
		{ID: 3, Enabled: true, Priority: 2},
	}
	got, err := RouteProvider(providers)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if got.ID != 2 {
		t.Errorf("期望 ID=2（priority=1），实际 ID=%d", got.ID)
	}
}

// TestRouteProviderSkipsDisabled 路由：禁用供应商被跳过。
func TestRouteProviderSkipsDisabled(t *testing.T) {
	providers := []ProviderCandidate{
		{ID: 1, Enabled: false, Priority: 1}, // 禁用但优先级最高 → 跳过
		{ID: 2, Enabled: true, Priority: 5},
	}
	got, err := RouteProvider(providers)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if got.ID != 2 {
		t.Errorf("期望跳过禁用供应商选 ID=2，实际 ID=%d", got.ID)
	}
}

// TestRouteProviderEmpty 路由：无可用供应商返回 ErrNoProvider。
func TestRouteProviderEmpty(t *testing.T) {
	if _, err := RouteProvider(nil); !errors.Is(err, ErrNoProvider) {
		t.Errorf("期望 ErrNoProvider，实际 %v", err)
	}
	if _, err := RouteProvider([]ProviderCandidate{{ID: 1, Enabled: false, Priority: 1}}); !errors.Is(err, ErrNoProvider) {
		t.Errorf("期望 ErrNoProvider（全部禁用），实际 %v", err)
	}
}

// ---------- 加解密 ----------

// TestSecretRoundTrip 加解密往返一致。
func TestSecretRoundTrip(t *testing.T) {
	const keySecret = "test-key-secret"
	const plain = "sk-1234567890abcdef"

	enc, err := EncryptSecret(plain, keySecret)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if enc == plain {
		t.Error("密文不应等于明文")
	}
	dec, err := DecryptSecret(enc, keySecret)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if dec != plain {
		t.Errorf("往返不一致：期望 %q 实际 %q", plain, dec)
	}
}

// TestSecretWrongKey 密钥不匹配时解密失败（GCM 认证失败）。
func TestSecretWrongKey(t *testing.T) {
	enc, err := EncryptSecret("sk-secret", "key-a")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if _, err := DecryptSecret(enc, "key-b"); err == nil {
		t.Error("错误密钥解密应失败")
	}
}

// TestSecretBadCipher 非法密文解密失败。
func TestSecretBadCipher(t *testing.T) {
	if _, err := DecryptSecret("not-base64!!", "key"); err == nil {
		t.Error("非法 base64 应失败")
	}
}

// ---------- 客户端 ----------

// TestChatSuccess 正常调用：请求格式正确 + 解析内容与 token。
func TestChatSuccess(t *testing.T) {
	// mock 上游：校验请求体并返回标准 OpenAI 响应
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 校验路径与认证头
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("路径错误: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-mock" {
			t.Errorf("认证头错误: %s", got)
		}
		// 校验请求体关键字段
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("请求体解析失败: %v", err)
		}
		if req.Model != "deepseek-chat" || len(req.Messages) != 2 || req.Messages[0].Role != "system" {
			t.Errorf("请求体字段错误: %+v", req)
		}
		// 返回标准响应（含 usage）
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "你好，这是摘要。"}}],
			"usage": {"prompt_tokens": 120, "completion_tokens": 45}
		}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "sk-mock", "deepseek-chat", 30*time.Second)
	res, err := client.Chat(context.Background(), "系统提示", "用户输入", 800)
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if res.Text != "你好，这是摘要。" {
		t.Errorf("文本错误: %q", res.Text)
	}
	if res.InTokens != 120 || res.OutTokens != 45 {
		t.Errorf("token 统计错误: in=%d out=%d", res.InTokens, res.OutTokens)
	}
}

// TestChatHTTPError 上游非 2xx：透出错误信息。
func TestChatHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid API key"}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "sk-bad", "deepseek-chat", 10*time.Second)
	_, err := client.Chat(context.Background(), "p", "i", 100)
	if err == nil {
		t.Fatal("应返回错误")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("错误信息应含状态码与原因: %v", err)
	}
}

// TestChatEmptyChoices 响应无 choices：视为上游异常。
func TestChatEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices": [], "usage": {}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "sk", "m", 10*time.Second)
	if _, err := client.Chat(context.Background(), "p", "i", 100); err == nil {
		t.Error("choices 为空应报错")
	}
}

// ---------- Embedding ----------

// TestEmbeddingSuccess 正常嵌入：请求格式正确 + 解析向量。
func TestEmbeddingSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("路径错误: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"embedding": [0.1, 0.2, 0.3]}]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "sk-mock", "m", 10*time.Second)
	res, err := client.Embedding(context.Background(), EmbeddingRequest{Model: "m", Text: "你好"})
	if err != nil {
		t.Fatalf("嵌入失败: %v", err)
	}
	if len(res.Vector) != 3 || res.Vector[1] != 0.2 {
		t.Errorf("向量解析错误: %v", res.Vector)
	}
}

// ---------- 流式 ----------

// TestChatStream 流式对话：逐 chunk 解析增量文本，遇 [DONE] 结束。
func TestChatStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			`data: {"choices":[{"delta":{"content":"你"}}]}` + "\n\n" +
				`data: {"choices":[{"delta":{"content":"好"}}]}` + "\n\n" +
				`data: [DONE]` + "\n\n",
		))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "sk", "m", 10*time.Second)
	stream, err := client.ChatStream(context.Background(), ChatRequest{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 10})
	if err != nil {
		t.Fatalf("创建流失败: %v", err)
	}
	defer stream.Close()

	var got string
	for {
		chunk, recvErr := stream.Recv()
		if recvErr != nil {
			break // io.EOF 结束
		}
		got += chunk.Text
	}
	if got != "你好" {
		t.Errorf("流式拼接错误: %q", got)
	}
}

// ---------- Manager ----------

// TestManagerRegisterGet 注册与按名获取供应商。
func TestManagerRegisterGet(t *testing.T) {
	m := NewManager()
	p := NewOpenAICompatProvider("deepseek", "http://x", "sk", "m", time.Second)
	m.Register(p)

	if got, ok := m.Get("deepseek"); !ok || got.Name() != "deepseek" {
		t.Error("注册后应能按名获取")
	}
	if _, ok := m.Get("not-exist"); ok {
		t.Error("未注册供应商不应命中")
	}
}

// TestManagerChatNotFound 未注册供应商调用应返回 ErrProviderNotFound。
func TestManagerChatNotFound(t *testing.T) {
	m := NewManager()
	if _, err := m.Chat(context.Background(), "nope", ChatRequest{}); !errors.Is(err, ErrProviderNotFound) {
		t.Errorf("期望 ErrProviderNotFound，实际 %v", err)
	}
	if _, err := m.Embedding(context.Background(), "nope", EmbeddingRequest{}); !errors.Is(err, ErrProviderNotFound) {
		t.Errorf("期望 ErrProviderNotFound（嵌入），实际 %v", err)
	}
}
