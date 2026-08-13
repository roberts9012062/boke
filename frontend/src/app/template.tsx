// src/app/template.tsx
// 路由级模板：每次导航重新挂载，为全站页面提供统一的入场动画（读取页面展示）。
// 说明：Next.js App Router 中 template 位于 layout 与 page 之间，导航时仅 page 区重新入场；
// 减少动效开启时（data-motion="reduced"）动画由 globals.css 全局规则自动关闭。
import type { ReactNode } from "react";

// Template 全站页面入场动画（上移淡入 0.3s，一次性）。
export default function Template({ children }: { children: ReactNode }) {
  return <div className="animate-fade-up">{children}</div>;
}
