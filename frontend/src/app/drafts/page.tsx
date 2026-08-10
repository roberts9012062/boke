// src/app/drafts/page.tsx
// 草稿箱（设计稿 M/冷月/草稿箱 390）：
// 草稿箱 N 篇 → 每条（标题/摘要/类型徽标/时间/继续编辑）。
"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { apiDrafts } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import type { PostSummary } from "@/types/api";

// 类型徽标文案（设计稿：文字/图片/音频）
const TYPE_LABEL: Record<string, string> = {
  text: "文字",
  image: "图片",
  audio: "音频",
  video: "视频",
};

// DraftsPage 草稿箱（需登录）。
export default function DraftsPage() {
  const router = useRouter();
  const { user, loading } = useAuth();
  const [drafts, setDrafts] = useState<PostSummary[]>([]);
  const [loaded, setLoaded] = useState<boolean>(false);

  // 加载草稿列表
  useEffect(() => {
    if (loading) {
      return;
    }
    if (!user) {
      router.replace("/login");
      return;
    }
    apiDrafts()
      .then(setDrafts)
      .catch(() => setDrafts([]))
      .finally(() => setLoaded(true));
  }, [loading, user, router]);

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-[720px] flex-1 px-4 py-6 pb-20">
        <h1 className="mb-4 font-display text-xl font-semibold text-ink">
          草稿箱
          {loaded && <span className="ml-2 text-sm font-normal text-ink-3">{drafts.length} 篇</span>}
        </h1>

        {/* 加载中 */}
        {!loaded && <div className="h-32 animate-pulse rounded-lg bg-muted" aria-hidden />}

        {/* 空状态 */}
        {loaded && drafts.length === 0 && (
          <div className="rounded-lg border border-line bg-elevated py-16 text-center">
            <p className="text-sm text-ink-2">还没有草稿</p>
            <Link href="/compose" className="mt-3 inline-block text-sm text-glow hover:underline">
              去写一帖 →
            </Link>
          </div>
        )}

        {/* 草稿列表（设计稿：标题/摘要/类型徽标/时间/继续编辑） */}
        {loaded && drafts.length > 0 && (
          <div className="space-y-3">
            {drafts.map((draft) => (
              <div key={draft.id} className="rounded-lg border border-line bg-elevated p-4">
                <div className="flex items-center gap-2">
                  <span className="rounded-full bg-accent-soft px-2 py-0.5 text-xs text-glow">
                    {TYPE_LABEL[draft.content_type] ?? draft.content_type}
                  </span>
                  {/* 标题：有标题用标题，无标题显示「未命名草稿」（设计稿文案） */}
                  <p className="truncate text-sm font-medium text-ink">
                    {draft.title || "未命名草稿"}
                  </p>
                </div>
                {/* 摘要独立一行（设计稿：月光从窗格里淌进来…） */}
                {draft.summary && (
                  <p className="mt-1 line-clamp-1 text-xs text-ink-2">{draft.summary}</p>
                )}
                <div className="mt-2 flex items-center justify-between">
                  <p className="text-xs text-ink-3">
                    {draft.published_at
                      ? new Date(draft.published_at).toLocaleString("zh-CN")
                      : "今天"}
                  </p>
                  <Link
                    href={`/compose?draft=${draft.id}`}
                    className="text-xs text-glow hover:underline"
                  >
                    继续编辑 →
                  </Link>
                </div>
              </div>
            ))}
          </div>
        )}
      </main>
      <MobileTabbar />
    </div>
  );
}
