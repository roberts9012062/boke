// src/components/user-card.tsx
// 桌面左栏用户卡片（设计稿 D/冷月/首页 左栏）：
// 头像 + 昵称 + @账号 + 简介 + 统计（帖子/获赞/话题）。
// M1.1 为静态骨架（文案与设计稿一致），M1.2 接入真实用户数据。
"use client";

// 用户统计项配置（文案与设计稿一致：帖子/获赞/话题）
const STATS = [
  { label: "帖子", value: "128" },
  { label: "获赞", value: "1.2k" },
  { label: "话题", value: "36" },
] as const;

// UserCard 桌面左栏用户卡片（仅 ≥768px 显示，移动端在「我的」页展示）。
export function UserCard() {
  return (
    <aside className="hidden w-[260px] shrink-0 md:block">
      <section className="rounded-lg border border-line bg-elevated p-5">
        {/* 头像 + 昵称 + 账号 */}
        <div className="flex items-center gap-3">
          {/* 头像占位（M1.2 接入用户头像） */}
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted font-display text-lg text-ink-2">
            月
          </div>
          <div className="min-w-0">
            <p className="font-display text-base font-semibold text-ink">阿月</p>
            <p className="text-xs text-ink-3">@yueyan</p>
          </div>
        </div>

        {/* 简介（设计稿文案） */}
        <p className="mt-3 text-sm leading-relaxed text-ink-2">
          写短句，收声音，偶尔录一点夜色。
        </p>

        {/* 统计：帖子 / 获赞 / 话题 */}
        <div className="mt-4 flex divide-x divide-line border-t border-line pt-4">
          {STATS.map((stat) => (
            <div key={stat.label} className="flex-1 px-2 text-center first:pl-0 last:pr-0">
              <p className="text-base font-semibold text-ink">{stat.value}</p>
              <p className="mt-0.5 text-xs text-ink-3">{stat.label}</p>
            </div>
          ))}
        </div>
      </section>
    </aside>
  );
}
