// src/components/post-actions.tsx
// 帖子卡片底部互动条（设计稿：128 赞 / 36 评论 / 12 收藏 / — 分享）：
// 点赞（真实落库）/ 评论数（进详情评论区）/ 收藏数 / 分享面板 / 插件操作槽。
// 从 post-card 抽出（说说卡与文章卡共用；点赞未登录提示登录）。
"use client";

import Link from "next/link";
import { useState } from "react";

import PluginSlot from "@/components/plugin-slot";
import { SharePanel } from "@/components/share-panel";
import { apiLikePost, apiUnlikePost, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import type { PostSummary } from "@/types/api";

// PostActions 帖子互动条（参数：post 帖子摘要）。
export function PostActions({ post }: { post: PostSummary }) {
  const { user } = useAuth();
  const [liked, setLiked] = useState<boolean>(false);
  const [likeCount, setLikeCount] = useState<number>(post.like_count);
  const [error, setError] = useState<string>("");
  const [shareOpen, setShareOpen] = useState<boolean>(false); // 分享面板（#17）

  // 点赞/取消（真实落库；未登录提示登录）
  const handleLike = async () => {
    if (!user) {
      setError("请先登录后再点赞");
      return;
    }
    try {
      if (liked) {
        const result = await apiUnlikePost(post.id);
        setLikeCount(result.like_count);
        setLiked(false);
      } else {
        const result = await apiLikePost(post.id);
        setLikeCount(result.like_count);
        setLiked(true);
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "操作失败");
    }
  };

  return (
    <>
      {/* 底部互动条（收藏数 M1.7 接入列表聚合，点赞真实落库） */}
      <div className="mt-4 flex items-center gap-6 border-t border-line pt-3 text-xs text-ink-3">
        <button
          type="button"
          onClick={() => void handleLike()}
          className={`flex items-center gap-1 transition-colors ${
            liked ? "text-like" : "hover:text-ink"
          }`}
        >
          {/* 爱心弹跳（liked 切换时重播 animate-pop） */}
          <span aria-hidden className={`inline-block ${liked ? "animate-pop" : ""}`}>
            {liked ? "♥" : "♡"}
          </span>
          <span>{likeCount}</span>
        </button>
        <Link href={`/posts/${post.id}#comments`} className="flex items-center gap-1 hover:text-ink">
          <span aria-hidden>💬</span>
          <span>{post.comment_count}</span>
        </Link>
        <Link href={`/posts/${post.id}`} className="flex items-center gap-1 hover:text-ink">
          <span aria-hidden>🔖</span>
          <span>{post.favorite_count ?? 0}</span>
        </Link>
        {/* 分享（设计稿《分享面板》，#17 激活） */}
        <button
          type="button"
          onClick={() => setShareOpen(true)}
          className="flex items-center gap-1 transition-colors hover:text-ink"
        >
          <span aria-hidden>↗</span>
          <span>分享</span>
        </button>
        {/* 插件扩展点：post.card.actions（时间线卡片操作区——TTS 朗读等轻量动作；
            props 透传帖子摘要数据供插件消费，无插件订阅时不占位） */}
        <PluginSlot
          slot="post.card.actions"
          props={{ post: { id: post.id, title: post.title ?? "", summary: post.summary } }}
        />
      </div>
      {/* 点赞提示（未登录） */}
      {error && <p className="mt-2 text-xs text-like">{error}</p>}

      {/* 分享面板（作者 · 话题 元信息 + 图片帖海报图） */}
      {shareOpen && (
        <SharePanel
          title={post.title || post.summary || "分享帖子"}
          content={post.summary}
          media={post.media.filter((m) => m.type === "image").map((m) => m.url)}
          meta={`${post.author.nickname}${post.tags.length > 0 ? ` · ${post.tags.map((t) => t.name).join(" ")}` : ""}`}
          shareUrl={typeof window !== "undefined" ? `${window.location.origin}/posts/${post.id}` : `/posts/${post.id}`}
          onClose={() => setShareOpen(false)}
        />
      )}
    </>
  );
}
