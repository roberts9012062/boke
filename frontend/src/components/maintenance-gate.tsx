// src/components/maintenance-gate.tsx
// 全站维护拦截（M2）：挂载于根布局，覆盖全部前台页面（含引导页/登录页/注册页）。
// 策略：挂载时读取站点元信息（/api/v1/meta，维护时后端放行该接口），
//   维护开启（maintenance_mode=on）时重定向到维护页。
// 放行路径：/admin 前缀（含 /admin-login，管理员可进入后台关闭开关）、/maintenance（维护页自身）。
// 兜底：业务接口 503（错误码 6004）时由 api.ts 统一跳转维护页。
"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";

// MaintenanceGate 维护拦截组件（纯客户端，不渲染额外 DOM）。
export function MaintenanceGate({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();

  useEffect(() => {
    // 后台与维护页不拦截（管理员可进后台关闭开关；维护页自身不请求业务接口）
    if (pathname.startsWith("/admin") || pathname.startsWith("/maintenance")) {
      return;
    }
    fetch("/api/v1/meta")
      .then((r) => r.json())
      .then((body: { data?: { maintenance_mode?: string } }) => {
        if (body.data?.maintenance_mode === "on") {
          router.replace("/maintenance");
        }
      })
      .catch(() => {
        // 网络异常静默（离线场景由 OfflineOverlay 处理）
      });
  }, [pathname, router]);

  return <>{children}</>;
}
