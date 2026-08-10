// src/components/ui/empty-state.tsx
// 统一空状态组件（M1.7 组件化，需求文档 2.3 通用状态页）：
// 图标 + 主文案 + 次文案 + 可选操作按钮。
"use client";

import Link from "next/link";

// EmptyStateProps 空状态参数。
interface EmptyStateProps {
  icon?: string; // 图标字符（默认 🌙）
  title: string; // 主文案（如「还没有帖子」）
  description?: string; // 次文案（如「写下第一条月色短句吧」）
  actionText?: string; // 操作按钮文案（可选）
  actionHref?: string; // 操作按钮跳转（可选）
}

// EmptyState 统一空状态（居中卡片形态，与设计稿空状态画板一致）。
export function EmptyState({
  icon = "🌙",
  title,
  description,
  actionText,
  actionHref,
}: EmptyStateProps) {
  return (
    <div className="rounded-lg border border-line bg-elevated py-16 text-center">
      <span className="text-3xl" aria-hidden>
        {icon}
      </span>
      <p className="mt-3 font-display text-lg text-ink">{title}</p>
      {description && <p className="mt-1 text-sm text-ink-2">{description}</p>}
      {actionText && actionHref && (
        <Link
          href={actionHref}
          className="mt-5 inline-block rounded-full bg-accent px-6 py-2 text-sm font-medium text-on-accent transition-opacity hover:opacity-90"
        >
          {actionText}
        </Link>
      )}
    </div>
  );
}
