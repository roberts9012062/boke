// src/lib/api-openapi.ts
// 接口开放模块 API 封装：开放接口目录 + API Key 凭证管理（后台「接口开放」页面）。
// 说明：与后端 /admin/open-api/* 接口一一对应；独立文件避免 api.ts 持续膨胀。
import { get, post, del } from "./api";

// ---------- 类型（与后端 internal/model/openapi.go 手工同步） ----------

// CatalogParam 开放接口的参数说明。
export interface CatalogParam {
  name: string; // 参数名
  type: "string" | "integer" | "array" | "object"; // 类型
  location: "query" | "path" | "body"; // 位置
  required: boolean; // 是否必填
  description: string; // 参数说明
}

// CatalogEntry 开放接口目录项（后台多选展示与 AI 手册生成的数据源）。
export interface CatalogEntry {
  endpoint: string; // 接口标识（Key 绑定用，如 posts.list）
  method: string; // HTTP 方法（GET / POST）
  path: string; // 开放网关路径（/api/v1/open/...，含路由参数）
  name: string; // 接口名称（中文）
  description: string; // 功能描述
  params: CatalogParam[]; // 参数说明列表
}

// OpenAPIKey 凭证记录（列表展示）。
export interface OpenAPIKey {
  id: number; // 凭证 ID
  name: string; // 备注名
  key: string; // API Key（oa_ 前缀明文）
  endpoints: string[]; // 已授权接口标识数组
  expires_at: string | null; // 过期时间（null=永久有效）
  last_used_at: string | null; // 最近调用时间（null=从未调用）
  created_at: string; // 创建时间（ISO8601）
}

// CreateOpenAPIKeyReq 生成 Key 请求。
export interface CreateOpenAPIKeyReq {
  name: string; // 备注名（可选）
  endpoints: string[]; // 勾选的接口标识（≥1）
  expire_days: number | null; // 过期天数（正整数；null=永久有效）
}

// ---------- 后台管理 ----------

// 开放接口目录（页面多选数据源）。
export function apiOpenApiCatalog(): Promise<{ items: CatalogEntry[] }> {
  return get<{ items: CatalogEntry[] }>("/admin/open-api/catalog");
}

// 凭证列表（按创建时间倒序）。
export function apiOpenApiKeys(): Promise<{ items: OpenAPIKey[] }> {
  return get<{ items: OpenAPIKey[] }>("/admin/open-api/keys");
}

// 生成凭证（多选接口 + 过期天数；返回完整记录含明文 Key）。
export function apiCreateOpenApiKey(req: CreateOpenAPIKeyReq): Promise<OpenAPIKey> {
  return post<OpenAPIKey>("/admin/open-api/keys", req);
}

// 删除凭证。
export function apiDeleteOpenApiKey(id: number): Promise<{ id: number }> {
  return del<{ id: number }>(`/admin/open-api/keys/${id}`);
}
