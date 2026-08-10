// src/app/publish-success/[id]/page.tsx
// 发布成功页（设计稿 D/冷月/发布成功 1400×700）：
// 发布成功 → 你的帖子已出现在首页与话题流中。→ 帖子标题+摘要 → 查看帖子/回到首页。
"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { apiPostDetail } from "@/lib/api";
import { excerpt } from "@/lib/utils";
import type { PostDetail } from "@/types/api";

// PublishSuccessPage 发布成功页（发布后跳转）。
export default function PublishSuccessPage() {
  const params = useParams<{ id: string }>();
  const postId = Number(params.id);
  const [post, setPost] = useState<PostDetail | null>(null);

  // 拉取刚发布的帖子（展示标题与摘要）
  useEffect(() => {
    if (postId) {
      apiPostDetail(postId)
        .then(setPost)
        .catch(() => {
          // 拉取失败不阻塞页面（仅展示占位）
        });
    }
  }, [postId]);

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto flex w-full max-w-[720px] flex-1 flex-col items-center justify-center px-6 py-12">
        {/* 成功图标 */}
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-accent-soft text-3xl text-glow">
          ✓
        </div>

        {/* 文案（设计稿逐字） */}
        <h1 className="mt-5 font-display text-2xl font-semibold text-ink">发布成功</h1>
        <p className="mt-2 text-sm text-ink-2">你的帖子已出现在首页与话题流中。</p>

        {/* 帖子摘要卡片（设计稿：标题「窗台上的银」= 正文首句 + 摘要） */}
        {post && (
          <div className="mt-8 w-full rounded-lg border border-line bg-elevated p-5">
            <p className="font-display text-base font-medium text-ink">
              {post.title || excerpt(post.content, 20)}
            </p>
            <p className="mt-1 line-clamp-2 text-sm text-ink-2">{post.summary}</p>
          </div>
        )}

        {/* 操作按钮（设计稿：查看帖子 / 回到首页） */}
        <div className="mt-8 flex gap-3">
          <Link
            href={`/posts/${postId}`}
            className="rounded-full bg-accent px-7 py-2.5 text-sm font-medium text-on-accent transition-opacity hover:opacity-90"
          >
            查看帖子
          </Link>
          <Link
            href="/"
            className="rounded-full border border-line px-7 py-2.5 text-sm text-ink-2 transition-colors hover:text-ink"
          >
            回到首页
          </Link>
        </div>
      </main>
      <MobileTabbar />
    </div>
  );
}
