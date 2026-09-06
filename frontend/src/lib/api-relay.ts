// src/lib/api-relay.ts
// 中继站（大世界）API 封装：后台配置 / 连接测试 / 前台聚合流。
// 复用 api.ts 的 get/put/post 便捷函数（统一携带凭证与响应解析）。
import { get, post, put } from "./api";

// RelayConfig 对接配置（GET /admin/relay）。
export interface RelayConfig {
  enabled: boolean;
  url: string;
  site_key: string;
  mode: "public" | "bridged" | string;
  default_category: string;
  local_retention_days: number;
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

// apiRelayTest 连接测试（实时握手）。
export function apiRelayTest(body: { url: string; site_key: string; mode: string }): Promise<RelayHandshakeResp> {
  return post<RelayHandshakeResp>("/admin/relay/test", body);
}

// apiRelaySave 保存配置（保存后订阅任务自动重启）。
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

// apiWorldContents 大世界聚合流（本地缓存分页）。
export function apiWorldContents(params: { category?: string; before?: number; limit?: number }): Promise<{ items: RelayCacheItem[] }> {
  const qs = new URLSearchParams();
  if (params.category) qs.set("category", params.category);
  if (params.before) qs.set("before", String(params.before));
  if (params.limit) qs.set("limit", String(params.limit));
  const suffix = qs.toString() ? `?${qs.toString()}` : "";
  return get<{ items: RelayCacheItem[] }>(`/relay/contents${suffix}`);
}
