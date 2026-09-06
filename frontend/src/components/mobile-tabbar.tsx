// src/components/mobile-tabbar.tsx
// 移动端底部 Tab 导航（设计稿 M/冷月/首页，390px）：
// 首页 / 搜索 / 发帖（中间凸起主按钮）/ 通知（未读角标）/ 我的。
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";

import { apiUnreadCount } from "@/lib/api";
import { useAuth } from "@/lib/auth";

// 导航项配置（文案与设计稿一致；primary = 中间凸起发帖按钮）
interface TabItem {
  href: string; // 跳转路径
  label: string; // 显示文案
  icon: string; // 图标字符
  primary?: boolean; // 是否为发帖主按钮（中间凸起）
}

const TABS: readonly TabItem[] = [
  { href: "/", label: "首页", icon: "⌂" },
  { href: "/world", label: "大世界", icon: "🌐" },
  { href: "/compose", label: "发帖", icon: "＋", primary: true },
  { href: "/notifications", label: "通知", icon: "🔔" },
  { href: "/me", label: "我的", icon: "👤" },
];

// MobileTabbar 移动端固定底部导航（仅 <768px 显示）。
// M1.7：通知 Tab 增加未读角标（30s 轮询，与桌面菜单一致，需求 3.8）。
export function MobileTabbar() {
  const pathname = usePathname();
  const { user } = useAuth();
  const [unread, setUnread] = useState<number>(0);

  // 未读通知角标轮询（登录后生效）
  useEffect(() => {
    if (!user) {
      setUnread(0);
      return;
    }
    const poll = () => {
      apiUnreadCount()
        .then((r) => setUnread(r.unread))
        .catch(() => {
          // 轮询失败静默
        });
    };
    poll();
    const timer = setInterval(poll, 30000);
    return () => clearInterval(timer);
  }, [user]);

  return (
    <nav
      className="fixed inset-x-0 bottom-0 z-40 border-t border-line bg-elevated/95 backdrop-blur md:hidden"
      aria-label="移动端导航"
    >
      <div className="grid h-14 grid-cols-5">
        {TABS.map((tab) => {
          // 当前 Tab 高亮（根路径精确匹配，其余前缀匹配）
          const active = tab.href === "/" ? pathname === "/" : pathname.startsWith(tab.href);
          // 发帖为中间凸起主按钮：悬浮于导航条上方（按压回缩 + hover 提亮）
          if (tab.primary) {
            return (
              <Link
                key={tab.href}
                href={tab.href}
                className="relative flex items-center justify-center transition duration-[var(--yy-duration-fast)] active:scale-95"
                aria-label={tab.label}
              >
                <span className="absolute -top-4 flex h-12 w-12 items-center justify-center rounded-full bg-accent text-xl text-on-accent shadow-lg transition-[box-shadow,opacity] duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] hover:opacity-90 hover:shadow-xl">
                  {tab.icon}
                </span>
              </Link>
            );
          }
          return (
            <Link
              key={tab.href}
              href={tab.href}
              className={`relative flex flex-col items-center justify-center gap-0.5 text-[11px] transition-colors duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] ${
                active ? "text-glow" : "text-ink-3"
              }`}
            >
              <span className="relative text-lg leading-none" aria-hidden>
                {tab.icon}
                {/* 通知未读角标（仅通知 Tab，需求 3.8；出现时弹跳） */}
                {tab.href === "/notifications" && unread > 0 && (
                  <span className="absolute -right-2.5 -top-1 flex h-4 min-w-4 animate-pop items-center justify-center rounded-full bg-like px-1 text-[10px] text-white">
                    {unread > 99 ? "99+" : unread}
                  </span>
                )}
              </span>
              <span>{tab.label}</span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
