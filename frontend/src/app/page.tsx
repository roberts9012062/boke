// src/app/page.tsx
// 首页（时间线）：桌面三栏（用户卡片 / 时间线 / 热门话题）+ 移动单栏。
// 设计依据：D/冷月/首页（1400）+ M/冷月/首页（390）。
// M1.3：时间线接入真实数据（全部/文/图/音/影过滤 + 分页加载）。
// 文章形态（post_kind=article）渲染 article-card（标题可点击进详情阅读）。
// 大世界（B-4'）：filter=world 时中栏直接渲染中继站聚合流（跨站卡片 + 30s 轮询），
// 不跳独立页面；/world 独立路由保留给移动端底部导航。
"use client";

import { useEffect, useRef, useState } from "react";

import { ArticleCard } from "@/components/article-card";
import { BgmWidget } from "@/components/bgm-widget";
import { DesktopNav } from "@/components/desktop-nav";
import { FeedTabs } from "@/components/feed-tabs";
import { HotTopics } from "@/components/hot-topics";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { Reveal } from "@/components/motion/reveal";
import { PostCard } from "@/components/post-card";
import { PostCardSkeletonList } from "@/components/post-card-skeleton";
import { UserCard } from "@/components/user-card";
import { WorldCard } from "@/components/world-cards";
import { useAuth } from "@/lib/auth";
import { apiFollowingFeed, apiTimeline } from "@/lib/api";
import { apiWorldContents, type RelayCacheItem } from "@/lib/api-relay";
import type { PageResult, PostSummary } from "@/types/api";

// worldFilterKey 大世界 Tab 的 filter 值（占据原"全部"左侧的首个位置）。
const worldFilterKey = "world";

// worldPageLimit 大世界每页条数（按发布时间倒序，before 游标分页）。
const worldPageLimit = 20;

