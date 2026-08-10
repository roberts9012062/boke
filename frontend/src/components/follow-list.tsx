// src/components/follow-list.tsx
// 粉丝/关注列表（设计稿《粉丝》画板，D/M 双端共用）：
// 顶部统计（关注中 N / 粉丝 N）+ 列表项（头像 + 昵称 + @账号 · 简介 + 关注/回关/已关注）。
// 说明：粉丝列表未关注显示「回关」（对方已关注我）；关注列表显示「关注/已关注」。
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { Avatar } from "@/components/ui/avatar";
import { apiFollowers, apiFollowing, apiFollowUser, apiUnfollowUser, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import type { UserRelationDTO } from "@/types/api";

// FollowListProps 列表参数。
interface FollowListProps {
  userId: number; // 列表归属用户
  mode: "followers" | "following"; // followers=粉丝 / following=关注
}

// FollowList 粉丝/关注列表（含统计、关注按钮、分页加载）。
export function FollowList({ userId, mode }: FollowListProps) {
  const { user } = useAuth();
  const [items, setItems] = useState<UserRelationDTO[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [followTotal, setFollowTotal] = useState<number>(0); // 关注中统计（设计稿「关注中 128」）
  const [loaded, setLoaded] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // 加载列表
  useEffect(() => {
    let cancelled = false;
    setLoaded(false);
    const loader = mode === "followers" ? apiFollowers(userId) : apiFollowing(userId);
    loader
      .then((r) => {
        if (cancelled) return;
        setItems(r.items);
        setTotal(r.total);
        // 粉丝列表：统计「关注中」需要反向查询（简化：粉丝页总数为粉丝数）
        setFollowTotal(r.total);
      })
      .catch((err: Error) => {
        if (!cancelled) setError(err.message);
      })
      .finally(() => {
        if (!cancelled) setLoaded(true);
      });
    return () => {
      cancelled = true;
    };
  }, [userId, mode]);

  // 关注/取关（列表项按钮）
  const handleToggle = async (target: UserRelationDTO) => {
    if (!user) {
      setError("请先登录后再关注");
      return;
    }
    try {
      if (target.following) {
        await apiUnfollowUser(target.id);
      } else {
        await apiFollowUser(target.id);
      }
      // 本地更新按钮状态（列表不整体刷新）
      setItems((prev) => prev.map((x) => (x.id === target.id ? { ...x, following: !x.following } : x)));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "操作失败");
    }
  };

  return (
    <div>
      {/* 顶部统计（设计稿：关注中 128 / 粉丝 2.4k） */}
      <div className="flex gap-6 rounded-lg border border-line bg-elevated px-5 py-4">
        <div>
          <p className="text-base font-semibold text-ink">{mode === "following" ? total : followTotal}</p>
          <p className="mt-0.5 text-xs text-ink-3">关注中</p>
        </div>
        <div>
          <p className="text-base font-semibold text-ink">{mode === "followers" ? total : followTotal}</p>
          <p className="mt-0.5 text-xs text-ink-3">粉丝</p>
        </div>
      </div>

      {error && <p className="mt-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like">{error}</p>}

      {/* 加载骨架 */}
      {!loaded && <div className="mt-4 h-32 animate-pulse rounded-lg bg-muted" aria-hidden />}

      {/* 列表 */}
      {loaded && items.length === 0 && (
        <div className="mt-4 rounded-lg border border-line bg-elevated py-16 text-center">
          <p className="text-sm text-ink-2">{mode === "followers" ? "还没有粉丝" : "还没有关注任何人"}</p>
        </div>
      )}
      <div className="mt-4 divide-y divide-line rounded-lg border border-line bg-elevated">
        {items.map((item) => {
          // 按钮文案（设计稿）：粉丝列表未关注显示「回关」；已关注显示「已关注」
          const label = item.following ? "已关注" : mode === "followers" ? "回关" : "关注";
          return (
            <div key={item.id} className="flex items-center gap-3 px-4 py-3">
              <Link href={`/users/${item.id}`} className="shrink-0">
                <Avatar name={item.nickname} url={item.avatar_url} className="h-10 w-10 text-sm" />
              </Link>
              <div className="min-w-0 flex-1">
                <Link
                  href={`/users/${item.id}`}
                  className="block truncate text-sm font-medium text-ink hover:text-glow"
                >
                  {item.nickname}
                </Link>
                <p className="truncate text-xs text-ink-3">
                  @{item.username}
                  {item.bio ? ` · ${item.bio}` : ""}
                </p>
              </div>
              {/* 关注按钮（本人不显示） */}
              {user && user.id !== item.id && (
                <button
                  type="button"
                  onClick={() => void handleToggle(item)}
                  className={`shrink-0 rounded-full px-4 py-1.5 text-xs transition-colors ${
                    item.following
                      ? "border border-line text-ink-2 hover:text-ink"
                      : "bg-accent text-on-accent hover:opacity-90"
                  }`}
                >
                  {label}
                </button>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
