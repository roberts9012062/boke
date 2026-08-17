// frontend/src/app/plugins/[pluginId]/[page]/page.tsx
// 插件前台公开页面壳（site.page 能力）：
//   插件 frontend/manifest.json 声明 pages: [{route, entry, scope: "site"}] →
//   壳路由 /plugins/{pluginId}/{route}（前台布局，访客可访问）→
//   动态加载插件页面（ESM registerPage 契约 / sandbox iframe 强隔离，复用 admin 壳机制）。
// 直链防护：经公开接口 plugin-extensions 校验插件 running 且已声明该 site 页面路由。
"use client";

import { useParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { apiPluginExtensions } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { fetchManifest, loadModule, type PluginUser } from "@/plugins/loader";
import { makePluginApi, registry, type PluginApiClient } from "@/plugins/registry";
import { mountSandbox } from "@/plugins/sandbox";

// PluginPageCtx 插件页面注册上下文（registerPage 入参；与 admin 壳契约一致）。
export interface PluginPageCtx {
  container: HTMLElement; // 页面容器（插件渲染目标）
  api: PluginApiClient; // 受限 API 客户端（访客调用需登录的插件接口会 401，公开数据走宿主公开端点）
  user: PluginUser | null; // 脱敏用户信息（访客为 null）
  params: { pluginId: string; page: string }; // 路由参数
}

// PluginPageModule 页面模块（默认导出 registerPage）。
interface PluginPageModule {
  default: (ctx: PluginPageCtx) => (() => void) | void;
}

// SitePluginPage 插件前台公开页面壳。
export default function SitePluginPage() {
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
        // 直链防护（公开接口仅返回 running 插件）：找不到即未启用/不存在
        const ext = await apiPluginExtensions();
        const inst = (ext.items ?? []).find((p) => p.plugin_id === pluginId);
        if (!inst) {
          setError("插件未启用或不存在");
          return;
        }
        if (!(inst.site_pages ?? []).includes(page)) {
          setError(`插件未声明前台页面 ${page}（需在 frontend/manifest.json 声明 scope: "site"）`);
          return;
        }
        setPluginName(inst.name);
        // 查找页面声明并按 sandbox 标志分发（manifest 二次校验 scope，双保险）
        const manifest = await fetchManifest(pluginId);
        const pageDecl = manifest.pages?.find((p) => p.route === page && p.scope === "site");
        if (!pageDecl) {
          setError(`插件「${inst.name}」未声明前台页面 ${page}`);
          return;
        }
        if (pageDecl.sandbox) {
          // 沙箱模式：entry 为 HTML 页面，经 iframe 强隔离加载（访客无沙箱令牌，公开数据走宿主端点）
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
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-[960px] flex-1 px-4 py-6 pb-20">
        {/* 移动端顶部品牌行(与其他前台页一致) */}
        <div className="mb-4 md:hidden">
          <span className="font-display text-lg font-bold text-ink">{pluginName || "插件页面"}</span>
        </div>
        <h1 className="hidden font-display text-xl font-semibold text-ink md:block">
          {pluginName || "插件页面"} · {page}
        </h1>
        {error && (
          <p className="mt-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
            {error}
          </p>
        )}
        <div ref={containerRef} className="mt-4" />
      </main>
      <MobileTabbar />
    </div>
  );
}
