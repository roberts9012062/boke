// frontend/src/components/plugin-slot.tsx
// 插件扩展点槽位组件（M3.6）：按槽位挂载 running 插件的前端扩展。
// M3.9 增强：
//   - fallback 默认内容：存在 mode:"replace" 插件时隐藏，否则渲染 + append 插件共存
//   - props 透传：槽位向插件 register(ctx) 传业务参数（评论对象/页面参数等）
//   - extensions 清单模块级缓存（comment.item 每条评论挂载时避免重复请求）
// 说明：
//   - 挂载时拉取 running 插件清单 → 动态加载其 frontend 模块 → register(ctx) 渲染
//   - ctx.user 来自 AuthProvider（未登录为 null）；ctx.api 为受限插件 API 客户端
//   - 插件停用/卸载后刷新页面即移除（registry 按 running 过滤）
"use client";

import { useEffect, useRef, useState } from "react";

import { apiPluginExtensions } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { fetchManifest } from "@/plugins/loader";
import { registry } from "@/plugins/registry";
import type { PluginUser } from "@/plugins/loader";

// running 插件扩展清单缓存（slot → items；防 comment.item 每条评论重复请求；30 秒过期）。
const extensionsCache = new Map<string, { items: { plugin_id: string }[]; at: number }>();
const EXTENSIONS_TTL = 30_000;

// fetchExtensionsCached 拉取 running 插件清单（带缓存）。
async function fetchExtensionsCached(slot: string): Promise<{ plugin_id: string }[]> {
  const cached = extensionsCache.get(slot);
  if (cached && Date.now() - cached.at < EXTENSIONS_TTL) {
    return cached.items;
  }
  const r = await apiPluginExtensions();
  extensionsCache.set(slot, { items: r.items, at: Date.now() });
  return r.items;
}

// PluginSlot 插件槽位。
// 参数：slot 槽位名（theme.header/post.footer/comment.footer/admin.menu/comment.item）；
//      fallback 默认内容（replace 模式插件存在时隐藏）；props 透传给插件 register(ctx)；
//      onPluginsChange 回调（M4.2：槽位实际挂载的插件数，0=无插件订阅，供父组件切换布局）。
export default function PluginSlot({
  slot,
  fallback,
  props,
  onPluginsChange,
}: {
  slot: string;
  fallback?: React.ReactNode;
  props?: Record<string, unknown>;
  onPluginsChange?: (count: number) => void;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const { user } = useAuth();
  const [error, setError] = useState<string>("");
  // 是否存在 replace 模式插件（决定 fallback 是否渲染）
  const [replaced, setReplaced] = useState<boolean>(false);
  // 保存最新回调（避免回调引用变化触发重挂载）
  const onPluginsChangeRef = useRef(onPluginsChange);
  onPluginsChangeRef.current = onPluginsChange;

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

    // 判定 replace + 统计订阅该槽位的插件数（并行拉 manifests）
    const inspectPlugins = async (
      items: { plugin_id: string }[],
    ): Promise<{ hasReplace: boolean; count: number }> => {
      const manifests = await Promise.all(
        items.map(async (p) => {
          try {
            return await fetchManifest(p.plugin_id);
          } catch {
            return null;
          }
        }),
      );
      const subscribed = manifests.filter(
        (m): m is NonNullable<typeof m> => m !== null && m.extensionPoints.some((pt) => pt.slot === slot),
      );
      const hasReplace = subscribed.some((m) =>
        m.extensionPoints.some((pt) => pt.slot === slot && pt.mode === "replace"),
      );
      return { hasReplace, count: subscribed.length };
    };

    fetchExtensionsCached(slot)
      .then(async (items) => {
        if (cancelled) {
          return;
        }
        const { hasReplace, count } = await inspectPlugins(items);
        if (cancelled) {
          return;
        }
        setReplaced(hasReplace);
        // 通知父组件：该槽位实际有多少插件订阅（0 = 无插件内容）
        onPluginsChangeRef.current?.(count);
        // 逐个挂载 running 插件的该槽位扩展（单插件失败不影响其他）
        return Promise.all(
          items.map(async (p) => {
            try {
              cleanups.push(await registry.mountSlot(p.plugin_id, slot, el, pluginUser, props));
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
    // 注意：props 为挂载时快照（不含在依赖中）——插件经 props.onChange 类回调回写状态，
    // 若 props 变化触发重挂载会导致插件表单输入失焦/重置（M4.1 约定：回调键以 on 开头，
    // 值变化不重挂载；现有使用点 props 均为静态数据，行为不变）
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slot, user]);

  return (
    <div data-plugin-slot={slot}>
      {/* 默认内容：无 replace 插件时渲染（append 插件与默认内容共存） */}
      {!replaced && fallback != null && <div data-plugin-fallback>{fallback}</div>}
      {/* 插件挂载点（replace 模式下仅插件内容） */}
      <div ref={containerRef} />
      {error && <p className="text-xs text-like">{error}</p>}
    </div>
  );
}
