// src/app/admin/layout.tsx
// 后台布局（设计稿 D/冷月/后台仪表盘 + D/冷月/敏感词 等画板侧栏）：
// 左侧固定导航：设计稿 13 项（带线框图标，顺序与画板一致）
// + 底部「内容治理」补充组（审核队列/敏感词/封禁管理——M2 新增，设计稿侧栏未含，独立分组记录差异）
// + 顶部（月言 · 管理 / 用户信息 / 返回站点）+ 主内容区。
"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { NavIcon } from "@/components/admin/nav-icons";
import PluginSlot from "@/components/plugin-slot";
import { apiInstalledPlugins } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { canAccessAdmin, isSuperAdmin } from "@/lib/rbac";

// 导航项：icon 对应 NavIcon 图标键；available=false 显示建设中占位。
interface NavItem {
  href: string;
  label: string;
  icon: string;
  available: boolean;
}

// 主菜单（设计稿侧栏 13 项：插件为一级可展开菜单，二级为插件相关子项）
const MAIN_ITEMS: NavItem[] = [
  { href: "/admin", label: "仪表盘", icon: "dashboard", available: true },
  { href: "/admin/posts", label: "内容管理", icon: "posts", available: true },
  { href: "/admin/comments", label: "评论", icon: "comments", available: true },
  { href: "/admin/users", label: "用户/访客", icon: "users", available: true },
  { href: "/admin/media", label: "媒体库", icon: "media", available: true }, // M2.9 激活
  { href: "/admin/tags", label: "标签分类", icon: "tags", available: true }, // M2.9 激活
  { href: "/admin/roles", label: "角色权限", icon: "roles", available: true }, // M5 激活
  { href: "/admin/settings", label: "站点设置", icon: "settings", available: true },
  { href: "/admin/seo", label: "SEO 设置", icon: "seo", available: true }, // M4 激活
  { href: "/admin/seo-health", label: "健康度", icon: "health", available: true }, // M4 激活
  { href: "/admin/serp", label: "SERP 预览", icon: "serp", available: true }, // M4 激活
  { href: "/admin/ai", label: "AI 设置", icon: "ai", available: true }, // M4-AI 激活
  { href: "/admin/reports", label: "数据报表", icon: "report", available: true }, // M4-报表 激活
  { href: "/admin/backup", label: "备份导出", icon: "backup", available: true }, // M4-报表 激活
];

// 插件二级子菜单（插件为一级菜单，子项：插件商城 / 我的插件；M3.1 激活）
const PLUGIN_SUB_ITEMS: NavItem[] = [
  { href: "/admin/plugin-market", label: "插件商城", icon: "market", available: true },
  { href: "/admin/plugins", label: "我的插件", icon: "plugins", available: true },
];

// 内容治理组（M2 新增：设计稿侧栏未含此三项，独立分组置于底部，差异记录）
const MODERATION_ITEMS: NavItem[] = [
  { href: "/admin/audit", label: "审核队列", icon: "comments", available: true },
  { href: "/admin/sensitive-words", label: "敏感词", icon: "tags", available: true },
  { href: "/admin/bans", label: "封禁管理", icon: "roles", available: true },
];

// NavList 渲染导航项列表（图标 + 标签 + 建设中徽标）。
function NavList({ items, pathname }: { items: NavItem[]; pathname: string }) {
  return (
    <>
      {items.map((item) => {
        const active = item.href === "/admin" ? pathname === "/admin" : pathname.startsWith(item.href);
        const href = item.available ? item.href : "/admin/under-construction";
        return (
          <Link
            key={item.href}
            href={href}
            className={`mb-0.5 flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors ${
              active ? "bg-accent-soft font-medium text-glow" : "text-ink-2 hover:bg-muted hover:text-ink"
            }`}
          >
            <NavIcon name={item.icon} className="h-[18px] w-[18px] shrink-0" />
            <span className="flex-1">{item.label}</span>
            {!item.available && <span className="text-[10px] text-ink-3">建设中</span>}
          </Link>
        );
      })}
    </>
  );
}

