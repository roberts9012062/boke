// src/app/admin/posts/page.tsx
// 后台内容管理（设计稿 D/冷月/后台内容 1400×1100）：
// 内容管理 + 管理全站帖子·审核·编辑·上下架 + 搜索 + 新建帖子
// + 统计条（全部/文字/图片/音频）+ 表格（内容/类型/状态/互动/时间/操作）。
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { apiAdminDeletePost, apiAdminPosts, apiAdminSetPostStatus } from "@/lib/api";
import { timeAgo } from "@/lib/utils";
import type { AdminPost } from "@/types/api";

// 类型/状态文案映射
const TYPE_LABEL: Record<string, string> = { text: "文字", image: "图片", audio: "音频", video: "视频" };
const STATUS_LABEL: Record<string, string> = {
  published: "已发布",
  draft: "草稿",
  taken_down: "已下架",
  deleted: "已删除",
};

// AdminPosts 内容管理。
export default function AdminPosts() {
  const [items, setItems] = useState<AdminPost[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [type, setType] = useState<string>("");
  const [status, setStatus] = useState<string>("");
  const [keyword, setKeyword] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);

  // 加载列表（筛选变化时）
  useEffect(() => {
    setLoading(true);
    apiAdminPosts({ type, status, q: keyword || undefined })
      .then((r) => {
        setItems(r.items);
        setTotal(r.total);
      })
      .catch(() => setItems([]))
      .finally(() => setLoading(false));
  }, [type, status, keyword]);

  // 上下架切换
  const toggleStatus = async (post: AdminPost) => {
    const next = post.status === "published" ? "taken_down" : "published";
    await apiAdminSetPostStatus(post.id, next);
    // 本地更新
    setItems((prev) => prev.map((p) => (p.id === post.id ? { ...p, status: next } : p)));
  };

  // 删除（二次确认）
  const handleDelete = async (postId: number) => {
    if (!window.confirm("确定删除该帖子？删除后不可恢复")) {
      return;
    }
    await apiAdminDeletePost(postId);
    setItems((prev) => prev.filter((p) => p.id !== postId));
  };

  // 统计条（设计稿：全部 128 / 文字 64 / …）
  const countByType = (t: string) => items.filter((p) => p.content_type === t).length;

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

      {/* 搜索 + 筛选 */}
      <div className="mt-4 flex flex-wrap items-center gap-3">
        <input
          type="text"
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          placeholder="搜索标题、标签…"
          className="h-9 w-56 rounded-full border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
        {/* 类型筛选（设计稿：全部/文字/图片/音频/视频） */}
        <select
          value={type}
          onChange={(e) => setType(e.target.value)}
          className="h-9 rounded-full border border-line bg-elevated px-3 text-sm text-ink-2"
        >
          <option value="">全部类型</option>
          <option value="text">文字</option>
          <option value="image">图片</option>
          <option value="audio">音频</option>
          <option value="video">视频</option>
        </select>
        {/* 状态筛选 */}
        <select
          value={status}
          onChange={(e) => setStatus(e.target.value)}
          className="h-9 rounded-full border border-line bg-elevated px-3 text-sm text-ink-2"
        >
          <option value="">全部状态</option>
          <option value="published">已发布</option>
          <option value="draft">草稿</option>
          <option value="taken_down">已下架</option>
        </select>
        <span className="text-xs text-ink-3">
          共 {total} 条 · 文字 {countByType("text")} · 图片 {countByType("image")} · 音频 {countByType("audio")}
        </span>
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
              <th className="px-4 py-3 font-normal">发布时间</th>
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
                <td className="px-4 py-3 text-xs text-ink-3">
                  {post.published_at ? timeAgo(post.published_at) : "—"}
                </td>
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
    </div>
  );
}
