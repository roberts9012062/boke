// src/components/comment-section.tsx
// 评论区（设计稿 D/冷月/楼中楼 + 帖子详情）：
// 「评论 · N」标题 → 写一条评论…（开放，无需登录）→ 顶层评论 + 楼中楼
// （展开/收起回复、@提及回复、评论点赞、匿名昵称弹层、删除）。
// 动效：匿名昵称弹层遮罩淡入/面板缩放进出场；楼中楼展开内容上移淡入。
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { CommentItem } from "@/components/comment-item";
import PluginSlot from "@/components/plugin-slot";
import { usePresence } from "@/components/motion/use-presence";
import { apiComments, apiCreateComment, apiReplyComment, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { clearGuest, ensureGuest, readGuest } from "@/lib/guest";
import type { CommentDTO } from "@/types/api";

// CommentSection 评论区（开放评论，无需登录）。
// 参数：postId 帖子 ID；initialCount 帖子评论计数（展示用）。
export function CommentSection({ postId, initialCount = 0 }: { postId: number; initialCount?: number }) {
  const { user } = useAuth();
  const [comments, setComments] = useState<CommentDTO[]>([]);
  const [count, setCount] = useState<number>(initialCount);
  const [input, setInput] = useState<string>("");
  const [guestNameInput, setGuestNameInput] = useState<string>("");
  const [showGuestDialog, setShowGuestDialog] = useState<boolean>(false);
  const [replyingTo, setReplyingTo] = useState<CommentDTO | null>(null);
  const [expanded, setExpanded] = useState<Record<number, boolean>>({});
  const [submitting, setSubmitting] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const inputRef = useRef<HTMLInputElement>(null);
  // 匿名昵称弹层进出场（离场 180ms 与 animate-scale-out 一致）
  const { mounted: guestMounted, leaving: guestLeaving } = usePresence(showGuestDialog, 180);

  // 加载评论列表
  const loadComments = useCallback(async () => {
    try {
      const list = await apiComments(postId);
      setComments(list);
      // 同步计数（顶层 + 全部回复）
      const total = list.reduce((sum, c) => sum + 1 + c.reply_count, 0);
      setCount(total);
    } catch {
      // 加载失败静默（保留空态）
    }
  }, [postId]);

  useEffect(() => {
    void loadComments();
  }, [loadComments]);

  // 展开/收起回复（设计稿：收起 3 条回复 / 展开 2 条回复）
  const toggleExpand = (commentId: number) => {
    setExpanded((prev) => ({ ...prev, [commentId]: !prev[commentId] }));
  };

  // 点击「回复」：进入回复模式并聚焦输入框（@提及前缀由提交时生成）
  const handleReply = (comment: CommentDTO) => {
    setReplyingTo(comment);
    inputRef.current?.focus();
  };

  // 提交评论/回复：已登录直接提交；访客先弹昵称弹层
  const handleSubmit = () => {
    const content = input.trim();
    if (!content) {
      return;
    }
    if (user) {
      void doSubmit(content);
      return;
    }
    // 访客：已有匿名身份直接提交，否则弹昵称输入
    if (readGuest()) {
      void doSubmit(content);
    } else {
      setShowGuestDialog(true);
    }
  };

  // 实际提交（content 已含 @提及 前缀）
  const doSubmit = async (rawContent: string) => {
    setError("");
    setSubmitting(true);
    try {
      // 生成 @提及 前缀（回复时）
      let content = rawContent;
      if (replyingTo) {
        const target = replyingTo.author ? replyingTo.author.nickname : replyingTo.guest_name;
        content = `@${target} ${rawContent}`;
      }
      // 访客：确保匿名身份
      let guestToken = "";
      if (!user) {
        const guest = await ensureGuest("");
        if (!guest) {
          setError("匿名身份获取失败，请稍后再试");
          return;
        }
        guestToken = guest.guest_token;
      }
      // 发表（顶层或回复）
      if (replyingTo) {
        await apiReplyComment(replyingTo.id, content, guestToken);
      } else {
        await apiCreateComment(postId, content, guestToken);
      }
      // 重置状态并刷新列表
      setInput("");
      setReplyingTo(null);
      setShowGuestDialog(false);
      await loadComments();
    } catch (err) {
      // 匿名身份失效自愈：本地 token 无效（如后端重启清空内存）时
      // 清除本地身份并重新弹出昵称层（产品闭环，防永久卡死）
      if (err instanceof ApiError && err.code === 1001 && !user) {
        clearGuest();
        setInput("");
        setShowGuestDialog(true);
        return;
      }
      setError(err instanceof ApiError ? err.message : "评论失败，请稍后再试");
    } finally {
      setSubmitting(false);
    }
  };

  // 匿名昵称弹层确认
  const confirmGuest = async () => {
    const guest = await ensureGuest(guestNameInput.trim());
    if (!guest) {
      setError("匿名身份获取失败，请稍后再试");
      return;
    }
    setShowGuestDialog(false);
    await doSubmit(input.trim());
  };

  // 删除评论回调（刷新列表）
  const handleDeleted = () => {
    void loadComments();
  };

  return (
    <section id="comments" className="mt-6">
      {/* 标题（设计稿：评论 · 36） */}
      <h2 className="font-display text-base font-semibold text-ink">评论 · {count}</h2>

      {/* 输入框（设计稿：写一条评论…（开放，无需登录）） */}
      <div className="mt-3 flex items-center gap-2">
        <input
          ref={inputRef}
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              handleSubmit();
            }
          }}
          placeholder={replyingTo ? `回复 @${replyingTo.author ? replyingTo.author.nickname : replyingTo.guest_name}…` : "写一条评论…（开放，无需登录）"}
          className="h-10 flex-1 rounded-full border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
        <button
          type="button"
          onClick={handleSubmit}
          disabled={submitting || input.trim() === ""}
          className="h-10 rounded-full bg-accent px-5 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
        >
          {submitting ? "发送中…" : "发送"}
        </button>
      </div>
      {/* 回复模式提示 */}
      {replyingTo && (
        <p className="mt-2 text-xs text-ink-3">
          正在回复
          <span className="mx-1 text-glow">
            @{replyingTo.author ? replyingTo.author.nickname : replyingTo.guest_name}
          </span>
          <button
            type="button"
            onClick={() => setReplyingTo(null)}
            className="ml-2 text-ink-2 underline"
          >
            取消
          </button>
        </p>
      )}
      {error && (
        <p className="mt-2 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
          {error}
        </p>
      )}

      {/* 匿名昵称弹层（设计稿：开放，无需登录 → 首次弹昵称输入） */}
      {guestMounted && (
        <div
          className={`fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-6 ${
            guestLeaving ? "animate-fade-out" : "animate-fade-in"
          }`}
          onClick={() => setShowGuestDialog(false)}
        >
          <div
            className={`w-full max-w-sm rounded-lg border border-line bg-elevated p-5 ${
              guestLeaving ? "animate-scale-out" : "animate-scale-in"
            }`}
            onClick={(e) => e.stopPropagation()}
            role="dialog"
            aria-label="匿名评论昵称"
          >
            <p className="font-display text-base font-semibold text-ink">评论身份</p>
            <p className="mt-1 text-xs text-ink-3">访客评论无需登录，可自填昵称（可选）</p>
            <input
              type="text"
              value={guestNameInput}
              onChange={(e) => setGuestNameInput(e.target.value)}
              placeholder="你的昵称（留空使用「匿名访客」）"
              className="mt-3 h-10 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setShowGuestDialog(false)}
                className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink"
              >
                取消
              </button>
              <button
                type="button"
                onClick={() => void confirmGuest()}
                className="rounded-full bg-accent px-5 py-1.5 text-sm text-on-accent"
              >
                继续
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 评论列表（楼中楼） */}
      <div className="mt-4 divide-y divide-line">
        {comments.length === 0 && (
          <p className="py-10 text-center text-sm text-ink-3">还没有评论，写下第一条</p>
        )}
        {comments.map((comment) => (
          <div key={comment.id}>
            <CommentItem
              comment={comment}
              onReply={handleReply}
              onDeleted={handleDeleted}
            />
            {/* 插件扩展点：comment.item（M3.9 每条评论独立槽位——顶层评论；楼中楼不挂载防性能爆炸） */}
            <PluginSlot slot="comment.item" props={{ comment }} />
            {/* 楼中楼：展开/收起回复（设计稿） */}
            {comment.reply_count > 0 && (
              <div className="ml-4 border-l border-line pl-4">
                {/* 已展开：显示全部回复（上移淡入） */}
                {expanded[comment.id] && (
                  <div className="animate-fade-up divide-y divide-line">
                    {comment.replies.map((reply) => (
                      <CommentItem
                        key={reply.id}
                        comment={reply}
                        onReply={handleReply}
                        onDeleted={handleDeleted}
                      />
                    ))}
                  </div>
                )}
                {/* 展开/收起按钮（设计稿：收起 3 条回复 / 展开 2 条回复） */}
                <button
                  type="button"
                  onClick={() => toggleExpand(comment.id)}
                  className="py-2 text-xs text-ink-3 transition-colors hover:text-glow"
                >
                  {expanded[comment.id] ? `收起 ${comment.reply_count} 条回复` : `展开 ${comment.reply_count} 条回复`}
                </button>
              </div>
            )}
          </div>
        ))}
      </div>
      {/* 插件扩展点：comment.footer（M3.6；差异记录：文档为单条评论下方，先挂评论区底部） */}
      <PluginSlot slot="comment.footer" />
    </section>
  );
}
