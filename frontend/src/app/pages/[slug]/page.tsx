// src/app/pages/[slug]/page.tsx
// 自定义页面（前台）：按 slug 渲染后台创建的独立页面（关于页/友链页等）。
// 仅已发布页面可见（草稿/不存在统一提示不存在）；
// html/markdown 正文复用 PostContent（DOMPurify 消毒）；
// page 格式（AI 构建器产物）为完整 HTML 文档，沙箱 iframe 整页渲染保留全部样式。
"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { PostContent } from "@/components/post-content";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { ApiError } from "@/lib/api";
import { apiPageBySlug } from "@/lib/api-pages";
import type { PagePublicDetail } from "@/lib/api-pages";
import { injectHeightReport } from "@/lib/page-html";

// PageFrameProps AI 整页沙箱渲染参数。
interface PageFrameProps {
  html: string; // 完整 HTML 文档
  title: string; // 页面标题（加载前占位显示）
}

// PageFrame page 格式渲染：iframe srcDoc + sandbox="allow-scripts"（隔离源，AI 代码
// 无法访问站点 cookie/localStorage/DOM）+ postMessage 高度自适应铺满文档流。
function PageFrame({ html, title }: PageFrameProps) {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const [height, setHeight] = useState<number>(600);

  // 监听沙箱内高度上报（校验消息来源为当前 iframe）
  useEffect(() => {
    const onMessage = (e: MessageEvent): void => {
      const data = e.data as { type?: string; height?: number } | null;
      if (
        data &&
        data.type === "yy-page-height" &&
        typeof data.height === "number" &&
        e.source === iframeRef.current?.contentWindow
      ) {
        setHeight(Math.min(Math.max(data.height, 320), 20000));
      }
    };
    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, []);

  return (
    <iframe
      ref={iframeRef}
      title={title}
      srcDoc={injectHeightReport(html)}
      sandbox="allow-scripts"
      className="w-full rounded-lg border border-line bg-elevated"
      style={{ height: `${height}px` }}
    />
  );
}

// CustomPageView 自定义页面视图。
export default function CustomPageView() {
  const params = useParams<{ slug: string }>();
  const slug = params.slug;

  const [page, setPage] = useState<PagePublicDetail | null>(null);
  const [error, setError] = useState<string>("");

  // 加载页面详情（竞态保护：切换 slug 时丢弃旧请求的迟到响应）
  useEffect(() => {
    let cancelled = false;
    // 记录进入前标题（离开时复原，避免 SEO 输出残留到其他页面）
    const previousTitle = document.title;
    apiPageBySlug(slug)
      .then((detail) => {
        if (cancelled) {
          return;
        }
        setPage(detail);
        // SEO 输出：自定义页面标题写入文档（与帖子详情页同模式）
        document.title = `${detail.title} · 月言`;
      })
      .catch((err) => {
        if (cancelled) {
          return;
        }
        // 草稿/不存在统一「页面不存在」，不泄露存在性
        setError(err instanceof ApiError && err.code === 2002 ? "页面不存在" : "页面加载失败");
      });
    return () => {
      cancelled = true;
      document.title = previousTitle;
    };
  }, [slug]);

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-[1080px] flex-1 px-4 py-6 pb-20 md:py-8">
        {/* 加载失败（草稿/不存在统一 404 语义） */}
        {error && (
          <div className="py-20 text-center">
            <p className="text-lg text-ink">{error}</p>
            <Link href="/" className="mt-4 inline-block text-sm text-glow hover:underline">
              ← 返回首页
            </Link>
          </div>
        )}

        {/* 加载骨架 */}
        {!page && !error && (
          <div className="space-y-4">
            <div className="h-14 animate-pulse rounded-lg bg-muted" aria-hidden />
            <div className="h-64 animate-pulse rounded-lg bg-muted" aria-hidden />
          </div>
        )}

        {page && page.content_format === "page" && (
          /* AI 构建页：完整 HTML 文档沙箱整页渲染（文档自带标题样式，不叠加站点文章壳） */
          <PageFrame html={page.content} title={page.title} />
        )}

        {page && page.content_format !== "page" && (
          <article className="rounded-lg border border-line bg-elevated p-6 md:p-10">
            {/* 页头：标题 + 更新时间 + 访问路径 */}
            <header className="border-b border-line pb-6">
              <h1 className="font-display text-2xl font-bold text-ink md:text-3xl">
                {page.title}
              </h1>
              <p className="mt-2 text-xs text-ink-3">
                更新于 {new Date(page.updated_at).toLocaleDateString("zh-CN")} · /pages/{page.slug}
              </p>
            </header>

            {/* 正文（html 走 DOMPurify 消毒渲染，markdown 走 react-markdown） */}
            <div className="pt-6">
              <PostContent content={page.content} format={page.content_format} />
            </div>
          </article>
        )}
      </main>
      <MobileTabbar />
    </div>
  );
}
