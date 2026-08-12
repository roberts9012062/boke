// src/app/admin/posts/page.tsx
// 后台内容管理（设计稿 D/冷月/后台内容 1400×1100）：
// 内容管理 + 管理全站帖子·审核·编辑·上下架 + 搜索 + 新建帖子
// + 类型计数 Tab（全部/文字/图片/音频/视频，走查纠偏）+ 状态 Tab（全部/已发布/草稿/已下架）
// + 表格（内容/类型/状态/互动/更新/操作）+ 分页（走查纠偏补）。
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { apiAdminDeletePost, apiAdminPosts, apiAdminSetPostStatus, apiDashboard } from "@/lib/api";
import { formatDateTime } from "@/lib/utils";
import type { AdminPost } from "@/types/api";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

// 类型/状态文案映射
const TYPE_LABEL: Record<string, string> = { text: "文字", image: "图片", audio: "音频", video: "视频" };
const STATUS_LABEL: Record<string, string> = {
  published: "已发布",
  draft: "草稿",
  taken_down: "已下架",
  deleted: "已删除",
};
// 类型 Tab（设计稿统计条：全部/文字/图片/音频/视频）
const TYPE_TABS = [
  { key: "", label: "全部" },
  { key: "text", label: "文字" },
  { key: "image", label: "图片" },
  { key: "audio", label: "音频" },
  { key: "video", label: "视频" },
];
// 状态 Tab（设计稿：全部/已发布/草稿/已下架）
const STATUS_TABS = [
  { key: "", label: "全部" },
  { key: "published", label: "已发布" },
  { key: "draft", label: "草稿" },
  { key: "taken_down", label: "已下架" },
];
// 每页条数（后端 parsePage 默认）
const PAGE_SIZE = 20;

