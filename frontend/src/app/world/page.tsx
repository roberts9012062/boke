// src/app/world/page.tsx
// 「大世界」聚合流页（B-4'）：中继站分发给本站的跨站内容（说说全文卡 / 文章摘要卡）。
// M0 轮询刷新（30s）；卡片标注来源站；文章按钮按 read_url / origin_url 渲染。
"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { apiWorldContents, apiWorldStatus, type RelayCacheItem } from "@/lib/api-relay";

// fmtTime 相对时间展示。
function fmtTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  return new Date(iso).toLocaleDateString("zh-CN");
}

// MomentCard 说说卡片：来源站 + 全文 + 图片。
function MomentCard({ item }: { item: RelayCacheItem }) {
  const p = item.payload;
  return (
    <article className="rounded-xl border border-line bg-card p-4">
      <header className="flex items-center gap-2 text-sm">
        {p.site.avatar ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={p.site.avatar} alt="" className="h-8 w-8 rounded-full object-cover" />
        ) : (
          <span className="flex h-8 w-8 items-center justify-center rounded-full bg-accent-soft text-xs text-glow">
            {p.site.name.slice(0, 1)}
          </span>
        )}
        <span className="font-medium text-ink">{p.site.name}</span>
        <span className="text-xs text-ink-3">· {fmtTime(item.published_at)}</span>
        {p.site.mode === "bridged" && (
          <span className="ml-auto rounded-full bg-muted px-2 py-0.5 text-[10px] text-ink-3">桥接</span>
        )}
      </header>
      <p className="mt-3 whitespace-pre-wrap break-words text-[15px] leading-relaxed text-ink">{p.moment?.text}</p>
      {p.moment && p.moment.images.length > 0 && (
        <div className="mt-3 grid grid-cols-3 gap-1.5">
          {p.moment.images.slice(0, 9).map((src) => (
            // eslint-disable-next-line @next/next/no-img-element
            <img key={src} src={src} alt="" loading="lazy" className="aspect-square w-full rounded-lg object-cover" />
          ))}
        </div>
      )}
      <footer className="mt-3 flex items-center gap-2 text-xs text-ink-3">
        <span className="rounded-full bg-muted px-2 py-0.5">{p.category}</span>
        {p.tags.map((t) => (
          <span key={t}>#{t}</span>
        ))}
      </footer>
    </article>
  );
}

// ArticleCard 文章卡片：标题 + 摘要 + 阅读按钮（桥接站跳中继站，公网站回原站）。
function ArticleCard({ item }: { item: RelayCacheItem }) {
  const p = item.payload;
  const a = p.article;
  if (!a) return null;
  const readURL = a.read_url || a.origin_url;
  const readLabel = a.read_url ? "在中继站阅读" : "阅读原文";
  return (
    <article className="rounded-xl border border-line bg-card p-4">
      <header className="flex items-center gap-2 text-sm">
        <span className="flex h-8 w-8 items-center justify-center rounded-full bg-accent-soft text-xs text-glow">
          {p.site.name.slice(0, 1)}
        </span>
        <span className="font-medium text-ink">{p.site.name}</span>
        <span className="text-xs text-ink-3">· {fmtTime(item.published_at)} · 文章</span>
      </header>
      <h2 className="mt-3 text-base font-semibold text-ink">{a.title}</h2>
      {a.summary && <p className="mt-1 line-clamp-2 text-sm text-ink-2">{a.summary}</p>}
      <div className="mt-3 flex items-center justify-between">
        <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-ink-3">{p.category}</span>
        <a
          href={readURL}
          target="_blank"
          rel="noopener noreferrer"
          className="rounded-full bg-glow px-4 py-1.5 text-xs font-medium text-white transition-opacity hover:opacity-90"
        >
          {readLabel}
        </a>
      </div>
    </article>
  );
}

export default function WorldPage() {
  const [items, setItems] = useState<RelayCacheItem[]>([]);
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [updatedAt, setUpdatedAt] = useState<string>("");

  // load 拉取聚合流（首屏与轮询共用）
  const load = useCallback(() => {
    apiWorldContents({ limit: 30 })
      .then((d) => {
        setItems(d.items ?? []);
        setUpdatedAt(new Date().toLocaleTimeString("zh-CN"));
      })
      .catch(() => setItems([]));
  }, []);

  useEffect(() => {
    apiWorldStatus()
      .then((d) => setEnabled(d.enabled))
      .catch(() => setEnabled(false));
    load();
    const timer = setInterval(load, 30000); // M0 轮询刷新（30s）
    return () => clearInterval(timer);
  }, [load]);

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

        {enabled === false && (
          <p className="rounded-xl border border-line bg-card p-6 text-center text-sm text-ink-2">
            站长尚未开启「大世界」，请先在后台完成中继站对接。
          </p>
        )}
        {enabled !== false && items.length === 0 && (
          <p className="rounded-xl border border-line bg-card p-6 text-center text-sm text-ink-2">
            大世界还很安静——第一个发说说的人就是你。
            {updatedAt && <span className="mt-1 block text-xs text-ink-3">最近刷新 {updatedAt} · 每 30 秒自动更新</span>}
          </p>
        )}
        <div className="space-y-4">
          {items.map((item) =>
            item.payload.kind === "article" ? (
              <ArticleCard key={item.content_id} item={item} />
            ) : (
              <MomentCard key={item.content_id} item={item} />
            ),
          )}
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
