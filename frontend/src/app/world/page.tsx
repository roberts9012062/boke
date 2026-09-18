// src/app/world/page.tsx
// 「大世界」独立页（移动端底部导航入口）：中继站分发的跨站聚合流。
// 桌面端主入口在首页时间线 Tab（「🌐 大世界」，filter=world，中栏原位渲染）；
// 本页复用同一套卡片组件（world-cards），支持分类/标签筛选，30s 轮询刷新。
"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { WorldCard } from "@/components/world-cards";
import { WorldFilters } from "@/components/world-filters";
import {
  apiRelayTaxonomy,
  apiWorldContents,
  apiWorldStatus,
  type RelayCacheItem,
  type RelayPublishOptions,
} from "@/lib/api-relay";

// pageLimit 每页条数（M0 一次取足；游标分页由首页 Tab 承载）。
const pageLimit = 30;

export default function WorldPage() {
  const [items, setItems] = useState<RelayCacheItem[]>([]);
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [updatedAt, setUpdatedAt] = useState<string>("");
  // 筛选状态（空 = 全部）；taxonomy 为筛选候选（分类 + 分类→规范标签）
  const [category, setCategory] = useState<string>("");
  const [tag, setTag] = useState<string>("");
  const [taxonomy, setTaxonomy] = useState<RelayPublishOptions | null>(null);

  // load 拉取聚合流（首屏与轮询共用；带分类/标签过滤）
  const load = useCallback(() => {
    apiWorldContents({ limit: pageLimit, category, tag })
      .then((d) => {
        setItems(d.items ?? []);
        setUpdatedAt(new Date().toLocaleTimeString("zh-CN"));
      })
      .catch(() => setItems([]));
  }, [category, tag]);

  useEffect(() => {
    apiWorldStatus()
      .then((d) => setEnabled(d.enabled))
      .catch(() => setEnabled(false));
    apiRelayTaxonomy()
      .then((d) => setTaxonomy(d))
      .catch(() => setTaxonomy(null)); // 规范加载失败仅隐藏筛选区，不影响内容展示
    load();
    const timer = setInterval(load, 30000); // M0 轮询刷新（30s）
    return () => clearInterval(timer);
  }, [load]);

  // 切换分类：清空标签（标签筛选归属分类）
  const switchCategory = (next: string) => {
    setCategory(next);
    setTag("");
  };

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-2xl flex-1 px-4 py-6">
        <header className="mb-4 flex items-center justify-between">
          <h1 className="font-display text-xl font-semibold text-ink">🌐 大世界</h1>
          <Link href="/" className="text-sm text-ink-2 transition-colors hover:text-ink">
            ← 返回时间线
          </Link>
        </header>

        {/* 分类/标签筛选（规范加载成功且启用大世界时显示） */}
        {enabled !== false && taxonomy && (
          <div className="mb-4">
            <WorldFilters
              categories={taxonomy.categories}
              taxonomy={taxonomy.taxonomy}
              category={category}
              tag={tag}
              onCategoryChange={switchCategory}
              onTagChange={setTag}
            />
          </div>
        )}

        {enabled === false && (
          <p className="rounded-xl border border-line bg-card p-6 text-center text-sm text-ink-2">
            站长尚未开启「大世界」，请先在后台完成中继站对接。
          </p>
        )}
        {enabled !== false && items.length === 0 && (
          <p className="rounded-xl border border-line bg-card p-6 text-center text-sm text-ink-2">
            {category !== "" || tag !== "" ? "该筛选条件下暂无内容。" : "大世界还很安静——第一个发说说的人就是你。"}
            {updatedAt && <span className="mt-1 block text-xs text-ink-3">最近刷新 {updatedAt} · 每 30 秒自动更新</span>}
          </p>
        )}
        <div className="space-y-4">
          {items.map((item) => (
            <WorldCard key={item.content_id} item={item} />
          ))}
        </div>
        {items.length > 0 && (
          <p className="mt-4 text-center text-xs text-ink-3">
            {items.length} 条 · 最近刷新 {updatedAt} · 每 30 秒自动更新
          </p>
        )}
      </main>
      <MobileTabbar />
    </div>
  );
}
