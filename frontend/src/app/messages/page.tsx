// src/app/messages/page.tsx
// 消息中心（设计稿《消息》画板，D 双栏 1400 / M 单栏 390）：
// 消息 → 搜索对话…（占位）→ Tab（全部/未读）→ 会话列表（头像/昵称/最后消息/时间/未读徽标）。
// 桌面（≥1024px）：左列表 + 右会话详情内嵌；移动端：点击会话跳 /messages/[id]。
"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { ChatView } from "@/components/message/chat-view";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { Avatar } from "@/components/ui/avatar";
import { apiConversations } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { timeAgo } from "@/lib/utils";
import type { ConversationDTO } from "@/types/api";

// MessagesPage 消息中心（需登录）。
export default function MessagesPage() {
  const router = useRouter();
  const { user, loading } = useAuth();
  const [tab, setTab] = useState<string>(""); // 空=全部；unread=未读（设计稿 Tab）
  const [items, setItems] = useState<ConversationDTO[]>([]);
  const [activeId, setActiveId] = useState<number | null>(null); // 桌面选中会话
  const [query, setQuery] = useState<string>(""); // 对话搜索关键词（客户端过滤昵称/账号）
  const [loaded, setLoaded] = useState<boolean>(false);

  // 加载会话列表（Tab 切换时刷新）
  useEffect(() => {
    if (loading) {
      return;
    }
    if (!user) {
      setLoaded(true);
      return;
    }
    apiConversations(tab)
      .then((r) => {
        setItems(r.items);
        // 选中会话不在新列表时清空选中（切「未读」Tab 后右侧不再指向被过滤会话）
        setActiveId((cur) => (cur !== null && r.items.some((c) => c.id === cur) ? cur : null));
      })
      .catch(() => setItems([]))
      .finally(() => setLoaded(true));
  }, [tab, loading, user]);

  // 未登录：提示
  if (loaded && !user) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center">
        <p className="text-lg text-ink">请先登录查看消息</p>
        <Link href="/login" className="mt-4 text-sm text-glow hover:underline">
          去登录 →
        </Link>
        <MobileTabbar />
      </div>
    );
  }

  // 当前选中会话（桌面右侧详情）
  const active = items.find((c) => c.id === activeId) ?? null;
  // 搜索过滤（客户端按对方昵称/账号过滤；M 后置修复：此前搜索框为只读占位）
  const keyword = query.trim().toLowerCase();
  const visibleItems = keyword
    ? items.filter(
        (c) =>
          c.peer.nickname.toLowerCase().includes(keyword) ||
          c.peer.username.toLowerCase().includes(keyword),
      )
    : items;

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto flex w-full max-w-[1400px] flex-1 gap-6 px-4 py-6 pb-20 lg:px-6">
        {/* 左栏：会话列表（移动端占满宽度，桌面 320px） */}
        <section className="w-full lg:w-[320px] lg:shrink-0">
          <h1 className="font-display text-xl font-semibold text-ink">消息</h1>

          {/* 搜索对话（客户端按对方昵称/账号过滤） */}
          <input
            type="text"
            placeholder="搜索对话…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="mt-3 h-9 w-full rounded-full border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
          />

          {/* Tab（设计稿：全部 / 未读） */}
          <div className="mt-3 flex gap-2">
            {[
              { key: "", label: "全部" },
              { key: "unread", label: "未读" },
            ].map((t) => (
              <button
                key={t.key}
                type="button"
                onClick={() => setTab(t.key)}
                className={`rounded-full px-4 py-1.5 text-sm transition-colors ${
                  tab === t.key
                    ? "bg-accent-soft font-medium text-glow"
                    : "bg-muted text-ink-2 hover:text-ink"
                }`}
              >
                {t.label}
              </button>
            ))}
          </div>

          {/* 加载骨架 */}
          {!loaded && <div className="mt-3 h-32 animate-pulse rounded-lg bg-muted" aria-hidden />}

          {/* 空态 */}
          {loaded && items.length === 0 && (
            <div className="mt-3 rounded-lg border border-line bg-elevated py-12 text-center">
              <p className="text-sm text-ink-2">{tab === "unread" ? "没有未读消息" : "还没有会话"}</p>
              <p className="mt-1 text-xs text-ink-3">在他人主页点「私信」发起对话</p>
            </div>
          )}
          {/* 搜索无结果 */}
          {loaded && items.length > 0 && visibleItems.length === 0 && (
            <div className="mt-3 rounded-lg border border-line bg-elevated py-12 text-center">
              <p className="text-sm text-ink-2">没有匹配的会话</p>
              <p className="mt-1 text-xs text-ink-3">换个关键词试试</p>
            </div>
          )}

          {/* 会话列表（设计稿：北巷 2 分钟前 今晚的声音帖很好听 + 未读徽标） */}
          <div className="mt-3 divide-y divide-line overflow-hidden rounded-lg border border-line bg-elevated">
            {visibleItems.map((c) => (
              <button
                key={c.id}
                type="button"
                onClick={() => {
                  // 桌面选中内嵌；移动端跳详情页
                  if (window.innerWidth >= 1024) {
                    setActiveId(c.id);
                  } else {
                    router.push(`/messages/${c.id}`);
                  }
                }}
                className={`flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-muted ${
                  activeId === c.id ? "bg-accent-soft/50" : ""
                }`}
              >
                <Avatar name={c.peer.nickname} url={c.peer.avatar_url} className="h-10 w-10 text-sm" />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center justify-between">
                    <p className="truncate text-sm font-medium text-ink">{c.peer.nickname}</p>
                    {c.last_message_at && (
                      <p className="shrink-0 text-[10px] text-ink-3">{timeAgo(c.last_message_at)}</p>
                    )}
                  </div>
                  <div className="mt-0.5 flex items-center justify-between gap-2">
                    <p className="truncate text-xs text-ink-2">{c.last_message || "打个招呼吧"}</p>
                    {/* 未读徽标（设计稿：2） */}
                    {c.unread > 0 && (
                      <span className="flex h-4 min-w-4 shrink-0 items-center justify-center rounded-full bg-like px-1 text-[10px] text-white">
                        {c.unread > 99 ? "99+" : c.unread}
                      </span>
                    )}
                  </div>
                </div>
              </button>
            ))}
          </div>
        </section>

        {/* 右栏：会话详情（桌面 ≥1024px 显示；移动端由 [id] 页承载） */}
        {active && (
          <section className="hidden min-w-0 flex-1 lg:block">
            <div className="h-[calc(100vh-8rem)] overflow-hidden rounded-lg border border-line bg-elevated">
              <ChatView conversation={active} />
            </div>
          </section>
        )}
        {!active && (
          <section className="hidden min-w-0 flex-1 lg:block">
            <div className="flex h-[calc(100vh-8rem)] items-center justify-center rounded-lg border border-line bg-elevated">
              <div className="text-center">
                <span className="text-3xl" aria-hidden>
                  💬
                </span>
                <p className="mt-3 text-sm text-ink-2">选择一个会话开始对话</p>
                <p className="mt-1 text-xs text-ink-3">在他人主页点「私信」发起对话</p>
              </div>
            </div>
          </section>
        )}
      </main>
      <MobileTabbar />
    </div>
  );
}
