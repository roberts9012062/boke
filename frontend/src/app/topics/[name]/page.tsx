// src/app/topics/[name]/page.tsx
// 话题详情页（设计稿 D/冷月/话题详情 1400×1400）：
// #话题名 + 描述 + 关注话题按钮 + 统计（浏览/帖子/关注）+ 最新 Tab + 帖子流。
"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { PostCard } from "@/components/post-card";
import { PostCardSkeletonList } from "@/components/post-card-skeleton";
import { apiFollowTopic, apiTopicDetail, apiTopicPosts, apiUnfollowTopic, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import type { PostSummary, TopicDTO } from "@/types/api";

// TopicDetailPage 话题详情页。
export default function TopicDetailPage() {
  const params = useParams<{ name: string }>();
  const name = decodeURIComponent(params.name);
  const { user } = useAuth();

  const [topic, setTopic] = useState<TopicDTO | null>(null);
  const [posts, setPosts] = useState<PostSummary[]>([]);
  const [following, setFollowing] = useState<boolean>(false);
  const [sort, setSort] = useState<string>("latest"); // latest=最新 / hot=热门（设计稿 Tab）
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string>("");

  // 加载话题详情与帖子流
  useEffect(() => {
    if (!name) {
      setError("话题不存在");
      return;
    }
    setLoading(true);
    apiTopicDetail(name)
      .then((detail) => {
        setTopic(detail);
        setFollowing(detail.following);
      })
      .catch((err: Error) => setError(err.message));
    apiTopicPosts(name, sort)
      .then((result) => setPosts(result.items))
      .catch(() => setPosts([]))
      .finally(() => setLoading(false));
  }, [name, sort]);

  // 关注/取消话题
  const handleFollow = async () => {
    if (!user) {
      setError("请先登录后再关注话题");
      return;
    }
    try {
      if (following) {
        await apiUnfollowTopic(name);
        setFollowing(false);
        setTopic((t) => (t ? { ...t, following: false, follow_count: Math.max(t.follow_count - 1, 0) } : t));
      } else {
        await apiFollowTopic(name);
        setFollowing(true);
        setTopic((t) => (t ? { ...t, following: true, follow_count: t.follow_count + 1 } : t));
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "操作失败");
    }
  };

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-[720px] flex-1 px-4 py-6 pb-20">
        {/* 加载失败 */}
        {error && (
          <div className="py-20 text-center">
            <p className="text-lg text-ink">{error}</p>
            <Link href="/topics" className="mt-4 inline-block text-sm text-glow hover:underline">
              ← 返回话题列表
            </Link>
          </div>
        )}

        {/* 话题头部（设计稿：#月色随笔 + 描述 + 关注话题 + 统计） */}
        {topic && (
          <section className="rounded-lg border border-line bg-elevated p-6">
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0">
                <h1 className="font-display text-2xl font-semibold text-ink">{topic.name}</h1>
                {topic.description && (
                  <p className="mt-2 text-sm leading-relaxed text-ink-2">{topic.description}</p>
                )}
              </div>
              {/* 关注话题按钮（设计稿文案） */}
              <button
                type="button"
                onClick={() => void handleFollow()}
                className={`shrink-0 rounded-full px-5 py-2 text-sm transition-colors ${
                  following
                    ? "border border-line text-ink-2 hover:text-ink"
                    : "bg-accent text-on-accent hover:opacity-90"
                }`}
              >
                {following ? "已关注" : "关注话题"}
              </button>
            </div>
            {/* 统计（设计稿：浏览/帖子/关注） */}
            <div className="mt-4 flex divide-x divide-line border-t border-line pt-4 text-center">
              <div className="flex-1">
                <p className="text-base font-semibold text-ink">
                  {topic.browse_count.toLocaleString()}
                </p>
                <p className="mt-0.5 text-xs text-ink-3">浏览</p>
              </div>
              <div className="flex-1">
                <p className="text-base font-semibold text-ink">{topic.post_count}</p>
                <p className="mt-0.5 text-xs text-ink-3">帖子</p>
              </div>
              <div className="flex-1">
                <p className="text-base font-semibold text-ink">{topic.follow_count}</p>
                <p className="mt-0.5 text-xs text-ink-3">关注</p>
              </div>
            </div>
          </section>
        )}

        {/* 排序 Tab（设计稿：最新/热门/精华；精华 M2 置灰） */}
        <div className="mt-5 flex gap-2 border-b border-line pb-3">
          {[
            { key: "latest", label: "最新" },
            { key: "hot", label: "热门" },
            { key: "essence", label: "精华", disabled: true },
          ].map((t) => (
            <button
              key={t.key}
              type="button"
              disabled={t.disabled}
              onClick={() => setSort(t.key)}
              className={`rounded-full px-4 py-1.5 text-sm transition-colors ${
                t.disabled
                  ? "cursor-not-allowed text-ink-3/60"
                  : sort === t.key
                    ? "bg-accent-soft font-medium text-glow"
                    : "bg-muted text-ink-2 hover:text-ink"
              }`}
            >
              {t.label}
              {t.disabled && <span className="ml-1 text-[10px]">M2</span>}
            </button>
          ))}
        </div>

        {/* 帖子流 */}
        <div className="mt-6">
          {loading ? (
            <PostCardSkeletonList count={3} />
          ) : (
            <div className="post-card-list space-y-4">
              {posts.map((post) => (
                <PostCard key={post.id} post={post} />
              ))}
              {posts.length === 0 && (
                <div className="rounded-lg border border-line bg-elevated py-14 text-center">
                  <p className="text-sm text-ink-2">该话题还没有帖子</p>
                </div>
              )}
            </div>
          )}
        </div>
      </main>
      <MobileTabbar />
    </div>
  );
}
