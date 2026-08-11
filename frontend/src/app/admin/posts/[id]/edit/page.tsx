// src/app/admin/posts/[id]/edit/page.tsx
// 后台编辑帖子（设计稿 D/冷月/后台编辑·文字/图片/音频/视频 四画板，1400×1000）：
// 面包屑（内容管理 / 编辑 · 文字帖）+ 标题行（状态徽标 + 预览/保存草稿/更新发布）
// + 左栏编辑区（PostEditForm 四类型表单）+ 右栏信息面板（发布信息/互动数据/操作/SEO 占位）。
"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { PostEditForm, type PostEditFormHandle } from "@/components/admin/post-edit-form";
import { PostEditPanel } from "@/components/admin/post-edit-panel";
import { apiAdminPostDetail } from "@/lib/api";
import type { AdminPostDetail } from "@/types/api";

// 状态徽标文案与样式
const STATUS_LABEL: Record<string, string> = {
  published: "已发布",
  draft: "草稿",
  taken_down: "已下架",
};

// AdminPostEdit 后台编辑帖子页。
export default function AdminPostEdit() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const postId = Number(params.id);
  const formRef = useRef<PostEditFormHandle>(null);

  const [detail, setDetail] = useState<AdminPostDetail | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [loadError, setLoadError] = useState<string>("");
  const [saving, setSaving] = useState<boolean>(false);
  const [savedTip, setSavedTip] = useState<string>("");

  // 加载后台编辑详情
  useEffect(() => {
    if (!Number.isFinite(postId) || postId <= 0) {
      setLoadError("帖子不存在");
      setLoading(false);
      return;
    }
    apiAdminPostDetail(postId)
      .then(setDetail)
      .catch((err) => setLoadError(err instanceof Error ? err.message : "加载失败"))
      .finally(() => setLoading(false));
  }, [postId]);

  // 保存（草稿/发布；成功后刷新详情）
  const handleSave = async (status: "draft" | "published") => {
    if (!formRef.current || saving) {
      return;
    }
    setSaving(true);
    setSavedTip("");
    const ok = await formRef.current.save(status);
    setSaving(false);
    if (ok) {
      setSavedTip(status === "draft" ? "已保存草稿" : "已更新发布");
      apiAdminPostDetail(postId).then(setDetail).catch(() => undefined);
    }
  };

  // 加载失败 / 不存在
  if (!loading && (loadError || !detail)) {
    return (
      <div className="py-16 text-center">
        <p className="text-ink">{loadError || "帖子不存在"}</p>
        <Link href="/admin/posts" className="mt-4 inline-block text-sm text-glow hover:underline">
          返回内容管理
        </Link>
      </div>
    );
  }

  // 加载中
  if (!detail) {
    return <p className="py-16 text-center text-sm text-ink-3">加载中…</p>;
  }

  // 面包屑与标题的类型文案（设计稿：面包屑「编辑 · 文字」无「帖」，标题「编辑 · 文字帖」）
  const typeLabel = detail.content_type === "text" ? "文字" : detail.content_type === "image" ? "图片" : detail.content_type === "audio" ? "音频" : "视频";

  return (
    <div>
      {/* 面包屑（设计稿：内容管理 / 编辑 · 文字） */}
      <div className="flex items-center gap-2 text-xs text-ink-3">
        <Link href="/admin/posts" className="hover:text-ink">
          内容管理
        </Link>
        <span>/</span>
        <span className="text-ink-2">编辑 · {typeLabel}</span>
      </div>

      {/* 标题行：标题 + 状态徽标 + 预览/保存草稿/更新发布（设计稿按钮组） */}
      <div className="mt-2 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <h1 className="font-display text-xl font-semibold text-ink">编辑 · {typeLabel}帖</h1>
          <span
            className={`rounded-full px-2.5 py-0.5 text-xs ${
              detail.status === "published"
                ? "bg-accent-soft text-glow"
                : detail.status === "taken_down"
                  ? "bg-like/10 text-like"
                  : "bg-muted text-ink-3"
            }`}
          >
            {STATUS_LABEL[detail.status] ?? detail.status}
          </span>
        </div>
        <div className="flex items-center gap-2">
          {/* 预览：新标签打开前台详情 */}
          <a
            href={`/posts/${detail.id}`}
            target="_blank"
            rel="noreferrer"
            className="rounded-full border border-line px-4 py-2 text-sm text-ink-2 hover:text-ink"
          >
            预览
          </a>
          <button
            type="button"
            disabled={saving}
            onClick={() => void handleSave("draft")}
            className="rounded-full border border-line px-4 py-2 text-sm text-ink-2 hover:text-ink disabled:opacity-60"
          >
            保存草稿
          </button>
          <button
            type="button"
            disabled={saving}
            onClick={() => void handleSave("published")}
            className="rounded-full bg-accent px-4 py-2 text-sm font-medium text-on-accent hover:opacity-90 disabled:opacity-60"
          >
            更新发布
          </button>
        </div>
      </div>

      {/* 保存成功提示 */}
      {savedTip && (
        <p className="mt-3 rounded-md bg-accent-soft px-3 py-2 text-sm text-glow" role="status">
          {savedTip}
        </p>
      )}

      {/* 两栏布局：左编辑区（表单）/ 右信息面板 */}
      <div className="mt-5 grid gap-6 lg:grid-cols-[1fr_300px]">
        <div className="rounded-lg border border-line bg-elevated p-6">
          <PostEditForm
            ref={formRef}
            detail={detail}
            onSaved={() => {
              setSavedTip("");
              void router.refresh();
            }}
          />
        </div>
        <aside>
          <PostEditPanel
            detail={detail}
            onChanged={() => {
              apiAdminPostDetail(postId).then(setDetail).catch(() => undefined);
            }}
          />
        </aside>
      </div>
    </div>
  );
}
