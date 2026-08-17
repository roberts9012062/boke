// src/lib/api-pages.ts
// 自定义页面模块 API 封装：前台按 slug 取已发布页面 + 后台页面 CRUD。
// 说明：与后端 /pages/:slug、/admin/pages* 接口一一对应；独立文件避免 api.ts 持续膨胀。
import { get, post, put, del } from "./api";

// ---------- 类型（与后端 internal/model/page.go 手工同步） ----------

// AdminPageItem 后台页面列表项（不含正文）。
export interface AdminPageItem {
  id: number; // 页面 ID
  slug: string; // 路由标识（前台访问 /pages/{slug}）
  title: string; // 标题
  status: "draft" | "published"; // 状态：draft 草稿 / published 已发布
  description: string; // SEO 描述
  updated_at: string; // 更新时间（ISO8601）
  created_at: string; // 创建时间（ISO8601）
}

// CustomPageDetail 后台编辑回显（完整实体，含正文）。
export interface CustomPageDetail {
  id: number; // 页面 ID
  slug: string; // 路由标识
  title: string; // 标题
  content: string; // 正文（富文本 HTML 或 AI 生成的完整文档）
  content_format: "html" | "markdown" | "page"; // 正文格式（page = AI 构建器整页文档）
  description: string; // SEO 描述
  status: "draft" | "published"; // 状态
  created_at: string; // 创建时间（ISO8601）
  updated_at: string; // 更新时间（ISO8601）
}

// PageInput 页面创建/更新输入（全量覆盖）。
export interface PageInput {
  slug: string; // 路由标识（小写字母/数字/连字符）
  title: string; // 标题
  content: string; // 正文（富文本 HTML 或完整页面文档）
  content_format: "html" | "markdown" | "page"; // 正文格式
  description: string; // SEO 描述
  status: "draft" | "published"; // 状态
}

// PagePublicDetail 前台页面详情（仅已发布页面返回）。
export interface PagePublicDetail {
  slug: string; // 路由标识
  title: string; // 标题
  content: string; // 正文
  content_format: "html" | "markdown" | "page"; // 正文格式
  description: string; // SEO 描述
  updated_at: string; // 更新时间（ISO8601）
}

// ---------- 前台（公开） ----------

// 按 slug 取已发布页面（草稿/不存在返回 404 错误）。
export function apiPageBySlug(slug: string): Promise<PagePublicDetail> {
  return get<PagePublicDetail>(`/pages/${encodeURIComponent(slug)}`);
}

// ---------- 后台管理 ----------

// 页面列表（含草稿）。
export function apiAdminPages(): Promise<{ items: AdminPageItem[] }> {
  return get<{ items: AdminPageItem[] }>("/admin/pages");
}

// 页面详情（编辑回显，含正文）。
export function apiAdminPage(id: number): Promise<CustomPageDetail> {
  return get<CustomPageDetail>(`/admin/pages/${id}`);
}

// 创建页面（返回新页面 ID）。
export function apiAdminCreatePage(input: PageInput): Promise<{ id: number }> {
  return post<{ id: number }>("/admin/pages", input);
}

// 更新页面（全量覆盖）。
export function apiAdminUpdatePage(id: number, input: PageInput): Promise<{ id: number }> {
  return put<{ id: number }>(`/admin/pages/${id}`, input);
}

// 删除页面。
export function apiAdminDeletePage(id: number): Promise<void> {
  return del<void>(`/admin/pages/${id}`);
}
