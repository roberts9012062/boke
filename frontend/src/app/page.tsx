// src/app/page.tsx
// 首页（时间线）：桌面三栏（用户卡片 / 时间线 / 热门话题）+ 移动单栏。
// 设计依据：D/冷月/首页（1400）+ M/冷月/首页（390）。
// M1.3：时间线接入真实数据（全部/图/音/影过滤 + 分页加载）。
"use client";

import { useEffect, useRef, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { FeedTabs } from "@/components/feed-tabs";
import { HotTopics } from "@/components/hot-topics";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { Reveal } from "@/components/motion/reveal";
import { PostCard } from "@/components/post-card";
import { PostCardSkeletonList } from "@/components/post-card-skeleton";
import { UserCard } from "@/components/user-card";
import { apiFollowingFeed, apiTimeline } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import type { PostSummary } from "@/types/api";

// HomePage 首页：三栏布局 + 真实时间线数据 + 推荐/关注流切换（设计稿关注流）。
export default function HomePage() {
  const { user } = useAuth();
  // 时间线数据状态
  const [posts, setPosts] = useState<PostSummary[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [loading, setLoading] = useState<boolean>(true);
  const [feed, setFeed] = useState<string>("timeline"); // timeline=推荐 / following=关注
  const [filter, setFilter] = useState<string>(""); // 空 = 全部；image/audio/video
  const [page, setPage] = useState<number>(1);
  const [loadingMore, setLoadingMore] = useState<boolean>(false);
  const [feedError, setFeedError] = useState<string>("");
  // 同步加载守卫：scroll 事件快速连续触发时，state 守卫是异步的，需 ref 立即阻断并发重复请求
  const loadingMoreRef = useRef<boolean>(false);

  // 加载时间线（feed/过滤条件变化时重置分页）
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setFeedError("");
    const loader = feed === "following" ? apiFollowingFeed(1) : apiTimeline({ type: filter, page: 1 });
    loader
      .then((result) => {
        if (cancelled) return;
        setPosts(result.items);
        setTotal(result.total);
        setPage(1);
      })
      .catch(() => {
        if (!cancelled) {
          setPosts([]);
          if (feed === "following") {
            setFeedError("关注流需要登录后查看");
          }
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [feed, filter, user]);

  // 加载更多（滚动到底部触发；ref 同步守卫防并发，拼接按 id 去重兜底）
  const loadMore = async () => {
    if (loadingMoreRef.current || posts.length >= total) {
      return;
    }
    loadingMoreRef.current = true;
    setLoadingMore(true);
    try {
      const result =
        feed === "following"
          ? await apiFollowingFeed(page + 1)
          : await apiTimeline({ type: filter, page: page + 1 });
      setPosts((prev) => {
        // 按 id 去重（防御性：极端竞态下避免同一条重复出现，消除 key 冲突）
        const seen = new Set<number>(prev.map((p) => p.id));
        const fresh = result.items.filter((item) => !seen.has(item.id));
        return [...prev, ...fresh];
      });
      setPage((p) => p + 1);
      setTotal(result.total);
    } catch {
      // 加载失败静默（下次滚动重试）
    } finally {
      loadingMoreRef.current = false;
      setLoadingMore(false);
    }
  };

  // 滚动到底加载更多
  useEffect(() => {
    const onScroll = () => {
      if (window.innerHeight + window.scrollY >= document.body.offsetHeight - 200) {
        void loadMore();
      }
    };
    window.addEventListener("scroll", onScroll);
    return () => window.removeEventListener("scroll", onScroll);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [posts, total, loadingMore, filter, feed, page]);

  return (
    <div className="flex min-h-screen flex-col">
      {/* 顶部导航（桌面） */}
      <DesktopNav />

      {/* 主体：三栏布局（<768px 时隐藏左右栏，仅时间线） */}
      <main className="mx-auto flex w-full max-w-[1400px] flex-1 gap-6 px-6 py-6">
        {/* 左栏：用户卡片 */}
        <UserCard />

        {/* 中栏：时间线 */}
        <section className="min-w-0 flex-1">
          <div className="mb-4 flex items-center justify-between">
            <h1 className="font-display text-xl font-semibold text-ink">时间线</h1>
            {/* 推荐/关注 切换（设计稿关注流） */}
            <div className="flex gap-1 rounded-full border border-line p-0.5">
              {[
                { key: "timeline", label: "推荐" },
                { key: "following", label: "关注" },
              ].map((f) => (
                <button
                  key={f.key}
                  type="button"
                  onClick={() => {
                    // 切到关注流时重置类型过滤（关注流接口不支持 type 参数）
                    setFeed(f.key);
                    setFilter("");
                  }}
                  className={`rounded-full px-4 py-1 text-sm transition-colors ${
                    feed === f.key
                      ? "bg-accent-soft font-medium text-glow"
                      : "text-ink-2 hover:text-ink"
                  }`}
                >
                  {f.label}
                </button>
              ))}
            </div>
          </div>
          {/* 过滤 Tab：全部 / 图 / 音 / 影（选中即过滤） */}
          <FeedTabs active={filter} onChange={setFilter} />

          {/* 关注流未登录提示 */}
          {feedError && (
            <p className="mt-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like">{feedError}</p>
          )}

          {/* 加载中：骨架屏 */}
          {loading && (
            <div className="mt-4">
              <PostCardSkeletonList count={3} />
            </div>
          )}

          {/* 帖子流（Reveal 滚动进场：滚动加载更多的新卡片淡入上移） */}
          {!loading && (
            <div className="mt-4 post-card-list space-y-4">
              {posts.map((post) => (
                <Reveal key={post.id}>
                  <PostCard post={post} />
                </Reveal>
              ))}

              {/* 空状态：无帖子 */}
              {posts.length === 0 && (
                <div className="rounded-lg border border-line bg-elevated py-16 text-center">
                  <p className="font-display text-lg text-ink">还没有帖子</p>
                  <p className="mt-2 text-sm text-ink-2">写下第一条月色短句吧</p>
                </div>
              )}

              {/* 加载更多提示 */}
              {posts.length > 0 && posts.length < total && (
                <p className="py-4 text-center text-xs text-ink-3">
                  {loadingMore ? "加载中…" : "继续滚动加载更多"}
                </p>
              )}
            </div>
          )}
        </section>

        {/* 右栏：热门话题 */}
        <HotTopics />
      </main>

      {/* 移动端底部导航 */}
      <MobileTabbar />
    </div>
  );
}
