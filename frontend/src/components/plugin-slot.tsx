// frontend/src/components/plugin-slot.tsx
// 插件扩展点槽位组件（M3.6）：按槽位挂载 running 插件的前端扩展。
// 说明：
//   - 挂载时拉取 running 插件清单 → 动态加载其 frontend 模块 → register(ctx) 渲染
//   - ctx.user 来自 AuthProvider（未登录为 null）；ctx.api 为受限插件 API 客户端
//   - 插件停用/卸载后刷新页面即移除（registry 按 running 过滤）
"use client";

import { useEffect, useRef, useState } from "react";

import { apiPluginExtensions } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { registry } from "@/plugins/registry";
import type { PluginUser } from "@/plugins/loader";

// PluginSlot 插件槽位。
// 参数：slot 槽位名（theme.header / post.footer / comment.footer / admin.menu）。
export default function PluginSlot({ slot }: { slot: string }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const { user } = useAuth();
  const [error, setError] = useState<string>("");

  useEffect(() => {
    const el = containerRef.current;
    if (!el) {
      return;
    }
    let cancelled = false;
    const cleanups: (() => void)[] = [];

    // 用户信息脱敏（插件可见的最小面）
    const pluginUser: PluginUser | null = user
      ? { id: user.id, name: user.nickname || user.username, role: user.role ?? "" }
      : null;

    apiPluginExtensions()
      .then((r) => {
        if (cancelled) {
          return;
        }
        // 逐个挂载 running 插件的该槽位扩展（单插件失败不影响其他）
        return Promise.all(
          r.items.map(async (p) => {
            try {
              cleanups.push(await registry.mountSlot(p.plugin_id, slot, el, pluginUser));
            } catch {
              /* 单个插件加载失败静默（其余插件不受影响） */
            }
          }),
        );
      })
      .catch(() => {
        if (!cancelled) {
          setError("插件扩展加载失败");
        }
      });

    return () => {
      cancelled = true;
      cleanups.forEach((fn) => fn());
    };
  }, [slot, user]);

  return (
    <div ref={containerRef} data-plugin-slot={slot}>
      {error && <p className="text-xs text-like">{error}</p>}
    </div>
  );
}
