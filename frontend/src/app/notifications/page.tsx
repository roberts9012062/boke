// src/app/notifications/page.tsx
// 通知页（设计稿 D/冷月/通知 1400×1100）：
// Tab（全部/赞/评论/关注/系统 带未读角标）+ 全部通知 + 3 条未读·近 7 日
// + 全部已读 + 时间分组（近 7 日/更早折叠）+ 未读高亮 + 30s 轮询角标。
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { apiMarkAllRead, apiNotifications, apiUnreadCount, apiMarkRead } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { timeAgo } from "@/lib/utils";
import type { NotificationDTO } from "@/types/api";

// Tab 配置（设计稿：全部/赞/评论/关注/系统）
const TABS = [
  { key: "", label: "全部" },
  { key: "like", label: "赞" },
  { key: "comment", label: "评论" },
  { key: "follow", label: "关注" },
  { key: "system", label: "系统" },
] as const;

// NotificationsPage 通知页（需登录）。
export default function NotificationsPage() {
  const { user, loading } = useAuth();
  const [tab, setTab] = useState<string>("");
  const [items, setItems] = useState<NotificationDTO[]>([]);
  const [unread, setUnread] = useState<number>(0);
  const [loaded, setLoaded] = useState<boolean>(false);
  const [showOlder, setShowOlder] = useState<boolean>(false); // 更早通知折叠（需求 3.8「近 7 日」分区）

  // 加载通知（Tab 切换时刷新）
  useEffect(() => {
    if (loading) {
      return;
    }
    if (!user) {
      setLoaded(true);
      return;
    }
    apiNotifications(tab)
      .then((r) => setItems(r.items))
      .catch(() => setItems([]))
      .finally(() => setLoaded(true));
  }, [tab, loading, user]);

  // 未读角标轮询（30s，需求 3.8）
  useEffect(() => {
    if (!user) {
      return;
    }
    const poll = () => {
      apiUnreadCount()
        .then((r) => setUnread(r.unread))
        .catch(() => {
          // 轮询失败静默
        });
    };
    poll();
    const timer = setInterval(poll, 30000);
    return () => clearInterval(timer);
  }, [user]);

  // 全部已读
  const handleMarkAll = async () => {
    await apiMarkAllRead();
    setUnread(0);
    setItems((prev) => prev.map((n) => ({ ...n, read: true })));
  };

  // 点击通知：跳转 + 标记已读
  const handleClick = async (n: NotificationDTO) => {
    if (!n.read) {
      void apiMarkRead(n.id);
      setUnread((u) => Math.max(u - 1, 0));
      setItems((prev) => prev.map((x) => (x.id === n.id ? { ...x, read: true } : x)));
    }
  };

  // 未登录：提示
  if (loaded && !user) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center">
        <p className="text-lg text-ink">请先登录查看通知</p>
        <Link href="/login" className="mt-4 text-sm text-glow hover:underline">
          去登录 →
        </Link>
        <MobileTabbar />
      </div>
    );
  }

  // 分区（M1.7 技术债 #4 修复）：近 7 日 / 更早（需求 3.8「首屏近 7 日分区，更早折叠」）
  const splitByWeek = (list: NotificationDTO[]): { recent: NotificationDTO[]; older: NotificationDTO[] } => {
    const weekAgo = new Date();
    weekAgo.setDate(weekAgo.getDate() - 7);
    const recent: NotificationDTO[] = [];
    const older: NotificationDTO[] = [];
    for (const n of list) {
      if (new Date(n.created_at) >= weekAgo) {
        recent.push(n);
      } else {
        older.push(n);
      }
    }
    return { recent, older };
  };
  const { recent, older } = splitByWeek(items);

  // 渲染通知列表（分组行）
  const renderList = (list: NotificationDTO[]) => (
    <div className="divide-y divide-line rounded-lg border border-line bg-elevated">
      {list.map((n) => (
        <Link
          key={n.id}
          href={n.link || "#"}
          onClick={() => void handleClick(n)}
          className={`flex items-start gap-3 px-4 py-3 transition-colors hover:bg-muted ${
            !n.read ? "bg-accent-soft/40" : ""
          }`}
        >
          {/* 触发者头像（系统通知用图标占位） */}
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-muted text-sm text-ink-2">
            {n.actor ? n.actor.nickname.charAt(0) : "系"}
          </span>
          <div className="min-w-0 flex-1">
            <p className="text-sm text-ink">
              <span className="font-medium">{n.actor ? n.actor.nickname : "系统"}</span>
              <span className="text-ink-2"> {n.title}</span>
              {/* 未读圆点 */}
              {!n.read && (
                <span className="ml-2 inline-block h-2 w-2 rounded-full bg-like" aria-label="未读" />
              )}
            </p>
            {n.content && (
              <p className="mt-0.5 line-clamp-1 text-xs text-ink-3">{n.content}</p>
            )}
            <p className="mt-1 text-[11px] text-ink-3">{timeAgo(n.created_at)}</p>
          </div>
        </Link>
      ))}
    </div>
  );

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-[720px] flex-1 px-4 py-6 pb-20">
        <div className="flex items-center justify-between">
          <h1 className="font-display text-xl font-semibold text-ink">通知</h1>
          {/* 全部已读（设计稿按钮） */}
          <button
            type="button"
            onClick={() => void handleMarkAll()}
            disabled={unread === 0}
            className="rounded-full border border-line px-4 py-1.5 text-xs text-ink-2 hover:text-ink disabled:opacity-50"
          >
            全部已读
          </button>
        </div>

        {/* Tab（设计稿：全部 5 / 赞 2 / 评论 1 … 带角标） */}
        <div className="mt-4 flex gap-2 border-b border-line pb-3">
          {TABS.map((t) => (
            <button
              key={t.key}
              type="button"
              onClick={() => setTab(t.key)}
              className={`flex items-center gap-1.5 rounded-full px-4 py-1.5 text-sm transition-colors ${
                tab === t.key
                  ? "bg-accent-soft font-medium text-glow"
                  : "bg-muted text-ink-2 hover:text-ink"
              }`}
            >
              {t.label}
              {/* 未读角标（全部 Tab 显示总未读） */}
              {((t.key === "" && unread > 0) || (t.key !== "" && items.filter((n) => !n.read && n.type === t.key).length > 0)) && (
                <span className="flex h-4 min-w-4 items-center justify-center rounded-full bg-like px-1 text-[10px] text-white">
                  {t.key === "" ? unread : items.filter((n) => !n.read && n.type === t.key).length}
                </span>
              )}
            </button>
          ))}
        </div>

        {/* 未读提示（设计稿：3 条未读 · 近 7 日） */}
        {unread > 0 && (
          <p className="mt-3 text-xs text-ink-3">
            {unread} 条未读 · 近 7 日
          </p>
        )}

        {/* 列表区标题（设计稿：全部通知） */}
        <h2 className="mt-5 text-sm font-medium text-ink-2">全部通知</h2>

        {/* 通知列表（近 7 日 / 更早折叠，需求 3.8） */}
        <div className="mt-3 space-y-6">
          {loaded && items.length === 0 && (
            <div className="rounded-lg border border-line bg-elevated py-16 text-center">
              <p className="text-sm text-ink-2">暂无通知</p>
            </div>
          )}

          {/* 近 7 日分区（首屏展示） */}
          {recent.length > 0 && (
            <section>
              <h2 className="text-xs font-medium text-ink-3">近 7 日</h2>
              <div className="mt-2">{renderList(recent)}</div>
            </section>
          )}

          {/* 更早分区（默认折叠，点击展开） */}
          {older.length > 0 && (
            <section>
              <button
                type="button"
                onClick={() => setShowOlder((v) => !v)}
                className="flex w-full items-center justify-between text-xs font-medium text-ink-3 transition-colors hover:text-ink"
              >
                <span>更早 · {older.length} 条</span>
                <span aria-hidden>{showOlder ? "收起 ▲" : "展开 ▼"}</span>
              </button>
              {showOlder && <div className="mt-2">{renderList(older)}</div>}
            </section>
          )}
        </div>
      </main>
      <MobileTabbar />
    </div>
  );
}

