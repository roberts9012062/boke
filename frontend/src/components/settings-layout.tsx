// src/components/settings-layout.tsx
// 设置页分区布局（设计稿《账号安全》等画板顶部 Tab：资料/隐私/通知/外观/安全）。
// 说明：供隐私/通知/安全三页使用；资料（/settings/profile）、外观（/settings/theme）两页
//       为早期页面，顶部 Tab 栏独立实现（样式与本文案一致，均为真实链接，2026-08 后置修复统一）。
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

// 分区 Tab 配置（设计稿：资料/隐私/通知/外观/安全）
const TABS = [
  { key: "profile", label: "资料", href: "/settings/profile" },
  { key: "privacy", label: "隐私", href: "/settings/privacy" },
  { key: "notifications", label: "通知", href: "/settings/notifications" },
  { key: "theme", label: "外观", href: "/settings/theme" },
  { key: "security", label: "安全", href: "/settings/security" },
] as const;

// SettingsLayout 设置页布局：分区 Tab + 内容区。
// 参数：active 当前分区 key；children 内容。
export function SettingsLayout({ active, children }: { active: string; children: React.ReactNode }) {
  const pathname = usePathname();
  return (
    <div className="mx-auto w-full max-w-[720px] px-4 py-6 pb-20">
      {/* 分区 Tab（设计稿：资料 / 隐私 / 通知 / 外观 / 安全） */}
      <div className="flex gap-1 overflow-x-auto rounded-full border border-line p-1">
        {TABS.map((tab) => {
          const isActive = active === tab.key || pathname === tab.href;
          return (
            <Link
              key={tab.key}
              href={tab.href}
              className={`shrink-0 rounded-full px-4 py-1.5 text-sm transition-colors ${
                isActive ? "bg-accent-soft font-medium text-glow" : "text-ink-2 hover:text-ink"
              }`}
            >
              {tab.label}
            </Link>
          );
        })}
      </div>
      <div className="mt-5">{children}</div>
    </div>
  );
}
