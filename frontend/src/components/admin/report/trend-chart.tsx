// src/components/admin/report/trend-chart.tsx
// 报表页四维趋势柱状图（M4-报表，设计稿《数据报表》「近 7 日互动」）：
// 浏览/新帖/获赞/评论 四系列分组柱状图，SVG 自绘（零依赖，复用 M1.7 图表模式）。
"use client";

import type { ReportTrendPoint } from "@/lib/api-report";

// 系列配置（柱色与图例文案，与仪表盘 TrendChart 同色系扩展）。
const SERIES: readonly { key: "views" | "posts" | "likes" | "comments"; label: string; color: string }[] = [
  { key: "views", label: "浏览", color: "#7aa2f7" },
  { key: "posts", label: "新帖", color: "#9ece6a" },
  { key: "likes", label: "获赞", color: "#e0af68" },
  { key: "comments", label: "评论", color: "#bb9af7" },
];

// ReportTrendChart 四维趋势柱状图。
// 参数：data 趋势点数组（7 或 30 日）；高度按最大值归一化。
export function ReportTrendChart({ data }: { data: ReportTrendPoint[] }) {
  // 最大值（0 兜底，避免除零）
  const max = Math.max(1, ...data.flatMap((d) => [d.views, d.posts, d.likes, d.comments]));
  // 布局参数（视图坐标；30 日时柱宽收窄避免挤压）
  const groupWidth = data.length > 14 ? 18 : 44;
  const barWidth = data.length > 14 ? 3 : 8;
  const height = 180;
  const chartWidth = Math.max(data.length * groupWidth, 120);

  return (
    <div>
      {/* 图例 */}
      <div className="flex flex-wrap gap-4 text-xs text-ink-2">
        {SERIES.map((s) => (
          <span key={s.key} className="flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-sm" style={{ backgroundColor: s.color }} aria-hidden />
            {s.label}
          </span>
        ))}
      </div>

      {/* 柱状图（分组柱：每日期 4 根） */}
      <svg
        viewBox={`0 0 ${chartWidth} ${height + 24}`}
        className="mt-3 w-full"
        role="img"
        aria-label="数据报表趋势柱状图"
      >
        {data.map((d, i) => {
          const x = i * groupWidth;
          return (
            <g key={d.date}>
              {SERIES.map((s, j) => {
                const barH = (d[s.key] / max) * (height - 10);
                const bx = x + j * (barWidth + (barWidth === 8 ? 2 : 1));
                return (
                  <rect
                    key={s.key}
                    x={bx}
                    y={height - barH}
                    width={barWidth}
                    height={Math.max(barH, d[s.key] > 0 ? 2 : 0)}
                    rx="1.5"
                    fill={s.color}
                  />
                );
              })}
              {/* 日期标签（30 日时隔天显示避免重叠） */}
              {(data.length <= 14 || i % 2 === 0) && (
                <text
                  x={x + groupWidth / 2}
                  y={height + 16}
                  textAnchor="middle"
                  fontSize="9"
                  className="fill-ink-3"
                >
                  {d.date}
                </text>
              )}
            </g>
          );
        })}
      </svg>
    </div>
  );
}
