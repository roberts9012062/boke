// src/app/users/[id]/page.tsx
// 用户主页（设计稿 D/冷月/他人主页 + M/冷月/他人主页）：
// 资料头（头像/昵称/@账号/简介/统计）+ 关注按钮 + Tab（帖子/媒体/赞过）。
"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { PostCard } from "@/components/post-card";
import { PostCardSkeletonList } from "@/components/post-card-skeleton";
import { Avatar } from "@/components/ui/avatar";
import {
  apiFollowUser,
  apiLikedPosts,
  apiOpenConversation,
  apiUnfollowUser,
  apiUserPosts,
  ApiError,
  get,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import type { PostSummary, UserProfile } from "@/types/api";

// Tab 配置（设计稿：帖子/媒体/赞过）
const TABS = [
  { key: "posts", label: "帖子" },
  { key: "media", label: "媒体" },
  { key: "liked", label: "赞过" },
] as const;

// UserProfilePage 用户主页。
export default function UserProfilePage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const userId = Number(params.id);
  const { user } = useAuth();

  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [tab, setTab] = useState<string>("posts");
  const [posts, setPosts] = useState<PostSummary[]>([]);
  const [following, setFollowing] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);

  // 加载资料
  useEffect(() => {
    if (!userId) {
      setError("用户不存在");
      return;
    }
    setLoading(true);
    get<UserProfile>(`/users/${userId}`)
      .then(setProfile)
      .catch((err: Error) => setError(err.message));
  }, [userId]);

  // 加载 Tab 内容（帖子/媒体/赞过）
  useEffect(() => {
    if (!userId || error) {
      return;
    }
    let cancelled = false;
    setLoading(true);
    const loader =
      tab === "liked"
        ? apiLikedPosts(userId)
        : apiUserPosts(userId, tab === "media" ? "image" : "");
    loader
      .then((r) => {
        if (!cancelled) setPosts(r.items);
      })
      .catch(() => {
        if (!cancelled) setPosts([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [userId, tab, error]);

  // 关注/取关
  const handleFollow = async () => {
    if (!user) {
      setError("请先登录后再关注");
      return;
    }
    try {
      if (following) {
        await apiUnfollowUser(userId);
        setFollowing(false);
      } else {
        await apiFollowUser(userId);
        setFollowing(true);
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "操作失败");
    }
  };

  // 发起私信（M2：创建/打开会话后进入会话详情）
  const handleMessage = async () => {
    if (!user) {
      setError("请先登录后再发私信");
      return;
    }
    try {
      const conversation = await apiOpenConversation(userId);
      router.push(`/messages/${conversation.id}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "发起会话失败");
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
            <Link href="/" className="mt-4 inline-block text-sm text-glow hover:underline">
              ← 返回首页
            </Link>
          </div>
        )}

        {/* 资料头 */}
        {profile && (
          <section className="rounded-lg border border-line bg-elevated p-6">
            <div className="flex items-center gap-4">
              <Avatar name={profile.nickname} url={profile.avatar_url} className="h-16 w-16 text-2xl" />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <p className="font-display text-lg font-semibold text-ink">{profile.nickname}</p>
                  {profile.role === "admin" && (
                    <span className="rounded-full bg-accent-soft px-2 py-0.5 text-xs text-glow">
                      站长
                    </span>
                  )}
                </div>
                <p className="mt-0.5 text-sm text-ink-3">@{profile.username}</p>
                {profile.bio && <p className="mt-1 text-sm text-ink-2">{profile.bio}</p>}
              </div>
              {/* 操作按钮（本人不显示）：关注/取关 + 私信（M2） */}
              {user && user.id !== userId && (
                <div className="flex shrink-0 gap-2">
                  <button
                    type="button"
                    onClick={() => void handleMessage()}
                    className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 transition-colors hover:text-ink"
                  >
                    私信
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleFollow()}
                    className={`rounded-full px-5 py-2 text-sm transition-colors ${
                      following
                        ? "border border-line text-ink-2 hover:text-ink"
                        : "bg-accent text-on-accent hover:opacity-90"
                    }`}
                  >
                    {following ? "已关注" : "关注"}
                  </button>
                </div>
              )}
            </div>
            {/* 统计（设计稿个人主页：帖子 / 获赞 / 浏览） */}
            <div className="mt-5 flex divide-x divide-line border-t border-line pt-4">
              {[
                { label: "帖子", value: profile.post_count },
                { label: "获赞", value: profile.like_count },
                { label: "浏览", value: profile.view_count },
              ].map((stat) => (
                <div key={stat.label} className="flex-1 px-2 text-center first:pl-0 last:pr-0">
                  <p className="text-base font-semibold text-ink">
                    {stat.value >= 10000
                      ? `${(stat.value / 10000).toFixed(1).replace(/\.0$/, "")}w`
                      : stat.value.toLocaleString()}
                  </p>
                  <p className="mt-0.5 text-xs text-ink-3">{stat.label}</p>
                </div>
              ))}
            </div>
            {/* 粉丝/关注入口（设计稿未占用统计区，作为资料头下链接行） */}
            <div className="mt-3 flex gap-4 text-xs text-ink-3">
              <Link href={`/users/${userId}/followers`} className="transition-colors hover:text-glow">
                粉丝 {profile.follower_count}
              </Link>
              <Link href={`/users/${userId}/following`} className="transition-colors hover:text-glow">
                关注 {profile.following_count}
              </Link>
            </div>
          </section>
        )}

        {/* Tab（帖子/媒体/赞过） */}
        {profile && (
          <div className="mt-5 flex gap-2 border-b border-line pb-3">
            {TABS.map((t) => (
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
        )}

        {/* 内容区 */}
        <div className="mt-4">
          {loading ? (
            <PostCardSkeletonList count={2} />
          ) : (
            <div className="post-card-list space-y-4">
              {posts.map((post) => (
                <PostCard key={post.id} post={post} />
              ))}
              {posts.length === 0 && (
                <div className="rounded-lg border border-line bg-elevated py-14 text-center">
                  <p className="text-sm text-ink-2">这里还没有内容</p>
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
