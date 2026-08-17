// src/lib/site-meta.ts
// 站点元信息读取（头部导航自定义）：模块级 Promise 缓存 + useSiteMeta hook。
// 说明：DesktopNav 被全站十余个页面各自挂载，若每页都请求 /meta 会重复拉取——
// 模块级单例缓存保证每个页面会话只请求一次；管理端保存导航后调用 invalidate 生效。
import { useEffect, useState } from "react";

import type { SiteMeta, SiteNavLink } from "@/types/api";

// DEFAULT_NAV 默认头部导航（未配置/加载失败时回退，与原硬编码「首页/话题」一致）。
export const DEFAULT_NAV: SiteNavLink[] = [
  { label: "首页", url: "/", new_tab: false },
  { label: "话题", url: "/topics", new_tab: false },
];

// DEFAULT_SITE_NAME 默认站点名（meta 未就绪时的品牌占位，与后端兜底一致）。
export const DEFAULT_SITE_NAME = "月言";

// metaPromise 模块级缓存单例（null = 尚未发起请求）。
let metaPromise: Promise<SiteMeta | null> | null = null;

// fetchSiteMeta 拉取站点元信息（公开接口裸 fetch，不带凭证；失败返回 null 不抛错——
// 头部导航必须永不阻塞页面渲染）。
export function fetchSiteMeta(): Promise<SiteMeta | null> {
  if (!metaPromise) {
    metaPromise = fetch("/api/v1/meta")
      .then((res) => (res.ok ? res.json() : null))
      .then((body) => (body && body.code === 0 ? (body.data as SiteMeta) : null))
      .catch(() => null);
  }
  return metaPromise;
}

// invalidateSiteMeta 清除缓存（管理端保存导航/站点名后调用，前台下次读取时重新拉取）。
export function invalidateSiteMeta(): void {
  metaPromise = null;
}

// SiteMetaState useSiteMeta 返回状态。
interface SiteMetaState {
  meta: SiteMeta | null; // 元信息（null = 未就绪/加载失败）
  nav: SiteNavLink[]; // 生效导航（已配置且非空用配置值，否则回退默认）
}

// useSiteMeta 站点元信息 hook（头部导航消费；加载完成前先渲染默认导航，避免闪烁跳动）。
export function useSiteMeta(): SiteMetaState {
  const [meta, setMeta] = useState<SiteMeta | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetchSiteMeta().then((m) => {
      if (!cancelled) {
        setMeta(m);
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const nav = meta?.nav && meta.nav.length > 0 ? meta.nav : DEFAULT_NAV;
  return { meta, nav };
}
