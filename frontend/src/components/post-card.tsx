// src/components/post-card.tsx
// 说说卡片（设计稿 Card Text/Image/Audio/Video）：
// 作者行（头像/昵称/@账号·相对时间）→ 正文（截断 3-4 行）→ 媒体预览
// （图片网格 / 音频播放条）→ 标签 → 底部互动条（赞/评论/收藏/分享，见 post-actions）。
// 动效：卡片 hover 轻浮起（阴影+描边加深）；图片 hover 微放大。
// 说明：文章形态（post_kind=article）由 article-card 渲染，本组件只负责说说。
"use client";

import Link from "next/link";

import { AudioPlayer } from "@/components/audio-player";
import { MomentImageGrid } from "@/components/moment-image-grid";
import { MusicRefPlayer } from "@/components/music-ref-player";
import { PluginBlock } from "@/components/plugin-block";
import { PostActions } from "@/components/post-actions";
import { Avatar } from "@/components/ui/avatar";
import { useAppearance } from "@/lib/appearance";
import { timeAgo } from "@/lib/utils";
import type { PostSummary } from "@/types/api";

// PostCard 说说卡片（列表形态；详情页为独立渲染，不复用本组件）。
// 参数：post 帖子摘要。
export function PostCard({ post }: { post: PostSummary }) {
  const { settings } = useAppearance(); // 外观设置（自动播放媒体开关）

  return (
    <article className="rounded-lg border border-line bg-elevated p-5 transition-[translate,box-shadow,border-color] duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] hover:-translate-y-0.5 hover:border-accent/40 hover:shadow-[var(--yy-shadow-card-hover)]">
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

      {/* 媒体预览：图片按数量自适应（1 原图 / 2-4 横排 / 5-9 九宫格）*/}
      {post.content_type === "image" && (post.media?.length ?? 0) > 0 && (
        <MomentImageGrid media={post.media} postHref={`/posts/${post.id}`} />
      )}
      {post.content_type === "audio" && (post.media?.length ?? 0) > 0 && (
        <div className="mt-3">
          <AudioPlayer src={post.media[0].url} duration={0} autoplay={settings.autoplayMedia} />
        </div>
      )}

      {/* B站视频嵌入（bilibili-video 插件：列表直接渲染插件播放器块） */}
      {post.bilibili && (
        <div className="mt-3">
          <PluginBlock type="bilibili" props={post.bilibili} />
        </div>
      )}

      {/* 音乐嵌入（M7：正文内嵌音乐，列表直接渲染播放器）——
          网易云引用（song_id）用自研播放器；第三方 iframe（QQ/旧网易云）保持 iframe */}
      {post.music && post.music.song_id && (
        <div className="mt-3">
          <MusicRefPlayer
            songId={post.music.song_id}
            title={post.music.title}
            artist={post.music.artist}
            coverUrl={post.music.cover_url}
            platform={post.music.platform === "qq" ? "qq" : "netease"}
          />
        </div>
      )}
      {post.music && !post.music.song_id && post.music.url && (
        <div className="mt-3 overflow-hidden rounded-lg border border-line bg-elevated">
          <iframe
            src={post.music.url}
            title={`内嵌音乐（${post.music.platform}）`}
            allow="autoplay"
            allowFullScreen
            style={{
              width: "100%",
              height:
                post.music.platform === "netease" && post.music.kind === "song"
                  ? 66
                  : post.music.platform === "netease"
                    ? 430
                    : 110,
              border: 0,
              display: "block",
            }}
          />
        </div>
      )}

      {/* 视频预览（M2：内嵌播放器，点击卡片进详情页完整播放） */}
      {post.content_type === "video" && (post.media?.length ?? 0) > 0 && (
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

      {/* 底部互动条（赞/评论/收藏/分享 + 插件槽；共用组件） */}
      <PostActions post={post} />
    </article>
  );
}
