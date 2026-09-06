// src/components/world-cards.tsx
// 大世界（中继站聚合流）卡片：说说全文卡 / 文章摘要卡。
// 由首页时间线（filter=world）与 /world 独立页共用；数据为信封 ContentPayload 快照。
"use client";

import type { RelayCacheItem } from "@/lib/api-relay";

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

// SiteAvatar 来源站头像（无头像时取站名首字占位）。
function SiteAvatar({ name, avatar }: { name: string; avatar: string }) {
  if (avatar) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={avatar} alt="" className="h-8 w-8 rounded-full object-cover" />;
  }
  return (
    <span className="flex h-8 w-8 items-center justify-center rounded-full bg-accent-soft text-xs text-glow">
      {name.slice(0, 1)}
    </span>
  );
}

// WorldMomentCard 说说卡片：来源站 + 全文 + 图片九宫格。
export function WorldMomentCard({ item }: { item: RelayCacheItem }) {
  const p = item.payload;
  return (
    <article className="rounded-xl border border-line bg-card p-4">
      <header className="flex items-center gap-2 text-sm">
        <SiteAvatar name={p.site.name} avatar={p.site.avatar} />
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

// WorldArticleCard 文章卡片：标题 + 摘要 + 阅读按钮（桥接站跳中继站，公网站回原站）。
export function WorldArticleCard({ item }: { item: RelayCacheItem }) {
  const p = item.payload;
  const a = p.article;
  if (!a) {
    return <WorldMomentCard item={item} />;
  }
  const readURL = a.read_url || a.origin_url;
  const readLabel = a.read_url ? "在中继站阅读" : "阅读原文";
  return (
    <article className="rounded-xl border border-line bg-card p-4">
      <header className="flex items-center gap-2 text-sm">
        <SiteAvatar name={p.site.name} avatar={p.site.avatar} />
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

// WorldCard 按内容形态分发卡片。
export function WorldCard({ item }: { item: RelayCacheItem }) {
  if (item.payload.kind === "article") {
    return <WorldArticleCard item={item} />;
  }
  return <WorldMomentCard item={item} />;
}
