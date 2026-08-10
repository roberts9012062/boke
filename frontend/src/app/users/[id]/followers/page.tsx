// src/app/users/[id]/followers/page.tsx
// 粉丝列表页（设计稿《粉丝》画板）：粉丝 → 关注中/粉丝统计 → 粉丝列表（回关/已关注）。
"use client";

import Link from "next/link";
import { useParams } from "next/navigation";

import { DesktopNav } from "@/components/desktop-nav";
import { FollowList } from "@/components/follow-list";
import { MobileTabbar } from "@/components/mobile-tabbar";

// FollowersPage 粉丝列表页。
export default function FollowersPage() {
  const params = useParams<{ id: string }>();
  const userId = Number(params.id);

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-[640px] flex-1 px-4 py-6 pb-20">
        {/* 标题 + 返回（设计稿：粉丝） */}
        <div className="flex items-center gap-3">
          <Link href={`/users/${userId}`} className="text-sm text-ink-3 hover:text-ink" aria-label="返回主页">
            ←
          </Link>
          <h1 className="font-display text-xl font-semibold text-ink">粉丝</h1>
        </div>
        <div className="mt-5">
          <FollowList userId={userId} mode="followers" />
        </div>
      </main>
      <MobileTabbar />
    </div>
  );
}
