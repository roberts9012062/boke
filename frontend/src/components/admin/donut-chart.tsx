// src/components/admin/donut-chart.tsx
// 内容分布环形图（M1.7，需求 4.2）：纯 SVG 分段圆环 + 居中总数 + 图例。
// 说明：设计稿为环形图；不引入图表库（KISS/YAGNI），SVG 与主题令牌适配。
"use client";

// DonutSegment 分段数据。
interface DonutSegment {
  key: string; // 类型键
  label: string; // 显示名（文字/图片/音频/视频）
  value: number; // 数量
}

// 分段配色（按顺序取，与双主题都可读的中性色）
const SEGMENT_COLORS: readonly string[] = ["#7aa2f7", "#9ece6a", "#e0af68", "#bb9af7"];

// DonutChartProps 环形图参数。
interface DonutChartProps {
  segments: DonutSegment[]; // 分段数据（value 为 0 的分段跳过）
  size?: number; // 圆环外径（px）
  thickness?: number; // 圆环厚度（px）
}

// DonutChart 环形图：按占比绘制分段圆弧，中心显示总数。
export function DonutChart({ segments, size = 160, thickness = 20 }: DonutChartProps) {
  // 过滤空分段并计算总数
  const data = segments.filter((s) => s.value > 0);
  const total = data.reduce((sum, s) => sum + s.value, 0);

  // 空数据：灰色整环占位
  if (total === 0) {
    return (
      <div className="flex items-center gap-6">
        <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} aria-label="内容分布（暂无数据）">
          <circle
            cx={size / 2}
            cy={size / 2}
            r={(size - thickness) / 2}
            fill="none"
            stroke="var(--yy-border)"
            strokeWidth={thickness}
          />
        </svg>
        <p className="text-xs text-ink-3">暂无内容</p>
      </div>
    );
  }

  // 计算各分段圆弧（stroke-dasharray 分段 + 偏移）
  const radius = (size - thickness) / 2;
  const circumference = 2 * Math.PI * radius;
  let offset = 0;
  const arcs = data.map((s, i) => {
    const len = (s.value / total) * circumference;
    const arc = {
      ...s,
      color: SEGMENT_COLORS[i % SEGMENT_COLORS.length],
      dash: `${len} ${circumference - len}`,
      offset: -offset, // 逆时针偏移对齐（起点在 12 点方向）
    };
    offset += len;
    return arc;
  });

  return (
    <div className="flex flex-col items-center gap-4 sm:flex-row sm:gap-6">
      {/* 圆环 */}
      <svg
        width={size}
        height={size}
        viewBox={`0 0 ${size} ${size}`}
        className="-rotate-90"
        role="img"
        aria-label="内容分布环形图"
      >
        {/* 背景环 */}
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="var(--yy-border)"
          strokeWidth={thickness}
        />
        {/* 分段弧 */}
        {arcs.map((a) => (
          <circle
            key={a.key}
            cx={size / 2}
            cy={size / 2}
            r={radius}
            fill="none"
            stroke={a.color}
            strokeWidth={thickness}
            strokeDasharray={a.dash}
            strokeDashoffset={a.offset}
            strokeLinecap="butt"
          />
        ))}
      </svg>

      {/* 图例（占比 + 数量） */}
      <div className="space-y-2">
        {arcs.map((a) => (
          <div key={a.key} className="flex items-center gap-2 text-xs text-ink-2">
            <span className="h-2.5 w-2.5 rounded-sm" style={{ backgroundColor: a.color }} aria-hidden />
            <span className="w-8">{a.label}</span>
            <span className="font-medium text-ink">{Math.round((a.value / total) * 100)}%</span>
            <span className="text-ink-3">{a.value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
