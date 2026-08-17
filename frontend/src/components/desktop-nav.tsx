// src/components/desktop-nav.tsx
// 桌面顶部导航（设计稿画板 Nav Desktop，1400px）：
// 品牌（站点名）→ 自定义导航项（后台「头部导航」配置，支持两级：hover 动画下拉二级菜单）
// → 搜索框 → 发帖按钮 → 用户菜单。
"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useState } from "react";

import { DEFAULT_SITE_NAME, useSiteMeta } from "@/lib/site-meta";
import { useSitePluginNav } from "@/lib/site-plugin-nav";
import type { SiteNavLink } from "@/types/api";

// ThemeToggle 主题快速切换按钮（小图标：冷月/薄雾）。
import { ThemeToggle } from "./theme-toggle";
// UserMenu 用户菜单（登录态/登录入口）。
import { UserMenu } from "./user-menu";
// PluginSlot 插件扩展点槽位（M3.6：theme.header 注入点）。
import PluginSlot from "./plugin-slot";

// isExternalLink 是否外部链接（http/https 开头；其余视为站内路径走 Link）。
function isExternalLink(url: string): boolean {
  return url.startsWith("http://") || url.startsWith("https://");
}

// isActive 当前路由是否命中导航项（首页精确匹配，其余前缀匹配）。
function isActive(pathname: string, url: string): boolean {
  if (!url) {
    return false;
  }
  if (url === "/") {
    return pathname === "/";
  }
  return pathname === url || pathname.startsWith(`${url}/`);
}

// linkClass 导航项样式（命中高亮 / 未命中次级色；hover 回归主色）。
function linkClass(active: boolean): string {
  return active
    ? "font-medium text-glow transition-colors duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] hover:text-ink"
    : "text-ink-2 transition-colors duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] hover:text-ink";
}

// SubItem 二级菜单项（竖排小号；站内 Link / 外链 a）。
function SubItem({ item }: { item: SiteNavLink }) {
  if (isExternalLink(item.url)) {
    return (
      <a
        href={item.url}
        target={item.new_tab ? "_blank" : undefined}
        rel={item.new_tab ? "noreferrer" : undefined}
        className="block rounded-md px-3 py-2 text-sm text-ink-2 transition-colors duration-[var(--yy-duration-base)] hover:bg-muted hover:text-ink"
      >
        {item.label}
      </a>
    );
  }
  return (
    <Link
      href={item.url}
      className="block rounded-md px-3 py-2 text-sm text-ink-2 transition-colors duration-[var(--yy-duration-base)] hover:bg-muted hover:text-ink"
    >
      {item.label}
    </Link>
  );
}

// DropdownMenu 二级下拉面板（纯 CSS 动画：默认隐藏下移，group-hover 淡入上移展开）。
function DropdownMenu({ children }: { children: SiteNavLink[] }) {
  return (
    <div
      className="invisible absolute left-1/2 top-full z-50 min-w-[160px] -translate-x-1/2 translate-y-1 rounded-lg border border-line bg-elevated p-1.5 opacity-0 shadow-lg shadow-black/10 transition-all duration-200 ease-out group-hover:visible group-hover:translate-y-0 group-hover:opacity-100"
    >
      {children.map((child) => (
        <SubItem key={`${child.label}-${child.url}`} item={child} />
      ))}
    </div>
  );
}

// TopLevelLink 一级导航交互元素（有 URL 渲染链接；纯分组渲染 span 仅承载下拉）。
function TopLevelLink({ item, active }: { item: SiteNavLink; active: boolean }) {
  if (item.url === "") {
    return <span className={`cursor-default ${linkClass(active)}`}>{item.label} ▾</span>;
  }
  if (isExternalLink(item.url)) {
    return (
      <a
        href={item.url}
        target={item.new_tab ? "_blank" : undefined}
        rel={item.new_tab ? "noreferrer" : undefined}
        className={linkClass(active)}
      >
        {item.label}
        {item.children && item.children.length > 0 ? " ▾" : ""}
      </a>
    );
  }
  return (
    <Link href={item.url} className={linkClass(active)}>
      {item.label}
      {item.children && item.children.length > 0 ? " ▾" : ""}
    </Link>
  );
}

// NavItem 单个一级导航项（含二级下拉；有子项时外层 group 触发 hover 下拉）。
function NavItem({ item, pathname }: { item: SiteNavLink; pathname: string }) {
  const active = isActive(pathname, item.url) ||
    (item.children ?? []).some((c) => isActive(pathname, c.url));
  const hasChildren = (item.children ?? []).length > 0;
  const inner = <TopLevelLink item={item} active={active} />;
  if (!hasChildren) {
    return inner;
  }
  return (
    <div className="group relative py-2">
      {inner}
      <DropdownMenu>{item.children ?? []}</DropdownMenu>
    </div>
  );
}

// DesktopNav 桌面端顶部导航（仅 ≥768px 显示，移动端用 MobileTabbar）。
// M1.7：搜索框可交互（回车跳转 /search?q=，与搜索页 URL 参数联动）。
// 导航自定义：useSiteMeta 读后台配置，未配置/加载失败回退默认「首页/话题」。
export function DesktopNav() {
  const router = useRouter();
  const pathname = usePathname();
  const { meta, nav } = useSiteMeta();
  // 插件注册的前台导航项（site.page 扩展）：追加在管理员配置(nav_links)之后展示
  const pluginNav = useSitePluginNav();
  const [keyword, setKeyword] = useState<string>("");

  // 回车提交搜索
  const handleSearch = () => {
    const q = keyword.trim();
    if (q) {
      router.push(`/search?q=${encodeURIComponent(q)}`);
    }
  };

  return (
    <header className="sticky top-0 z-40 border-b border-line bg-elevated/90 backdrop-blur">
      <nav className="mx-auto flex h-16 max-w-[1400px] items-center gap-8 px-6">
        {/* 品牌（站点名可配置，hover 微亮反馈） */}
        <Link
          href="/"
          className="font-display text-2xl font-bold tracking-wide text-ink transition-colors duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] hover:text-glow"
        >
          {meta?.site_name || DEFAULT_SITE_NAME}
        </Link>

        {/* 主导航（移动端隐藏，仅保留品牌；管理员配置项 + 插件注册项，支持二级下拉） */}
        <div className="hidden items-center gap-6 text-sm md:flex">
          {[...nav, ...pluginNav.items].map((item) => (
            <NavItem key={`${item.label}-${item.url}`} item={item} pathname={pathname} />
          ))}
        </div>

        {/* 搜索框（≥1024px 显示；移动端由底部导航进入搜索页；768-1023 中间断点隐藏防拥挤） */}
        <div className="ml-auto flex items-center gap-4">
          <div className="hidden lg:flex">
            <input
              type="text"
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  handleSearch();
                }
              }}
              placeholder="搜索…"
              className="h-9 w-56 rounded-full border border-line bg-muted px-4 text-sm text-ink transition-[border-color,background-color] duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </div>
          <ThemeToggle />
          {/* 插件扩展点：theme.header（M3.6，主题页头右侧） */}
          <PluginSlot slot="theme.header" />
          <Link
            href="/compose"
            className="hidden rounded-full bg-accent px-5 py-2 text-sm font-medium text-on-accent transition duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] hover:opacity-90 active:scale-95 md:block"
          >
            发帖
          </Link>
          {/* 用户菜单（登录态/登录入口） */}
          <UserMenu />
        </div>
      </nav>
    </header>
  );
}
