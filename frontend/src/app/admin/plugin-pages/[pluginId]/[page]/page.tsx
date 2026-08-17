// frontend/src/app/admin/plugin-pages/[pluginId]/[page]/page.tsx
// 插件独立页面壳（M3.9 admin.page 能力）：
//   插件 frontend/manifest.json 声明 pages: [{route, entry}] →
//   壳路由 /admin/plugin-pages/{pluginId}/{route}（admin layout 权限守卫下）→
//   动态加载插件页面模块（registerPage(ctx) 契约）渲染到容器。
//   页面资产经 /plugin-assets（静态白名单），无需后端新路由。
"use client";

import { useParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { apiInstalledPlugins } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { fetchManifest, loadModule, type PluginUser } from "@/plugins/loader";
import { makePluginApi, registry, type PluginApiClient } from "@/plugins/registry";
import { mountSandbox } from "@/plugins/sandbox";

// PluginPageCtx 插件页面注册上下文（registerPage 入参）。
export interface PluginPageCtx {
  container: HTMLElement; // 页面容器（插件渲染目标）
  api: PluginApiClient; // 受限 API 客户端（registry 共享实现，E2 去重）
  user: PluginUser | null; // 脱敏用户信息
  params: { pluginId: string; page: string }; // 路由参数
}

// PluginPageModule 页面模块（默认导出 registerPage）。
interface PluginPageModule {
  default: (ctx: PluginPageCtx) => (() => void) | void;
}

// PluginPage 插件独立页面壳。
export default function PluginPage() {
  const params = useParams<{ pluginId: string; page: string }>();
  const pluginId = params.pluginId;
  const page = params.page;
  const containerRef = useRef<HTMLDivElement>(null);
  const { user } = useAuth();
  const [error, setError] = useState<string>("");
  const [pluginName, setPluginName] = useState<string>("");

  useEffect(() => {
    const container = containerRef.current;
    if (!container) {
      return;
    }
    let cancelled = false;
    let cleanup: (() => void) | undefined;

    const pluginUser: PluginUser | null = user
      ? { id: user.id, name: user.nickname || user.username, role: user.role ?? "" }
      : null;

    (async () => {
      try {
        // 校验插件已安装且 running（admin layout 已守卫权限，此处防直链未安装插件）
        const installed = await apiInstalledPlugins();
        const inst = installed.items.find((p) => p.plugin_id === pluginId);
        if (!inst || inst.state !== "running") {
          setError("插件未启用或不存在");
          return;
        }
        setPluginName(inst.name);
        // 查找页面声明并按 sandbox 标志分发（E1：沙箱模式 iframe 强隔离，缺省同源 ESM）
        const manifest = await fetchManifest(pluginId);
        const pageDecl = manifest.pages?.find((p) => p.route === page);
        if (!pageDecl) {
          setError(`插件「${inst.name}」未声明页面 ${page}`);
          return;
        }
        if (pageDecl.sandbox) {
          // 沙箱模式：entry 为 HTML 页面，经 iframe 加载（无同源权限；通信走 postMessage）
          if (cancelled) {
            return;
          }
          cleanup = mountSandbox(pluginId, container, pageDecl.entry, pluginUser);
          return;
        }
        const mod = (await loadModule(pluginId, pageDecl.entry)) as unknown as PluginPageModule;
        if (cancelled) {
          return;
        }
        // registerPage 可返回清理函数或空（void）；仅函数赋值给 cleanup（类型收窄）
        const result = mod.default({
          container,
          api: makePluginApi(pluginId),
          user: pluginUser,
          params: { pluginId, page },
        });
        if (typeof result === "function") {
          cleanup = result;
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "插件页面加载失败");
        }
      }
    })();

    return () => {
      cancelled = true;
      cleanup?.();
      registry.unmountPlugin(pluginId); // 清理同插件槽位扩展（页面切换时）
    };
  }, [pluginId, page, user]);

  return (
    <div className="mx-auto max-w-[720px]">
      <h1 className="font-display text-xl font-semibold text-ink">
        {pluginName || "插件页面"} · {page}
      </h1>
      {error && (
        <p className="mt-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
          {error}
        </p>
      )}
      <div ref={containerRef} className="mt-4" />
    </div>
  );
}