// AdminLayout 后台布局（非 admin 跳转前台）。
export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const { user, loading } = useAuth();
  // 插件一级菜单展开状态（默认展开，对齐设计稿展开形态）
  const [pluginsOpen, setPluginsOpen] = useState<boolean>(true);
  // 插件子项激活时自动展开
  const pluginActive = pathname.startsWith("/admin/plugin-market") || pathname.startsWith("/admin/plugins");
  // 已装插件侧栏声明（M3.2 前端扩展点：running 插件声明的入口合并进插件子菜单）
  const [pluginNavItems, setPluginNavItems] = useState<{ href: string; label: string; icon: string }[]>([]);

  // 加载已装插件的侧栏声明（仅 running 状态生效）
  useEffect(() => {
    apiInstalledPlugins()
      .then((r) => {
        setPluginNavItems(
          r.items
            .filter((p) => p.state === "running" && p.nav?.href)
            .map((p) => ({ href: p.nav!.href, label: p.nav!.label, icon: p.nav!.icon })),
        );
      })
      .catch(() => undefined);
  }, [pathname]);

  // 未登录/非 admin：跳转
  useEffect(() => {
    if (!loading && (!user || !canAccessAdmin(user.role))) {
      router.replace("/admin-login");
    }
  }, [loading, user, router]);

  // 恢复中：骨架
  if (loading || !user || !canAccessAdmin(user.role)) {
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
        {/* 品牌（设计稿：月言 / 管理后台） */}
        <Link href="/admin" className="flex items-center gap-2 px-5 py-4">
          <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-accent font-display text-sm font-bold text-on-accent">
            月
          </span>
          <span className="font-display text-sm font-semibold text-ink">月言 · 管理</span>
        </Link>

        {/* 主菜单（设计稿 12 项平铺 + 插件一级可展开） */}
        <nav className="flex-1 overflow-y-auto px-2 py-2">
          <NavList items={MAIN_ITEMS.slice(0, 8)} pathname={pathname} />

          {/* 插件（一级菜单，可展开；二级：插件商城/我的插件） */}
          <div className="mb-0.5">
            <button
              type="button"
              onClick={() => setPluginsOpen((v) => !v)}
              className={`flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors ${
                pluginActive ? "bg-accent-soft font-medium text-glow" : "text-ink-2 hover:bg-muted hover:text-ink"
              }`}
            >
              <NavIcon name="plugins" className="h-[18px] w-[18px] shrink-0" />
              <span className="flex-1 text-left">插件</span>
              <span className="text-xs text-ink-3" aria-hidden>
                {pluginsOpen ? "▾" : "▸"}
              </span>
            </button>
            {/* 二级子菜单（缩进，插件相关：内置商城/我的插件 + 已装插件动态入口） */}
            {pluginsOpen && (
              <div className="ml-[18px] mt-0.5 border-l border-line pl-2">
                {PLUGIN_SUB_ITEMS.map((item) => {
                  const active = pathname.startsWith(item.href);
                  const href = item.available ? item.href : "/admin/under-construction";
                  return (
                    <Link
                      key={item.href}
                      href={href}
                      className={`mb-0.5 flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm transition-colors ${
                        active ? "bg-accent-soft font-medium text-glow" : "text-ink-2 hover:bg-muted hover:text-ink"
                      }`}
                    >
                      <span className="flex-1">{item.label}</span>
                      {!item.available && <span className="text-[10px] text-ink-3">建设中</span>}
                    </Link>
                  );
                })}
                {/* 动态入口（已装 running 插件声明的侧栏项，M3.2 前端扩展点） */}
                {pluginNavItems.map((item) => {
                  const active = pathname.startsWith(item.href);
                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      className={`mb-0.5 flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm transition-colors ${
                        active ? "bg-accent-soft font-medium text-glow" : "text-ink-2 hover:bg-muted hover:text-ink"
                      }`}
                    >
                      <NavIcon name={item.icon} className="h-4 w-4 shrink-0" />
                      <span className="flex-1">{item.label}</span>
                    </Link>
                  );
                })}
                {/* 插件扩展点：admin.menu（M3.6，侧栏菜单区） */}
                <PluginSlot slot="admin.menu" />
              </div>
            )}
          </div>

          <NavList items={MAIN_ITEMS.slice(8)} pathname={pathname} />

          {/* 内容治理组（M2 补充：分隔线 + 小标题，设计稿侧栏未含） */}
          <div className="mb-1 mt-3 px-3 text-[10px] uppercase tracking-wider text-ink-3">
            内容治理
          </div>
          <NavList items={MODERATION_ITEMS} pathname={pathname} />
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
            {isSuperAdmin(user.role) && (
              <span className="rounded-full bg-accent-soft px-2 py-0.5 text-xs text-glow">站长</span>
            )}
          </div>
        </header>

        <main className="p-6">{children}</main>
      </div>
    </div>
  );
}
