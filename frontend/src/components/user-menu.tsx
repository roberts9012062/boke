// src/components/user-menu.tsx
// 桌面导航用户菜单：未登录显示「登录」入口；登录后显示头像 + 下拉菜单
// （我的主页 / 运营后台(仅 admin) / 退出登录）。
// 设计依据：M/冷月/我的 画板菜单项（查看主页 / 运营后台 / 退出登录）。
"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { Avatar } from "@/components/ui/avatar";
import { apiUnreadCount } from "@/lib/api";
import { useAuth } from "@/lib/auth";

// UserMenu 导航区用户菜单（桌面 ≥768px 显示，移动端在「我的」页）。
export function UserMenu() {
  const router = useRouter();
  const { user, loading, logout } = useAuth();
  const [open, setOpen] = useState<boolean>(false);
  const [unread, setUnread] = useState<number>(0);
  const menuRef = useRef<HTMLDivElement>(null);

  // 未读通知角标轮询（30s，需求 3.8）
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

  // 点击外部关闭菜单
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  // 登出处理（登出后回首页）
  const handleLogout = async () => {
    setOpen(false);
    await logout();
    router.push("/");
  };

  // 会话恢复中：占位（避免闪烁显示「登录」）
  if (loading) {
    return <div className="hidden h-9 w-9 animate-pulse rounded-full bg-muted md:block" aria-hidden />;
  }

  // 未登录：显示登录入口
  if (!user) {
    return (
      <Link
        href="/login"
        className="hidden rounded-full border border-line px-4 py-2 text-sm text-ink-2 transition-colors hover:text-ink md:block"
      >
        登录
      </Link>
    );
  }

  return (
    <div className="relative hidden md:block" ref={menuRef}>
      {/* 头像按钮（M1.7：真实头像，无头像回退首字）+ 未读角标 */}
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="relative flex h-9 w-9 items-center justify-center rounded-full border border-line text-sm font-medium text-ink transition-colors hover:border-accent"
        aria-label="用户菜单"
        aria-expanded={open}
      >
        <Avatar name={user.nickname} url={user.avatar_url} className="h-9 w-9 text-sm" />
        {/* 未读通知角标 */}
        {unread > 0 && (
          <span className="absolute -right-1 -top-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-like px-1 text-[10px] text-white">
            {unread > 99 ? "99+" : unread}
          </span>
        )}
      </button>

      {/* 下拉菜单 */}
      {open && (
        <div className="absolute right-0 top-11 w-44 overflow-hidden rounded-lg border border-line bg-elevated shadow-lg">
          {/* 用户信息头 */}
          <div className="border-b border-line px-4 py-3">
            <p className="truncate text-sm font-medium text-ink">{user.nickname}</p>
            <p className="truncate text-xs text-ink-3">@{user.username}</p>
          </div>
          {/* 菜单项 */}
          <div className="py-1">
            <Link
              href={`/users/${user.id}`}
              onClick={() => setOpen(false)}
              className="block px-4 py-2 text-sm text-ink-2 transition-colors hover:bg-muted hover:text-ink"
            >
              我的主页
            </Link>
            {/* 仅 admin 显示后台入口（设计稿「运营后台」） */}
            {user.role === "admin" && (
              <Link
                href="/admin"
                onClick={() => setOpen(false)}
                className="block px-4 py-2 text-sm text-ink-2 transition-colors hover:bg-muted hover:text-ink"
              >
                运营后台
              </Link>
            )}
            <button
              type="button"
              onClick={handleLogout}
              className="block w-full px-4 py-2 text-left text-sm text-like transition-colors hover:bg-muted"
            >
              退出登录
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
