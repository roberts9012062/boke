// src/app/settings/theme/page.tsx
// 外观设置页（设计稿《设置》外观 Tab，D/M 双端）：
// 主题模式（冷月/薄雾/跟随系统）+ 阅读字号 + 内容密度 + 减少动效/高对比/自动播放开关
// + 恢复默认 / 保存外观；更改即时生效（lib/appearance.tsx + theme.tsx）。
"use client";

import Link from "next/link";
import { useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { useAppearance, type Density, type FontScale } from "@/lib/appearance";
import { useTheme, type ThemeMode } from "@/lib/theme";

// 主题选项（设计稿：冷月银辉 · 深夜银辉，低眩光 / 月下薄雾 · 晨雾浅灰，日间阅读 / 跟随系统）
const THEME_OPTIONS: readonly { mode: ThemeMode; title: string; desc: string; swatch: string }[] = [
  { mode: "cool-moon", title: "冷月银辉", desc: "深夜银辉 · 低眩光", swatch: "bg-[#10141f]" },
  { mode: "mist", title: "月下薄雾", desc: "晨雾浅灰 · 日间阅读", swatch: "bg-[#e9edf2]" },
  { mode: "system", title: "跟随系统", desc: "根据设备深浅色自动切换", swatch: "bg-gradient-to-r from-[#10141f] to-[#e9edf2]" },
];

// 字号档位（设计稿：小/中/大/特大）
const FONT_OPTIONS: readonly { key: FontScale; label: string }[] = [
  { key: "small", label: "小" },
  { key: "medium", label: "中" },
  { key: "large", label: "大" },
  { key: "xlarge", label: "特大" },
];

// 内容密度档位（设计稿：紧凑/舒适/宽松）
const DENSITY_OPTIONS: readonly { key: Density; label: string; desc: string }[] = [
  { key: "compact", label: "紧凑", desc: "列表更密，一屏更多帖" },
  { key: "cozy", label: "舒适", desc: "默认间距，推荐阅读" },
  { key: "loose", label: "宽松", desc: "大留白，沉浸浏览" },
];

// 开关项（设计稿：减少动效/高对比文本/自动播放媒体）
const TOGGLE_OPTIONS: readonly { key: "reduceMotion" | "highContrast" | "autoplayMedia"; label: string; desc: string }[] = [
  { key: "reduceMotion", label: "减少动效", desc: "降低过渡与微动效，更安静" },
  { key: "highContrast", label: "高对比文本", desc: "增强正文字与背景的对比" },
  { key: "autoplayMedia", label: "自动播放媒体", desc: "时间线中的视频/音频自动播放" },
];

// ThemeSettingsPage 外观设置页（未登录也可切换，需求 3.11 + 设计稿设置页）。
export default function ThemeSettingsPage() {
  const { mode, setMode } = useTheme();
  const { settings, update, reset } = useAppearance();
  const [savedTip, setSavedTip] = useStateTip();

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-[640px] flex-1 px-4 py-6 pb-20">
        {/* 顶部 Tab（设计稿设置页：资料/隐私/通知/外观/安全；M 后置修复：
            五页均已上线，由原先「M2 上线」灰按钮改为真实链接） */}
        <div className="flex gap-2">
          {[
            { key: "profile", label: "资料", href: "/settings/profile" },
            { key: "privacy", label: "隐私", href: "/settings/privacy" },
            { key: "notify", label: "通知", href: "/settings/notifications" },
            { key: "appearance", label: "外观", href: "/settings/theme", active: true },
            { key: "security", label: "安全", href: "/settings/security" },
          ].map((t) => (
            <Link
              key={t.key}
              href={t.href}
              className={`rounded-full px-4 py-1.5 text-sm transition-colors ${
                t.active
                  ? "bg-accent-soft font-medium text-glow"
                  : "text-ink-3/60 hover:text-ink"
              }`}
            >
              {t.label}
            </Link>
          ))}
        </div>

        <h1 className="mt-4 font-display text-xl font-semibold text-ink">外观</h1>
        <p className="mt-1 text-xs text-ink-3">调整主题与阅读体验，随心切换月色</p>

        {/* 主题模式（设计稿：冷月银辉/月下薄雾/跟随系统三卡片） */}
        <section className="mt-6">
          <h2 className="text-sm font-medium text-ink">主题模式</h2>
          <p className="mt-0.5 text-xs text-ink-3">跟随系统 · 可手动覆盖</p>
          <div className="mt-3 grid gap-3 sm:grid-cols-3">
            {THEME_OPTIONS.map((opt) => {
              const selected = mode === opt.mode;
              return (
                <button
                  key={opt.mode}
                  type="button"
                  onClick={() => setMode(opt.mode)}
                  aria-pressed={selected}
                  className={`rounded-lg border p-4 text-left transition-colors ${
                    selected ? "border-accent bg-accent-soft/60" : "border-line bg-elevated hover:border-ink-3/40"
                  }`}
                >
                  <span className={`block h-12 rounded-md border border-line ${opt.swatch}`} aria-hidden />
                  <p className={`mt-2.5 text-sm font-medium ${selected ? "text-glow" : "text-ink"}`}>
                    {opt.title}
                  </p>
                  <p className="mt-0.5 text-xs text-ink-3">{opt.desc}</p>
                  {selected && (
                    <span className="mt-1.5 inline-flex items-center gap-1 text-xs font-medium text-glow">
                      <span aria-hidden>●</span> 当前
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        </section>

        {/* 阅读字号（设计稿：影响帖文与评论 + 预览） */}
        <section className="mt-7">
          <h2 className="text-sm font-medium text-ink">阅读字号</h2>
          <p className="mt-0.5 text-xs text-ink-3">影响帖文与评论</p>
          <div className="mt-3 flex gap-2">
            {FONT_OPTIONS.map((opt) => (
              <button
                key={opt.key}
                type="button"
                onClick={() => update({ fontScale: opt.key })}
                aria-pressed={settings.fontScale === opt.key}
                className={`flex-1 rounded-full border py-2 text-sm transition-colors ${
                  settings.fontScale === opt.key
                    ? "border-accent bg-accent-soft font-medium text-glow"
                    : "border-line text-ink-2 hover:text-ink"
                }`}
              >
                {opt.label}
              </button>
            ))}
          </div>
          {/* 字号预览（设计稿：预览 · 夜航笔记 段落） */}
          <div className="mt-3 rounded-lg border border-line bg-elevated p-4">
            <p className="text-xs text-ink-3">预览 · 夜航笔记</p>
            <p className="mt-1.5 text-sm leading-relaxed text-ink">
              月光从窗格里淌进来，把桌案上的字迹洗成银白。长巷尽头风声低语，像有人在念旧日的诗。
            </p>
          </div>
        </section>

        {/* 内容密度（设计稿：紧凑/舒适/宽松） */}
        <section className="mt-7">
          <h2 className="text-sm font-medium text-ink">内容密度</h2>
          <div className="mt-3 grid gap-2 sm:grid-cols-3">
            {DENSITY_OPTIONS.map((opt) => (
              <button
                key={opt.key}
                type="button"
                onClick={() => update({ density: opt.key })}
                aria-pressed={settings.density === opt.key}
                className={`rounded-lg border p-3 text-left transition-colors ${
                  settings.density === opt.key
                    ? "border-accent bg-accent-soft/60"
                    : "border-line bg-elevated hover:border-ink-3/40"
                }`}
              >
                <p className={`text-sm font-medium ${settings.density === opt.key ? "text-glow" : "text-ink"}`}>
                  {opt.label}
                </p>
                <p className="mt-0.5 text-xs text-ink-3">{opt.desc}</p>
              </button>
            ))}
          </div>
        </section>

        {/* 开关项（设计稿：减少动效/高对比文本/自动播放媒体） */}
        <section className="mt-7">
          <h2 className="text-sm font-medium text-ink">阅读增强</h2>
          <div className="mt-3 divide-y divide-line rounded-lg border border-line bg-elevated">
            {TOGGLE_OPTIONS.map((opt) => (
              <label key={opt.key} className="flex cursor-pointer items-center justify-between px-4 py-3">
                <span>
                  <span className="block text-sm text-ink">{opt.label}</span>
                  <span className="mt-0.5 block text-xs text-ink-3">{opt.desc}</span>
                </span>
                {/* 滑块开关（iOS 风格：圆点 + 轨道，选中平移变色） */}
                <span className="relative inline-flex h-5 w-9 shrink-0">
                  <input
                    type="checkbox"
                    checked={settings[opt.key]}
                    onChange={(e) => update({ [opt.key]: e.target.checked })}
                    className="peer sr-only"
                    aria-label={opt.label}
                  />
                  <span className="absolute inset-0 rounded-full bg-muted transition-colors peer-checked:bg-accent" />
                  <span className="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform peer-checked:translate-x-4" />
                </span>
              </label>
            ))}
          </div>
        </section>

        {/* 底部操作（设计稿：更改即时生效，可随时恢复默认 / 恢复默认 / 保存外观） */}
        <p className="mt-6 text-center text-xs text-ink-3">更改即时生效，可随时恢复默认</p>
        <div className="mt-3 flex justify-center gap-3">
          <button
            type="button"
            onClick={reset}
            className="rounded-full border border-line px-6 py-2.5 text-sm text-ink-2 transition-colors hover:text-ink"
          >
            恢复默认
          </button>
          <button
            type="button"
            onClick={() => setSavedTip(true)}
            className="rounded-full bg-accent px-8 py-2.5 text-sm font-medium text-on-accent transition-opacity hover:opacity-90"
          >
            保存外观
          </button>
        </div>
        {savedTip && (
          <p className="mt-3 text-center text-xs text-glow" role="status">
            已保存（更改已即时生效）
          </p>
        )}
      </main>
      <MobileTabbar />
    </div>
  );
}

// useStateTip 保存成功提示（3 秒后自动消失）。
function useStateTip(): [boolean, (v: boolean) => void] {
  const [show, setShow] = useState<boolean>(false);
  const setAndHide = (v: boolean) => {
    setShow(v);
    if (v) {
      setTimeout(() => setShow(false), 3000);
    }
  };
  return [show, setAndHide];
}
