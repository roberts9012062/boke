// src/app/admin/under-construction/page.tsx
// 后台建设中占位页（决策 5：媒体库/标签/角色/插件/插件商城/SEO/健康度/SERP 均指向此页）。
"use client";

import Link from "next/link";

// UnderConstruction 建设中占位（设计稿风格：图标 + 文案 + 返回）。
export default function UnderConstruction() {
  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center">
      <div className="flex h-16 w-16 items-center justify-center rounded-full bg-accent-soft text-3xl text-glow">
        🚧
      </div>
      <p className="mt-4 font-display text-lg font-medium text-ink">该模块建设中</p>
      <p className="mt-1 text-sm text-ink-2">规划见路线图（后续里程碑上线）</p>
      <Link
        href="/admin"
        className="mt-6 rounded-full border border-line px-6 py-2 text-sm text-ink-2 hover:text-ink"
      >
        ← 返回仪表盘
      </Link>
    </div>
  );
}
