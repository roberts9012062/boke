// src/components/user-card.tsx
// 桌面左栏用户卡片（设计稿 D/冷月/首页 左栏）：
// 头像 + 昵称 + @账号 + 简介 + 统计（帖子/获赞/话题）。
// 已登录接入 AuthContext 真实资料（M 后置修复：此前为硬编码假身份）；
// 未登录保留设计稿静态占位（阿月 / @yueyan，仅文案形态示意）。
"use client";

import { useAuth } from "@/lib/auth";
import { Avatar } from "@/components/ui/avatar";

// 未登录时的静态占位数据（与设计稿文案一致：阿月 / @yueyan / 简介 / 128 帖）。
const PLACEHOLDER = {
  nickname: "阿月",
  username: "yueyan",
  bio: "写短句，收声音，偶尔录一点夜色。",
  stats: [128, 1200, 36],
} as const;

// STAT_LABELS 统计项文案（设计稿：帖子/获赞/话题，顺序与数据一致）。
const STAT_LABELS = ["帖子", "获赞", "话题"] as const;

// formatCount 统计数字格式化（≥1000 缩写为 1.2k 形态，与设计稿一致）。
function formatCount(value: number): string {
  if (value >= 1000) {
    const k = value / 1000;
    const rounded = k >= 10 ? Math.round(k) : Math.round(k * 10) / 10;
    return `${rounded}k`;
  }
  return String(value);
}

// UserCard 桌面左栏用户卡片（仅 ≥768px 显示，移动端在「我的」页展示）。
export function UserCard() {
  const { user } = useAuth();
  // 已登录：真实资料；未登录：静态占位（设计稿）
  const nickname = user?.nickname ?? PLACEHOLDER.nickname;
  const username = user?.username ?? PLACEHOLDER.username;
  const bio = user?.bio || PLACEHOLDER.bio;
  const stats: number[] = user
    ? [user.post_count, user.like_count, user.topic_count]
    : [...PLACEHOLDER.stats];

  return (
    <aside className="hidden w-[260px] shrink-0 md:block">
      <section className="rounded-lg border border-line bg-elevated p-5 transition-[box-shadow,border-color] duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] hover:border-accent/40 hover:shadow-[var(--yy-shadow-card)]">
        {/* 头像 + 昵称 + 账号 */}
        <div className="flex items-center gap-3">
          <Avatar name={nickname} url={user?.avatar_url ?? ""} className="h-12 w-12 text-lg" />
          <div className="min-w-0">
            <p className="truncate font-display text-base font-semibold text-ink">{nickname}</p>
            <p className="truncate text-xs text-ink-3">@{username}</p>
          </div>
        </div>

        {/* 简介 */}
        <p className="mt-3 text-sm leading-relaxed text-ink-2">{bio}</p>

        {/* 统计：帖子 / 获赞 / 话题 */}
        <div className="mt-4 flex divide-x divide-line border-t border-line pt-4">
          {STAT_LABELS.map((label, index) => (
            <div key={label} className="flex-1 px-2 text-center first:pl-0 last:pr-0">
              <p className="text-base font-semibold text-ink">{formatCount(stats[index] ?? 0)}</p>
              <p className="mt-0.5 text-xs text-ink-3">{label}</p>
            </div>
          ))}
        </div>
      </section>
    </aside>
  );
}
