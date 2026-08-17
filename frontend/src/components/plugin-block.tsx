"use client";

// frontend/src/components/plugin-block.tsx
// 插件内容块渲染组件（B4 keyed renderer 的分发端）：按块类型查注册表，
// 加载提供方插件的 ESM 模块并 register(ctx) 渲染（槽位契约复用，slot 名为 "block:{type}"）。
// 挂载协议与 PluginSlot 一致：useEffect + ref DOM + cleanup 函数精确清理。
import { useEffect, useRef, useState } from "react";

import { useAuth } from "@/lib/auth";
import { fetchBlockRegistry } from "@/plugins/block-registry";
import { loadModule, type PluginUser } from "@/plugins/loader";
import { makePluginApi } from "@/plugins/registry";

// BlockStatus 块渲染状态（加载中 / 已挂载 / 未注册占位）。
type BlockStatus = "loading" | "ready" | "missing";

// PluginBlock 内容块渲染组件（宿主正文嵌入协议的分发端）。
// 参数：type 块类型（data-plugin-block 取值）；props 块参数（data-props 解析结果）。
export function PluginBlock({
  type,
  props,
}: {
  type: string;
  props: Record<string, unknown>;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const { user } = useAuth();
  const [status, setStatus] = useState<BlockStatus>("loading");

  useEffect(() => {
    const el = containerRef.current;
    if (!el) {
      return;
    }
    let cancelled = false;
    let cleanup: (() => void) | undefined;

    const render = async (): Promise<void> => {
      const registry = await fetchBlockRegistry();
      const entry = registry.get(type);
      if (cancelled) {
        return;
      }
      if (!entry) {
        setStatus("missing"); // 提供方插件未启用/未安装：占位提示
        return;
      }
      try {
        const mod = await loadModule(entry.pluginId, entry.entry);
        if (cancelled) {
          return;
        }
        const pluginUser: PluginUser | null = user
          ? { id: user.id, name: user.nickname || user.username, role: user.role ?? "" }
          : null;
        cleanup =
          mod.default({
            slot: `block:${type}`,
            el,
            api: makePluginApi(entry.pluginId),
            user: pluginUser,
            props,
          }) ?? undefined;
        if (!cancelled) {
          setStatus("ready");
        }
      } catch {
        if (!cancelled) {
          setStatus("missing"); // 模块加载失败按未注册占位（不影响正文其余部分）
        }
      }
    };
    void render();

    return () => {
      cancelled = true;
      cleanup?.();
    };
    // props 为挂载时快照（与 PluginSlot 约定一致：回调键变化不触发重挂载）
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [type, user]);

  return (
    <div data-plugin-block={type}>
      <div ref={containerRef} />
      {status === "loading" && <div className="h-4" />}
      {status === "missing" && (
        <div className="my-2 rounded-md border border-dashed border-line px-3 py-2 text-xs text-muted">
          内容块「{type}」需要对应插件启用后展示
        </div>
      )}
    </div>
  );
}
