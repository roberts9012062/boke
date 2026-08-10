// src/app/admin/users/page.tsx
// 后台用户管理（需求 4.5 + M2 封禁增强）：
// 用户表格（头像/昵称/@账号/角色/帖子数/状态/注册时间）+ 搜索；
// 封禁弹层（原因 + 永久/限时，写 ban_records）+ 解封。
"use client";

import { useEffect, useState } from "react";

import { apiAdminSetUserStatus, apiAdminUsers, ApiError } from "@/lib/api";
import { timeAgo } from "@/lib/utils";
import type { UserProfile } from "@/types/api";

// AdminUsers 用户管理。
export default function AdminUsers() {
  const [items, setItems] = useState<UserProfile[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [keyword, setKeyword] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  // 封禁弹层状态
  const [banTarget, setBanTarget] = useState<UserProfile | null>(null);
  const [banReason, setBanReason] = useState<string>("");
  const [banUntil, setBanUntil] = useState<string>(""); // 空 = 永久
  const [banError, setBanError] = useState<string>("");

  useEffect(() => {
    setLoading(true);
    apiAdminUsers({ q: keyword || undefined })
      .then((r) => {
        setItems(r.items);
        setTotal(r.total);
      })
      .catch(() => setItems([]))
      .finally(() => setLoading(false));
  }, [keyword]);

  // 解封（直接执行）
  const unban = async (user: UserProfile) => {
    try {
      await apiAdminSetUserStatus(user.id, "active");
      setItems((prev) => prev.map((u) => (u.id === user.id ? { ...u, status: "active" } : u)));
    } catch (err) {
      setBanError(err instanceof ApiError ? err.message : "解封失败");
    }
  };

  // 确认封禁（原因 + 期限 → 写 ban_records）
  const confirmBan = async () => {
    if (!banTarget) {
      return;
    }
    setBanError("");
    if (!banReason.trim()) {
      setBanError("请填写封禁原因");
      return;
    }
    try {
      // until：空 = 永久；否则转 ISO8601（本地日期 → UTC）
      const until = banUntil
        ? new Date(`${banUntil}T23:59:59`).toISOString()
        : "";
      await apiAdminSetUserStatus(banTarget.id, "banned", banReason.trim(), until);
      setItems((prev) =>
        prev.map((u) => (u.id === banTarget.id ? { ...u, status: "banned" } : u)),
      );
      setBanTarget(null);
      setBanReason("");
      setBanUntil("");
    } catch (err) {
      setBanError(err instanceof ApiError ? err.message : "封禁失败");
    }
  };

  return (
    <div>
      <h1 className="font-display text-xl font-semibold text-ink">用户管理</h1>
      <p className="mt-0.5 text-xs text-ink-3">管理注册用户与访客权限 · 共 {total} 位用户</p>

      <div className="mt-4 flex items-center gap-3">
        <input
          type="text"
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          placeholder="搜索用户名、邮箱…"
          className="h-9 w-56 rounded-full border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
      </div>

      <div className="mt-4 overflow-x-auto rounded-lg border border-line bg-elevated">
        <table className="w-full min-w-[720px] text-left text-sm">
          <thead>
            <tr className="border-b border-line text-xs text-ink-3">
              <th className="px-4 py-3 font-normal">用户</th>
              <th className="px-4 py-3 font-normal">角色</th>
              <th className="px-4 py-3 font-normal">帖子数</th>
              <th className="px-4 py-3 font-normal">状态</th>
              <th className="px-4 py-3 font-normal">注册时间</th>
              <th className="px-4 py-3 font-normal">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {items.map((user) => (
              <tr key={user.id} className="hover:bg-muted/40">
                <td className="px-4 py-3">
                  <div className="flex items-center gap-3">
                    <span className="flex h-8 w-8 items-center justify-center rounded-full bg-muted text-xs text-ink-2">
                      {user.nickname.charAt(0)}
                    </span>
                    <div>
                      <p className="text-ink">{user.nickname}</p>
                      <p className="text-xs text-ink-3">@{user.username}</p>
                    </div>
                  </div>
                </td>
                <td className="px-4 py-3 text-ink-2">
                  {user.role === "admin" ? (
                    <span className="rounded-full bg-accent-soft px-2 py-0.5 text-xs text-glow">admin</span>
                  ) : (
                    "user"
                  )}
                </td>
                <td className="px-4 py-3 text-ink-3">{user.post_count}</td>
                <td className="px-4 py-3">
                  <span
                    className={`rounded-full px-2 py-0.5 text-xs ${
                      user.status === "banned"
                        ? "bg-like/10 text-like"
                        : "bg-accent-soft text-glow"
                    }`}
                  >
                    {user.status === "banned" ? "已封禁" : "正常"}
                  </span>
                </td>
                <td className="px-4 py-3 text-xs text-ink-3">{timeAgo(user.created_at)}</td>
                <td className="px-4 py-3">
                  {user.status === "banned" ? (
                    <button
                      type="button"
                      onClick={() => void unban(user)}
                      className="text-xs text-glow hover:underline"
                    >
                      解封
                    </button>
                  ) : (
                    <button
                      type="button"
                      onClick={() => {
                        setBanTarget(user);
                        setBanReason("");
                        setBanUntil("");
                        setBanError("");
                      }}
                      className="text-xs text-like hover:underline"
                    >
                      封禁
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!loading && items.length === 0 && (
          <p className="py-12 text-center text-sm text-ink-3">没有匹配的用户</p>
        )}
      </div>

      {/* 封禁弹层（M2：原因 + 永久/限时，写 ban_records） */}
      {banTarget && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-6"
          role="dialog"
          aria-modal="true"
          aria-label="封禁用户"
          onClick={() => setBanTarget(null)}
        >
          <div
            className="w-full max-w-[400px] rounded-xl border border-line bg-elevated p-6"
            onClick={(e) => e.stopPropagation()}
          >
            <h2 className="font-display text-lg font-semibold text-ink">
              封禁 {banTarget.nickname}
            </h2>
            <p className="mt-1 text-xs text-ink-3">封禁后该用户无法登录与发布内容</p>

            {/* 封禁原因（必填，写 ban_records） */}
            <label htmlFor="ban-reason" className="mt-4 block text-sm text-ink-2">
              封禁原因
            </label>
            <input
              id="ban-reason"
              type="text"
              value={banReason}
              onChange={(e) => setBanReason(e.target.value)}
              placeholder="如：发布违规内容"
              maxLength={200}
              className="mt-1.5 h-10 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />

            {/* 期限：空 = 永久；可选日期 */}
            <label htmlFor="ban-until" className="mt-4 block text-sm text-ink-2">
              解封时间（留空 = 永久封禁）
            </label>
            <input
              id="ban-until"
              type="date"
              value={banUntil}
              onChange={(e) => setBanUntil(e.target.value)}
              className="mt-1.5 h-10 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink focus:border-accent focus:outline-none"
            />

            {banError && <p className="mt-3 text-xs text-like">{banError}</p>}

            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setBanTarget(null)}
                className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink"
              >
                取消
              </button>
              <button
                type="button"
                onClick={() => void confirmBan()}
                className="rounded-full bg-like px-5 py-2 text-sm font-medium text-white hover:opacity-90"
              >
                确认封禁
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
