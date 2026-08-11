// src/app/admin/reports/page.tsx
// 数据报表页（设计稿《数据报表》#235/#242）：
// 页头（近 30 日趋势 · 导出 CSV + 7/30 切换 + 刷新）+ 4 统计卡（今日新帖带待审徽标）
// + 四维趋势柱状图 + 内容分布环形图 + 待处理块 + 最近动态 + 快捷操作。
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { DonutChart } from "@/components/admin/donut-chart";
import { ReportTrendChart } from "@/components/admin/report/trend-chart";
import { apiReportExportCsv, apiReportOverview, type ReportOverview } from "@/lib/api-report";

// ReportPage 数据报表页。
export default function ReportPage() {
  const [data, setData] = useState<ReportOverview | null>(null);
  const [days, setDays] = useState<number>(30); // 趋势视图（设计稿默认近 30 日）
  const [refreshTick, setRefreshTick] = useState<number>(0); // 手动刷新计数
  const [loading, setLoading] = useState<boolean>(true);
  const [exporting, setExporting] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // 加载报表（视图切换/手动刷新时）
  useEffect(() => {
    setLoading(true);
    apiReportOverview(days)
      .then(setData)
      .catch(() => setData(null))
      .finally(() => setLoading(false));
  }, [days, refreshTick]);

  // 导出 CSV（当前视图天数）
  const handleExport = async () => {
    setExporting(true);
    setError("");
    try {
      await apiReportExportCsv(days);
    } catch (err) {
      setError(err instanceof Error ? err.message : "导出失败");
    } finally {
      setExporting(false);
    }
  };

  // 内容分布分段（环形图）
  const typeSegments = data
    ? [
        { key: "text", label: "文字", value: data.type_counts.text ?? 0 },
        { key: "image", label: "图片", value: data.type_counts.image ?? 0 },
        { key: "audio", label: "音频", value: data.type_counts.audio ?? 0 },
        { key: "video", label: "视频", value: data.type_counts.video ?? 0 },
      ]
    : [];

  // 统计卡（设计稿 4 卡：浏览/获赞/评论/今日新帖）
  const cards = data
    ? [
        { label: "浏览", value: data.views_7d, trend: data.views_trend },
        { label: "获赞", value: data.likes_7d, trend: data.likes_trend },
        { label: "评论", value: data.comments_7d, trend: data.comments_trend },
        { label: "今日新帖", value: data.posts_today, trend: null as number | null },
      ]
    : [];

  // 待处理块（设计稿：评论待审 N（处理）/ 内容举报 N（查看）/ 敏感词命中 N（复核））
  const pendingItems = data
    ? [
        { label: "评论待审", value: data.pending.comments, href: "/admin/comments", action: "处理" },
        { label: "内容举报", value: data.pending.reports, href: "/admin/audit", action: "查看" },
        { label: "敏感词命中", value: data.pending.sensitive, href: "/admin/sensitive-words", action: "复核" },
      ]
    : [];

  return (
    <div>
      {/* 页头（设计稿：数据报表 / 近 30 日趋势 · 导出 CSV + 近 7 日 + 刷新） */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="font-display text-xl font-semibold text-ink">数据报表</h1>
          <p className="mt-0.5 text-xs text-ink-3">近 {days} 日趋势 · 导出 CSV</p>
        </div>
        <div className="flex items-center gap-2">
          {/* 7/30 日视图切换 */}
          <div className="flex rounded-full bg-muted p-0.5 text-xs">
            {[7, 30].map((d) => (
              <button
                key={d}
                type="button"
                onClick={() => setDays(d)}
                className={`rounded-full px-3 py-1 transition-colors ${
                  days === d ? "bg-accent font-medium text-on-accent" : "text-ink-2 hover:text-ink"
                }`}
              >
                近 {d} 日
              </button>
            ))}
          </div>
          <button
            type="button"
            onClick={() => void handleExport()}
            disabled={exporting || !data}
            className="rounded-full border border-line px-4 py-1.5 text-xs text-ink-2 hover:text-ink disabled:opacity-50"
          >
            {exporting ? "导出中…" : "导出 CSV"}
          </button>
          <button
            type="button"
            onClick={() => setRefreshTick((t) => t + 1)} // 手动刷新（触发重新加载）
            className="rounded-full border border-line px-4 py-1.5 text-xs text-ink-2 hover:text-ink"
          >
            刷新
          </button>
        </div>
      </div>

      {error && <p className="mt-3 rounded-lg bg-like/10 px-3 py-2 text-sm text-ink">{error}</p>}

      {/* 统计卡（设计稿 4 卡；今日新帖带「N 待审需处理」徽标） */}
      <div className="mt-4 grid grid-cols-2 gap-4 lg:grid-cols-4">
        {cards.map((card) => (
          <div key={card.label} className="rounded-lg border border-line bg-elevated p-4">
            <p className="text-xs text-ink-3">{card.label}</p>
            <p className="mt-1 font-display text-2xl font-semibold text-ink">{card.value.toLocaleString()}</p>
            {card.trend !== null ? (
              <p className={`mt-1 text-xs ${card.trend >= 0 ? "text-glow" : "text-like"}`}>
                {card.trend >= 0 ? "+" : ""}
                {Math.round(card.trend)}% 较上周
              </p>
            ) : (
              <p className="mt-1 flex items-center gap-1.5 text-xs text-ink-3">
                {data && data.pending_audit > 0 && (
                  <span className="rounded-full bg-like/15 px-2 py-0.5 text-like">{data.pending_audit} 待审需处理</span>
                )}
              </p>
            )}
          </div>
        ))}
      </div>

      {/* 四维趋势（设计稿「近 7 日互动」；报表页含浏览维度，7/30 可切换） */}
      {data && data.trend.length > 0 && (
        <section className="mt-6 rounded-lg border border-line bg-elevated p-5">
          <h2 className="font-display text-base font-semibold text-ink">互动趋势（浏览 / 新帖 / 获赞 / 评论）</h2>
          <div className="mt-4">
            <ReportTrendChart data={data.trend} />
          </div>
        </section>
      )}

      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        {/* 内容分布（环形图，复用仪表盘） */}
        <section className="rounded-lg border border-line bg-elevated p-5">
          <h2 className="font-display text-base font-semibold text-ink">内容分布</h2>
          <div className="mt-4">
            <DonutChart segments={typeSegments} />
          </div>
        </section>

        {/* 待处理块（设计稿：评论待审/内容举报/敏感词命中 + 跳转） */}
        <section className="rounded-lg border border-line bg-elevated p-5">
          <h2 className="font-display text-base font-semibold text-ink">待处理</h2>
          <div className="mt-3 space-y-2">
            {pendingItems.map((item) => (
              <div
                key={item.label}
                className="flex items-center justify-between rounded-lg bg-muted/50 px-4 py-3"
              >
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

      {/* 最近动态（复用仪表盘样式） */}
      <section className="mt-6 rounded-lg border border-line bg-elevated p-5">
        <div className="flex items-center justify-between">
          <h2 className="font-display text-base font-semibold text-ink">最近动态</h2>
          <Link href="/admin/posts" className="text-xs text-glow hover:underline">
            查看全部 →
          </Link>
        </div>
        <div className="mt-3 divide-y divide-line">
          {data?.activities.map((a, i) => (
            <div key={`${a.kind}-${a.id}-${i}`} className="py-2.5">
              <p className="text-sm text-ink">
                <span className="font-medium">{a.actor}</span>
                <span className="text-ink-2">
                  {a.kind === "post" ? " 发布了新帖" : a.kind === "comment" ? " 评论了你" : " 加入了月言"}
                </span>
              </p>
              <p className="mt-0.5 line-clamp-1 text-xs text-ink-3">{a.content}</p>
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
          <Link href="/compose" className="rounded-full bg-accent px-5 py-2 text-sm text-on-accent hover:opacity-90">
            发新帖
          </Link>
          <Link href="/admin/comments" className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink">
            审评论
          </Link>
          <Link href="/admin/settings" className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink">
            设置
          </Link>
        </div>
      </section>

      {/* 加载骨架 */}
      {loading && <div className="mt-4 h-48 animate-pulse rounded-lg bg-muted" aria-hidden />}
    </div>
  );
}
