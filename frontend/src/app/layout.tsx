// src/app/layout.tsx
// 根布局：全局字体/主题/外观/认证 Provider + 双端布局骨架（桌面三栏 / 移动底部导航）。
import type { Metadata } from "next";
import "./globals.css";

import { AuthProvider } from "@/lib/auth";
import { ThemeProvider } from "@/lib/theme";
import { AppearanceProvider } from "@/lib/appearance";
import { MaintenanceGate } from "@/components/maintenance-gate";
import { OfflineOverlay } from "@/components/ui/offline";

// 站点元信息（与设计稿/seed 站点名一致：月言）
export const metadata: Metadata = {
  title: "月言 · 月色微博客",
  description: "写短句，收声音，偶尔录一点夜色。",
};

// RootLayout 根布局：主题与认证 Provider 包裹全局内容 + 离线检测覆盖层（M1.7）。
export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" data-theme="cool-moon" suppressHydrationWarning>
      <body className="min-h-screen bg-bg font-ui text-ink antialiased">
        <ThemeProvider>
          <AppearanceProvider>
            <AuthProvider>
              {/* 全站维护拦截（M2）：维护开启时前台重定向维护页，覆盖引导页/登录页 */}
              <MaintenanceGate>
                {children}
                <OfflineOverlay />
              </MaintenanceGate>
            </AuthProvider>
          </AppearanceProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
