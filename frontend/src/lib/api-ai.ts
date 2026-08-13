// src/lib/api-ai.ts
// AI 模块 API 封装（M4）：供应商管理 / 任务配置 / 用量统计 / 内置场景 / 统一接口。
// 说明：与后端 /admin/ai/* 接口一一对应；独立文件避免 api.ts 持续膨胀。
import { get, post, put, del, authHeaders } from "./api";

// ---------- 供应商 ----------

// AiProviderDTO 供应商 DTO（API Key 不回显，仅标记是否已配置）。
export interface AiProviderDTO {
  id: number; // 供应商 ID
  name: string; // 名称
  base_url: string; // 接口地址
  api_key_set: boolean; // 是否已配置 API Key
  models: string[]; // 模型列表
  enabled: boolean; // 是否启用
  priority: number; // 路由优先级
  price_input: number; // 输入单价（元/百万 token）
  price_output: number; // 输出单价（元/百万 token）
}

// AiProviderInput 供应商新增/编辑输入（编辑时 api_key 留空 = 保持原值）。
export interface AiProviderInput {
  name: string;
  base_url: string;
  api_key: string;
  models: string[];
  enabled: boolean;
  priority: number;
  price_input: number;
  price_output: number;
}

// 供应商列表。
export function apiAiProviders(): Promise<{ items: AiProviderDTO[] }> {
  return get<{ items: AiProviderDTO[] }>("/admin/ai/providers");
}

// 新增供应商（返回 id）。
export function apiAiCreateProvider(input: AiProviderInput): Promise<{ id: number }> {
  return post<{ id: number }>("/admin/ai/providers", input);
}

// 更新供应商。
export function apiAiUpdateProvider(id: number, input: AiProviderInput): Promise<void> {
  return put<void>(`/admin/ai/providers/${id}`, input);
}

// 删除供应商。
export function apiAiDeleteProvider(id: number): Promise<void> {
  return del<void>(`/admin/ai/providers/${id}`);
}

// 测试供应商连通性（成功返回「连通正常」）。
export function apiAiTestProvider(id: number): Promise<{ message: string }> {
  return post<{ message: string }>(`/admin/ai/providers/${id}/test`, {});
}

// 拉取供应商模型清单（以表单 base_url + api_key 直连，不落库）。
export function apiAiFetchModels(baseUrl: string, apiKey: string): Promise<{ models: string[] }> {
  return post<{ models: string[] }>("/admin/ai/providers/fetch-models", { base_url: baseUrl, api_key: apiKey });
}

// ---------- 任务配置 ----------

// AiTaskDTO 任务 DTO（绑定供应商名）。
export interface AiTaskDTO {
  task_name: string; // 任务名
  provider_id: number | null; // 绑定供应商（null=自动路由）
  provider_name: string; // 供应商名
  model: string; // 模型名（空=供应商默认）
  prompt_template: string; // 提示词模板
  max_tokens: number; // 最大输出 token
  enabled: boolean; // 是否启用
}

// 任务列表。
export function apiAiTasks(): Promise<{ items: AiTaskDTO[] }> {
  return get<{ items: AiTaskDTO[] }>("/admin/ai/tasks");
}

// 更新任务配置。
export function apiAiUpdateTask(
  taskName: string,
  input: { provider_id: number | null; model: string; prompt_template: string; max_tokens: number },
): Promise<void> {
  return put<void>(`/admin/ai/tasks/${taskName}`, input);
}

// 启停任务。
export function apiAiToggleTask(taskName: string, enabled: boolean): Promise<void> {
  return post<void>(`/admin/ai/tasks/${taskName}/toggle`, { enabled });
}

// ---------- 用量统计 ----------

// AiUsageSummary 用量汇总。
export interface AiUsageSummary {
  today_calls: number; // 今日调用次数
  today_tokens: number; // 今日 token 总量
  total_calls: number; // 累计调用次数
  total_tokens: number; // 累计 token 总量
  today_cost: number; // 今日费用（元）
  total_cost: number; // 累计费用（元）
}

