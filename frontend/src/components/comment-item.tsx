// src/components/comment-item.tsx
// 单条评论（设计稿楼中楼画板）：
// 作者（登录昵称或匿名昵称）· 相对时间 → 内容（@提及）→ 赞 / 回复 / 删除。
"use client";

import { useState } from "react";

import { apiDeleteComment, apiLikeComment } from "@/lib/api";
import { timeAgo } from "@/lib/utils";
import type { CommentDTO } from "@/types/api";

// CommentItem 单条评论（顶层与子回复共用）。
// 参数：comment 评论；onReply 点击回复（父组件处理输入框聚焦）；onDeleted 删除回调。
export function CommentItem({
  comment,
  onReply,
  onDeleted,
}: {
  comment: CommentDTO;
  onReply: (comment: CommentDTO) => void;
  onDeleted: (commentId: number) => void;
}) {
  const [liked, setLiked] = useState<boolean>(comment.liked);
  const [likeCount, setLikeCount] = useState<number>(comment.like_count);
  const [deleting, setDeleting] = useState<boolean>(false);

  // 显示名称：登录用户昵称或匿名昵称（设计稿：路过的猫 / 匿名访客 · 昨天）
  const displayName = comment.author ? comment.author.nickname : comment.guest_name;

  // 点赞（登录用户；未登录时接口返回 1001 提示）
  const handleLike = async () => {
    try {
      const result = await apiLikeComment(comment.id);
      setLiked((v) => !v);
      setLikeCount(result.like_count);
    } catch {
      // 未登录：ApiError 1001 由调用方统一提示（此处静默，交互反馈在评论区层）
    }
  };

  // 删除（仅本人）
  const handleDelete = async () => {
    if (deleting) {
      return;
    }
    setDeleting(true);
    try {
      await apiDeleteComment(comment.id);
      onDeleted(comment.id);
    } catch {
      setDeleting(false);
    }
  };

  return (
    <div className="py-3">
      <div className="flex items-baseline gap-2">
        {/* 作者名（登录加粗，匿名用 ink-2） */}
        <span className={`text-sm ${comment.author ? "font-medium text-ink" : "text-ink-2"}`}>
          {displayName}
        </span>
        <span className="text-xs text-ink-3">{timeAgo(comment.created_at)}</span>
      </div>

      {/* 评论内容（保留 @提及 原文） */}
      <p className="mt-1 whitespace-pre-wrap break-words text-sm leading-relaxed text-ink">
        {comment.content}
      </p>

      {/* 操作行：赞 / 回复 / 删除 */}
      <div className="mt-1.5 flex items-center gap-4 text-xs text-ink-3">
        <button
          type="button"
          onClick={() => void handleLike()}
          className={`flex items-center gap-1 transition-colors ${
            liked ? "text-like" : "hover:text-ink"
          }`}
        >
          <span aria-hidden>{liked ? "♥" : "♡"}</span>
          <span>{likeCount > 0 ? likeCount : "赞"}</span>
        </button>
        <button
          type="button"
          onClick={() => onReply(comment)}
          className="transition-colors hover:text-ink"
        >
          回复
        </button>
        {/* 仅本人可删除 */}
        {comment.is_author && (
          <button
            type="button"
            onClick={() => void handleDelete()}
            disabled={deleting}
            className="text-ink-3 transition-colors hover:text-like disabled:opacity-60"
          >
            删除
          </button>
        )}
      </div>
    </div>
  );
}
