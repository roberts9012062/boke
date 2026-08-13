// src/components/motion/use-dismiss.ts
// 弹层关闭过渡 hook：组件由父级条件渲染（挂载即打开），关闭时先播离场动画再回调 onClose。
// 与 use-presence 的差别：use-presence 需要外部 open 状态驱动，本 hook 适合「挂载即打开」的弹层
// （lightbox / share-panel / report-dialog 等由父组件 {open && <Panel/>} 条件渲染的场景）。
"use client";

import { useCallback, useRef, useState } from "react";

// DismissState 关闭过渡状态。
export interface DismissState {
  closing: boolean; // 是否正在离场（用于切换离场动画类）
  close: () => void; // 关闭入口（防重入：重复调用忽略）
}

// useDismiss 关闭过渡。
// 参数：onClose 关闭回调（离场动画播完后调用）；exitDurationMs 离场动画时长（毫秒）。
export function useDismiss(onClose: () => void, exitDurationMs: number): DismissState {
  const [closing, setClosing] = useState<boolean>(false);
  const closingRef = useRef<boolean>(false);
  // 始终指向最新的关闭回调（避免闭包捕获旧引用）
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  const close = useCallback(() => {
    // 防重入：已在离场中则忽略重复关闭
    if (closingRef.current) {
      return;
    }
    closingRef.current = true;
    setClosing(true);
    setTimeout(() => {
      onCloseRef.current();
    }, exitDurationMs);
  }, [exitDurationMs]);

  return { closing, close };
}
