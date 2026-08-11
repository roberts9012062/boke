// src/components/post-card.tsx
// 帖子卡片（设计稿 Card Text/Image/Audio/Video）：
// 作者行（头像/昵称/@账号·相对时间）→ 正文（截断 3-4 行）→ 媒体预览
// （图片网格 / 音频播放条）→ 标签 → 底部互动条（赞/评论/收藏/分享）。
"use client";

import Link from "next/link";
import { useState } from "react";

import { AudioPlayer } from "@/components/audio-player";
import { SharePanel } from "@/components/share-panel";
import { Avatar } from "@/components/ui/avatar";
import { useAppearance } from "@/lib/appearance";
import { apiLikePost, apiUnlikePost, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { timeAgo } from "@/lib/utils";
import type { PostSummary } from "@/types/api";

// PostCard 帖子卡片（列表形态；详情页为独立渲染，不复用本组件）。
// 参数：post 帖子摘要。
export function PostCard({ post }: { post: PostSummary }) {
  const { user } = useAuth();
  const { settings } = useAppearance(); // 外观设置（自动播放媒体开关）
  const [liked, setLiked] = useState<boolean>(false);
  const [likeCount, setLikeCount] = useState<number>(post.like_count);
  const [error, setError] = useState<string>("");
  const [shareOpen, setShareOpen] = useState<boolean>(false); // 分享面板（#17）
  // 正文显示：列表用摘要（列表摘要）

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
    <article className="rounded-lg border border-line bg-elevated p-5">
      {/* 作者行：头像 + 昵称 + @账号·时间 */}
      <div className="flex items-center gap-3">
        <Link href={`/users/${post.author.id}`} className="shrink-0">
          <Avatar name={post.author.nickname} url={post.author.avatar_url} className="h-10 w-10 text-sm" />
        </Link>
        <div className="min-w-0">
          <Link
            href={`/users/${post.author.id}`}
            className="block truncate text-sm font-medium text-ink hover:text-glow"
          >
            {post.author.nickname}
          </Link>
          <p className="truncate text-xs text-ink-3">
            @{post.author.username} · {timeAgo(post.published_at)}
          </p>
        </div>
      </div>

      {/* 正文（列表 3-4 行截断；详情展开） */}
      <p
        className="mt-3 line-clamp-4 whitespace-pre-wrap break-words text-sm leading-relaxed text-ink"
      >
        {post.summary}
      </p>

      {/* 媒体预览：图片网格（列表首图）/ 音频播放条 */}
      {post.content_type === "image" && post.media.length > 0 && (
        <Link href={`/posts/${post.id}`} className="mt-3 block">
          {/* 首图为主图，多图右侧小图（设计稿图片帖网格） */}
          <div className="grid grid-cols-2 gap-1 overflow-hidden rounded-lg">
            {post.media.slice(0, 2).map((m) => (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                key={m.id}
                src={m.url}
                alt=""
                loading="lazy"
                className={`aspect-[4/3] w-full object-cover ${
                  post.media.length === 1 ? "col-span-2" : ""
                }`}
              />
            ))}
          </div>
        </Link>
      )}
      {post.content_type === "audio" && post.media.length > 0 && (
        <div className="mt-3">
          <AudioPlayer src={post.media[0].url} duration={0} autoplay={settings.autoplayMedia} />
        </div>
      )}

      {/* 视频预览（M2：内嵌播放器，点击卡片进详情页完整播放） */}
      {post.content_type === "video" && post.media.length > 0 && (
        <Link href={`/posts/${post.id}`} className="mt-3 block" aria-label="查看视频帖子">
          <video
            src={post.media[0].url}
            preload="metadata"
            playsInline
            autoPlay={settings.autoplayMedia}
            muted={settings.autoplayMedia}
            className="aspect-video w-full rounded-lg bg-black object-contain"
          />
        </Link>
      )}

      {/* 标签（# 前缀，点击进话题页） */}
      {post.tags.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-2">
          {post.tags.map((tag) => (
            <Link
              key={tag.slug}
              href={`/topics/${encodeURIComponent(tag.name.replace("#", ""))}`}
              className="text-xs text-glow hover:underline"
            >
              {tag.name}
            </Link>
          ))}
        </div>
      )}

      {/* 底部互动条（设计稿：128 赞 / 36 评论 / 12 收藏 / — 分享） */}
      {/* 收藏数 M1.7 接入列表聚合（post.favorite_count），点赞真实落库 */}
      <div className="mt-4 flex items-center gap-6 border-t border-line pt-3 text-xs text-ink-3">
        <button
          type="button"
          onClick={() => void handleLike()}
          className={`flex items-center gap-1 transition-colors ${
            liked ? "text-like" : "hover:text-ink"
          }`}
        >
          <span aria-hidden>{liked ? "♥" : "♡"}</span>
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
    </article>
  );
}
