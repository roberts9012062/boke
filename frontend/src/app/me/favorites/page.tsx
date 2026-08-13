// src/app/me/favorites/page.tsx
// 我的收藏页（设计稿 M/冷月/收藏 390）：
// 我的收藏 + Tab（全部/文字/图片/音频）+ 收藏列表（作者/内容/类型·收藏于时间）。
// M1.7：展示「收藏于 x 前」（后端 favorited_at 字段），列表形态对齐设计稿。
"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { Reveal } from "@/components/motion/reveal";
import { Avatar } from "@/components/ui/avatar";
import { EmptyState } from "@/components/ui/empty-state";
import { apiFavorites } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { timeAgo } from "@/lib/utils";
import type { PostSummary } from "@/types/api";

// 类型文案（设计稿：文字 · 收藏于 2 天前）
const TYPE_LABEL: Record<string, string> = {
  text: "文字",
  image: "图片",
  audio: "音频",
  video: "视频",
};

// FavoritesPage 我的收藏（需登录）。
export default function FavoritesPage() {
  const router = useRouter();
  const { user, loading } = useAuth();
  const [items, setItems] = useState<PostSummary[]>([]);
  const [tab, setTab] = useState<string>(""); // 空=全部；text/image/audio（设计稿 Tab）
  const [loaded, setLoaded] = useState<boolean>(false);

  useEffect(() => {
    if (loading) {
      return;
    }
    if (!user) {
      router.replace("/login");
      return;
    }
    apiFavorites()
      .then((r) => setItems(r.items))
      .catch(() => setItems([]))
      .finally(() => setLoaded(true));
  }, [loading, user, router]);

  // 按 Tab 过滤
  const filtered = items.filter((post) => tab === "" || post.content_type === tab);

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-[720px] flex-1 px-4 py-6 pb-20">
        <h1 className="font-display text-xl font-semibold text-ink">我的收藏</h1>

        {/* Tab（设计稿：全部/文字/图片/音频） */}
        <div className="mt-4 flex gap-2 border-b border-line pb-3">
          {[
            { key: "", label: "全部" },
            { key: "text", label: "文字" },
            { key: "image", label: "图片" },
            { key: "audio", label: "音频" },
          ].map((t) => (
            <button
              key={t.key}
              type="button"
              onClick={() => setTab(t.key)}
              className={`rounded-full px-4 py-1.5 text-sm transition-colors ${
                tab === t.key
                  ? "bg-accent-soft font-medium text-glow"
                  : "bg-muted text-ink-2 hover:text-ink"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>

        {!loaded && <div className="mt-4 h-32 animate-pulse rounded-lg bg-muted" aria-hidden />}

        {/* 空状态（设计稿《空状态》：还没有收藏 / 遇到想慢慢读的帖子，点一下书签就放在这里。 / 去首页逛逛） */}
        {loaded && items.length === 0 && (
          <div className="mt-4">
            <EmptyState
              icon="🔖"
              title="还没有收藏"
              description="遇到想慢慢读的帖子，点一下书签就放在这里。"
              actionText="去首页逛逛"
              actionHref="/"
            />
          </div>
        )}

        {/* 收藏列表（设计稿：作者 + 内容 + 「文字 · 收藏于 2 天前」；Reveal 滚动进场） */}
        {loaded && filtered.length > 0 && (
          <div className="mt-4 divide-y divide-line rounded-lg border border-line bg-elevated">
            {filtered.map((post) => (
              <Reveal key={post.id}>
                <Link
                  href={`/posts/${post.id}`}
                  className="flex items-start gap-3 px-4 py-3.5 transition-colors hover:bg-muted"
                >
                <Avatar name={post.author.nickname} url={post.author.avatar_url} className="h-9 w-9 text-sm" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm text-ink-2">
                    <span className="font-medium text-ink">{post.author.nickname}</span>
                    {"  "}
                    {post.summary || post.title}
                  </p>
                  <p className="mt-1 text-[11px] text-ink-3">
                    {TYPE_LABEL[post.content_type] ?? post.content_type} · 收藏于{" "}
                    {post.favorited_at ? timeAgo(post.favorited_at) : "近期"}
                  </p>
                </div>
                </Link>
              </Reveal>
            ))}
          </div>
        )}
        {loaded && items.length > 0 && filtered.length === 0 && (
          <div className="mt-4">
            <EmptyState icon="🔖" title="该分类暂无收藏" description="切换其他分类看看" />
          </div>
        )}
      </main>
      <MobileTabbar />
    </div>
  );
}
