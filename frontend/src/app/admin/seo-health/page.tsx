// src/app/admin/seo-health/page.tsx
// 后台 SEO 健康度（设计稿 D/冷月/SEO·健康度 1400×1080）：
// 四卡片（综合评分/待修复问题/元信息覆盖/可收录页面）+ 近 7 日健康分趋势（SVG 折线）
// + 问题类型分布（SVG 条形）+ 优先修复（P0/P1 分级）+ 问题列表 + 批量修复。
"use client";

import { useEffect, useState } from "react";

import { Modal } from "@/components/ui/modal";
import { apiSeoBatchFix, apiSeoHealth, apiSeoScan, ApiError, type SeoHealthSummary } from "@/lib/api";

// AdminSeoHealth SEO 健康度页。
export default function AdminSeoHealth() {
  const [summary, setSummary] = useState<SeoHealthSummary | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string>("");
  const [fixOpen, setFixOpen] = useState<boolean>(false);
  const [fixing, setFixing] = useState<boolean>(false);
  const [fixDone, setFixDone] = useState<boolean>(false);

  // 加载健康度
  const load = () => {
    setLoading(true);
    apiSeoHealth()
      .then(setSummary)
      .catch((err) => setError(err instanceof ApiError ? err.message : "加载失败"))
      .finally(() => setLoading(false));
  };
  useEffect(load, []);

  // 重新扫描
  const rescan = async () => {
    setError("");
    setLoading(true);
    try {
      setSummary(await apiSeoScan());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "扫描失败");
    } finally {
      setLoading(false);
    }
  };

  // 批量修复（设计稿：将自动处理 N 个可修复问题 → 确认修复 N 项）
  const confirmFix = async () => {
    setFixing(true);
    try {
      const r = await apiSeoBatchFix();
      setFixDone(true);
      setSummary((prev) => (prev ? { ...prev, pending_issues: Math.max(0, prev.pending_issues - r.fixed) } : prev));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "批量修复失败");
    } finally {
      setFixing(false);
    }
  };

  // 综合评分颜色（设计稿：良好/需改进）
  const scoreColor = (summary?.avg_score ?? 0) >= 75 ? "text-glow" : (summary?.avg_score ?? 0) >= 50 ? "text-ink" : "text-like";

  return (
    <div>
      <h1 className="font-display text-xl font-semibold text-ink">SEO 健康度</h1>
      <p className="mt-0.5 text-xs text-ink-3">
        全站评分 · 更新于 刚刚
        <button type="button" onClick={() => void rescan()} className="ml-3 text-glow hover:underline">
          重新扫描
        </button>
      </p>

      {/* 四卡片（设计稿：综合评分/待修复问题/元信息覆盖/可收录页面） */}
      {!loading && summary && (
        <div className="mt-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
          {/* 综合评分 */}
          <div className="rounded-lg border border-line bg-elevated p-4">
            <p className={`font-display text-3xl font-semibold ${scoreColor}`}>{summary.avg_score}</p>
            <p className="mt-1 text-xs text-ink-3">综合评分 · 满分 100</p>
            <span className={`mt-1 inline-block rounded-full px-2 py-0.5 text-[10px] ${summary.avg_score >= 75 ? "bg-accent-soft text-glow" : "bg-like/10 text-like"}`}>
              {summary.avg_score >= 75 ? "良好" : summary.avg_score >= 50 ? "一般" : "需改进"}
            </span>
          </div>
          {/* 待修复问题 */}
          <div className="rounded-lg border border-line bg-elevated p-4">
            <p className="font-display text-3xl font-semibold text-ink">{summary.pending_issues}</p>
            <p className="mt-1 text-xs text-ink-3">待修复问题</p>
            <span className="mt-1 inline-block rounded-full bg-muted px-2 py-0.5 text-[10px] text-ink-3">
              {summary.priorities.map((p) => `${p.level}×${p.message.match(/\d+/)?.[0] ?? ""}`).join(" · ") || "P0×0 · P1×0"}
            </span>
          </div>
          {/* 元信息覆盖 */}
          <div className="rounded-lg border border-line bg-elevated p-4">
            <p className="font-display text-3xl font-semibold text-ink">{summary.meta_coverage}%</p>
            <p className="mt-1 text-xs text-ink-3">元信息覆盖 · 有标题+描述</p>
          </div>
          {/* 可收录页面 */}
          <div className="rounded-lg border border-line bg-elevated p-4">
            <p className="font-display text-3xl font-semibold text-ink">{summary.indexable}</p>
            <p className="mt-1 text-xs text-ink-3">可收录页面 · noindex {summary.noindex}</p>
          </div>
        </div>
      )}

      {/* 近 7 日健康分趋势（设计稿 SVG 折线，周一~周日） */}
      {!loading && summary && summary.trend.length > 0 && (
        <div className="mt-4 rounded-lg border border-line bg-elevated p-5">
          <h2 className="text-sm font-semibold text-ink">近 7 日健康分趋势</h2>
          <TrendChart data={summary.trend} />
        </div>
      )}

      {/* 问题类型分布（设计稿：缺标题 28% / 缺描述 35% / 重复标题 22% / 弱 OG 15%） */}
      {!loading && summary && summary.distribution.length > 0 && (
        <div className="mt-4 rounded-lg border border-line bg-elevated p-5">
          <h2 className="text-sm font-semibold text-ink">问题类型分布</h2>
          <div className="mt-4 space-y-3">
            {summary.distribution.map((d) => (
              <div key={d.code}>
                <div className="flex justify-between text-xs">
                  <span className="text-ink-2">{d.label}</span>
                  <span className="text-ink-3">{d.percent}%</span>
                </div>
                <div className="mt-1 h-2 overflow-hidden rounded-full bg-muted">
                  <div className="h-full rounded-full bg-accent" style={{ width: `${d.percent}%` }} />
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 操作：批量修复（设计稿《SEO·批量修复》） */}
      <div className="mt-4 flex gap-3">
        <button
          type="button"
          disabled={!summary || summary.pending_issues === 0}
          onClick={() => {
            setFixOpen(true);
            setFixDone(false);
            setError("");
          }}
          className="rounded-full bg-accent px-5 py-2 text-sm font-medium text-on-accent hover:opacity-90 disabled:opacity-60"
        >
          批量修复
        </button>
      </div>

      {error && (
        <p className="mt-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
          {error}
        </p>
      )}

      {/* 优先修复 + 问题列表 */}
      {loading ? (
        <div className="mt-5 space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-16 animate-pulse rounded-lg bg-muted" aria-hidden />
          ))}
        </div>
      ) : !summary || summary.items.length === 0 ? (
        <div className="mt-8 rounded-lg border border-line bg-elevated py-14 text-center">
          <p className="text-sm text-ink-2">没有待修复问题 🎉</p>
        </div>
      ) : (
        <div className="mt-5 overflow-hidden rounded-lg border border-line bg-elevated">
          {/* 优先修复（P0/P1 分级，设计稿） */}
          {summary.priorities.length > 0 && (
            <div className="border-b border-line bg-muted/30 px-5 py-3">
              <p className="text-xs font-medium text-ink">优先修复</p>
              <div className="mt-2 space-y-1.5">
                {summary.priorities.map((p, i) => (
                  <div key={i} className="flex items-center gap-2 text-xs">
                    <span className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${p.level === "P0" ? "bg-like/15 text-like" : "bg-muted text-ink-2"}`}>
                      {p.level}
                    </span>
                    <span className="text-ink">{p.message}</span>
                    <span className="text-ink-3">· {p.hint}</span>
                    <span className="ml-auto rounded-full bg-muted px-2 py-0.5 text-[10px] text-ink-3">{p.where}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
          {/* 问题列表（帖子维度） */}
          {summary.items.map((item) => (
            <div key={item.post_id} className="border-b border-line px-5 py-4 last:border-b-0">
              <div className="flex items-center justify-between">
                <a href={`/posts/${item.post_id}`} className="text-sm font-medium text-ink hover:text-glow">
                  {item.post_title || `帖子 #${item.post_id}`}
                </a>
                <span
                  className={`rounded-full px-2.5 py-0.5 text-xs ${
                    item.score >= 75 ? "bg-accent-soft text-glow" : item.score >= 50 ? "bg-like/10 text-like" : "bg-like/15 text-like"
                  }`}
                >
                  {item.score} 分
                </span>
              </div>
              <ul className="mt-2 space-y-1">
                {item.issues.map((issue) => (
                  <li key={issue.code} className="text-xs text-ink-2">
                    · {issue.message}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      )}

      {/* 批量修复弹层（设计稿《SEO·批量修复》：确认修复 N 项 → 成功） */}
      <Modal open={fixOpen} title={fixDone ? "批量修复完成" : "一键批量修复？"} onClose={() => setFixOpen(false)}>
        {fixDone ? (
          <div className="py-4 text-center">
            <p className="text-4xl" aria-hidden>
              ✓
            </p>
            <p className="mt-3 text-sm text-ink-2">已自动补齐缺省 SEO 标题与描述</p>
            <button
              type="button"
              onClick={() => {
                setFixOpen(false);
                void load();
              }}
              className="mt-5 rounded-full bg-accent px-8 py-2 text-sm font-medium text-on-accent hover:opacity-90"
            >
              完成
            </button>
          </div>
        ) : (
          <>
            <p className="text-sm text-ink-2">
              将自动处理 <span className="font-semibold text-glow">{summary?.pending_issues ?? 0}</span>{" "}
              个可修复问题（补齐缺省 SEO 标题与描述），建议先预览变更。
            </p>
            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setFixOpen(false)}
                disabled={fixing}
                className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink disabled:opacity-60"
              >
                再想想
              </button>
              <button
                type="button"
                onClick={() => void confirmFix()}
                disabled={fixing}
                className="rounded-full bg-accent px-5 py-2 text-sm font-medium text-on-accent hover:opacity-90 disabled:opacity-60"
              >
                {fixing ? "修复中…" : `确认修复 ${summary?.pending_issues ?? 0} 项`}
              </button>
            </div>
          </>
        )}
      </Modal>
    </div>
  );
}

// TrendChart 近 7 日健康分趋势（SVG 折线，零依赖）。
function TrendChart({ data }: { data: { date: string; score: number }[] }) {
  const W = 640;
  const H = 160;
  const pad = 28;
  const max = 100;
  const stepX = (W - pad * 2) / Math.max(1, data.length - 1);
  const points = data.map((d, i) => ({
    x: pad + i * stepX,
    y: H - pad - (d.score / max) * (H - pad * 2),
    ...d,
  }));
  const path = points.map((p, i) => `${i === 0 ? "M" : "L"}${p.x},${p.y}`).join(" ");
  const area = `${path} L${points[points.length - 1]?.x ?? pad},${H - pad} L${pad},${H - pad} Z`;

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="mt-3 w-full" role="img" aria-label="近 7 日健康分趋势折线图">
      {/* 网格线（0/50/100） */}
      {[0, 50, 100].map((v) => (
        <g key={v}>
          <line x1={pad} y1={H - pad - (v / max) * (H - pad * 2)} x2={W - pad} y2={H - pad - (v / max) * (H - pad * 2)} stroke="var(--yy-border, #2a3348)" strokeDasharray="4 4" />
          <text x={2} y={H - pad - (v / max) * (H - pad * 2) + 3} fontSize="10" fill="var(--yy-text3, #8a94ab)">
            {v}
          </text>
        </g>
      ))}
      {/* 面积 + 折线 */}
      <path d={area} fill="var(--yy-accent, #a8b8d8)" opacity="0.12" />
      <path d={path} fill="none" stroke="var(--yy-accent, #a8b8d8)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      {/* 数据点 + 日期 */}
      {points.map((p) => (
        <g key={p.date}>
          <circle cx={p.x} cy={p.y} r="3" fill="var(--yy-accent, #a8b8d8)" />
          <text x={p.x} y={H - 8} fontSize="10" textAnchor="middle" fill="var(--yy-text3, #8a94ab)">
            {p.date.slice(3)}
          </text>
          <text x={p.x} y={p.y - 8} fontSize="10" textAnchor="middle" fill="var(--yy-text2, #9aa3b8)">
            {p.score.toFixed(0)}
          </text>
        </g>
      ))}
    </svg>
  );
}
