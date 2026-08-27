// src/components/user-card.tsx
// 桌面左栏站长卡片（设计稿 D/冷月/首页 左栏）：
// 头像 + 昵称 + @账号 + 简介 + 统计（帖子/获赞/话题）。
// 数据源：GET /api/v1/meta 附带的站长公开摘要（superadmin 用户真实资料与统计，
// 后台「账号设置」修改昵称/头像/简介后实时生效）；无站长（极端场景）回退静态占位。
// 点击卡片跳转站长个人主页。
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { Avatar } from "@/components/ui/avatar";
import { apiSiteMeta, type OwnerSummary } from "@/lib/api";

// 无站长数据时的静态占位（仅形态示意，正常部署不会出现）。
const PLACEHOLDER = {
  nickname: "站长",
  username: "owner",
  bio: "暂无简介。",
  stats: [0, 0, 0],
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

// UserCard 桌面左栏站长卡片（仅 ≥768px 显示，移动端在「我的」页展示）。
export function UserCard() {
  const [owner, setOwner] = useState<OwnerSummary | null>(null);

  // 拉取站长真实资料（失败静默回退占位，不阻断首页渲染）
  useEffect(() => {
    let cancelled = false;
    apiSiteMeta()
      .then((meta) => {
        if (!cancelled) setOwner(meta.owner ?? null);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, []);

  const nickname = owner?.nickname || PLACEHOLDER.nickname;
  const username = owner?.username || PLACEHOLDER.username;
  const bio = owner?.bio || PLACEHOLDER.bio;
  const stats: number[] = owner
    ? [owner.post_count, owner.like_count, owner.topic_count]
    : [...PLACEHOLDER.stats];

  // 卡片主体（有站长数据时可点击进入其主页）
  const card = (
    <section className="rounded-lg border border-line bg-elevated p-5 transition-[box-shadow,border-color] duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] hover:border-accent/40 hover:shadow-[var(--yy-shadow-card)]">
      {/* 头像 + 昵称 + 账号 */}
      <div className="flex items-center gap-3">
        <Avatar name={nickname} url={owner?.avatar_url ?? ""} className="h-12 w-12 text-lg" />
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
  );

  return (
    <aside className="hidden w-[260px] shrink-0 md:block">
      {owner ? (
        <Link href={`/users/${owner.id}`} aria-label={`查看 ${nickname} 的主页`}>
          {card}
        </Link>
      ) : (
        card
      )}
    </aside>
  );
}
