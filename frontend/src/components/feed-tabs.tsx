// src/components/feed-tabs.tsx
// 时间线过滤 Tab（设计稿 D/冷月/首页 中栏顶部）：全部 / 图 / 音 / 影。
// M1.3：选中 Tab 即触发时间线过滤（受控组件，由父页面管理状态）。
"use client";

// Tab 配置（文案与设计稿一致；影 = 视频，M2 后生效）
const TABS = [
  { key: "", label: "全部" },
  { key: "image", label: "图" },
  { key: "audio", label: "音" },
  { key: "video", label: "影" },
] as const;

// FeedTabs 时间线过滤 Tab（受控：active 当前选中值，onChange 回调）。
export function FeedTabs({
  active,
  onChange,
}: {
  active: string; // 当前选中（空 = 全部）
  onChange: (key: string) => void; // 切换回调
}) {
  return (
    <div className="flex items-center gap-2 border-b border-line pb-3">
      {TABS.map((tab) => {
        const selected = tab.key === active;
        return (
          <button
            key={tab.key}
            type="button"
            onClick={() => onChange(tab.key)}
            className={`rounded-full px-4 py-1.5 text-sm transition-colors ${
              selected
                ? "bg-accent-soft font-medium text-glow"
                : "bg-muted text-ink-2 hover:text-ink"
            }`}
          >
            {tab.label}
          </button>
        );
      })}
    </div>
  );
}
