// src/components/admin/page-builder/page-builder.tsx
// AI 页面构建器主容器：顶部路由/标题/状态/保存 + 左侧实时预览 + 右侧 AI 对话。
// AI 生成的页面为完整 HTML 文档（content_format="page"，前台沙箱 iframe 整页渲染）。
"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";

import { AiChatPanel } from "@/components/admin/page-builder/ai-chat-panel";
import { CodeDrawer } from "@/components/admin/page-builder/code-drawer";
import { PreviewPane } from "@/components/admin/page-builder/preview-pane";
import { ApiError } from "@/lib/api";
import { apiAdminCreatePage, apiAdminUpdatePage } from "@/lib/api-pages";
import type { CustomPageDetail } from "@/lib/api-pages";

// PageBuilderProps 构建器参数。
interface PageBuilderProps {
  pageId: number | null; // 页面 ID（null = 新建）
  initial: CustomPageDetail | null; // 编辑回显（新建为 null；仅消费 page 格式内容）
}

// PageBuilder AI 页面构建器。
export function PageBuilder({ pageId, initial }: PageBuilderProps) {
  const router = useRouter();
  const [title, setTitle] = useState<string>(initial?.title ?? "");
  const [slug, setSlug] = useState<string>(initial?.slug ?? "");
  const [published, setPublished] = useState<boolean>(initial?.status === "published");
  const [html, setHtml] = useState<string>(initial?.content ?? "");
  const [generating, setGenerating] = useState<boolean>(false);
  const [showCode, setShowCode] = useState<boolean>(false);
  const [saving, setSaving] = useState<boolean>(false);
  const [saved, setSaved] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // AI 生成完成 → 应用到预览（构建器标记生成结束）
  const handleApply = (nextHtml: string): void => {
    setHtml(nextHtml);
    setGenerating(false);
    setSaved(false);
  };

  // 保存（新建返回 ID 后跳真实地址，地址栏可直接分享）
  const handleSave = async () => {
    setError("");
    setSaved(false);
    if (!title.trim()) {
      setError("页面标题不能为空");
      return;
    }
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug.trim())) {
      setError("路由标识需为小写字母/数字/连字符组合（如 about-me）");
      return;
    }
    if (!html.trim()) {
      setError("还没有页面内容，请先让 AI 生成或粘贴 HTML");
      return;
    }
    setSaving(true);
    try {
      const input = {
        slug: slug.trim(),
        title: title.trim(),
        content: html,
        content_format: "page" as const,
        description: "",
        status: published ? ("published" as const) : ("draft" as const),
      };
      if (pageId === null) {
        const result = await apiAdminCreatePage(input);
        router.replace(`/admin/pages/${result.id}/build`);
      } else {
        await apiAdminUpdatePage(pageId, input);
      }
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex h-[calc(100vh-7.5rem)] flex-col">
      {/* 顶部栏：返回 + 标题 + 路由（前缀固定 /pages/，slug 自定义）+ 状态 + 保存 */}
      <div className="mb-3 flex flex-wrap items-center gap-3">
        <Link href="/admin/pages" className="text-sm text-ink-3 hover:text-ink">
          ← 页面列表
        </Link>
        <input
          type="text"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          maxLength={200}
          placeholder="页面标题"
          className="h-10 w-52 rounded-lg border border-line bg-muted px-3 text-sm font-medium text-ink focus:border-accent focus:outline-none"
        />
        <div className="flex h-10 items-center overflow-hidden rounded-lg border border-line bg-muted text-sm">
          <span className="border-r border-line px-3 text-ink-3">/pages/</span>
          <input
            type="text"
            value={slug}
            onChange={(e) => setSlug(e.target.value.toLowerCase())}
            maxLength={100}
            placeholder="about-me"
            className="h-full w-40 bg-transparent px-3 text-ink focus:outline-none"
          />
        </div>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => setPublished(false)}
            className={`rounded-full border px-4 py-1.5 text-sm transition-colors ${
              !published ? "border-accent bg-accent-soft text-glow" : "border-line text-ink-2 hover:text-ink"
            }`}
          >
            草稿
          </button>
          <button
            type="button"
            onClick={() => setPublished(true)}
            className={`rounded-full border px-4 py-1.5 text-sm transition-colors ${
              published ? "border-accent bg-accent-soft text-glow" : "border-line text-ink-2 hover:text-ink"
            }`}
          >
            发布
          </button>
        </div>
        <button
          type="button"
          onClick={() => setShowCode((v) => !v)}
          className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 transition-colors hover:text-ink"
        >
          {showCode ? "隐藏代码" : "查看代码"}
        </button>
        {pageId !== null && published && initial?.slug && (
          <a
            href={`/pages/${slug || initial.slug}`}
            target="_blank"
            rel="noreferrer"
            className="text-sm text-glow hover:underline"
          >
            前台查看 ↗
          </a>
        )}
        {saved && <span className="text-sm text-glow">保存成功{published ? "，前台已可见" : "（草稿）"}</span>}
        {error && <span className="text-sm text-like" role="alert">{error}</span>}
        <button
          type="button"
          onClick={() => void handleSave()}
          disabled={saving}
          className="ml-auto rounded-full bg-accent px-8 py-2 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
        >
          {saving ? "保存中…" : "保存"}
        </button>
      </div>

      {/* 主体：左预览（含代码抽屉覆盖层）+ 右 AI 对话 */}
      <div className="flex min-h-0 flex-1 gap-4">
        <div className="relative flex min-h-0 min-w-0 flex-1 flex-col">
          <PreviewPane html={html} generating={generating} />
          <CodeDrawer
            open={showCode}
            html={html}
            onApply={(next) => {
              setHtml(next);
              setSaved(false);
            }}
            onClose={() => setShowCode(false)}
          />
        </div>
        <div className="min-h-0 w-[380px] shrink-0">
          <AiChatPanel
            currentHtml={html}
            onApply={handleApply}
            onBusyChange={setGenerating}
          />
        </div>
      </div>
    </div>
  );
}
