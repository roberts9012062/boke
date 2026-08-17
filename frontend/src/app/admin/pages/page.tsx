// src/app/admin/pages/page.tsx
// 后台自定义页面列表：标题/访问路径/状态/更新时间 + 新建/编辑/删除。
// 自定义页面是独立内容页（关于页/友链页等），前台经 /pages/{slug} 访问。
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ApiError } from "@/lib/api";
import { apiAdminDeletePage, apiAdminPages } from "@/lib/api-pages";
import type { AdminPageItem } from "@/lib/api-pages";

// AdminPagesPage 自定义页面列表页。
export default function AdminPagesPage() {
  const [items, setItems] = useState<AdminPageItem[]>([]);
  const [loaded, setLoaded] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  // 删除确认状态（null = 关闭；否则为待删除页面）
  const [deleteTarget, setDeleteTarget] = useState<AdminPageItem | null>(null);
  const [deleting, setDeleting] = useState<boolean>(false);

  // 加载页面列表
  useEffect(() => {
    apiAdminPages()
      .then((r) => setItems(r.items ?? []))
      .catch((err) => setError(err instanceof ApiError ? err.message : "加载失败"))
      .finally(() => setLoaded(true));
  }, []);

  // 确认删除（删除后从列表移除）
  const handleDelete = async () => {
    if (!deleteTarget) {
      return;
    }
    setDeleting(true);
    try {
      await apiAdminDeletePage(deleteTarget.id);
      setItems((prev) => prev.filter((p) => p.id !== deleteTarget.id));
      setDeleteTarget(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "删除失败");
      setDeleteTarget(null);
    } finally {
      setDeleting(false);
    }
  };

  // 格式化时间（YYYY-MM-DD HH:mm）
  const formatTime = (iso: string): string => {
    const d = new Date(iso);
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  };

  return (
    <div>
      {/* 页头：标题 + 新建入口 */}
      <div className="mb-5 flex items-center justify-between">
        <div>
          <h1 className="font-display text-xl font-semibold text-ink">自定义页面</h1>
          <p className="mt-0.5 text-xs text-ink-3">创建独立内容页（关于/友链等），前台经 /pages/{"{slug}"} 访问</p>
        </div>
        <Link
          href="/admin/pages/new/build"
          className="rounded-full bg-accent px-5 py-2 text-sm font-medium text-on-accent transition-opacity hover:opacity-90"
        >
          新建页面
        </Link>
      </div>

      {error && (
        <p className="mb-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
          {error}
        </p>
      )}

      {/* 列表表格 */}
      <div className="overflow-hidden rounded-lg border border-line bg-elevated">
        {loaded && items.length === 0 ? (
          <div className="px-6 py-16 text-center text-sm text-ink-3">
            还没有自定义页面，点击右上角「新建页面」创建第一页
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-line text-left text-xs text-ink-3">
                <th className="px-4 py-3 font-normal">标题</th>
                <th className="px-4 py-3 font-normal">访问路径</th>
                <th className="px-4 py-3 font-normal">状态</th>
                <th className="px-4 py-3 font-normal">更新时间</th>
                <th className="px-4 py-3 text-right font-normal">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {!loaded
                ? // 加载骨架（与行数对齐的占位条）
                  [0, 1, 2].map((i) => (
                    <tr key={i}>
                      <td colSpan={5} className="px-4 py-4">
                        <div className="h-4 animate-pulse rounded bg-muted" aria-hidden />
                      </td>
                    </tr>
                  ))
                : items.map((page) => (
                    <tr key={page.id} className="transition-colors hover:bg-muted/50">
                      <td className="px-4 py-3">
                        <Link
                          href={`/admin/pages/${page.id}/edit`}
                          className="font-medium text-ink hover:text-glow"
                        >
                          {page.title}
                        </Link>
                      </td>
                      <td className="px-4 py-3">
                        <a
                          href={`/pages/${page.slug}`}
                          target="_blank"
                          rel="noreferrer"
                          className="text-glow hover:underline"
                        >
                          /pages/{page.slug}
                        </a>
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className={`rounded-full px-2 py-0.5 text-xs ${
                            page.status === "published"
                              ? "bg-accent-soft text-glow"
                              : "bg-muted text-ink-3"
                          }`}
                        >
                          {page.status === "published" ? "已发布" : "草稿"}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-ink-3">{formatTime(page.updated_at)}</td>
                      <td className="px-4 py-3 text-right">
                        <Link
                          href={`/admin/pages/${page.id}/edit`}
                          className="mr-3 text-ink-2 hover:text-glow"
                        >
                          编辑
                        </Link>
                        <Link
                          href={`/admin/pages/${page.id}/build`}
                          className="mr-3 text-ink-2 hover:text-glow"
                        >
                          AI 构建
                        </Link>
                        <button
                          type="button"
                          onClick={() => setDeleteTarget(page)}
                          className="text-ink-2 hover:text-like"
                        >
                          删除
                        </button>
                      </td>
                    </tr>
                  ))}
            </tbody>
          </table>
        )}
      </div>

      {/* 删除二次确认 */}
      <ConfirmDialog
        open={deleteTarget !== null}
        title="删除自定义页面"
        description={`确定删除「${deleteTarget?.title ?? ""}」？该操作不可恢复，前台 /pages/${deleteTarget?.slug ?? ""} 将立即失效。`}
        danger
        loading={deleting}
        onConfirm={() => void handleDelete()}
        onClose={() => setDeleteTarget(null)}
      />
    </div>
  );
}
