// src/app/admin/page.tsx
// 后台仪表盘（设计稿 D/冷月/后台仪表盘 1400×1080）：
// 运营总览（近 7 日浏览/获赞/评论/新帖 + 环比 + 近 7 日 + 刷新）+ 近 7 日互动趋势（M1.7）
// + 内容分布（环形图 M1.7）+ 待处理块（走查纠偏）+ 最近动态（带相对时间）+ 快捷操作。
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { DonutChart } from "@/components/admin/donut-chart";
import { TrendChart } from "@/components/admin/trend-chart";
import { apiDashboard, type DashboardData } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

// AdminDashboard 后台仪表盘。
export default function AdminDashboard() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [updatedAt, setUpdatedAt] = useState<string>("刚刚");

  // 加载仪表盘
  const load = () => {
    setLoading(true);
    apiDashboard()
      .then((d) => {
        setData(d);
        setUpdatedAt("刚刚");
      })
      .catch(() => {
        // 加载失败保持空态
      })
      .finally(() => setLoading(false));
  };

  useEffect(load, []);

  // 内容分布分段（设计稿：文字 42%/图片 31%/音频 15%/视频 12%）
  const typeSegments = data
    ? [
        { key: "text", label: "文字", value: data.type_counts.text ?? 0 },
        { key: "image", label: "图片", value: data.type_counts.image ?? 0 },
        { key: "audio", label: "音频", value: data.type_counts.audio ?? 0 },
        { key: "video", label: "视频", value: data.type_counts.video ?? 0 },
      ]
    : [];

  // 运营总览卡片（设计稿：12.4k 浏览 +8% 较上周）
  const cards = data
    ? [
        { label: "浏览", value: data.views_7d, trend: data.views_trend },
        { label: "获赞", value: data.likes_7d, trend: data.likes_trend },
        { label: "评论", value: data.comments_7d, trend: data.comments_trend },
        { label: "今日新帖", value: data.posts_7d, trend: data.posts_trend },
      ]
    : [];

  return (
    <div>
      {/* 运营总览（设计稿：近 7 日数据 · 更新于 刚刚 + 近 7 日 + 刷新） */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="font-display text-xl font-semibold text-ink">运营总览</h1>
          <p className="mt-0.5 text-xs text-ink-3">近 7 日数据 · 更新于 {updatedAt}</p>
        </div>
        <div className="flex items-center gap-2">
          {/* 近 7 日标签（设计稿头部控件；7/30 切换在数据报表页） */}
          <span className="rounded-full bg-accent px-3 py-1.5 text-xs font-medium text-on-accent">近 7 日</span>
          <button
            type="button"
            onClick={load}
            className="rounded-full border border-line px-4 py-1.5 text-xs text-ink-2 hover:text-ink"
          >
            刷新
          </button>
        </div>
      </div>

      {/* 指标卡片组 */}
      <div className="mt-4 grid grid-cols-2 gap-4 lg:grid-cols-4">
        {cards.map((card) => (
          <div key={card.label} className="rounded-lg border border-line bg-elevated p-4">
            <p className="text-xs text-ink-3">{card.label}</p>
            <p className="mt-1 font-display text-2xl font-semibold text-ink">
              {card.value.toLocaleString()}
            </p>
            {/* 环比（设计稿：+8% 较上周） */}
            <p className={`mt-1 text-xs ${card.trend >= 0 ? "text-glow" : "text-like"}`}>
              {card.trend >= 0 ? "+" : ""}
              {Math.round(card.trend)}% 较上周
            </p>
          </div>
        ))}
      </div>

      {/* 近 7 日互动趋势（M1.7：柱状图，数据来自 trend_series） */}
      {data && data.trend_series && data.trend_series.length > 0 && (
        <section className="mt-6 rounded-lg border border-line bg-elevated p-5">
          <h2 className="font-display text-base font-semibold text-ink">近 7 日互动趋势</h2>
          <div className="mt-4">
            <TrendChart data={data.trend_series} />
          </div>
        </section>
      )}

      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        {/* 内容分布（设计稿环形图；M1.7 由条形升级为环形图） */}
        <section className="rounded-lg border border-line bg-elevated p-5">
          <h2 className="font-display text-base font-semibold text-ink">内容分布</h2>
          <div className="mt-4">
            <DonutChart segments={typeSegments} />
          </div>
        </section>

        {/* 待处理块（设计稿：评论待审 N（处理）/ 内容举报 N（查看）/ 敏感词命中 N（复核）；走查纠偏补） */}
        <section className="rounded-lg border border-line bg-elevated p-5">
          <h2 className="font-display text-base font-semibold text-ink">待处理</h2>
          <div className="mt-3 space-y-2">
            {[
              { label: "评论待审", value: data?.pending.comments ?? 0, href: "/admin/comments", action: "处理" },
              { label: "内容举报", value: data?.pending.reports ?? 0, href: "/admin/audit", action: "查看" },
              { label: "敏感词命中", value: data?.pending.sensitive ?? 0, href: "/admin/sensitive-words", action: "复核" },
            ].map((item) => (
              <div key={item.label} className="flex items-center justify-between rounded-lg bg-muted/50 px-4 py-3">
                <div>
                  <p className="text-sm font-medium text-ink">{item.label}</p>
                  <p className="mt-0.5 text-xs text-ink-3">{item.value} 条</p>
                </div>
                <Link
                  href={item.href}
                  className={`rounded-full px-3 py-1 text-xs ${
                    item.value > 0 ? "bg-accent text-on-accent hover:opacity-90" : "border border-line text-ink-3"
                  }`}
                >
                  {item.action}
                </Link>
              </div>
            ))}
          </div>
        </section>
      </div>

      {/* 最近动态（设计稿：北巷 评论了你 · 2 分钟前；走查纠偏补相对时间） */}
      <section className="mt-6 rounded-lg border border-line bg-elevated p-5">
        <div className="flex items-center justify-between">
          <h2 className="font-display text-base font-semibold text-ink">最近动态</h2>
          <Link href="/admin/posts" className="text-xs text-glow hover:underline">
            查看全部 →
          </Link>
        </div>
        <div className="mt-3 divide-y divide-line">
          {data?.activities.map((a, i) => (
            <div key={`${a.kind}-${a.id}-${i}`} className="flex items-center justify-between py-2.5">
              <p className="text-sm text-ink">
                <span className="font-medium">{a.actor}</span>
                <span className="text-ink-2">
                  {a.kind === "post" ? " 发布了新帖" : a.kind === "comment" ? " 评论了你" : " 加入了月言"}
                </span>
              </p>
              <span className="shrink-0 pl-3 text-xs text-ink-3">{timeAgo(a.created_at)}</span>
            </div>
          ))}
          {(!data || data.activities.length === 0) && (
            <p className="py-8 text-center text-xs text-ink-3">暂无动态</p>
          )}
        </div>
      </section>

      {/* 快捷操作（设计稿：发新帖/审评论/设置） */}
      <section className="mt-6 rounded-lg border border-line bg-elevated p-5">
        <h2 className="font-display text-base font-semibold text-ink">快捷操作</h2>
        <div className="mt-3 flex gap-3">
          <Link
            href="/compose"
            className="rounded-full bg-accent px-5 py-2 text-sm text-on-accent hover:opacity-90"
          >
            发新帖
          </Link>
          <Link
            href="/admin/comments"
            className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink"
          >
            审评论
          </Link>
          <Link
            href="/admin/settings"
            className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink"
          >
            设置
          </Link>
        </div>
      </section>

      {/* 加载骨架 */}
      {loading && <div className="mt-4 h-48 animate-pulse rounded-lg bg-muted" aria-hidden />}
    </div>
  );
}
