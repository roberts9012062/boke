// src/components/admin/post-edit-panel.tsx
// 后台编辑帖子 · 右栏信息面板（设计稿《后台编辑》四画板右侧）：
// 发布信息（类型/状态/可见性/创建/更新）+ 互动数据（赞/评/览）+ 操作（下架/删除）+ SEO 占位。
"use client";

import { useEffect, useState } from "react";

import { apiAdminDeletePost, apiAdminSetPostStatus, apiSaveSeoMeta, apiSeoMeta, ApiError } from "@/lib/api";
import { formatCompact, formatDateTime } from "@/lib/utils";
import type { AdminPostDetail } from "@/types/api";

// 类型/状态/可见性文案映射（与后台列表页一致）
const TYPE_LABEL: Record<string, string> = { text: "文字", image: "图片", audio: "音频", video: "视频" };
const STATUS_LABEL: Record<string, string> = {
  published: "已发布",
  draft: "草稿",
  taken_down: "已下架",
};
const VISIBILITY_LABEL: Record<string, string> = {
  public: "公开",
  followers: "仅关注者",
  private: "仅自己",
};

// PostEditPanel 右栏信息面板。
// 参数：detail 帖子详情；onChanged 操作完成回调（刷新页面数据）。
export function PostEditPanel({
  detail,
  onChanged,
}: {
  detail: AdminPostDetail;
  onChanged: () => void;
}) {
  // SEO 面板状态（M4 激活：设计稿《后台编辑·文字·SEO》：SEO 标题/SEO 描述/本帖 SEO/SEO 选项）
  const [seoTitle, setSeoTitle] = useState<string>("");
  const [seoDesc, setSeoDesc] = useState<string>("");
  const [seoKeywords, setSeoKeywords] = useState<string>("");
  const [seoOgImage, setSeoOgImage] = useState<string>("");
  const [seoLoaded, setSeoLoaded] = useState<boolean>(false);
  const [seoSaved, setSeoSaved] = useState<boolean>(false);
  const [seoError, setSeoError] = useState<string>("");

  // 加载帖子 SEO 元数据（设计稿：SEO 标题/描述/本帖 SEO）
  useEffect(() => {
    apiSeoMeta(detail.id)
      .then((meta) => {
        setSeoTitle(meta.title);
        setSeoDesc(meta.description);
        setSeoKeywords(meta.keywords);
        setSeoOgImage(meta.og_image);
      })
      .catch(() => undefined)
      .finally(() => setSeoLoaded(true));
  }, [detail.id]);

  // 保存 SEO 元数据
  const saveSeo = async () => {
    setSeoError("");
    setSeoSaved(false);
    try {
      await apiSaveSeoMeta(detail.id, {
        title: seoTitle,
        description: seoDesc,
        keywords: seoKeywords,
        og_image: seoOgImage,
      });
      setSeoSaved(true);
    } catch (err) {
      setSeoError(err instanceof ApiError ? err.message : "SEO 保存失败");
    }
  };
  // 下架/上架切换（复用内容管理上下架接口）
  const toggleStatus = async () => {
    const next = detail.status === "published" ? "taken_down" : "published";
    await apiAdminSetPostStatus(detail.id, next);
    onChanged();
  };

  // 删除帖子（二次确认）
  const handleDelete = async () => {
    if (!window.confirm("确定删除该帖子？删除后不可恢复")) {
      return;
    }
    await apiAdminDeletePost(detail.id);
    // 删除后跳回内容管理列表
    window.location.href = "/admin/posts";
  };

  return (
    <div className="space-y-5">
      {/* 发布信息（设计稿：类型/状态/可见性/创建/更新） */}
      <section className="rounded-lg border border-line bg-elevated p-5">
        <h2 className="text-sm font-semibold text-ink">发布信息</h2>
        <dl className="mt-3 space-y-2 text-xs">
          <div className="flex justify-between">
            <dt className="text-ink-3">类型</dt>
            <dd className="text-ink">{TYPE_LABEL[detail.content_type] ?? detail.content_type}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-ink-3">状态</dt>
            <dd className="text-ink">{STATUS_LABEL[detail.status] ?? detail.status}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-ink-3">可见性</dt>
            <dd className="text-ink">{VISIBILITY_LABEL[detail.visibility] ?? detail.visibility}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-ink-3">创建</dt>
            <dd className="text-ink">{formatDateTime(detail.created_at)}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-ink-3">更新</dt>
            <dd className="text-ink">{formatDateTime(detail.updated_at)}</dd>
          </div>
        </dl>
      </section>

      {/* 互动数据（设计稿：128 赞 / 24 评 / 1.2k 览，千位缩写） */}
      <section className="rounded-lg border border-line bg-elevated p-5">
        <h2 className="text-sm font-semibold text-ink">互动数据</h2>
        <div className="mt-3 flex gap-6 text-center">
          <div className="flex-1">
            <p className="text-lg font-semibold text-ink">{formatCompact(detail.like_count)}</p>
            <p className="text-xs text-ink-3">赞</p>
          </div>
          <div className="flex-1">
            <p className="text-lg font-semibold text-ink">{formatCompact(detail.comment_count)}</p>
            <p className="text-xs text-ink-3">评</p>
          </div>
          <div className="flex-1">
            <p className="text-lg font-semibold text-ink">{formatCompact(detail.view_count)}</p>
            <p className="text-xs text-ink-3">览</p>
          </div>
        </div>
      </section>

      {/* 操作（设计稿：下架 / 删除帖子） */}
      <section className="rounded-lg border border-line bg-elevated p-5">
        <h2 className="text-sm font-semibold text-ink">操作</h2>
        <div className="mt-3 flex flex-col gap-2">
          <button
            type="button"
            onClick={() => void toggleStatus()}
            className="rounded-full border border-line px-4 py-2 text-sm text-ink-2 hover:text-ink"
          >
            {detail.status === "published" ? "下架" : "上架"}
          </button>
          <button
            type="button"
            onClick={() => void handleDelete()}
            className="rounded-full border border-like/30 px-4 py-2 text-sm text-like hover:bg-like/10"
          >
            删除帖子
          </button>
        </div>
      </section>

      {/* SEO 面板（M4 激活：设计稿《后台编辑·文字·SEO》） */}
      <section className="rounded-lg border border-line bg-elevated p-5">
        <h2 className="text-sm font-semibold text-ink">SEO 标题 / SEO 描述</h2>
        <div className="mt-3 space-y-3">
          <div>
            <label htmlFor="seo-title" className="mb-1 block text-xs text-ink-3">
              SEO 标题
            </label>
            <input
              id="seo-title"
              type="text"
              value={seoTitle}
              onChange={(e) => setSeoTitle(e.target.value)}
              maxLength={300}
              placeholder={`${detail.title || "帖子标题"} · 月言`}
              className="h-9 w-full rounded-lg border border-line bg-muted px-3 text-xs text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
            <p className="mt-0.5 text-right text-[10px] text-ink-3">{seoTitle.length} 字</p>
          </div>
          <div>
            <label htmlFor="seo-desc" className="mb-1 block text-xs text-ink-3">
              SEO 描述
            </label>
            <textarea
              id="seo-desc"
              value={seoDesc}
              onChange={(e) => setSeoDesc(e.target.value)}
              rows={2}
              maxLength={500}
              className="w-full rounded-lg border border-line bg-muted px-3 py-2 text-xs text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </div>
          <div>
            <label htmlFor="seo-keywords" className="mb-1 block text-xs text-ink-3">
              关键词（逗号分隔）
            </label>
            <input
              id="seo-keywords"
              type="text"
              value={seoKeywords}
              onChange={(e) => setSeoKeywords(e.target.value)}
              className="h-9 w-full rounded-lg border border-line bg-muted px-3 text-xs text-ink focus:border-accent focus:outline-none"
            />
          </div>
          <div>
            <label htmlFor="seo-og" className="mb-1 block text-xs text-ink-3">
              自定义 OG 图（URL）
            </label>
            <input
              id="seo-og"
              type="text"
              value={seoOgImage}
              onChange={(e) => setSeoOgImage(e.target.value)}
              placeholder="/media/202608/xxx.jpg"
              className="h-9 w-full rounded-lg border border-line bg-muted px-3 text-xs text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </div>
          {seoError && (
            <p className="rounded-md bg-like/10 px-3 py-2 text-xs text-like" role="alert">
              {seoError}
            </p>
          )}
          {seoSaved && (
            <p className="rounded-md bg-accent-soft px-3 py-2 text-xs text-glow" role="status">
              SEO 已保存
            </p>
          )}
          <button
            type="button"
            onClick={() => void saveSeo()}
            disabled={!seoLoaded}
            className="rounded-full bg-accent px-5 py-1.5 text-xs font-medium text-on-accent hover:opacity-90 disabled:opacity-60"
          >
            保存 SEO
          </button>
        </div>
      </section>
    </div>
  );
}
