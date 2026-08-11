// src/app/settings/page.tsx
// 设置根路由（走查纠偏：设计稿《设置》画板 = 外观页内容，对应实现为 /settings/theme；
// 根路由重定向到外观设置，避免 404）。
import { redirect } from "next/navigation";

// SettingsIndex 设置根路由 → 外观设置。
export default function SettingsIndex() {
  redirect("/settings/theme");
}
