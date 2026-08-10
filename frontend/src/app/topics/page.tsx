// src/app/topics/page.tsx
// 话题页（设计稿 D/冷月/话题）：话题列表（#话题名 + N 帖），点击进话题详情。
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { apiTopics } from "@/lib/api";
import type { TopicDTO } from "@/types/api";

// TopicsPage 话题列表页。
export default function TopicsPage() {
  const [topics, setTopics] = useState<TopicDTO[]>([]);
  const [loading, setLoading] = useState<boolean>(true);

  useEffect(() => {
    apiTopics()
      .then(setTopics)
      .catch(() => setTopics([]))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-[720px] flex-1 px-4 py-6 pb-20">
        <h1 className="mb-4 font-display text-xl font-semibold text-ink">话题</h1>

        {/* 加载骨架 */}
        {loading && <div className="h-48 animate-pulse rounded-lg bg-muted" aria-hidden />}

        {/* 话题列表（设计稿：#月色随笔 328 帖） */}
        {!loading && (
          <div className="space-y-3">
            {topics.map((topic) => (
              <Link
                key={topic.slug}
                href={`/topics/${encodeURIComponent(topic.name.replace("#", ""))}`}
                className="block rounded-lg border border-line bg-elevated p-4 transition-colors hover:border-accent"
              >
                <div className="flex items-center justify-between">
                  <p className="font-display text-base font-medium text-glow">{topic.name}</p>
                  <p className="text-sm text-ink-3">{topic.post_count} 帖</p>
                </div>
                {topic.description && (
                  <p className="mt-1 line-clamp-1 text-xs text-ink-2">{topic.description}</p>
                )}
              </Link>
            ))}
            {/* 空状态 */}
            {topics.length === 0 && (
              <div className="rounded-lg border border-line bg-elevated py-16 text-center">
                <p className="text-sm text-ink-2">还没有话题</p>
                <p className="mt-1 text-xs text-ink-3">发帖时添加 #标签 即可创建话题</p>
              </div>
            )}
          </div>
        )}
      </main>
      <MobileTabbar />
    </div>
  );
}
