// src/app/admin/nav/page.tsx
// 头部导航管理页（独立一级菜单）：两级导航树编辑 + 创建面板。
// 逻辑在 components/admin/nav/nav-menu-manager.tsx（文件行数硬性指标拆分）。
"use client";

import { NavMenuManager } from "@/components/admin/nav/nav-menu-manager";

// AdminNavPage 头部导航管理页。
export default function AdminNavPage() {
  return (
    <div className="max-w-[760px]">
      <h1 className="font-display text-xl font-semibold text-ink">头部导航</h1>
      <p className="mt-0.5 text-xs text-ink-3">
        自定义前台桌面端头部导航：支持一级与二级（鼠标悬停一级项动画展开二级）；保存后即时生效
      </p>
      <NavMenuManager />
    </div>
  );
}
