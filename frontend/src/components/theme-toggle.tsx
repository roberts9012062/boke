// src/components/theme-toggle.tsx
// 主题快速切换按钮：点击在 冷月 / 薄雾 之间切换（遵循系统模式时先切为具体主题）。
// 设计依据：《主题设置》画板——冷月「深蓝夜色，护眼且沉静」、薄雾「浅灰雾面，日间更清晰」。
"use client";

import { useTheme } from "@/lib/theme";

// ThemeToggle 顶部导航主题切换按钮（半透明圆角胶囊，hover 高亮）。
export function ThemeToggle() {
  const { theme, setMode } = useTheme();

  // 点击切换：冷月 ⇄ 薄雾（并退出跟随系统模式，进入固定主题）
  const handleToggle = () => {
    setMode(theme === "cool-moon" ? "mist" : "cool-moon");
  };

  return (
    <button
      type="button"
      onClick={handleToggle}
      title={theme === "cool-moon" ? "当前：冷月，点击切换薄雾" : "当前：薄雾，点击切换冷月"}
      className="group flex h-9 items-center gap-1.5 rounded-full border border-line bg-muted px-3 text-sm text-ink-2 transition-colors hover:text-ink"
      aria-label="切换主题"
    >
      {/* 月亮图标（冷月）与云雾图标（薄雾）由文字符号表达，主题名实时显示；
          hover 轻旋转，切换主题时重播弹跳（key 随主题变化，animate-pop 无 fill 不锁定 transform） */}
      <span
        key={theme}
        aria-hidden
        className="inline-block animate-pop transition-transform duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] group-hover:rotate-12"
      >
        {theme === "cool-moon" ? "🌙" : "☁️"}
      </span>
      <span className="text-xs font-medium">{theme === "cool-moon" ? "冷月" : "薄雾"}</span>
    </button>
  );
}
