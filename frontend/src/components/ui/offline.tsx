// src/components/ui/offline.tsx
// 离线检测（M1.7 组件化，需求文档 2.3 无网络状态）：
// 监听 navigator.onLine，断网时显示全屏提示 + 自动重试（恢复后自动隐藏）。
// 设计稿《无网络》画板：离线 → 没有网络信号 → 请检查 Wi-Fi 或蜂窝数据 → 重试。
// 说明：初始状态不显示（避免加载初期网络未就绪的环境误报，如 Electron 沙箱）；
//       收到 offline 信号（事件或轮询）才显示，恢复 online 自动隐藏。
"use client";

import { useCallback, useEffect, useState } from "react";

// useOffline 监听浏览器离线状态（初始 false = 在线，仅在收到离线信号后为 true）。
function useOffline(): boolean {
  const [offline, setOffline] = useState<boolean>(false);
  useEffect(() => {
    // 事件监听（常规浏览器路径）
    const on = () => setOffline(false);
    const off = () => setOffline(true);
    window.addEventListener("online", on);
    window.addEventListener("offline", off);
    // 轮询兜底（事件不可靠环境，3s 周期）
    const timer = setInterval(() => {
      setOffline(!navigator.onLine);
    }, 3000);
    return () => {
      window.removeEventListener("online", on);
      window.removeEventListener("offline", off);
      clearInterval(timer);
    };
  }, []);
  return offline;
}

// OfflineOverlay 断网全屏提示（挂载在根布局，断网自动出现）。
export function OfflineOverlay() {
  const offline = useOffline();

  // 重试：刷新当前页（网络恢复后重新加载数据）
  const handleRetry = useCallback(() => {
    window.location.reload();
  }, []);

  // 在线时不渲染
  if (!offline) {
    return null;
  }

  return (
    <div
      className="fixed inset-0 z-[60] flex flex-col items-center justify-center bg-bg px-8 text-center"
      role="alert"
      aria-label="无网络"
    >
      <span className="text-4xl" aria-hidden>
        📡
      </span>
      <p className="mt-4 font-display text-xl font-semibold text-ink">没有网络信号</p>
      <p className="mt-2 text-sm text-ink-2">请检查 Wi-Fi 或蜂窝数据，恢复后自动重试。</p>
      <div className="mt-6 flex gap-3">
        {/* 返回首页（设计稿《无网络》画板按钮组；走查纠偏补） */}
        <a
          href="/"
          className="rounded-full bg-accent px-6 py-2.5 text-sm font-medium text-on-accent transition-opacity hover:opacity-90"
        >
          返回首页
        </a>
        <button
          type="button"
          onClick={handleRetry}
          className="rounded-full border border-line px-6 py-2.5 text-sm text-ink-2 hover:text-ink"
        >
          重试
        </button>
      </div>
    </div>
  );
}