// AiDayStat 单日用量（趋势图表）。
export interface AiDayStat {
  day: string; // 日期 YYYY-MM-DD
  calls: number; // 当日调用次数
  tokens: number; // 当日 token 总量
  cost: number; // 当日费用（元）
}

// 用量统计（汇总 + 近 7 日）。
export function apiAiUsage(): Promise<{ summary: AiUsageSummary; days: AiDayStat[] }> {
  return get<{ summary: AiUsageSummary; days: AiDayStat[] }>("/admin/ai/usage");
}

// ---------- 内置场景 ----------

// AI 生成帖子摘要（写入 seo_meta.summary，返回摘要文本）。
export function apiAiGenSummary(postId: number): Promise<{ summary: string }> {
  return post<{ summary: string }>(`/admin/ai/gen/summary?post_id=${postId}`, {});
}

// AI 生成标签建议（返回建议数组，前端确认后经帖子更新接口写入）。
export function apiAiGenTags(postId: number): Promise<{ tags: string[] }> {
  return post<{ tags: string[] }>(`/admin/ai/gen/tags?post_id=${postId}`, {});
}

// 批量 AI 审核评论（后台评论管理手动兜底）。
export function apiAiReviewComments(commentIds: number[]): Promise<{ ok: number; failed: number }> {
  return post<{ ok: number; failed: number }>("/admin/ai/review/comments", { comment_ids: commentIds });
}

// ---------- 统一接口（通用入口，插件前端/未来功能直调） ----------

// AI 统一非流式生成。
export function apiAiGenerate(input: { model: string; prompt: string; content: string }): Promise<{ text: string }> {
  return post<{ text: string }>("/admin/ai/generate", input);
}

// apiAiGenerateStream 统一流式生成（SSE；逐增量回调 onChunk）。
// 说明：走独立 fetch（request 封装期待 JSON 响应体，不适用 SSE）；错误码透传为 ApiError。
export async function apiAiGenerateStream(
  input: { model: string; prompt: string; content: string },
  onChunk: (text: string) => void,
): Promise<void> {
  const response = await fetch("/api/v1/admin/ai/generate/stream", {
    method: "POST",
    headers: { "Content-Type": "application/json", ...authHeaders() },
    body: JSON.stringify(input),
  });
  if (!response.ok || !response.body) {
    // 尝试解析统一错误体，透出后端提示
    let message = "AI 流式生成失败";
    try {
      const body = (await response.json()) as { message?: string };
      if (body.message) {
        message = body.message;
      }
    } catch {
      // 忽略解析失败，使用默认提示
    }
    throw new Error(message);
  }
  // 逐行解析 SSE：data: {"text":"..."} 直至 data: [DONE]
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  for (;;) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    let idx = buffer.indexOf("\n\n");
    while (idx >= 0) {
      const event = buffer.slice(0, idx);
      buffer = buffer.slice(idx + 2);
      const data = event.replace(/^data:\s*/, "");
      if (data && data !== "[DONE]") {
        try {
          const parsed = JSON.parse(data) as { text?: string };
          if (parsed.text) {
            onChunk(parsed.text);
          }
        } catch {
          // 忽略无法解析的 chunk（容错，不中断流）
        }
      }
      idx = buffer.indexOf("\n\n");
    }
  }
}

// AI 统一向量嵌入。
export function apiAiEmbedding(input: { model: string; text: string }): Promise<{ embedding: number[] }> {
  return post<{ embedding: number[] }>("/admin/ai/embedding", input);
}

// 智能回复助手（续写/润色/翻译）。
export function apiAiGenReply(postId: number, action: "continue" | "polish" | "translate"): Promise<{ text: string }> {
  return post<{ text: string }>(`/admin/ai/gen/reply?post_id=${postId}&action=${action}`, {});
}

// AI SEO 建议（标题/描述/关键词）。
export function apiAiGenSeoAdvice(postId: number): Promise<{ title: string; description: string; keywords: string[] }> {
  return post<{ title: string; description: string; keywords: string[] }>(`/admin/ai/gen/seo-advice?post_id=${postId}`, {});
}
