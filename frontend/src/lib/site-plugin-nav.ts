// src/lib/site-plugin-nav.ts
// 插件前台导航项读取（site.page 扩展）：running 插件经 frontend/manifest.json 的
// siteNav 声明注册前台头部导航项；desktop-nav 在管理员配置(nav_links)之后合并展示。
// DesktopNav 被全站页面各自挂载——模块级 30 秒 TTL 缓存避免重复请求（对齐 plugin-slot 模式）。
import { useEffect, useState } from "react";

import { apiPluginExtensions } from "@/lib/api";
import type { PluginExtensionItem } from "@/lib/api";
import type { SiteNavLink } from "@/types/api";

// 插件导航项缓存（null = 未加载；{items, at} = 30 秒内有效）。
let cache: { items: SiteNavLink[]; sources: PluginExtensionItem[]; at: number } | null = null;

// PLUGIN_NAV_TTL 缓存有效期（插件启停后导航最迟 30 秒自动跟随，与槽位缓存节奏一致）。
const PLUGIN_NAV_TTL = 30_000;

// fetchSitePluginNav 拉取插件导航项（带缓存；失败返回空——插件导航缺失不阻断头部渲染）。
export async function fetchSitePluginNav(): Promise<{
  items: SiteNavLink[];
  sources: PluginExtensionItem[];
}> {
  if (cache && Date.now() - cache.at < PLUGIN_NAV_TTL) {
    return { items: cache.items, sources: cache.sources };
  }
  try {
    const r = await apiPluginExtensions();
    // 汇总各插件声明的导航项（后端已做 label/path 白名单过滤；这里再兜底 path 安全校验）
    const items: SiteNavLink[] = [];
    for (const ext of r.items ?? []) {
      for (const nav of ext.site_nav ?? []) {
        if (nav.path.startsWith("/") && !nav.path.startsWith("//")) {
          items.push({ label: nav.label, url: nav.path, new_tab: false });
        }
      }
    }
    cache = {
      items,
      sources: (r.items ?? []).filter((e) => (e.site_nav ?? []).length > 0),
      at: Date.now(),
    };
  } catch {
    cache = { items: [], sources: [], at: Date.now() }; // 失败也缓存，避免每页头部连环重试
  }
  return { items: cache.items, sources: cache.sources };
}

// invalidateSitePluginNav 清除缓存（插件启停/卸载后调用可立即生效）。
export function invalidateSitePluginNav(): void {
  cache = null;
}

// useSitePluginNav 插件导航项 hook（前台头部与管理端只读展示共用）。
export function useSitePluginNav(): { items: SiteNavLink[]; sources: PluginExtensionItem[] } {
  const [state, setState] = useState<{ items: SiteNavLink[]; sources: PluginExtensionItem[] }>({
    items: [],
    sources: [],
  });

  useEffect(() => {
    let cancelled = false;
    fetchSitePluginNav().then((r) => {
      if (!cancelled) {
        setState(r);
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

  return state;
}
