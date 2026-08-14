// src/app/posts/[id]/page.tsx
// 帖子详情页（设计稿 M/冷月/帖子详情 390 + D/冷月/音频播放 + 灯箱）：
// 返回 → 作者/时间/可见性 → 完整正文 → 媒体（图片灯箱/音频播放器）
// → 互动条（赞/评论/收藏）→ 评论区（M1.4 已接入）。
"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { AudioPlayer } from "@/components/audio-player";
import { CommentSection } from "@/components/comment-section";
import { DesktopNav } from "@/components/desktop-nav";
import { ImageGallery, type GalleryStyle } from "@/components/image-gallery";
import { Lightbox } from "@/components/lightbox";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { PostContent } from "@/components/post-content";
import PluginSlot from "@/components/plugin-slot";
import { ReportDialog } from "@/components/report-dialog";
import { SharePanel } from "@/components/share-panel";
import {
  apiFavoritePost,
  apiLikePost,
  apiPostDetail,
  apiPostState,
  apiUnfavoritePost,
  apiUnlikePost,
  ApiError,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { readGuest } from "@/lib/guest";
import { timeAgo } from "@/lib/utils";
import type { PostDetail } from "@/types/api";

// PostDetailPage 帖子详情页。
export default function PostDetailPage() {
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const postId = Number(params.id);
  const { user } = useAuth();

  const [post, setPost] = useState<PostDetail | null>(null);
  const [error, setError] = useState<string>("");
  const [reportOpen, setReportOpen] = useState<boolean>(false); // M2 举报弹层
  const [shareOpen, setShareOpen] = useState<boolean>(false); // 分享面板（#17）
  const [lightboxIndex, setLightboxIndex] = useState<number | null>(null);

  // 互动状态（点赞/收藏真实落库，M1.4）
  const [liked, setLiked] = useState<boolean>(false);
  const [favorited, setFavorited] = useState<boolean>(false);
  const [likeCount, setLikeCount] = useState<number>(0);
  const [favoriteCount, setFavoriteCount] = useState<number>(0);
  const [reacting, setReacting] = useState<boolean>(false);

  // 加载详情（依赖 user 恢复：走查纠偏——刷新页面时 auth 恢复晚于首次请求会导致
  // is_author 误判（编辑按钮不显示），登录态恢复后重载）
  useEffect(() => {
    if (!postId) {
      setError("帖子不存在");
      return;
    }
    // 竞态保护：快速切换帖子时丢弃旧请求的迟到响应（此前无取消标志，旧响应会覆盖新帖状态）
    let cancelled = false;
    // 记录进入前的标题与 robots meta（离开/切换时复原，避免 SEO 输出残留到其他页面）
    const previousTitle = document.title;
    const previousRobots = document.querySelector('meta[name="robots"]')?.getAttribute("content") ?? null;

    apiPostDetail(postId, readGuest()?.guest_token)
      .then((detail) => {
        if (cancelled) {
          return;
        }
        setPost(detail);
        setLikeCount(detail.like_count);
        // SEO 输出（M4.1 插件通道）：自定义标题 + robots 收录策略写入文档
        document.title = detail.seo?.title
          ? `${detail.seo.title} · 月言`
          : detail.title
            ? `${detail.title} · 月言`
            : "月言";
        if (detail.seo?.robots) {
          let meta = document.querySelector('meta[name="robots"]');
          if (!meta) {
            meta = document.createElement("meta");
            meta.setAttribute("name", "robots");
            document.head.appendChild(meta);
          }
          meta.setAttribute("content", detail.seo.robots);
        }
      })
      .catch((err: Error) => {
        if (!cancelled) {
          setError(err.message);
        }
      });
    // 互动状态（未登录也返回收藏数）
    apiPostState(postId)
      .then((state) => {
        if (cancelled) {
          return;
        }
        setLiked(state.liked);
        setFavorited(state.favorited);
        setFavoriteCount(state.favorite_count);
      })
      .catch(() => {
        // 状态拉取失败不阻塞页面
      });
    return () => {
      cancelled = true;
      // 复原 SEO 输出（标题 + robots meta，防切换/返回后残留）
      document.title = previousTitle;
      if (previousRobots === null) {
        document.querySelector('meta[name="robots"]')?.remove();
      } else {
        let meta = document.querySelector('meta[name="robots"]');
        if (!meta) {
          meta = document.createElement("meta");
          meta.setAttribute("name", "robots");
          document.head.appendChild(meta);
        }
        meta.setAttribute("content", previousRobots);
      }
    };
  }, [postId, user?.id]); // user 恢复后重载（is_author 纠偏）

  // 点赞/取消（未登录提示登录）
  const handleLike = async () => {
    if (!user) {
      setError("请先登录后再点赞");
      return;
    }
    if (reacting) {
      return;
    }
    setReacting(true);
    try {
      if (liked) {
        const result = await apiUnlikePost(postId);
        setLikeCount(result.like_count);
        setLiked(false);
      } else {
        const result = await apiLikePost(postId);
        setLikeCount(result.like_count);
        setLiked(true);
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "操作失败");
    } finally {
      setReacting(false);
    }
  };

  // 收藏/取消
  const handleFavorite = async () => {
    if (!user) {
      setError("请先登录后再收藏");
      return;
    }
    if (reacting) {
      return;
    }
    setReacting(true);
    try {
      if (favorited) {
        const result = await apiUnfavoritePost(postId);
        setFavoriteCount(result.favorite_count);
        setFavorited(false);
      } else {
        const result = await apiFavoritePost(postId);
        setFavoriteCount(result.favorite_count);
        setFavorited(true);
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "操作失败");
    } finally {
      setReacting(false);
    }
  };

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-[720px] flex-1 px-4 py-6 pb-20">
        {/* 移动端顶部：月言 + 返回（设计稿 M/帖子详情） */}
        <div className="mb-4 flex items-center justify-between md:hidden">
          <span className="font-display text-lg font-bold text-ink">月言</span>
          <button
            type="button"
            onClick={() => router.back()}
            className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink"
          >
            返回
          </button>
        </div>
        {/* 加载失败（不存在/无权限统一 404 语义） */}
        {error && (
          <div className="py-20 text-center">
            <p className="text-lg text-ink">{error}</p>
            <Link href="/" className="mt-4 inline-block text-sm text-glow hover:underline">
              ← 返回首页
            </Link>
          </div>
        )}

        {/* 加载骨架 */}
        {!post && !error && (
          <div className="space-y-4">
            <div className="h-16 animate-pulse rounded-lg bg-muted" aria-hidden />
            <div className="h-48 animate-pulse rounded-lg bg-muted" aria-hidden />
          </div>
        )}

        {post && (
          <>
            {/* 作者行（设计稿：林月 / 2 小时前 · 公开） */}
            <div className="flex items-center gap-3">
              <Link
                href={`/users/${post.author.id}`}
                className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-muted font-display text-base text-ink-2"
              >
                {post.author.nickname.charAt(0) || "月"}
              </Link>
              <div className="min-w-0">
                <Link
                  href={`/users/${post.author.id}`}
                  className="block truncate text-sm font-medium text-ink hover:text-glow"
                >
                  {post.author.nickname}
                </Link>
                {/* 相对时间 + 可见性（设计稿：2 小时前 · 公开） */}
                <p className="text-xs text-ink-3">
                  {post.published_at ? timeAgo(post.published_at) : "草稿"}
                  {" · "}
                  {post.visibility === "public" ? "公开" : post.visibility === "followers" ? "仅关注者" : "私密"}
                </p>
              </div>
              {/* 作者可编辑（M1.5 徽标；走查纠偏：编辑按钮 → compose?edit= 设计稿《编辑帖子》） */}
              {post.is_author && (
                <span className="ml-auto flex items-center gap-2">
                  <Link
                    href={`/compose?edit=${post.id}`}
                    className="rounded-full border border-line px-3 py-1 text-xs text-ink-2 transition-colors hover:text-ink"
                  >
                    编辑
                  </Link>
                  <span className="rounded-full bg-accent-soft px-3 py-1 text-xs text-glow">我的帖子</span>
                </span>
              )}
            </div>

            {/* 完整正文（M5 富文本：html=消毒渲染，markdown=Markdown 渲染） */}
            <div className="mt-5">
              <PostContent content={post.content} format={post.content_format} />
            </div>

            {/* 图片：默认网格（点击开灯箱）；自定义风格按 ImageGallery 渲染 */}
            {post.content_type === "image" && post.media.length > 0 && (
              <div className="mt-4">
                {post.gallery_style ? (
                  <ImageGallery media={post.media} style={post.gallery_style as GalleryStyle} />
                ) : (
                  <div className="grid grid-cols-2 gap-1 overflow-hidden rounded-lg">
                    {post.media.map((m, i) => (
                      <button
                        key={m.id}
                        type="button"
                        onClick={() => setLightboxIndex(i)}
                        className="block w-full"
                      >
                        {/* eslint-disable-next-line @next/next/no-img-element */}
                        <img
                          src={m.url}
                          alt=""
                          loading="lazy"
                          className={`aspect-[4/3] w-full object-cover ${
                            post.media.length === 1 ? "col-span-2" : ""
                          }`}
                        />
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* 音频：播放器（设计稿：1:24 / 4:08） */}
            {post.content_type === "audio" && post.media.length > 0 && (
              <div className="mt-4">
                <AudioPlayer src={post.media[0].url} duration={0} autoplay={false} />
              </div>
            )}

            {/* 视频：播放器（设计稿《视频播放》：02:14 / 06:42 + 1080p；M2） */}
            {post.content_type === "video" && post.media.length > 0 && (
              <div className="mt-4">
                <video
                  src={post.media[0].url}
                  controls
                  preload="metadata"
                  playsInline
                  className="aspect-video w-full rounded-lg bg-black object-contain"
                />
              </div>
            )}

            {/* 标签 */}
            {post.tags.length > 0 && (
              <div className="mt-4 flex flex-wrap gap-2">
                {post.tags.map((tag) => (
                  <Link
                    key={tag.slug}
                    href={`/topics/${encodeURIComponent(tag.name.replace("#", ""))}`}
                    className="text-sm text-glow hover:underline"
                  >
                    {tag.name}
                  </Link>
                ))}
              </div>
            )}

            {/* 互动条（设计稿：128 赞 / 24 评论 / 6 收藏；M1.4 真实落库） */}
            <div className="mt-6 flex items-center gap-8 border-y border-line py-4 text-sm text-ink-2">
              {/* 点赞（未登录提示登录） */}
              <button
                type="button"
                onClick={() => void handleLike()}
                className={`flex items-center gap-1.5 transition-colors ${
                  liked ? "text-like" : "hover:text-ink"
                }`}
              >
                <span aria-hidden>{liked ? "♥" : "♡"}</span>
                <span>{likeCount}</span>
              </button>
              <a href="#comments" className="flex items-center gap-1.5 hover:text-ink">
                <span aria-hidden>💬</span>
                <span>{post.comment_count}</span>
              </a>
              {/* 收藏（真实落库） */}
              <button
                type="button"
                onClick={() => void handleFavorite()}
                className={`flex items-center gap-1.5 transition-colors ${
                  favorited ? "text-glow" : "hover:text-ink"
                }`}
              >
                <span aria-hidden>{favorited ? "🔖" : "▢"}</span>
                <span>{favoriteCount}</span>
              </button>
              {/* 分享（设计稿《分享面板》：复制链接/生成海报/私信好友/二维码） */}
              <button
                type="button"
                onClick={() => setShareOpen(true)}
                className="flex items-center gap-1.5 transition-colors hover:text-ink"
              >
                <span aria-hidden>↗</span>
                <span>分享</span>
              </button>
              {/* 举报（M2：设计稿《举报》弹层） */}
              <button
                type="button"
                onClick={() => {
                  if (!user) {
                    setError("请先登录后再举报");
                    return;
                  }
                  setReportOpen(true);
                }}
                className="ml-auto flex items-center gap-1.5 text-xs text-ink-3 transition-colors hover:text-like"
              >
                <span aria-hidden>⚠</span>
                <span>举报</span>
              </button>
            </div>
            {/* 互动提示（未登录提示等） */}
            {error && (
              <p className="mt-3 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
                {error}
              </p>
            )}

            {/* 评论区（M1.4：楼中楼 + 匿名评论） */}
            <div className="mt-6">
              <CommentSection postId={postId} initialCount={post.comment_count} />
            </div>
            {/* 插件扩展点：post.footer（M3.6，文章页脚） */}
            <PluginSlot slot="post.footer" />
          </>
        )}
      </main>

      {/* 举报弹层（M2） */}
      {reportOpen && post && (
        <ReportDialog
          targetType="post"
          targetId={post.id}
          onClose={() => setReportOpen(false)}
        />
      )}

      {/* 分享面板（#17：设计稿《分享面板》；作者 · 话题 元信息 + 图片帖海报图） */}
      {shareOpen && post && (
        <SharePanel
          title={post.title || post.summary || "分享帖子"}
          content={post.content}
          media={post.media.filter((m) => m.type === "image").map((m) => m.url)}
          meta={`${post.author.nickname}${post.tags.length > 0 ? ` · ${post.tags.map((t) => t.name).join(" ")}` : ""}`}
          shareUrl={typeof window !== "undefined" ? `${window.location.origin}/posts/${post.id}` : `/posts/${post.id}`}
          onClose={() => setShareOpen(false)}
        />
      )}

      {/* 灯箱 */}
      {lightboxIndex !== null && post && (
        <Lightbox
          images={post.media}
          index={lightboxIndex}
          onClose={() => setLightboxIndex(null)}
        />
      )}

      <MobileTabbar />
    </div>
  );
}
