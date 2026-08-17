// src/components/admin/page-edit-form.tsx
// 自定义页面编辑表单：标题/路由标识/状态/SEO 描述 + 富文本正文（复用发帖编辑器）。
// 新建（pageId=null）保存后跳到真实编辑地址；编辑场景原地更新。
"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

import { RichTextEditor } from "@/components/compose/rich-text-editor";
import { ApiError } from "@/lib/api";
import { apiAdminCreatePage, apiAdminUpdatePage } from "@/lib/api-pages";
import type { CustomPageDetail } from "@/lib/api-pages";

// PageEditFormProps 编辑表单参数。
interface PageEditFormProps {
  pageId: number | null; // 页面 ID（null = 新建）
  initial: CustomPageDetail | null; // 编辑回显（新建为 null）
}

// 正文长度上限（与后端 maxPageContentByte 对齐：200KB）
const MAX_CONTENT_LENGTH = 200 * 1024;

// PageEditForm 自定义页面编辑表单。
export function PageEditForm({ pageId, initial }: PageEditFormProps) {
  const router = useRouter();
  const [title, setTitle] = useState<string>(initial?.title ?? "");
  const [slug, setSlug] = useState<string>(initial?.slug ?? "");
  const [description, setDescription] = useState<string>(initial?.description ?? "");
  const [published, setPublished] = useState<boolean>(initial?.status === "published");
  const [content, setContent] = useState<string>(initial?.content ?? "");
  const [saving, setSaving] = useState<boolean>(false);
  const [saved, setSaved] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // 保存（新建返回 ID 后跳转真实编辑地址，刷新后地址栏可直接分享）
  const handleSave = async () => {
    setError("");
    setSaved(false);
    // 前端轻校验（后端仍会完整校验；此处为即时反馈）
    if (!title.trim()) {
      setError("页面标题不能为空");
      return;
    }
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug.trim())) {
      setError("路由标识需为小写字母/数字/连字符组合（如 about-me）");
      return;
    }
    setSaving(true);
    try {
      const input = {
        slug: slug.trim(),
        title: title.trim(),
        content,
        content_format: "html" as const,
        description: description.trim(),
        status: published ? ("published" as const) : ("draft" as const),
      };
      if (pageId === null) {
        const result = await apiAdminCreatePage(input);
        router.replace(`/admin/pages/${result.id}/edit`);
        setSaved(true);
      } else {
        await apiAdminUpdatePage(pageId, input);
        setSaved(true);
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="max-w-[920px]">
      {/* 页头：返回列表 + 标题 */}
      <div className="mb-5 flex items-center gap-3">
        <a href="/admin/pages" className="text-sm text-ink-3 hover:text-ink">
          ← 页面列表
        </a>
        <h1 className="font-display text-xl font-semibold text-ink">
          {pageId === null ? "新建页面" : "编辑页面"}
        </h1>
        {/* AI 构建器入口（对话式生成整页，与富文本编辑互为补充） */}
        <a
          href={pageId === null ? "/admin/pages/new/build" : `/admin/pages/${pageId}/build`}
          className="text-sm text-glow hover:underline"
        >
          用 AI 构建 →
        </a>
        {/* 已发布页面提供前台预览入口 */}
        {pageId !== null && initial && initial.status === "published" && (
          <a
            href={`/pages/${initial.slug}`}
            target="_blank"
            rel="noreferrer"
            className="text-sm text-glow hover:underline"
          >
            查看前台 ↗
          </a>
        )}
      </div>

      <div className="space-y-5 rounded-lg border border-line bg-elevated p-6">
        {/* 标题 */}
        <div>
          <label htmlFor="page-title" className="mb-1.5 block text-sm text-ink-2">
            页面标题
          </label>
          <input
            id="page-title"
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            maxLength={200}
            placeholder="如：关于我、友情链接"
            className="h-11 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink focus:border-accent focus:outline-none"
          />
        </div>

        {/* 路由标识 + 状态 */}
        <div className="grid gap-5 md:grid-cols-2">
          <div>
            <label htmlFor="page-slug" className="mb-1.5 block text-sm text-ink-2">
              路由标识（slug）
            </label>
            <input
              id="page-slug"
              type="text"
              value={slug}
              onChange={(e) => setSlug(e.target.value.toLowerCase())}
              maxLength={100}
              placeholder="about-me"
              className="h-11 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink focus:border-accent focus:outline-none"
            />
            <p className="mt-1 text-xs text-ink-3">前台访问：/pages/{slug || "…"}</p>
          </div>
          <div>
            <p className="mb-1.5 text-sm text-ink-2">发布状态</p>
            <div className="flex gap-3 pt-2">
              <button
                type="button"
                onClick={() => setPublished(false)}
                className={`rounded-full border px-5 py-2 text-sm transition-colors ${
                  !published ? "border-accent bg-accent-soft text-glow" : "border-line text-ink-2 hover:text-ink"
                }`}
              >
                草稿
              </button>
              <button
                type="button"
                onClick={() => setPublished(true)}
                className={`rounded-full border px-5 py-2 text-sm transition-colors ${
                  published ? "border-accent bg-accent-soft text-glow" : "border-line text-ink-2 hover:text-ink"
                }`}
              >
                发布
              </button>
              <p className="self-center text-xs text-ink-3">仅「发布」状态前台可见</p>
            </div>
          </div>
        </div>

        {/* SEO 描述 */}
        <div>
          <label htmlFor="page-desc" className="mb-1.5 block text-sm text-ink-2">
            SEO 描述（可选）
          </label>
          <textarea
            id="page-desc"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={2}
            maxLength={500}
            placeholder="搜索引擎结果页展示的页面简介"
            className="w-full rounded-lg border border-line bg-muted px-4 py-3 text-sm text-ink focus:border-accent focus:outline-none"
          />
        </div>

        {/* 正文（复用发帖富文本编辑器：图片/视频/音乐内嵌同帖能力） */}
        <div>
          <p className="mb-1.5 text-sm text-ink-2">页面内容</p>
          <RichTextEditor
            value={content}
            onChange={setContent}
            placeholder="写下这一页的内容…"
            maxLength={MAX_CONTENT_LENGTH}
          />
        </div>

        {error && (
          <p className="rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
            {error}
          </p>
        )}
        {saved && (
          <p className="rounded-md bg-accent-soft px-3 py-2 text-sm text-glow" role="status">
            保存成功{published ? "，前台已可见" : "（草稿状态前台不可见）"}
          </p>
        )}

        {/* 操作区 */}
        <div className="flex justify-end gap-2">
          <a
            href="/admin/pages"
            className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 transition-colors hover:text-ink"
          >
            返回列表
          </a>
          <button
            type="button"
            onClick={() => void handleSave()}
            disabled={saving}
            className="rounded-full bg-accent px-8 py-2 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
          >
            {saving ? "保存中…" : published ? "保存并发布" : "保存草稿"}
          </button>
        </div>
      </div>
    </div>
  );
}
