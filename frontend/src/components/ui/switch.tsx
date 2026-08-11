// src/components/ui/switch.tsx
// 开关组件（插件 UI 基座）：取代各页面内联复制的开关代码（settings/notifications/privacy 等）。
"use client";

// Switch 开关（布尔值切换）。
// 参数：checked 当前值；onChange 变化回调；label aria 标签（无障碍）。
export function Switch({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
}) {
  return (
    <button
      type="button"
      onClick={() => onChange(!checked)}
      className={`relative h-6 w-11 shrink-0 rounded-full transition-colors ${
        checked ? "bg-accent" : "bg-muted"
      }`}
      aria-label={label}
    >
      <span
        className={`absolute top-0.5 h-5 w-5 rounded-full bg-white transition-all ${
          checked ? "left-[22px]" : "left-0.5"
        }`}
      />
    </button>
  );
}
