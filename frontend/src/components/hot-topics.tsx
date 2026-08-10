// src/components/hot-topics.tsx
// 桌面右栏「热门话题」卡片（设计稿 D/冷月/首页 右栏）：
// 话题列表（#话题名 + 帖数），点击进入话题页。
// M1.1 为静态骨架（文案与设计稿一致），M1.5 接入话题接口。
"use client";

import Link from "next/link";

// 热门话题数据（设计稿文案：帖数降序）
const HOT_TOPICS = [
  { name: "月色随笔", posts: 328 },
  { name: "夜读", posts: 156 },
  { name: "城市声景", posts: 89 },
  { name: "短片练习", posts: 64 },
] as const;

// HotTopics 桌面右栏热门话题（仅 ≥768px 显示）。
export function HotTopics() {
  return (
    <aside className="hidden w-[280px] shrink-0 lg:block">
      <section className="rounded-lg border border-line bg-elevated p-5">
        <h2 className="font-display text-base font-semibold text-ink">热门话题</h2>
        <ul className="mt-3 space-y-1">
          {HOT_TOPICS.map((topic) => (
            <li key={topic.name}>
              <Link
                href={`/topics/${topic.name}`}
                className="group flex items-center justify-between rounded-md px-2 py-2 transition-colors hover:bg-muted"
              >
                <span className="text-sm text-glow group-hover:text-ink">
                  #{topic.name}
                </span>
                <span className="text-xs text-ink-3">{topic.posts}</span>
              </Link>
            </li>
          ))}
        </ul>
      </section>
    </aside>
  );
}
