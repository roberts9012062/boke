// src/app/admin/comments/page.tsx
// 后台评论管理（需求 4.4）：评论内容/作者（含匿名）/所属帖子/时间/状态 + 筛选/搜索。
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { apiAdminComments, apiAdminDeleteComment } from "@/lib/api";
import { timeAgo } from "@/lib/utils";
import type { AdminComment } from "@/types/api";

// AdminComments 评论管理。
export default function AdminComments() {
  const [items, setItems] = useState<AdminComment[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [keyword, setKeyword] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);

  useEffect(() => {
    setLoading(true);
    apiAdminComments({ q: keyword || undefined })
      .then((r) => {
        setItems(r.items);
        setTotal(r.total);
      })
      .catch(() => setItems([]))
      .finally(() => setLoading(false));
  }, [keyword]);

  // 删除（二次确认）
  const handleDelete = async (commentId: number) => {
    if (!window.confirm("确定删除该评论？")) {
      return;
    }
    await apiAdminDeleteComment(commentId);
    setItems((prev) => prev.filter((c) => c.id !== commentId));
  };

  return (
    <div>
      <h1 className="font-display text-xl font-semibold text-ink">评论管理</h1>
      <p className="mt-0.5 text-xs text-ink-3">共 {total} 条评论 · 含匿名访客评论</p>

      <div className="mt-4 flex items-center gap-3">
        <input
          type="text"
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          placeholder="搜索评论内容…"
          className="h-9 w-56 rounded-full border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
      </div>

      <div className="mt-4 overflow-x-auto rounded-lg border border-line bg-elevated">
        <table className="w-full min-w-[720px] text-left text-sm">
          <thead>
            <tr className="border-b border-line text-xs text-ink-3">
              <th className="px-4 py-3 font-normal">评论内容</th>
              <th className="px-4 py-3 font-normal">作者</th>
              <th className="px-4 py-3 font-normal">所属帖子</th>
              <th className="px-4 py-3 font-normal">时间</th>
              <th className="px-4 py-3 font-normal">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {items.map((comment) => (
              <tr key={comment.id} className="hover:bg-muted/40">
                <td className="max-w-[280px] px-4 py-3 text-ink">{comment.content}</td>
                <td className="px-4 py-3 text-ink-2">
                  {comment.author ? comment.author.nickname : comment.guest_name || "匿名访客"}
                  {!comment.author && <span className="ml-1 text-xs text-ink-3">(匿名)</span>}
                </td>
                <td className="px-4 py-3">
                  <Link href={`/posts/${comment.post_id}`} className="text-xs text-glow hover:underline">
                    帖子 #{comment.post_id}
                  </Link>
                </td>
                <td className="px-4 py-3 text-xs text-ink-3">{timeAgo(comment.created_at)}</td>
                <td className="px-4 py-3">
                  <button
                    type="button"
                    onClick={() => void handleDelete(comment.id)}
                    className="text-xs text-like hover:underline"
                  >
                    删除
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!loading && items.length === 0 && (
          <p className="py-12 text-center text-sm text-ink-3">没有匹配的评论</p>
        )}
      </div>
    </div>
  );
}
