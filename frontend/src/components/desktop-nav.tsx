// src/components/desktop-nav.tsx
// 桌面顶部导航（设计稿画板 Nav Desktop，1400px）：
// 月言（品牌）→ 首页 / 话题 → 搜索框 → 发帖按钮 → 用户菜单。
"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";

// ThemeToggle 主题快速切换按钮（小图标：冷月/薄雾）。
import { ThemeToggle } from "./theme-toggle";
// UserMenu 用户菜单（登录态/登录入口）。
import { UserMenu } from "./user-menu";
// PluginSlot 插件扩展点槽位（M3.6：theme.header 注入点）。
import PluginSlot from "./plugin-slot";

// DesktopNav 桌面端顶部导航（仅 ≥768px 显示，移动端用 MobileTabbar）。
// M1.7：搜索框可交互（回车跳转 /search?q=，与搜索页 URL 参数联动）。
export function DesktopNav() {
  const router = useRouter();
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
        {/* 品牌（hover 微亮反馈） */}
        <Link
          href="/"
          className="font-display text-2xl font-bold tracking-wide text-ink transition-colors duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] hover:text-glow"
        >
          月言
        </Link>

        {/* 主导航（移动端隐藏，仅保留品牌） */}
        <div className="hidden items-center gap-6 text-sm md:flex">
          <Link
            href="/"
            className="font-medium text-glow transition-colors duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] hover:text-ink"
          >
            首页
          </Link>
          <Link
            href="/topics"
            className="text-ink-2 transition-colors duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] hover:text-ink"
          >
            话题
          </Link>
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
