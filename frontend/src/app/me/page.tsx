// src/app/me/page.tsx
// 我的页（设计稿 M/冷月/我的 390）：
// 用户信息（昵称 @账号 站长徽标）→ 查看主页/编辑资料 → 菜单列表
// （个人资料/外观主题/运营后台(admin)/退出登录）。
// M1.7：真实头像展示（Avatar 组件）、主题设置页链接启用。
"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";

import { MobileTabbar } from "@/components/mobile-tabbar";
import { Avatar } from "@/components/ui/avatar";
import { useAuth } from "@/lib/auth";

// MyPage 我的页（移动端入口为底部 Tab「我的」；桌面也可直接访问）。
export default function MyPage() {
  const router = useRouter();
  const { user, loading, logout } = useAuth();

  // 登出处理
  const handleLogout = async () => {
    await logout();
    router.push("/");
  };

  // 会话恢复中：骨架占位
  if (loading) {
    return (
      <main className="min-h-screen">
        <div className="mx-auto max-w-[640px] px-6 py-6">
          <div className="h-20 animate-pulse rounded-lg bg-muted" aria-hidden />
          <div className="mt-4 space-y-2">
            <div className="h-12 animate-pulse rounded-lg bg-muted" aria-hidden />
            <div className="h-12 animate-pulse rounded-lg bg-muted" aria-hidden />
          </div>
        </div>
        <MobileTabbar />
      </main>
    );
  }

  // 未登录：提示登录
  if (!user) {
    return (
      <main className="flex min-h-screen flex-col items-center justify-center px-6">
        <p className="font-display text-xl text-ink">尚未登录</p>
        <p className="mt-2 text-sm text-ink-2">登录后查看你的主页与消息</p>
        <Link
          href="/login"
          className="mt-6 rounded-full bg-accent px-6 py-2.5 text-sm font-medium text-on-accent"
        >
          去登录
        </Link>
        <MobileTabbar />
      </main>
    );
  }

  return (
    <main className="min-h-screen pb-16">
      <div className="mx-auto max-w-[640px] px-4 py-6">
        {/* 用户信息头（昵称 + @账号 + 角色徽标） */}
        <section className="flex items-center gap-4 rounded-lg border border-line bg-elevated p-5">
          {/* 头像（M1.7：真实头像，无头像回退首字） */}
          <Avatar name={user.nickname} url={user.avatar_url} className="h-16 w-16 text-2xl" />
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <p className="font-display text-lg font-semibold text-ink">{user.nickname}</p>
              {/* 角色徽标（设计稿：站长） */}
              {user.role === "admin" && (
                <span className="rounded-full bg-accent-soft px-2 py-0.5 text-xs text-glow">
                  站长
                </span>
              )}
            </div>
            <p className="mt-0.5 text-sm text-ink-3">@{user.username}</p>
            {user.bio && <p className="mt-1 line-clamp-2 text-xs text-ink-2">{user.bio}</p>}
          </div>
        </section>

        {/* 快捷操作（设计稿：查看主页 / 编辑资料） */}
        <div className="mt-4 flex gap-3">
          <Link
            href={`/users/${user.id}`}
            className="flex-1 rounded-full border border-line py-2.5 text-center text-sm text-ink-2 transition-colors hover:text-ink"
          >
            查看主页
          </Link>
          <Link
            href="/settings/profile"
            className="flex-1 rounded-full border border-line py-2.5 text-center text-sm text-ink-2 transition-colors hover:text-ink"
          >
            编辑资料
          </Link>
        </div>

        {/* 菜单列表（设计稿：个人资料/外观主题/运营后台/退出登录） */}
        <nav className="mt-4 overflow-hidden rounded-lg border border-line bg-elevated">
          <Link
            href="/settings/profile"
            className="flex items-center justify-between border-b border-line px-4 py-3.5 text-sm text-ink transition-colors hover:bg-muted"
          >
            个人资料
            <span className="text-ink-3" aria-hidden>
              ›
            </span>
          </Link>
          <Link
            href="/settings/theme"
            className="flex items-center justify-between border-b border-line px-4 py-3.5 text-sm text-ink transition-colors hover:bg-muted"
          >
            外观主题
            <span className="text-xs text-ink-3">冷月银辉 · 月下薄雾</span>
          </Link>
          <Link
            href="/settings/notifications"
            className="flex items-center justify-between border-b border-line px-4 py-3.5 text-sm text-ink transition-colors hover:bg-muted"
          >
            通知偏好
            <span className="text-ink-3" aria-hidden>
              ›
            </span>
          </Link>
          <Link
            href="/settings/privacy"
            className="flex items-center justify-between border-b border-line px-4 py-3.5 text-sm text-ink transition-colors hover:bg-muted"
          >
            隐私与安全
            <span className="text-ink-3" aria-hidden>
              ›
            </span>
          </Link>
          {/* 消息中心（设计稿 M/冷月/我的；M2 私信启用，当前占位） */}
          <Link
            href="/messages"
            className="flex items-center justify-between border-b border-line px-4 py-3.5 text-sm text-ink transition-colors hover:bg-muted"
          >
            消息中心
            <span className="text-ink-3" aria-hidden>
              ›
            </span>
          </Link>
          {/* 运营后台（仅 admin，设计稿 M/冷月/我的） */}
          {user.role === "admin" && (
            <Link
              href="/admin"
              className="flex items-center justify-between border-b border-line px-4 py-3.5 text-sm text-ink transition-colors hover:bg-muted"
            >
              运营后台
              <span className="text-ink-3" aria-hidden>
                ›
              </span>
            </Link>
          )}
          <button
            type="button"
            onClick={handleLogout}
            className="w-full px-4 py-3.5 text-left text-sm text-like transition-colors hover:bg-muted"
          >
            退出登录
          </button>
        </nav>
      </div>
      <MobileTabbar />
    </main>
  );
}
