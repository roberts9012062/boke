// src/components/message/chat-view.tsx
// 私信会话视图（设计稿《消息》对话区）：
// 头部（对方昵称 @账号）→ 消息气泡（我右/对方左 + 时间）→ 底部输入（写消息… 发送）。
// 说明：MVP 无推送，15s 轮询新消息；发送后本地追加 + 滚动到底。
"use client";

import { useEffect, useRef, useState } from "react";

import { Avatar } from "@/components/ui/avatar";
import { apiMessages, apiSendMessage, ApiError } from "@/lib/api";
import type { ConversationDTO, MessageDTO } from "@/types/api";

// ChatViewProps 会话视图参数。
interface ChatViewProps {
  conversation: ConversationDTO; // 会话信息（对方 + 未读）
}

// formatTime 消息时间（今天显示 HH:MM，跨天显示 MM-DD HH:MM）。
function formatTime(iso: string): string {
  const date = new Date(iso);
  const now = new Date();
  const sameDay = date.toDateString() === now.toDateString();
  const hhmm = `${date.getHours().toString().padStart(2, "0")}:${date.getMinutes().toString().padStart(2, "0")}`;
  if (sameDay) {
    return hhmm;
  }
  return `${date.getMonth() + 1}-${date.getDate()} ${hhmm}`;
}

// formatDayGroup 消息日期分组标签（设计稿对话内「今天/昨天/更早」分隔）。
function formatDayGroup(iso: string): string {
  const date = new Date(iso);
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterday = new Date(today);
  yesterday.setDate(yesterday.getDate() - 1);
  const day = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  if (day.getTime() === today.getTime()) {
    return "今天";
  }
  if (day.getTime() === yesterday.getTime()) {
    return "昨天";
  }
  return `${date.getMonth() + 1} 月 ${date.getDate()} 日`;
}

// ChatView 私信会话视图（消息列表 + 输入发送 + 轮询）。
export function ChatView({ conversation }: ChatViewProps) {
  const [messages, setMessages] = useState<MessageDTO[]>([]);
  const [input, setInput] = useState<string>("");
  const [sending, setSending] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const bottomRef = useRef<HTMLDivElement>(null);

  // 加载消息（首次 + 会话切换时）
  useEffect(() => {
    let cancelled = false;
    setMessages([]);
    apiMessages(conversation.id)
      .then((r) => {
        if (cancelled) return;
        setMessages(r.items);
      })
      .catch((err: Error) => {
        if (!cancelled) setError(err.message);
      });
    return () => {
      cancelled = true;
    };
  }, [conversation.id]);

  // 轮询新消息（15s；打开会话后未读已清零，轮询只拉新内容）
  useEffect(() => {
    const timer = setInterval(() => {
      apiMessages(conversation.id)
        .then((r) => {
          // 轮询返回最近一页（升序）；按 ID 合并去重并追加到尾部——
          // 历史修复：此前用 r.items 整体替换，会话超过一页（30 条）时旧消息被覆盖丢失
          setMessages((prev) => {
            const known = new Set(prev.map((m) => m.id));
            const fresh = r.items.filter((m) => !known.has(m.id));
            return fresh.length === 0 ? prev : [...prev, ...fresh];
          });
        })
        .catch(() => {
          // 轮询失败静默
        });
    }, 15000);
    return () => clearInterval(timer);
  }, [conversation.id]);

  // 新消息时滚动到底
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.length]);

  // 发送
  const handleSend = async () => {
    const content = input.trim();
    if (!content || sending) {
      return;
    }
    setError("");
    setSending(true);
    try {
      const dto = await apiSendMessage(conversation.id, content);
      setMessages((prev) => [...prev, dto]);
      setInput("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "发送失败");
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      {/* 头部（对方信息；设计稿：北巷 @beixiang · 在线） */}
      <div className="flex items-center gap-3 border-b border-line px-4 py-3">
        <Avatar name={conversation.peer.nickname} url={conversation.peer.avatar_url} className="h-9 w-9 text-sm" />
        <div className="min-w-0">
          <p className="truncate text-sm font-medium text-ink">{conversation.peer.nickname}</p>
          <p className="truncate text-xs text-ink-3">
            @{conversation.peer.username}
            {/* 在线状态（最后登录 5 分钟内，设计稿「· 在线」） */}
            <span className={conversation.peer.online ? "text-glow" : ""}>
              {" "}· {conversation.peer.online ? "在线" : "离线"}
            </span>
          </p>
        </div>
      </div>

      {/* 消息列表（我右 / 对方左；按日期分组：今天/昨天/更早） */}
      <div className="flex-1 space-y-3 overflow-y-auto px-4 py-4">
        {messages.length === 0 && (
          <p className="py-10 text-center text-xs text-ink-3">
            还没有消息，写一句「月色正好」打个招呼吧
          </p>
        )}
        {messages.map((m, index) => {
          // 日期分组：与上一条跨天时插入分隔标签
          const prev = messages[index - 1];
          const showGroup = !prev || new Date(prev.created_at).toDateString() !== new Date(m.created_at).toDateString();
          return (
            <div key={m.id}>
              {showGroup && (
                <p className="my-3 text-center text-[10px] text-ink-3">{formatDayGroup(m.created_at)}</p>
              )}
              <div className={`flex ${m.is_mine ? "justify-end" : "justify-start"}`}>
                <div className={`max-w-[75%] ${m.is_mine ? "items-end" : "items-start"} flex flex-col`}>
                  {/* 气泡（设计稿：我方 accent 色 / 对方 muted） */}
                  <div
                    className={`rounded-2xl px-3.5 py-2 text-sm leading-relaxed ${
                      m.is_mine ? "rounded-br-sm bg-accent text-on-accent" : "rounded-bl-sm bg-muted text-ink"
                    }`}
                  >
                    {m.content}
                  </div>
                  <p className="mt-1 px-1 text-[10px] text-ink-3">{formatTime(m.created_at)}</p>
                </div>
              </div>
            </div>
          );
        })}
        <div ref={bottomRef} />
      </div>

      {/* 输入区（设计稿：写消息… 发送） */}
      <div className="border-t border-line px-4 py-3">
        {error && <p className="mb-2 text-xs text-like">{error}</p>}
        <div className="flex items-center gap-2">
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                void handleSend();
              }
            }}
            placeholder="写消息…"
            maxLength={1000}
            className="h-10 flex-1 rounded-full border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
          />
          <button
            type="button"
            onClick={() => void handleSend()}
            disabled={sending || !input.trim()}
            className="h-10 rounded-full bg-accent px-5 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-50"
          >
            {sending ? "发送中…" : "发送"}
          </button>
        </div>
      </div>
    </div>
  );
}