// AdminPosts 内容管理。
export default function AdminPosts() {
  const [items, setItems] = useState<AdminPost[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [page, setPage] = useState<number>(1);
  const [type, setType] = useState<string>("");
  const [status, setStatus] = useState<string>("");
  const [keyword, setKeyword] = useState<string>("");
  const [typeCounts, setTypeCounts] = useState<Record<string, number>>({}); // 类型计数（全局，走查纠偏）
  const [loading, setLoading] = useState<boolean>(true);
  // 删除确认弹窗（取代 window.confirm——设计稿「确认弹窗」风格）
  const [confirmItem, setConfirmItem] = useState<AdminPost | null>(null);
  const [deleting, setDeleting] = useState<boolean>(false);

  // 类型计数（内容分布接口，全局口径）
  useEffect(() => {
    apiDashboard()
      .then((d) => setTypeCounts(d.type_counts))
      .catch(() => setTypeCounts({}));
  }, []);

  // 加载列表（筛选/分页变化时）
  useEffect(() => {
    setLoading(true);
    apiAdminPosts({ type, status, q: keyword || undefined, page })
      .then((r) => {
        setItems(r.items);
        setTotal(r.total);
      })
      .catch(() => setItems([]))
      .finally(() => setLoading(false));
  }, [type, status, keyword, page]);

  // 上下架切换
  const toggleStatus = async (post: AdminPost) => {
    const next = post.status === "published" ? "taken_down" : "published";
    await apiAdminSetPostStatus(post.id, next);
    // 本地更新
    setItems((prev) => prev.map((p) => (p.id === post.id ? { ...p, status: next } : p)));
  };

  // 删除（二次确认弹窗）
  const handleDelete = async (postId: number) => {
    // 打开确认弹窗（从当前列表中找到对应帖子）
    setConfirmItem(items.find((p) => p.id === postId) ?? null);
  };

  // 确认删除（弹窗确认后执行）
  const confirmDelete = async () => {
    if (!confirmItem) return;
    setDeleting(true);
    try {
      await apiAdminDeletePost(confirmItem.id);
      setItems((prev) => prev.filter((p) => p.id !== confirmItem.id));
      setConfirmItem(null);
    } catch {
      setConfirmItem(null);
    } finally {
      setDeleting(false);
    }
  };

  // 分页（总页数）
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div>
      {/* 标题 + 描述（设计稿文案） */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="font-display text-xl font-semibold text-ink">内容管理</h1>
          <p className="mt-0.5 text-xs text-ink-3">管理全站帖子 · 审核 · 编辑 · 上下架</p>
        </div>
        <Link
          href="/compose"
          className="rounded-full bg-accent px-5 py-2 text-sm text-on-accent hover:opacity-90"
        >
          新建帖子
        </Link>
      </div>

      {/* 类型 Tab（设计稿统计条：全部 128 / 文字 64 / …，走查纠偏下拉改 Tab） */}
      <div className="mt-4 flex flex-wrap gap-2">
        {TYPE_TABS.map((t) => (
          <button
            key={t.key || "all"}
            type="button"
            onClick={() => {
              setType(t.key);
              setPage(1);
            }}
            className={`rounded-full px-4 py-1.5 text-sm transition-colors ${
              type === t.key ? "bg-accent-soft font-medium text-glow" : "bg-muted text-ink-2 hover:text-ink"
            }`}
          >
            {t.label}
            {t.key !== "" && <span className="ml-1 text-xs text-ink-3">{typeCounts[t.key] ?? 0}</span>}
          </button>
        ))}
      </div>

      {/* 搜索 + 状态 Tab（设计稿：全部/已发布/草稿/已下架） */}
      <div className="mt-3 flex flex-wrap items-center gap-3">
        <input
          type="text"
          value={keyword}
          onChange={(e) => {
            setKeyword(e.target.value);
            setPage(1);
          }}
          placeholder="搜索标题、标签…"
          className="h-9 w-56 rounded-full border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
        <div className="flex rounded-full bg-muted p-0.5 text-xs">
          {STATUS_TABS.map((t) => (
            <button
              key={t.key || "all-status"}
              type="button"
              onClick={() => {
                setStatus(t.key);
                setPage(1);
              }}
              className={`rounded-full px-3 py-1 transition-colors ${
                status === t.key ? "bg-accent font-medium text-on-accent" : "text-ink-2 hover:text-ink"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {/* 表格 */}
      <div className="mt-4 overflow-x-auto rounded-lg border border-line bg-elevated">
        <table className="w-full min-w-[720px] text-left text-sm">
          <thead>
            <tr className="border-b border-line text-xs text-ink-3">
              <th className="px-4 py-3 font-normal">内容</th>
              <th className="px-4 py-3 font-normal">类型</th>
              <th className="px-4 py-3 font-normal">状态</th>
              <th className="px-4 py-3 font-normal">互动</th>
              <th className="px-4 py-3 font-normal">更新</th>
              <th className="px-4 py-3 font-normal">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {items.map((post) => (
              <tr key={post.id} className="hover:bg-muted/40">
                <td className="max-w-[260px] px-4 py-3">
                  <Link href={`/posts/${post.id}`} className="block truncate text-ink hover:text-glow">
                    {post.summary || post.title || "（无内容）"}
                  </Link>
                </td>
                <td className="px-4 py-3 text-ink-2">{TYPE_LABEL[post.content_type] ?? post.content_type}</td>
                <td className="px-4 py-3">
                  <span
                    className={`rounded-full px-2 py-0.5 text-xs ${
                      post.status === "published"
                        ? "bg-accent-soft text-glow"
                        : post.status === "taken_down"
                          ? "bg-like/10 text-like"
                          : "bg-muted text-ink-3"
                    }`}
                  >
                    {STATUS_LABEL[post.status] ?? post.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-xs text-ink-3">
                  赞 {post.like_count} · 评 {post.comment_count}
                </td>
                {/* 更新（设计稿第 5 列「更新」，绝对时间；走查纠偏改列名与格式） */}
                <td className="px-4 py-3 text-xs text-ink-3">{formatDateTime(post.updated_at)}</td>
                <td className="px-4 py-3">
                  <div className="flex gap-2 text-xs">
                    {/* 编辑（M2：后台编辑表单，设计稿《后台编辑》四画板） */}
                    <Link href={`/admin/posts/${post.id}/edit`} className="text-glow hover:underline">
                      编辑
                    </Link>
                    <button
                      type="button"
                      onClick={() => void toggleStatus(post)}
                      className="text-glow hover:underline"
                    >
                      {post.status === "published" ? "下架" : "上架"}
                    </button>
                    <button
                      type="button"
                      onClick={() => void handleDelete(post.id)}
                      className="text-like hover:underline"
                    >
                      删除
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!loading && items.length === 0 && (
          <p className="py-12 text-center text-sm text-ink-3">没有匹配的内容</p>
        )}
      </div>

      {/* 分页（设计稿：显示 1–6 · 共 128 篇 + ‹ 1 2 3 ›；走查纠偏补） */}
      {total > 0 && (
        <div className="mt-4 flex items-center justify-between text-sm">
          <span className="text-xs text-ink-3">
            显示 {(page - 1) * PAGE_SIZE + 1}–{Math.min(page * PAGE_SIZE, total)} · 共 {total} 篇
          </span>
          <div className="flex items-center gap-2">
            <button
              type="button"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
              className="rounded-full border border-line px-3 py-1 text-xs text-ink-2 hover:text-ink disabled:opacity-40"
            >
              ‹ 上一页
            </button>
            <span className="text-xs text-ink-3">
              {page} / {totalPages}
            </span>
            <button
              type="button"
              disabled={page >= totalPages}
              onClick={() => setPage((p) => p + 1)}
              className="rounded-full border border-line px-3 py-1 text-xs text-ink-2 hover:text-ink disabled:opacity-40"
            >
              下一页 ›
            </button>
          </div>
        </div>
      )}

      {/* 删除确认弹窗（设计稿「确认弹窗」：问句标题 + 影响提示 + 取消/删除） */}
      <ConfirmDialog
        open={confirmItem !== null}
        title={`删除「${confirmItem?.title ?? ""}」？`}
        description="删除后无法恢复。"
        confirmText="删除"
        danger
        loading={deleting}
        onConfirm={() => void confirmDelete()}
        onClose={() => setConfirmItem(null)}
      />
    </div>
  );
}

