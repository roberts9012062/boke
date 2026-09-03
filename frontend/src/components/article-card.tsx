// src/components/article-card.tsx
// 文章卡片（时间轴长内容形态，post_kind=article）：
// 作者行 → 标题（点击进详情页阅读）→ 内容预览（后端 200 字摘要，过长省略 +
// 「阅读全文」引导）→ 图集网格 → 标签 → 底部互动条（共用 post-actions）。
// 与说说卡片（post-card）并列，由首页按 post_kind 分发渲染。
"use client";

import Link from "next/link";

import { PostActions } from "@/components/post-actions";
import { Avatar } from "@/components/ui/avatar";
import { timeAgo } from "@/lib/utils";
import type { PostSummary } from "@/types/api";

// ArticleCard 文章卡片（列表形态）。
// 参数：post 帖子摘要（summary 为后端按形态生成的 200 字预览，末尾带省略号表示截断）。
export function ArticleCard({ post }: { post: PostSummary }) {
  // 内容是否被截断（省略号结尾 → 展示「阅读全文」引导）
  const truncated = post.summary.endsWith("…");

  return (
    <article className="rounded-lg border border-line bg-elevated p-5 transition-[translate,box-shadow,border-color] duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] hover:-translate-y-0.5 hover:border-accent/40 hover:shadow-[var(--yy-shadow-card-hover)]">
      {/* 作者行：头像 + 昵称 + @账号·时间 + 文章角标 */}
      <div className="flex items-center gap-3">
        <Link href={`/users/${post.author.id}`} className="shrink-0">
          <Avatar name={post.author.nickname} url={post.author.avatar_url} className="h-10 w-10 text-sm" />
        </Link>
        <div className="min-w-0 flex-1">
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
        {/* 文章形态角标 */}
        <span className="shrink-0 rounded-full bg-accent-soft px-2.5 py-0.5 text-xs text-glow">文章</span>
      </div>

      {/* 标题（点击进详情页阅读；hover 变色下划线） */}
      <Link
        href={`/posts/${post.id}`}
        className="mt-3 block font-display text-lg font-semibold leading-snug text-ink transition-colors hover:text-glow hover:underline hover:decoration-accent/40 hover:underline-offset-4"
      >
        {post.title}
      </Link>

      {/* 内容预览（200 字摘要；过长省略，「阅读全文」进详情页） */}
      <p className="mt-2 whitespace-pre-wrap break-words text-sm leading-relaxed text-ink-2">
        {post.summary}
        {truncated && (
          <Link
            href={`/posts/${post.id}`}
            className="ml-2 whitespace-nowrap text-glow hover:underline"
          >
            阅读全文 →
          </Link>
        )}
      </p>

      {/* 图集网格（文章图片：首图为主图，多图右侧小图；hover 微放大）
          media 空数组兜底（?.）：历史/缓存响应可能为 null，读 length 会崩整页 */}
      {(post.media?.length ?? 0) > 0 && (
        <Link href={`/posts/${post.id}`} className="mt-3 block">
          <div className="group relative grid grid-cols-2 gap-1 overflow-hidden rounded-lg">
            {post.media.slice(0, 2).map((m) => (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                key={m.id}
                src={m.url}
                alt=""
                loading="lazy"
                className={`aspect-[4/3] w-full object-cover transition-transform duration-[var(--yy-duration-slow)] ease-[var(--yy-ease-out)] group-hover:scale-[1.03] ${
                  post.media.length === 1 ? "col-span-2" : ""
                }`}
              />
            ))}
            {/* 多图角标（第 3 张起计数） */}
            {post.media.length > 2 && (
              <span className="absolute bottom-1.5 right-1.5 rounded-md bg-black/55 px-1.5 py-0.5 text-[10px] font-medium text-white">
                共 {post.media.length} 图
              </span>
            )}
          </div>
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
