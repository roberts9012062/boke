// src/components/feed-tabs.tsx
// 时间线过滤 Tab（设计稿 D/冷月/首页 中栏顶部）：全部 / 文 / 图 / 音 / 影。
// 文 = 文章形态（kind=article），其余为媒体形态（type 过滤）；
// key 由首页统一解释（article → kind 参数，其余 → type 参数）。
// M1.3：选中 Tab 即触发时间线过滤（受控组件，由父页面管理状态）。
// 动效：胶囊指示器绝对定位，选中切换时以 left/top/width 过渡滑动到目标项（丝滑切换）。
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

// Tab 配置（文案与设计稿一致；影 = 视频，M2 后生效）
const TABS = [
  { key: "", label: "全部" },
  { key: "article", label: "文" },
  { key: "image", label: "图" },
  { key: "audio", label: "音" },
  { key: "video", label: "影" },
] as const;

// IndicatorRect 胶囊指示器位置与尺寸（相对容器内容区）。
interface IndicatorRect {
  left: number; // 水平偏移
  top: number; // 垂直偏移
  width: number; // 胶囊宽度
  height: number; // 胶囊高度
}

// FeedTabs 时间线过滤 Tab（受控：active 当前选中值，onChange 回调）。
export function FeedTabs({
  active,
  onChange,
}: {
  active: string; // 当前选中（空 = 全部）
  onChange: (key: string) => void; // 切换回调
}) {
  const [indicator, setIndicator] = useState<IndicatorRect>({ left: 0, top: 0, width: 0, height: 0 });
  const containerRef = useRef<HTMLDivElement>(null);
  const itemRefs = useRef<Record<string, HTMLButtonElement | null>>({});

  // 测量选中按钮位置，驱动胶囊滑动过去（字体缩放/窗口变化时重测）
  const measure = useCallback(() => {
    const container = containerRef.current;
    const item = itemRefs.current[active];
    if (!container || !item) {
      return;
    }
    const containerRect = container.getBoundingClientRect();
    const itemRect = item.getBoundingClientRect();
    setIndicator({
      left: itemRect.left - containerRect.left,
      top: itemRect.top - containerRect.top,
      width: itemRect.width,
      height: itemRect.height,
    });
  }, [active]);

  // 选中变化/挂载后测量一次
  useEffect(() => {
    measure();
  }, [measure]);

  // 窗口缩放重测（胶囊跟随布局）
  useEffect(() => {
    window.addEventListener("resize", measure);
    return () => window.removeEventListener("resize", measure);
  }, [measure]);

  return (
    <div ref={containerRef} className="relative flex items-center gap-2 border-b border-line pb-3">
      {/* 滑动胶囊指示器（绝对定位，随选中项平滑滑动；不拦截点击） */}
      <span
        aria-hidden
        className="pointer-events-none absolute rounded-full bg-accent-soft transition-[left,top,width,height] duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)]"
        style={{
          left: indicator.left,
          top: indicator.top,
          width: indicator.width,
          height: indicator.height,
        }}
      />
      {TABS.map((tab) => {
        const selected = tab.key === active;
        return (
          <button
            key={tab.key}
            type="button"
            ref={(el) => {
              itemRefs.current[tab.key] = el;
            }}
            onClick={() => onChange(tab.key)}
            className={`relative rounded-full px-4 py-1.5 text-sm transition-colors duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] ${
              selected ? "font-medium text-glow" : "text-ink-2 hover:text-ink"
            }`}
          >
            {tab.label}
          </button>
        );
      })}
    </div>
  );
}
