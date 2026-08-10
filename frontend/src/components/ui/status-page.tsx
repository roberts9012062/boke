// src/components/ui/status-page.tsx
// 统一状态页组件（M1.7 组件化，需求文档 2.3）：
// 404 / 500 / 维护中 共用形态（设计稿各画板文案不同，按钮组合不同）。
"use client";

import Link from "next/link";

// StatusPageProps 状态页参数。
interface StatusPageProps {
  code: string; // 大号状态码（如 404 / 500 / 维护）
  title: string; // 主文案（如「这条月色走丢了」）
  description: string; // 次文案（如「页面不存在，或已被作者收起。」）
  showSearch?: boolean; // 是否显示「去搜索」按钮（桌面设计稿有）
  retryable?: boolean; // 是否显示「重试」按钮（移动端设计稿有）
}

// StatusPage 统一状态页（404/500/维护）。
export function StatusPage({
  code,
  title,
  description,
  showSearch = false,
  retryable = false,
}: StatusPageProps) {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-bg px-8 text-center">
      {/* 状态码（设计稿：大号数字/文字） */}
      <p className="font-display text-6xl font-bold tracking-tight text-ink-3">{code}</p>
      <p className="mt-4 font-display text-xl font-semibold text-ink">{title}</p>
      <p className="mt-2 max-w-sm text-sm leading-relaxed text-ink-2">{description}</p>

      {/* 操作按钮组（设计稿：返回首页 / 去搜索 / 重试） */}
      <div className="mt-8 flex gap-3">
        <Link
          href="/"
          className="rounded-full bg-accent px-6 py-2.5 text-sm font-medium text-on-accent transition-opacity hover:opacity-90"
        >
          返回首页
        </Link>
        {showSearch && (
          <Link
            href="/search"
            className="rounded-full border border-line px-6 py-2.5 text-sm text-ink-2 transition-colors hover:text-ink"
          >
            去搜索
          </Link>
        )}
        {retryable && (
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="rounded-full border border-line px-6 py-2.5 text-sm text-ink-2 transition-colors hover:text-ink"
          >
            重试
          </button>
        )}
      </div>
    </div>
  );
}
