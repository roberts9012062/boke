// src/lib/appearance.tsx
// 外观偏好管理（M2 设计稿纠偏·设置页补全）：
// 阅读字号 / 内容密度 / 减少动效 / 高对比文本 / 自动播放媒体，
// localStorage 持久化 + html 属性即时生效（与主题 theme.tsx 同构）。
"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";

// 阅读字号档位（影响 html font-size，rem 全局缩放）
export type FontScale = "small" | "medium" | "large" | "xlarge";
// 内容密度档位（影响帖子列表间距）
export type Density = "compact" | "cozy" | "loose";

// 字号 → 根字号（px）。中 = 浏览器默认 16px
const FONT_SIZE: Record<FontScale, string> = {
  small: "14px",
  medium: "16px",
  large: "17px",
  xlarge: "19px",
};

// 外观设置（全部可选，便于恢复默认）
export interface AppearanceSettings {
  fontScale: FontScale; // 阅读字号
  density: Density; // 内容密度
  reduceMotion: boolean; // 减少动效
  highContrast: boolean; // 高对比文本
  autoplayMedia: boolean; // 时间线媒体自动播放
}

// 默认外观（设计稿推荐：舒适密度、中字号、跟随系统主题）
const DEFAULT_SETTINGS: AppearanceSettings = {
  fontScale: "medium",
  density: "cozy",
  reduceMotion: false,
  highContrast: false,
  autoplayMedia: false,
};

// localStorage 键名
const APPEARANCE_KEY = "yueyan-appearance";

// 外观上下文值
interface AppearanceContextValue {
  settings: AppearanceSettings; // 当前外观设置
  update: (patch: Partial<AppearanceSettings>) => void; // 局部更新（即时生效 + 持久化）
  reset: () => void; // 恢复默认
}

const AppearanceContext = createContext<AppearanceContextValue>({
  settings: DEFAULT_SETTINGS,
  update: () => {},
  reset: () => {},
});

// applyAppearance 将外观写入根元素（字号 + 数据属性，即时生效）。
function applyAppearance(settings: AppearanceSettings): void {
  if (typeof document === "undefined") {
    return;
  }
  const root = document.documentElement;
  root.style.fontSize = FONT_SIZE[settings.fontScale];
  root.dataset.density = settings.density;
  root.dataset.motion = settings.reduceMotion ? "reduced" : "normal";
  root.dataset.contrast = settings.highContrast ? "high" : "normal";
}

// readSaved 读取持久化外观（校验字段，损坏时回退默认）。
function readSaved(): AppearanceSettings {
  if (typeof window === "undefined") {
    return DEFAULT_SETTINGS;
  }
  try {
    const raw = localStorage.getItem(APPEARANCE_KEY);
    if (!raw) {
      return DEFAULT_SETTINGS;
    }
    const parsed = JSON.parse(raw) as Partial<AppearanceSettings>;
    return {
      fontScale: ["small", "medium", "large", "xlarge"].includes(parsed.fontScale ?? "")
        ? (parsed.fontScale as FontScale)
        : DEFAULT_SETTINGS.fontScale,
      density: ["compact", "cozy", "loose"].includes(parsed.density ?? "")
        ? (parsed.density as Density)
        : DEFAULT_SETTINGS.density,
      reduceMotion: Boolean(parsed.reduceMotion),
      highContrast: Boolean(parsed.highContrast),
      autoplayMedia: Boolean(parsed.autoplayMedia),
    };
  } catch {
    return DEFAULT_SETTINGS;
  }
}

// AppearanceProvider 外观偏好提供者（挂载于根布局）。
export function AppearanceProvider({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState<AppearanceSettings>(DEFAULT_SETTINGS);

  // 挂载时读取持久化设置并应用
  useEffect(() => {
    const saved = readSaved();
    setSettings(saved);
    applyAppearance(saved);
  }, []);

  // 设置变化时即时应用
  useEffect(() => {
    applyAppearance(settings);
  }, [settings]);

  // 局部更新（即时生效 + 持久化）
  const update = useCallback((patch: Partial<AppearanceSettings>) => {
    setSettings((prev) => {
      const next = { ...prev, ...patch };
      localStorage.setItem(APPEARANCE_KEY, JSON.stringify(next));
      return next;
    });
  }, []);

  // 恢复默认
  const reset = useCallback(() => {
    localStorage.setItem(APPEARANCE_KEY, JSON.stringify(DEFAULT_SETTINGS));
    setSettings(DEFAULT_SETTINGS);
  }, []);

  return (
    <AppearanceContext.Provider value={{ settings, update, reset }}>
      {children}
    </AppearanceContext.Provider>
  );
}

// useAppearance 读取外观上下文（组件内使用）。
export function useAppearance(): AppearanceContextValue {
  return useContext(AppearanceContext);
}
