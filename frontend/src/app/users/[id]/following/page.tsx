// src/app/users/[id]/following/page.tsx
// 关注列表页（设计稿《粉丝》画板同构）：关注 → 关注中/粉丝统计 → 关注列表（关注/已关注）。
"use client";

import Link from "next/link";
import { useParams } from "next/navigation";

import { DesktopNav } from "@/components/desktop-nav";
import { FollowList } from "@/components/follow-list";
import { MobileTabbar } from "@/components/mobile-tabbar";

// FollowingPage 关注列表页。
export default function FollowingPage() {
  const params = useParams<{ id: string }>();
  const userId = Number(params.id);

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-[640px] flex-1 px-4 py-6 pb-20">
        {/* 标题 + 返回（设计稿：关注中） */}
        <div className="flex items-center gap-3">
          <Link href={`/users/${userId}`} className="text-sm text-ink-3 hover:text-ink" aria-label="返回主页">
            ←
          </Link>
          <h1 className="font-display text-xl font-semibold text-ink">关注</h1>
        </div>
        <div className="mt-5">
          <FollowList userId={userId} mode="following" />
        </div>
      </main>
      <MobileTabbar />
    </div>
  );
}
