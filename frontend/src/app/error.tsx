// src/app/error.tsx
// 500 错误页（设计稿《500》画板）：「月光暂时熄了」+ 重试。
// 说明（M4 修复）：App Router 约定 app/error.tsx 在 RootLayout 内渲染（错误边界内容），
//   不能包含 <html>/<body>（根级错误才用 app/global-error.tsx）——
//   此前误含 html/body 导致「<html> cannot be a child of <body>」hydration 错误与错误边界二次崩溃。
"use client";

import Link from "next/link";

// ErrorBoundary 页面级错误边界（RootLayout 内渲染，继承主题 CSS 变量）。
// 参数：reset 由 Next 注入，点击重试时重置错误边界。
export default function GlobalError({ reset }: { reset: () => void }) {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-bg px-8 text-center">
      <p className="font-display text-5xl font-bold text-ink-3">500</p>
      <p className="mt-4 font-display text-xl font-semibold text-ink">月光暂时熄了</p>
      <p className="mt-2 text-sm text-ink-2">服务器出了点小差。我们已在修复，请稍后再试。</p>
      <div className="mt-8 flex gap-3">
        <Link
          href="/"
          className="rounded-full bg-accent px-6 py-2.5 text-sm font-medium text-on-accent hover:opacity-90"
        >
          返回首页
        </Link>
        <button
          type="button"
          onClick={reset}
          className="rounded-full border border-line px-6 py-2.5 text-sm text-ink-2 hover:text-ink"
        >
          重试
        </button>
      </div>
    </div>
  );
}
