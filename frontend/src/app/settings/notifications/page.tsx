// src/app/settings/notifications/page.tsx
// 通知偏好页（设计稿设置分区「通知」Tab）：
// 通知类型开关（赞/评论/回复/关注/系统/私信），本地持久化。
// 说明（MVP）：偏好存 localStorage；通知接口按偏好过滤后置（差异记录）。
"use client";

import { useEffect, useState } from "react";

import { SettingsLayout } from "@/components/settings-layout";

// localStorage 键（通知偏好）。
const NOTIFY_KEY = "yueyan-notify-prefs";

// 通知类型（与后端通知 type 对应：like/comment/reply/follow/system/message）。
const NOTIFY_TYPES = [
  { key: "like", label: "点赞", desc: "有人赞了你的帖子或评论" },
  { key: "comment", label: "评论", desc: "有人评论了你的帖子" },
  { key: "reply", label: "回复", desc: "有人回复了你的评论" },
  { key: "follow", label: "关注", desc: "有人关注了你" },
  { key: "system", label: "系统", desc: "系统公告与审核结果" },
  { key: "message", label: "私信", desc: "收到新的私信消息" },
] as const;

// 偏好类型（type → 开关）。
type NotifyPrefs = Record<(typeof NOTIFY_TYPES)[number]["key"], boolean>;

// 默认全部开启。
const DEFAULTS: NotifyPrefs = { like: true, comment: true, reply: true, follow: true, system: true, message: true };

// readPrefs 读取本地偏好（解析失败回退默认）。
function readPrefs(): NotifyPrefs {
  if (typeof window === "undefined") {
    return { ...DEFAULTS };
  }
  try {
    const raw = localStorage.getItem(NOTIFY_KEY);
    return raw ? { ...DEFAULTS, ...(JSON.parse(raw) as Partial<NotifyPrefs>) } : { ...DEFAULTS };
  } catch {
    return { ...DEFAULTS };
  }
}

// NotificationsPage 通知偏好设置。
export default function NotificationsPage() {
  const [prefs, setPrefs] = useState<NotifyPrefs>({ ...DEFAULTS });
  const [loaded, setLoaded] = useState<boolean>(false);

  // 加载本地偏好
  useEffect(() => {
    setPrefs(readPrefs());
    setLoaded(true);
  }, []);

  // 切换开关（即时持久化）
  const toggle = (key: keyof NotifyPrefs) => {
    setPrefs((prev) => {
      const next = { ...prev, [key]: !prev[key] };
      localStorage.setItem(NOTIFY_KEY, JSON.stringify(next));
      return next;
    });
  };

  return (
    <SettingsLayout active="notifications">
      <h1 className="font-display text-xl font-semibold text-ink">通知偏好</h1>
      <p className="mt-0.5 text-xs text-ink-3">选择你希望接收的通知类型</p>

      <div className="mt-5 rounded-lg border border-line bg-elevated p-6">
        <div className="divide-y divide-line">
          {NOTIFY_TYPES.map((item) => (
            <div key={item.key} className="flex items-center justify-between py-3">
              <div>
                <p className="text-sm text-ink">{item.label}</p>
                <p className="text-xs text-ink-3">{item.desc}</p>
              </div>
              <button
                type="button"
                onClick={() => toggle(item.key)}
                className={`relative h-6 w-11 shrink-0 rounded-full transition-colors ${
                  prefs[item.key] ? "bg-accent" : "bg-muted"
                }`}
                aria-label={`${item.label}通知开关`}
              >
                <span
                  className={`absolute top-0.5 h-5 w-5 rounded-full bg-white transition-all ${
                    prefs[item.key] ? "left-[22px]" : "left-0.5"
                  }`}
                />
              </button>
            </div>
          ))}
        </div>
        <p className="mt-3 text-xs text-ink-3">偏好保存在本机浏览器，即时生效</p>
        {!loaded && <div className="h-40 animate-pulse rounded-lg bg-muted" aria-hidden />}
      </div>
    </SettingsLayout>
  );
}
