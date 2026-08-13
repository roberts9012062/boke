// src/components/admin/ai/usage-tab.tsx
// AI 设置页 · 用量统计 Tab（M4）：汇总卡片 + 近 7 日调用趋势柱状图（SVG 零依赖）。
// 设计依据：无 AI 画板，柱状图复用 M1.7 仪表盘 SVG 自绘模式。
"use client";

import { useEffect, useState } from "react";

import { apiAiUsage, type AiDayStat, type AiUsageSummary } from "@/lib/api-ai";

// BarChart 近 7 日调用次数柱状图（SVG 自绘，零依赖）。
// 参数：days 按日统计（calls）；宽固定 560、高紧凑 140（后台卡片内不喧宾夺主），标尺 + 数值 + 星期标签。
function BarChart({ days }: { days: AiDayStat[] }) {
  const width = 560;
  const height = 140;
  const pad = 28;
  const max = Math.max(1, ...days.map((d) => d.calls)); // 峰值（防除零取 1）
  const barW = (width - pad * 2) / days.length;

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="w-full max-w-[560px]" role="img" aria-label="近 7 日 AI 调用趋势">
      {/* 横轴基线 */}
      <line x1={pad} y1={height - 24} x2={width - 8} y2={height - 24} className="stroke-line" strokeWidth="1" />
      {days.map((d, i) => {
        const h = Math.max(4, (d.calls / max) * (height - 56));
        const x = pad + i * barW + barW * 0.25;
        const y = height - 24 - h;
        // 星期标签（按日期字符串，避免时区偏差）
        const label = d.day.slice(5); // MM-DD
        return (
          <g key={d.day}>
            <rect x={x} y={y} width={barW * 0.5} height={h} rx="3" className={d.calls > 0 ? "fill-accent" : "fill-muted"} />
            {d.calls > 0 && (
              <text x={x + barW * 0.25} y={y - 6} textAnchor="middle" fontSize="11" className="fill-ink-2">
                {d.calls}
              </text>
            )}
            <text x={x + barW * 0.25} y={height - 8} textAnchor="middle" fontSize="10" className="fill-ink-3">
              {label}
            </text>
          </g>
        );
      })}
    </svg>
  );
}

// UsageTab 用量统计。
export function UsageTab() {
  const [summary, setSummary] = useState<AiUsageSummary | null>(null);
  const [days, setDays] = useState<AiDayStat[]>([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    apiAiUsage()
      .then((r) => {
        setSummary(r.summary);
        setDays(r.days);
      })
      .catch(() => {
        setSummary(null);
        setDays([]);
      })
      .finally(() => setLoaded(true));
  }, []);

  const cards = [
    { label: "今日调用", value: summary?.today_calls ?? 0, suffix: "次" },
    { label: "今日 Token", value: summary?.today_tokens ?? 0, suffix: "" },
    { label: "累计调用", value: summary?.total_calls ?? 0, suffix: "次" },
    { label: "累计 Token", value: summary?.total_tokens ?? 0, suffix: "" },
    { label: "今日费用", value: summary?.today_cost ?? 0, suffix: "元", decimals: 4 },
    { label: "累计费用", value: summary?.total_cost ?? 0, suffix: "元", decimals: 4 },
  ];

  // formatValue 数值展示（费用保留 4 位小数，其余千分位整数）。
  const formatValue = (c: (typeof cards)[number]) =>
    c.decimals !== undefined ? c.value.toFixed(c.decimals) : c.value.toLocaleString();

  return (
    <div>
      {!loaded && <div className="h-40 animate-pulse rounded-lg bg-muted" aria-hidden />}
      {loaded && (
        <>
          {/* 汇总卡片 */}
          <div className="grid grid-cols-2 gap-4 lg:grid-cols-3">
            {cards.map((c) => (
              <div key={c.label} className="rounded-lg border border-line bg-elevated p-4">
                <p className="text-xs text-ink-3">{c.label}</p>
                <p className="mt-1 font-display text-2xl font-semibold text-ink">
                  {formatValue(c)}
                  <span className="ml-0.5 text-xs font-normal text-ink-3">{c.suffix}</span>
                </p>
              </div>
            ))}
          </div>

          {/* 近 7 日趋势 */}
          <div className="mt-4 rounded-lg border border-line bg-elevated p-4">
            <p className="text-sm font-medium text-ink">近 7 日调用趋势</p>
            <p className="mt-0.5 text-xs text-ink-3">AI 场景实际调用次数（摘要/标签/评论审核）</p>
            <div className="mt-3">
              <BarChart days={days} />
            </div>
          </div>

          {/* 说明 */}
          <p className="mt-4 text-xs text-ink-3">
            说明：Token 用量随每次 AI 调用自动记录（ai_usage 表）；费用按供应商单价折算（未配置单价的供应商记 0）。
          </p>
        </>
      )}
    </div>
  );
}
