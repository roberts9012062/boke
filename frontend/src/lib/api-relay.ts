// src/lib/api-relay.ts
// 中继站（大世界）API 封装：后台配置 / 连接测试 / 前台聚合流。
// 复用 api.ts 的 get/put/post 便捷函数（统一携带凭证与响应解析）。
import { get, post, put } from "./api";

// RelayConfig 对接配置（GET /admin/relay）。key 隐藏保管：只回 has_key，明文永不回前端。
export interface RelayConfig {
  enabled: boolean;
  url: string;
  mode: "public" | "bridged" | string;
  default_category: string;
  local_retention_days: number;
  has_key: boolean;
  claim_pending?: boolean;
  relay_meta_json: string | null;
  last_seq: number;
  updated_at: string;
}

// RelayHandshakeMeta 握手元信息（连接测试回显 / meta 快照）。
export interface RelayHandshakeMeta {
  name: string;
  rules_md: string;
  max_sites: number;
  site_count: number;
  retention_days: number;
  categories: string[];
}

// RelayHandshakeResp 连接测试响应。
export interface RelayHandshakeResp {
  proto_ver: number;
  meta: RelayHandshakeMeta;
  quota: {
    daily_moments: number;
    daily_articles: number;
    media: { per_item_bytes: number; daily_items: number; daily_bytes: number; formats: string[] };
  };
  server_time: number;
}

// RelayContentPayload 大世界卡片负载（信封 ContentPayload）。
export interface RelayContentPayload {
  content_id: string;
  site: { id: number; name: string; url: string; avatar: string; mode: string };
  kind: "moment" | "article" | string;
  category: string;
  tags: string[];
  published_at: number;
  moment?: { text: string; images: string[]; videos?: string[]; audios?: string[] };
  article?: { title: string; summary: string; cover: string; origin_url: string; read_url?: string };
}

// RelayCacheItem 缓存条目。
export interface RelayCacheItem {
  content_id: string;
  payload: RelayContentPayload;
  published_at: string;
}

// apiRelayConfig 拉取对接配置。
export function apiRelayConfig(): Promise<RelayConfig> {
  return get<RelayConfig>("/admin/relay");
}

// RelayApplyResult 申请结果（自动通过时 key 由后端隐藏保存；待审核时为审核中状态）。
export interface RelayApplyResult {
  status: "approved" | "pending" | "rejected" | "idle" | string;
  relay_name: string;
  categories: string[];
}

// apiRelayApply 自助申请对接许可（后端代理调中继站 /api/v1/apply）。
export function apiRelayApply(body: { url: string; mode: string }): Promise<RelayApplyResult> {
  return post<RelayApplyResult>("/admin/relay/apply", body);
}

// apiRelayClaim 轮询申请审批结果（通过即自动领 key 隐藏保存）。
export function apiRelayClaim(): Promise<RelayApplyResult> {
  return get<RelayApplyResult>("/admin/relay/claim");
}

// apiRelaySave 保存配置（保存后订阅任务自动重启；site_key 传空表示沿用隐藏保管的 key）。
export function apiRelaySave(body: {
  enabled: boolean; url: string; site_key: string; mode: string;
  default_category: string; local_retention_days: number;
}): Promise<{ saved: boolean }> {
  return put<{ saved: boolean }>("/admin/relay", body);
}

// apiWorldStatus 前台判断大世界是否可见。
export function apiWorldStatus(): Promise<{ enabled: boolean }> {
  return get<{ enabled: boolean }>("/relay/status");
}

// RelayPublishOptions 发帖表单的中继站发布选项（GET /relay/publish-options）。
export interface RelayPublishOptions {
  enabled: boolean; // 中继站是否启用（未启用时表单隐藏发送选择区）
  categories: string[]; // 可选分类（握手快照为权威，缺省回退本地规范表）
  taxonomy: Record<string, string[]>; // 分类 → 规范标签
  default_category: string; // 预选分类（后台意向分类）
}

// apiRelayPublishOptions 拉取发帖表单的中继站发布选项（需登录）。
export function apiRelayPublishOptions(): Promise<RelayPublishOptions> {
  return get<RelayPublishOptions>("/relay/publish-options");
}

// apiRelayTaxonomy 拉取分类/标签规范（公开：大世界筛选渲染；enabled/默认分类无意义）。
export function apiRelayTaxonomy(): Promise<RelayPublishOptions> {
  return get<RelayPublishOptions>("/relay/taxonomy");
}

// apiWorldContents 大世界聚合流（本地缓存分页；category/tag 可选过滤）。
export function apiWorldContents(params: { category?: string; tag?: string; before?: number; limit?: number }): Promise<{ items: RelayCacheItem[] }> {
  const qs = new URLSearchParams();
  if (params.category) qs.set("category", params.category);
  if (params.tag) qs.set("tag", params.tag);
  if (params.before) qs.set("before", String(params.before));
  if (params.limit) qs.set("limit", String(params.limit));
  const suffix = qs.toString() ? `?${qs.toString()}` : "";
  return get<{ items: RelayCacheItem[] }>(`/relay/contents${suffix}`);
}
