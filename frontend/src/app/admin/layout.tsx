// src/app/admin/layout.tsx
// 后台布局（设计稿 D/冷月/后台仪表盘）：
// 左侧固定导航（仪表盘/内容管理/评论/用户·访客/站点设置 + 建设中占位项）
// + 顶部（月言 · 管理 / Admin / 返回站点）+ 主内容区。
"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";

import { useAuth } from "@/lib/auth";

// 侧栏导航配置（设计稿全量：MVP 5 项可用 + 8 项建设中占位）
const NAV_ITEMS = [
  { href: "/admin", label: "仪表盘", available: true },
  { href: "/admin/posts", label: "内容管理", available: true },
  { href: "/admin/comments", label: "评论", available: true },
  { href: "/admin/users", label: "用户/访客", available: true },
  { href: "/admin/settings", label: "站点设置", available: true },
  { href: "/admin/audit", label: "审核队列", available: true },
  { href: "/admin/sensitive-words", label: "敏感词", available: true },
  { href: "/admin/bans", label: "封禁管理", available: true },
  { href: "/admin/media", label: "媒体库", available: false },
  { href: "/admin/tags", label: "标签分类", available: false },
  { href: "/admin/roles", label: "角色权限", available: false },
  { href: "/admin/plugins", label: "插件", available: false },
  { href: "/admin/plugin-market", label: "插件商城", available: false },
  { href: "/admin/seo", label: "SEO 设置", available: false },
  { href: "/admin/seo-health", label: "健康度", available: false },
  { href: "/admin/serp", label: "SERP 预览", available: false },
] as const;

// AdminLayout 后台布局（非 admin 跳转前台）。
export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const { user, loading } = useAuth();

  // 未登录/非 admin：跳转
  useEffect(() => {
    if (!loading && (!user || user.role !== "admin")) {
      router.replace("/admin-login");
    }
  }, [loading, user, router]);

  // 恢复中：骨架
  if (loading || !user || user.role !== "admin") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-6 w-40 animate-pulse rounded bg-muted" aria-hidden />
      </div>
    );
  }

  return (
    <div className="flex min-h-screen bg-bg">
      {/* 左侧导航（设计稿固定侧栏） */}
      <aside className="fixed inset-y-0 left-0 z-30 flex w-52 flex-col border-r border-line bg-elevated">
        {/* 品牌（设计稿：月言 · 管理） */}
        <Link href="/admin" className="flex items-center gap-2 px-5 py-4">
          <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-accent font-display text-sm font-bold text-on-accent">
            月
          </span>
          <span className="font-display text-sm font-semibold text-ink">月言 · 管理</span>
        </Link>

        {/* 导航列表 */}
        <nav className="flex-1 overflow-y-auto px-2 py-2">
          {NAV_ITEMS.map((item) => {
            const active = item.href === "/admin" ? pathname === "/admin" : pathname.startsWith(item.href);
            // 建设中占位项：点击显示占位页（决策 5）
            const href = item.available ? item.href : "/admin/under-construction";
            return (
              <Link
                key={item.href}
                href={href}
                className={`mb-0.5 flex items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors ${
                  active
                    ? "bg-accent-soft font-medium text-glow"
                    : "text-ink-2 hover:bg-muted hover:text-ink"
                }`}
              >
                {item.label}
                {!item.available && <span className="text-[10px] text-ink-3">建设中</span>}
              </Link>
            );
          })}
        </nav>

        {/* 返回站点（设计稿：返回站点） */}
        <Link
          href="/"
          className="border-t border-line px-5 py-3 text-sm text-ink-2 transition-colors hover:text-ink"
        >
          ← 返回站点
        </Link>
      </aside>

      {/* 主内容区 */}
      <div className="ml-52 flex-1">
        {/* 顶部栏（设计稿：Admin / 用户信息） */}
        <header className="sticky top-0 z-20 flex h-14 items-center justify-between border-b border-line bg-elevated/90 px-6 backdrop-blur">
          <span className="text-sm text-ink-3">管理后台</span>
          <div className="flex items-center gap-2">
            <span className="flex h-8 w-8 items-center justify-center rounded-full bg-muted text-xs text-ink-2">
              {user.nickname.charAt(0)}
            </span>
            <span className="text-sm text-ink">{user.nickname}</span>
            {user.role === "admin" && (
              <span className="rounded-full bg-accent-soft px-2 py-0.5 text-xs text-glow">站长</span>
            )}
          </div>
        </header>

        <main className="p-6">{children}</main>
      </div>
    </div>
  );
}
