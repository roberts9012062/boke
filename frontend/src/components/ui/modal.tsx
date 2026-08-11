// src/components/ui/modal.tsx
// 通用弹层组件（插件 UI 基座）：取代各页面内联复制的弹层代码。
// 桌面居中 + 移动底部弹层（对齐现有覆盖层模式）；遮罩点击关闭；aria 无障碍。
"use client";

import type { ReactNode } from "react";

// Modal 通用弹层。
// 参数：open 是否显示；title 标题；onClose 关闭回调；children 内容；maxWidth 最大宽度（默认 420）。
export function Modal({
  open,
  title,
  onClose,
  children,
  maxWidth = "max-w-[420px]",
}: {
  open: boolean;
  title: string;
  onClose: () => void;
  children: ReactNode;
  maxWidth?: string;
}) {
  if (!open) {
    return null;
  }
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-6"
      role="dialog"
      aria-modal="true"
      aria-label={title}
      onClick={onClose}
    >
      <div
        className={`w-full ${maxWidth} rounded-xl border border-line bg-elevated p-6`}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="font-display text-lg font-semibold text-ink">{title}</h2>
        <div className="mt-4">{children}</div>
      </div>
    </div>
  );
}
