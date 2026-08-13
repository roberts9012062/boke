// src/components/ui/toast.tsx
// 轻量 Toast 提示（对齐设计稿「D/冷月/弹窗提示·Toast」：短消息自动消失）：
//   命令式调用 toast.success(msg) / toast.error(msg)；顶部居中，3 秒自动消失。
// 取代 alert（系统默认弹窗）；后续可扩展为完整 Toast 体系（后置规划 #169）。
// 动效：顶部滑入（slide-down）；消失前先播淡出动画再移除（离场 200ms）。
"use client";

import { useCallback, useEffect, useState } from "react";

// ToastItem 单条提示。
interface ToastItem {
  id: number; // 唯一 ID
  type: "success" | "error"; // 成功 / 错误
  message: string; // 文案
  leaving: boolean; // 离场动画标记（淡出中）
}

// 命令式 API（模块级单例状态，页面组件外可直接调用）。
let pushToast: ((type: "success" | "error", message: string) => void) | null = null;

// toast 命令式 API（alert 替代：toast.success / toast.error）。
export const toast = {
  success(message: string): void {
    pushToast?.("success", message);
  },
  error(message: string): void {
    pushToast?.("error", message);
  },
};

// ToastContainer 提示容器（挂载一次；经 toast 命令式 API 驱动）。
export function ToastContainer() {
  const [items, setItems] = useState<ToastItem[]>([]);
  let nextId = 1;

  // 注册命令式入口（组件挂载后可用）
  useEffect(() => {
    pushToast = (type, message) => {
      const id = nextId++;
      setItems((prev) => [...prev, { id, type, message, leaving: false }]);
      // 2.8 秒后进入离场态（淡出 200ms），3 秒整移除
      setTimeout(() => {
        setItems((prev) => prev.map((t) => (t.id === id ? { ...t, leaving: true } : t)));
      }, 2800);
      setTimeout(() => {
        setItems((prev) => prev.filter((t) => t.id !== id));
      }, 3000);
    };
    return () => {
      pushToast = null;
    };
  }, []);

  return (
    <div className="pointer-events-none fixed inset-x-0 top-4 z-[60] flex flex-col items-center gap-2 px-4">
      {items.map((t) => (
        <div
          key={t.id}
          role="status"
          className={`pointer-events-auto flex items-center gap-2 rounded-lg border border-line bg-elevated px-4 py-2.5 text-sm text-ink shadow-lg ${
            t.leaving ? "animate-fade-out" : "animate-slide-down"
          }`}
        >
          <span aria-hidden className={t.type === "success" ? "text-glow" : "text-like"}>
            {t.type === "success" ? "✓" : "✗"}
          </span>
          <span>{t.message}</span>
        </div>
      ))}
    </div>
  );
}

// useToast 组件内安全调用（pushToast 未挂载时静默——避免 SSR/早期调用报错）。
export function useToast() {
  return useCallback((type: "success" | "error", message: string) => {
    pushToast?.(type, message);
  }, []);
}
