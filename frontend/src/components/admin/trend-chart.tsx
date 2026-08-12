// src/components/admin/trend-chart.tsx
// 近 7 日互动趋势图（M1.7，需求 4.2）：纯 SVG 分组柱状图（新帖/获赞/评论）+ 图例。
// 说明：浏览埋点已上线（P1，post_views 按日去重），仪表盘趋势图保持设计稿三项；如需加入浏览维度可扩展 SERIES。
"use client";

import type { TrendPoint } from "@/lib/api";

// 系列配置（柱色与图例文案）
const SERIES: readonly { key: "posts" | "likes" | "comments"; label: string; color: string }[] = [
  { key: "posts", label: "新帖", color: "#7aa2f7" },
  { key: "likes", label: "获赞", color: "#9ece6a" },
  { key: "comments", label: "评论", color: "#e0af68" },
];

// TrendChartProps 趋势图参数。
interface TrendChartProps {
  data: TrendPoint[]; // 近 7 日数据（后端 trend_series）
}

// TrendChart 分组柱状图：每个日期 3 根柱（新帖/获赞/评论），柱高按最大值归一化。
export function TrendChart({ data }: TrendChartProps) {
  // 最大值（0 兜底，避免除零）
  const max = Math.max(1, ...data.flatMap((d) => [d.posts, d.likes, d.comments]));
  // 布局参数（视图坐标；高度紧凑化——后台卡片内不喧宾夺主）
  const groupWidth = 44; // 每组宽度（柱 12 + 间距 2）
  const barWidth = 12;
  const height = 120;
  const chartWidth = Math.max(data.length * groupWidth, 120);

  return (
    <div>
      <div className="flex gap-4 text-xs text-ink-2">
        {SERIES.map((s) => (
          <span key={s.key} className="flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-sm" style={{ backgroundColor: s.color }} aria-hidden />
            {s.label}
          </span>
        ))}
      </div>

      {/* 柱状图（SVG，高度按最大值归一化；max-w 限制防止 w-full 等比拉伸放大——7 日 viewBox 窄，满宽拉伸高度会爆炸） */}
      <svg
        viewBox={`0 0 ${chartWidth} ${height + 24}`}
        className="mt-3 w-full max-w-[320px]"
        role="img"
        aria-label="近 7 日互动趋势柱状图"
      >
        {data.map((d, i) => {
          const x = i * groupWidth;
          return (
            <g key={d.date}>
              {/* 三根柱（新帖/获赞/评论） */}
              {SERIES.map((s, j) => {
                const barH = (d[s.key] / max) * (height - 10);
                const bx = x + j * (barWidth + 2);
                return (
                  <rect
                    key={s.key}
                    x={bx}
                    y={height - barH}
                    width={barWidth}
                    height={barH}
                    rx={2}
                    fill={s.color}
                  >
                    {/* 数量提示（hover 显示） */}
                    <title>{`${d.date} ${s.label} ${d[s.key]}`}</title>
                  </rect>
                );
              })}
              {/* 日期标签 */}
              <text
                x={x + groupWidth / 2}
                y={height + 16}
                textAnchor="middle"
                className="fill-ink-3"
                style={{ fontSize: 10 }}
              >
                {d.date}
              </text>
            </g>
          );
        })}
      </svg>
    </div>
  );
}
