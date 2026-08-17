// src/lib/rbac.ts
// 角色权限前端工具（M5，设计稿《后台角色》五角色矩阵）：
// 角色中文名 / 后台访问判定 / 超级管理员判定 / 权限域中文名。
import type { UserProfile } from "@/types/api";

// 角色类型（与后端 casbin 常量对齐）。
export type Role = UserProfile["role"];

// ROLE_LABEL 角色中文名（设计稿《后台角色》《后台用户》角色列）。
export const ROLE_LABEL: Record<Role, string> = {
  superadmin: "超级管理员",
  editor: "编辑",
  author: "作者",
  visitor: "访客",
  restricted: "受限访客",
};

// 后台资源域中文名（设计稿《后台角色》权限范围列；与后端 casbin 域常量对齐）。
export const DOMAIN_LABEL: Record<string, string> = {
  dashboard: "仪表盘",
  posts: "内容管理",
  pages: "自定义页面",
  comments: "评论管理",
  users: "用户管理",
  media: "媒体库",
  tags: "标签分类",
  settings: "站点设置",
  roles: "角色权限",
  moderation: "内容治理",
  plugins: "插件",
  seo: "SEO",
  ai: "AI",
  reports: "数据报表",
  backups: "备份导出",
};

// canAccessAdmin 是否有后台访问权（M5：超管/编辑/作者可进后台；访客/受限访客不可）。
export function canAccessAdmin(role: string): boolean {
  return role === "superadmin" || role === "editor" || role === "author";
}

// isSuperAdmin 是否超级管理员（站长徽标/角色页只读判断）。
export function isSuperAdmin(role: string): boolean {
  return role === "superadmin";
}

// formatPermissions 权限域列表 → 中文短语（设计稿权限范围列：内容·评论·媒体）。
export function formatPermissions(domains: string[]): string {
  const labels = domains.map((d) => DOMAIN_LABEL[d] ?? d);
  if (labels.length === 0) {
    return "—";
  }
  return labels.join(" · ");
}
