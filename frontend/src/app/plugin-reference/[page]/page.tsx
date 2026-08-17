// src/app/plugin-reference/[page]/page.tsx
// 手册主题页：渲染 docs/plugin-reference/{slug}.md（slug 白名单校验，越界 404）。
// 尾部带上一篇/下一篇 pager（导航顺序推导，对齐 dsh reference 页脚）。
import Link from "next/link";
import { notFound } from "next/navigation";

import { Markdown } from "@/components/markdown";
import { docNeighbors, docHref, docTitle, readDocPage } from "@/lib/plugin-reference";

// 主题页参数（Next 15：params 为 Promise，需 await）。
interface PageProps {
  params: Promise<{ page: string }>;
}

// generateMetadata 页标题（导航数据派生）。
export async function generateMetadata({ params }: PageProps) {
  const { page } = await params;
  return { title: `${docTitle(page)} · 插件参考手册` };
}

// 主题页（服务端读取 + 白名单 404 + pager）。
export default async function PluginReferenceDocPage({ params }: PageProps) {
  const { page } = await params;
  const md = await readDocPage(page);
  if (md === null) {
    notFound();
  }
  const { prev, next } = docNeighbors(page);
  return (
    <>
      <article className="rounded-xl border border-line bg-elevated p-5 sm:p-8">
        <Markdown content={md} />
      </article>
      {/* 上一篇 / 下一篇（对齐 reference 站页脚 pager） */}
      <nav className="mt-4 flex items-stretch gap-3 text-sm" aria-label="相邻手册页">
        {prev ? (
          <Link
            href={docHref(prev.slug)}
            className="min-w-0 flex-1 rounded-xl border border-line bg-elevated px-4 py-3 transition-colors hover:border-accent-soft"
          >
            <span className="block text-xs text-ink-3">← 上一篇</span>
            <span className="block truncate font-medium text-ink-2">{prev.title}</span>
          </Link>
        ) : (
          <div className="flex-1" />
        )}
        {next ? (
          <Link
            href={docHref(next.slug)}
            className="min-w-0 flex-1 rounded-xl border border-line bg-elevated px-4 py-3 text-right transition-colors hover:border-accent-soft"
          >
            <span className="block text-xs text-ink-3">下一篇 →</span>
            <span className="block truncate font-medium text-ink-2">{next.title}</span>
          </Link>
        ) : (
          <div className="flex-1" />
        )}
      </nav>
    </>
  );
}
