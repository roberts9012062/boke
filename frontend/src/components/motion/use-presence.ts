// src/components/motion/use-presence.ts
// 弹层进出场状态 hook：关闭时先播离场动画再卸载（避免条件渲染瞬时消失）。
// 用法：const { mounted, leaving } = usePresence(open, 180); mounted 为 false 时不渲染。
"use client";

import { useEffect, useState } from "react";

// PresenceState 进出场状态。
export interface PresenceState {
  mounted: boolean; // 是否保持挂载（含离场动画播放期间）
  leaving: boolean; // 是否正在离场（用于切换离场动画类）
}

// usePresence 进出场状态机。
// 参数：open 目标显示状态；exitDurationMs 离场动画时长（毫秒，与 CSS 动画时长一致）。
export function usePresence(open: boolean, exitDurationMs: number): PresenceState {
  const [mounted, setMounted] = useState<boolean>(open);
  const [leaving, setLeaving] = useState<boolean>(false);

  useEffect(() => {
    // 打开：立即挂载并清除离场态
    if (open) {
      setMounted(true);
      setLeaving(false);
      return;
    }
    // 关闭且未挂载：无需处理
    if (!mounted) {
      return;
    }
    // 关闭：先进入离场态，播完离场动画后再卸载
    setLeaving(true);
    const timer = setTimeout(() => {
      setMounted(false);
      setLeaving(false);
    }, exitDurationMs);
    return () => clearTimeout(timer);
  }, [open, mounted, exitDurationMs]);

  return { mounted, leaving };
}
