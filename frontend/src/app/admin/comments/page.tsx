// src/app/admin/comments/page.tsx
// 后台评论管理（设计稿 D/冷月/后台评论 1400×1000）：
// 统计条（全部评论/今日新增/已屏蔽）+ 搜索 + 状态筛选（全部/已屏蔽）
// + 评论表格（内容/作者/帖子/状态/时间/操作：隐藏恢复 + 删除）。
// 差异记录：设计稿统计条含「待审核」，MVP 无评论审核流（M4 AI 审核），暂以三项统计呈现。
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import {
  apiAdminCommentStats,
  apiAdminComments,
  apiAdminDeleteComment,
  apiAdminSetCommentStatus,
  ApiError,
} from "@/lib/api";
import { apiAiReviewComments } from "@/lib/api-ai";
import { timeAgo } from "@/lib/utils";
import type { AdminComment } from "@/types/api";

// AdminComments 评论管理。
export default function AdminComments() {
  const [items, setItems] = useState<AdminComment[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [keyword, setKeyword] = useState<string>("");
  const [status, setStatus] = useState<string>(""); // "" 全部 / hidden 已屏蔽
  const [stats, setStats] = useState<{ total: number; today: number; hidden: number }>({
    total: 0,
    today: 0,
    hidden: 0,
  });
  const [loading, setLoading] = useState<boolean>(true);

  // 加载统计条（设计稿：全部评论 / 今日新增 / 已屏蔽）
  useEffect(() => {
    apiAdminCommentStats().then(setStats).catch(() => undefined);
  }, []);

  // 加载列表（筛选变化时）
  useEffect(() => {
    setLoading(true);
    apiAdminComments({ status: status || undefined, q: keyword || undefined })
      .then((r) => {
        setItems(r.items);
        setTotal(r.total);
      })
      .catch(() => setItems([]))
      .finally(() => setLoading(false));
  }, [keyword, status]);

  // 隐藏/恢复（M2：visible ↔ hidden，前台列表仅展示 visible）
  const toggleHidden = async (comment: AdminComment) => {
    const next = comment.status === "hidden" ? "visible" : "hidden";
    await apiAdminSetCommentStatus(comment.id, next);
    setItems((prev) => prev.map((c) => (c.id === comment.id ? { ...c, status: next } : c)));
    // 刷新统计条
    apiAdminCommentStats().then(setStats).catch(() => undefined);
  };

  // 删除（二次确认）
  const handleDelete = async (commentId: number) => {
    if (!window.confirm("确定删除该评论？")) {
      return;
    }
    await apiAdminDeleteComment(commentId);
    setItems((prev) => prev.filter((c) => c.id !== commentId));
    apiAdminCommentStats().then(setStats).catch(() => undefined);
  };

  // AI 审核单条评论（M4 手动兜底：高风险自动隐藏并进审核队列，刷新列表反映状态变化）
  const handleAiReview = async (commentId: number) => {
    try {
      const r = await apiAiReviewComments([commentId]);
      const msg = r.failed > 0 ? `审核完成，${r.failed} 条失败` : "AI 审核完成（高风险评论已隐藏并进入审核队列）";
      alert(msg);
      // 重新加载列表与统计（隐藏状态会变化）
      apiAdminComments({ status: status || undefined, q: keyword || undefined })
        .then((res) => {
          setItems(res.items);
          setTotal(res.total);
        })
        .catch(() => undefined);
      apiAdminCommentStats().then(setStats).catch(() => undefined);
    } catch (err) {
      alert(err instanceof ApiError ? err.message : "AI 审核失败");
    }
  };

  return (
    <div>
      {/* 标题（设计稿：评论审核）+ 副标题（审核、回复与处理不当评论） */}
      <h1 className="font-display text-xl font-semibold text-ink">评论审核</h1>
      <p className="mt-0.5 text-xs text-ink-3">审核、回复与处理不当评论</p>

      {/* 统计条（设计稿：全部评论 / 待审核 / 今日新增 / 已屏蔽；走查纠偏补待审核卡 = 已屏蔽数） */}
      <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
        {[
          { key: "全部评论", value: stats.total },
          { key: "待审核", value: stats.hidden },
          { key: "今日新增", value: stats.today },
          { key: "已屏蔽", value: stats.hidden },
        ].map((s) => (
          <div key={s.key} className="rounded-lg border border-line bg-elevated p-4">
            <p className="text-2xl font-semibold text-ink">{s.value}</p>
            <p className="mt-1 text-xs text-ink-3">{s.key}</p>
          </div>
        ))}
      </div>

      {/* 搜索 + 状态筛选 */}
      <div className="mt-4 flex flex-wrap items-center gap-3">
        <input
          type="text"
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          placeholder="搜索评论…"
          className="h-9 w-56 rounded-full border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
        <select
          value={status}
          onChange={(e) => setStatus(e.target.value)}
          className="h-9 rounded-full border border-line bg-elevated px-3 text-sm text-ink-2"
        >
          <option value="">全部评论</option>
          <option value="hidden">已屏蔽</option>
        </select>
        <span className="text-xs text-ink-3">共 {total} 条</span>
      </div>

      <div className="mt-4 overflow-x-auto rounded-lg border border-line bg-elevated">
        <table className="w-full min-w-[720px] text-left text-sm">
          <thead>
            <tr className="border-b border-line text-xs text-ink-3">
              <th className="px-4 py-3 font-normal">评论</th>
              <th className="px-4 py-3 font-normal">作者</th>
              <th className="px-4 py-3 font-normal">帖子</th>
              <th className="px-4 py-3 font-normal">状态</th>
              <th className="px-4 py-3 font-normal">时间</th>
              <th className="px-4 py-3 font-normal">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {items.map((comment) => (
              <tr key={comment.id} className="hover:bg-muted/40">
                <td className="max-w-[280px] px-4 py-3 text-ink">{comment.content}</td>
                <td className="px-4 py-3 text-ink-2">
                  {comment.author ? (
                    <span>
                      {comment.author.nickname}
                      <span className="ml-1 text-xs text-ink-3">@{comment.author.username}</span>
                    </span>
                  ) : (
                    <span>
                      {comment.guest_name || "匿名访客"}
                      <span className="ml-1 text-xs text-ink-3">(匿名)</span>
                    </span>
                  )}
                </td>
                <td className="px-4 py-3">
                  <Link href={`/posts/${comment.post_id}`} className="text-xs text-glow hover:underline">
                    帖子 #{comment.post_id}
                  </Link>
                </td>
                <td className="px-4 py-3">
                  <span
                    className={`rounded-full px-2 py-0.5 text-xs ${
                      comment.status === "hidden" ? "bg-like/10 text-like" : "bg-accent-soft text-glow"
                    }`}
                  >
                    {comment.status === "hidden" ? "已屏蔽" : "正常"}
                  </span>
                </td>
                <td className="px-4 py-3 text-xs text-ink-3">{timeAgo(comment.created_at)}</td>
                <td className="px-4 py-3">
                  <div className="flex gap-2 text-xs">
                    <button
                      type="button"
                      onClick={() => void handleAiReview(comment.id)}
                      className="text-glow hover:underline"
                    >
                      AI 审核
                    </button>
                    <button
                      type="button"
                      onClick={() => void toggleHidden(comment)}
                      className="text-glow hover:underline"
                    >
                      {comment.status === "hidden" ? "恢复" : "隐藏"}
                    </button>
                    <button
                      type="button"
                      onClick={() => void handleDelete(comment.id)}
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
          <p className="py-12 text-center text-sm text-ink-3">没有匹配的评论</p>
        )}
      </div>
    </div>
  );
}
