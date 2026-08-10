// src/lib/theme.tsx
// 主题管理（React 19 客户端组件）：冷月/薄雾即时切换，localStorage 持久化。
//
// 设计依据：《主题设置》画板——「冷月适合夜里阅读，薄雾更清亮通透。可随时切换。」
// 实现：
//   - data-theme="cool-moon" | "mist" 控制设计令牌（tokens.css）
//   - 未登录也可切换（需求文档 3.11：即时切换 + localStorage 持久化）
//   - 「跟随系统」模式：监听 prefers-color-scheme（设计稿主题设置页第三选项）
"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";

import type { ThemeName } from "@/types/api";

// 主题模式：固定双主题或跟随系统
export type ThemeMode = ThemeName | "system";

// localStorage 键名（避免与其他站点冲突）
const THEME_KEY = "yueyan-theme";
// 默认主题（与 .env/seed 的 settings.theme=cool-moon 一致）
const DEFAULT_THEME: ThemeName = "cool-moon";

// 主题上下文值：当前生效主题（跟随系统时解析为具体主题）+ 模式 + 切换方法
interface ThemeContextValue {
  theme: ThemeName; // 当前实际生效主题
  mode: ThemeMode; // 当前模式（含 system）
  setMode: (mode: ThemeMode) => void; // 设置模式（立即生效并持久化）
}

// 创建上下文（默认值兜底，Provider 未包裹时使用）
const ThemeContext = createContext<ThemeContextValue>({
  theme: DEFAULT_THEME,
  mode: DEFAULT_THEME,
  setMode: () => {},
});

// resolveSystemTheme 按系统深浅色偏好解析具体主题。
function resolveSystemTheme(): ThemeName {
  // 服务端渲染阶段无法访问浏览器偏好，返回默认主题
  if (typeof window === "undefined") {
    return DEFAULT_THEME;
  }
  return window.matchMedia("(prefers-color-scheme: light)").matches ? "mist" : "cool-moon";
}

// applyTheme 将主题写入根元素 data-theme（即时生效）。
function applyTheme(theme: ThemeName): void {
  // 仅客户端执行；根元素优先取 documentElement
  if (typeof document === "undefined") {
    return;
  }
  document.documentElement.setAttribute("data-theme", theme);
}

// ThemeProvider 主题上下文提供者（客户端组件）。
export function ThemeProvider({ children }: { children: ReactNode }) {
  // 初始化模式：localStorage 无记录时跟随系统偏好
  const [mode, setModeState] = useState<ThemeMode>(DEFAULT_THEME);

  // 挂载后读取持久化模式（避免 SSR 水合不一致）
  useEffect(() => {
    const saved = localStorage.getItem(THEME_KEY) as ThemeMode | null;
    if (saved === "cool-moon" || saved === "mist" || saved === "system") {
      setModeState(saved);
    } else {
      setModeState("system");
    }
    // 初始应用主题（防止水合前闪烁）
    applyTheme(saved && saved !== "system" ? saved : resolveSystemTheme());
  }, []);

  // 当前实际生效主题：system 模式跟随系统变化
  const [systemTheme, setSystemTheme] = useState<ThemeName>(DEFAULT_THEME);
  useEffect(() => {
    // 监听系统深浅色变化（跟随系统模式动态响应）
    const mq = window.matchMedia("(prefers-color-scheme: light)");
    const onChange = () => setSystemTheme(resolveSystemTheme());
    onChange();
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  // 计算当前生效主题
  const theme: ThemeName = mode === "system" ? systemTheme : mode;

  // 主题应用副作用：模式或系统主题变化时同步到根元素
  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  // 设置模式：持久化到 localStorage 并更新状态
  const setMode = useCallback((next: ThemeMode) => {
    setModeState(next);
    localStorage.setItem(THEME_KEY, next);
  }, []);

  return (
    <ThemeContext.Provider value={{ theme, mode, setMode }}>
      {children}
    </ThemeContext.Provider>
  );
}

// useTheme 读取主题上下文（组件内使用）。
export function useTheme(): ThemeContextValue {
  return useContext(ThemeContext);
}