// HomePage 首页：三栏布局 + 真实时间线数据 + 推荐/关注流切换（设计稿关注流）。
export default function HomePage() {
  const { user } = useAuth();
  // 时间线数据状态
  const [posts, setPosts] = useState<PostSummary[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [loading, setLoading] = useState<boolean>(true);
  const [feed, setFeed] = useState<string>("timeline"); // timeline=推荐 / following=关注
  const [filter, setFilter] = useState<string>(""); // 空 = 全部；world=大世界；article/image/audio/video=形态过滤
  const [page, setPage] = useState<number>(1);
  const [loadingMore, setLoadingMore] = useState<boolean>(false);
  const [feedError, setFeedError] = useState<string>("");
  // 大世界数据状态（独立于时间线：切回时间线时状态保留）
  const [worldItems, setWorldItems] = useState<RelayCacheItem[]>([]);
  const [worldLoading, setWorldLoading] = useState<boolean>(false);
  const [worldDone, setWorldDone] = useState<boolean>(false);
  const worldLoadingMoreRef = useRef<boolean>(false);
  // 同步加载守卫：scroll 事件快速连续触发时，state 守卫是异步的，需 ref 立即阻断并发重复请求
  const loadingMoreRef = useRef<boolean>(false);

  const inWorld = filter === worldFilterKey;

  // 拉取一页时间线（过滤条件解释：article → kind 参数，其余 → type 参数；纯读取函数）
  const fetchPage = (pageNum: number): Promise<PageResult<PostSummary>> => {
    if (feed === "following") {
      return apiFollowingFeed(pageNum);
    }
    return apiTimeline(
      filter === "article" ? { kind: "article", page: pageNum } : { type: filter, page: pageNum },
    );
  };

  // 加载时间线（feed/过滤条件变化时重置分页；大世界模式不触达本接口）
  useEffect(() => {
    if (inWorld) {
      return;
    }
    let cancelled = false;
    setLoading(true);
    setFeedError("");
    fetchPage(1)
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

  // 大世界：首屏加载 + 30s 轮询刷新（M0 轮询模式；只刷新首屏不打断浏览位置）
  useEffect(() => {
    if (!inWorld) {
      return;
    }
    let cancelled = false;
    const load = () => {
      setWorldLoading(true);
      apiWorldContents({ limit: worldPageLimit })
        .then((d) => {
          if (cancelled) return;
          setWorldItems(d.items ?? []);
          setWorldDone((d.items ?? []).length < worldPageLimit);
        })
        .catch(() => {
          if (!cancelled) setWorldItems([]);
        })
        .finally(() => {
          if (!cancelled) setWorldLoading(false);
        });
    };
    load();
    const timer = setInterval(load, 30000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [inWorld]);

  // 大世界加载更多：before = 当前最旧一条的发布时间（游标分页，按 id 去重）
  const loadMoreWorld = async () => {
    if (worldLoadingMoreRef.current || worldDone || worldItems.length === 0) {
      return;
    }
    worldLoadingMoreRef.current = true;
    setLoadingMore(true);
    try {
      const oldest = worldItems[worldItems.length - 1];
      const d = await apiWorldContents({ before: Math.floor(new Date(oldest.published_at).getTime() / 1000), limit: worldPageLimit });
      const fresh = (d.items ?? []).filter((it) => !worldItems.some((prev) => prev.content_id === it.content_id));
      if (fresh.length < worldPageLimit) {
        setWorldDone(true);
      }
      setWorldItems((prev) => [...prev, ...fresh]);
    } catch {
      // 加载失败静默（下次滚动重试）
    } finally {
      worldLoadingMoreRef.current = false;
      setLoadingMore(false);
    }
  };

  // 加载更多（滚动到底部触发；ref 同步守卫防并发，拼接按 id 去重兜底）
  const loadMore = async () => {
    if (inWorld) {
      await loadMoreWorld();
      return;
    }
    if (loadingMoreRef.current || posts.length >= total) {
      return;
    }
    loadingMoreRef.current = true;
    setLoadingMore(true);
    try {
      const result = await fetchPage(page + 1);
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
  }, [posts, total, loadingMore, filter, feed, page, worldItems, worldDone]);

  return (
    <div className="flex min-h-screen flex-col">
      {/* 顶部导航（桌面） */}
      <DesktopNav />

      {/* 主体：三栏布局（<768px 时隐藏左右栏，仅时间线） */}
      <main className="mx-auto flex w-full max-w-[1400px] flex-1 gap-6 px-6 py-6">
        {/* 左栏：用户卡片 */}
        <UserCard />

        {/* 中栏：时间线 / 大世界（filter=world 时切换数据源，同位置渲染） */}
        <section className="min-w-0 flex-1">
          <div className="mb-4 flex items-center justify-between">
            <h1 className="font-display text-xl font-semibold text-ink">
              {inWorld ? "🌐 大世界" : "时间线"}
            </h1>
            {/* 推荐/关注 切换（设计稿关注流；大世界模式下隐藏——两者数据源互斥） */}
            {!inWorld && (
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
            )}
            {inWorld && (
              <span className="text-xs text-ink-3">跨站聚合 · 每 30 秒自动更新</span>
            )}
          </div>
          {/* 过滤 Tab：大世界 / 全部 / 文 / 图 / 音 / 影（选中即过滤） */}
          <FeedTabs active={filter} onChange={setFilter} />

          {/* 关注流未登录提示 */}
          {feedError && (
            <p className="mt-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like">{feedError}</p>
          )}

          {/* 加载中：骨架屏 */}
          {loading && !inWorld && (
            <div className="mt-4">
              <PostCardSkeletonList count={3} />
            </div>
          )}

          {/* 大世界聚合流（中继站分发内容，中栏原位渲染） */}
          {inWorld && (
            <div className="mt-4 space-y-4">
              {worldLoading && worldItems.length === 0 && (
                <div>
                  <PostCardSkeletonList count={3} />
                </div>
              )}
              {!worldLoading && worldItems.length === 0 && (
                <div className="rounded-lg border border-line bg-elevated py-16 text-center">
                  <p className="font-display text-lg text-ink">大世界还很安静</p>
                  <p className="mt-2 text-sm text-ink-2">
                    站长尚未开启「大世界」或暂无跨站内容，发布一条试试
                  </p>
                </div>
              )}
              {worldItems.map((item) => (
                <Reveal key={item.content_id}>
                  <WorldCard item={item} />
                </Reveal>
              ))}
              {worldItems.length > 0 && !worldDone && (
                <p className="py-4 text-center text-xs text-ink-3">
                  {loadingMore ? "加载中…" : "继续滚动加载更多"}
                </p>
              )}
            </div>
          )}

          {/* 帖子流（Reveal 滚动进场：滚动加载更多的新卡片淡入上移） */}
          {!loading && !inWorld && (
            <div className="mt-4 post-card-list space-y-4">
              {posts.map((post) => (
                <Reveal key={post.id}>
                  {/* 按形态分发：文章（标题点击进详情阅读）/ 说说 */}
                  {post.post_kind === "article" ? <ArticleCard post={post} /> : <PostCard post={post} />}
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

      {/* 首页背景音乐悬浮播放器（QQ 音乐插件设置开启后显示） */}
      <BgmWidget />
    </div>
  );
}
