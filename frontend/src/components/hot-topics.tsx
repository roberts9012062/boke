// src/components/hot-topics.tsx
// 桌面右栏「热门话题」卡片（设计稿 D/冷月/首页 右栏）：
// 话题列表（#话题名 + 帖数），点击进入话题页。
// M 后置修复：此前为硬编码杜撰话题（死链）；现按帖数降序取真实话题前 4 条，
// 加载中显示骨架行，加载失败显示空态（不再伪造数据）。
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { apiTopics } from "@/lib/api";
import type { TopicDTO } from "@/types/api";

// MAX_TOPICS 展示条数（设计稿右栏 4 条）。
const MAX_TOPICS = 4;

// topicHref 话题详情链接（话题名去掉 # 前缀并 URL 编码，与话题列表页一致）。
function topicHref(name: string): string {
  return `/topics/${encodeURIComponent(name.replace("#", ""))}`;
}

// HotTopics 桌面右栏热门话题（仅 ≥768px 显示）。
export function HotTopics() {
  // topics：null=加载中（骨架）；[]=无话题/加载失败（空态）
  const [topics, setTopics] = useState<TopicDTO[] | null>(null);

  // 拉取话题并按帖数降序取前 N 条（失败静默空态，避免伪造数据与死链）
  useEffect(() => {
    let cancelled = false;
    apiTopics()
      .then((list) => {
        if (cancelled) {
          return;
        }
        const sorted = list.slice().sort((a, b) => b.post_count - a.post_count).slice(0, MAX_TOPICS);
        setTopics(sorted);
      })
      .catch(() => {
        if (!cancelled) {
          setTopics([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <aside className="hidden w-[280px] shrink-0 lg:block">
      <section className="rounded-lg border border-line bg-elevated p-5">
        <h2 className="font-display text-base font-semibold text-ink">热门话题</h2>

        {/* 加载骨架（灰条呼吸动画，与设计稿形态一致） */}
        {topics === null && (
          <ul className="mt-3 space-y-1" aria-hidden>
            {Array.from({ length: MAX_TOPICS }, (_, index) => (
              <li key={index} className="flex items-center justify-between px-2 py-2">
                <span className="h-4 w-20 animate-pulse rounded bg-muted" />
                <span className="h-3 w-6 animate-pulse rounded bg-muted" />
              </li>
            ))}
          </ul>
        )}

        {/* 空态（无话题或加载失败） */}
        {topics !== null && topics.length === 0 && (
          <p className="mt-3 px-2 py-2 text-sm text-ink-3">暂无话题</p>
        )}

        {/* 真实话题列表 */}
        {topics !== null && topics.length > 0 && (
          <ul className="mt-3 space-y-1">
            {topics.map((topic) => (
              <li key={topic.slug}>
                <Link
                  href={topicHref(topic.name)}
                  className="group flex items-center justify-between rounded-md px-2 py-2 transition-colors hover:bg-muted"
                >
                  <span className="truncate text-sm text-glow group-hover:text-ink">
                    {topic.name}
                  </span>
                  <span className="text-xs text-ink-3">{topic.post_count}</span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
    </aside>
  );
}
