// src/app/settings/security/page.tsx
// 账号安全页（设计稿 D/冷月/账号安全 1400×920）：
// 修改密码（校验当前密码 → 更新，其他设备旧会话失效）+ 绑定邮箱（已验证）
// + 登录设备（当前设备）+ 注销账号（设计稿《注销账号》弹层，MVP 占位）。
"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { SettingsLayout } from "@/components/settings-layout";
import { apiChangePassword, apiDeactivateAccount, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";

// 登录设备信息（当前设备：浏览器 UA 简化，设计稿「MacBook Pro · Chrome」）。
// 注意：SSR 期间 Node 自带 navigator（userAgent=Node.js/xx），与浏览器 UA 不一致——
// 若在渲染期直接计算会造成 hydration mismatch，故仅在客户端挂载后计算（见下方 useState/useEffect）。
function currentDevice(): string {
  if (typeof window === "undefined") {
    return "当前设备";
  }
  const ua = navigator.userAgent;
  const browser = ua.includes("Edg/")
    ? "Edge"
    : ua.includes("Chrome/")
      ? "Chrome"
      : ua.includes("Firefox/")
        ? "Firefox"
        : ua.includes("Safari/")
          ? "Safari"
          : "浏览器";
  const os = ua.includes("Windows")
    ? "Windows"
    : ua.includes("Mac OS")
      ? "macOS"
      : ua.includes("Android")
        ? "Android"
        : ua.includes("iPhone") || ua.includes("iPad")
          ? "iOS"
          : "未知系统";
  return `${os} · ${browser}`;
}

// SecurityPage 账号安全设置。
export default function SecurityPage() {
  const router = useRouter();
  const { user, logout } = useAuth();
  // 当前设备：SSR 与首帧输出占位，客户端挂载后计算真实 UA（避免 hydration mismatch）
  const [device, setDevice] = useState<string>("当前设备");
  useEffect(() => {
    setDevice(currentDevice());
  }, []);
  // 修改密码表单
  const [currentPassword, setCurrentPassword] = useState<string>("");
  const [newPassword, setNewPassword] = useState<string>("");
  const [confirmPassword, setConfirmPassword] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [success, setSuccess] = useState<string>("");
  const [saving, setSaving] = useState<boolean>(false);

  // 注销账号弹层（设计稿《注销账号》：输入「确认注销」→ 永久注销；需求 3.9 真实实现）
  const [showDelete, setShowDelete] = useState<boolean>(false);
  const [deleteConfirm, setDeleteConfirm] = useState<string>("");
  const [deleting, setDeleting] = useState<boolean>(false);

  // 更新密码（校验当前密码；成功后清空表单）
  const handleChangePassword = async () => {
    setError("");
    setSuccess("");
    if (!currentPassword || !newPassword || !confirmPassword) {
      setError("请填写完整密码信息");
      return;
    }
    if (newPassword !== confirmPassword) {
      setError("两次输入的新密码不一致");
      return;
    }
    setSaving(true);
    try {
      await apiChangePassword(currentPassword, newPassword);
      setSuccess("密码已更新，其他设备已退出登录");
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "密码更新失败");
    } finally {
      setSaving(false);
    }
  };

  // 永久注销（调用后端删除账号与全部数据 → 本地登出 → 跳登录页）
  const handleDeactivate = async () => {
    if (deleting) {
      return;
    }
    setDeleting(true);
    try {
      await apiDeactivateAccount();
      setShowDelete(false);
      await logout(); // 清空本地令牌与用户状态
      router.replace("/login");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "注销失败，请稍后再试");
      setDeleting(false);
    }
  };

  return (
    <SettingsLayout active="security">
      <h1 className="font-display text-xl font-semibold text-ink">账号安全</h1>
      <p className="mt-0.5 text-xs text-ink-3">管理密码、邮箱与登录设备</p>

      {/* 修改密码（设计稿：当前密码 / 新密码 / 确认新密码 / 更新密码） */}
      <section className="mt-5 rounded-lg border border-line bg-elevated p-6">
        <h2 className="text-sm font-semibold text-ink">修改密码</h2>
        <div className="mt-4 space-y-4">
          <div>
            <label htmlFor="current-password" className="mb-1.5 block text-sm text-ink-2">
              当前密码
            </label>
            <input
              id="current-password"
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              placeholder="••••••••"
              className="h-10 w-full max-w-sm rounded-lg border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </div>
          <div>
            <label htmlFor="new-password" className="mb-1.5 block text-sm text-ink-2">
              新密码
            </label>
            <input
              id="new-password"
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder="至少 8 位，含字母与数字"
              className="h-10 w-full max-w-sm rounded-lg border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </div>
          <div>
            <label htmlFor="confirm-password" className="mb-1.5 block text-sm text-ink-2">
              确认新密码
            </label>
            <input
              id="confirm-password"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="再次输入新密码"
              className="h-10 w-full max-w-sm rounded-lg border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </div>
          {error && (
            <p className="rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
              {error}
            </p>
          )}
          {success && (
            <p className="rounded-md bg-accent-soft px-3 py-2 text-sm text-glow" role="status">
              {success}
            </p>
          )}
          <button
            type="button"
            onClick={() => void handleChangePassword()}
            disabled={saving}
            className="rounded-full bg-accent px-6 py-2 text-sm font-medium text-on-accent hover:opacity-90 disabled:opacity-60"
          >
            更新密码
          </button>
        </div>
      </section>

      {/* 绑定邮箱（设计稿：已验证） */}
      <section className="mt-5 rounded-lg border border-line bg-elevated p-6">
        <h2 className="text-sm font-semibold text-ink">绑定邮箱</h2>
        <div className="mt-3 flex items-center gap-2 text-sm">
          <span className="text-ink">{user?.email ?? "—"}</span>
          <span className="rounded-full bg-accent-soft px-2 py-0.5 text-xs text-glow">已验证</span>
        </div>
      </section>

      {/* 登录设备（设计稿：MacBook Pro · Chrome / 杭州 · 当前设备；MVP 仅当前设备） */}
      <section className="mt-5 rounded-lg border border-line bg-elevated p-6">
        <h2 className="text-sm font-semibold text-ink">登录设备</h2>
        <div className="mt-3 flex items-center justify-between rounded-lg border border-line bg-muted/40 px-4 py-3 text-sm">
          <span className="text-ink">{device}</span>
          <span className="rounded-full bg-accent-soft px-2 py-0.5 text-xs text-glow">当前设备</span>
        </div>
        <p className="mt-2 text-xs text-ink-3">其他设备的会话管理将在后续版本开放（M4 规划）</p>
      </section>

      {/* 注销账号（设计稿《注销账号》弹层；需求 3.9） */}
      <section className="mt-5 rounded-lg border border-line bg-elevated p-6">
        <h2 className="text-sm font-semibold text-ink">注销账号</h2>
        <p className="mt-2 text-xs text-ink-3">注销后，你的帖子、草稿与关注关系将被永久删除，且无法恢复。</p>
        <button
          type="button"
          onClick={() => setShowDelete(true)}
          className="mt-3 rounded-full border border-like/30 px-5 py-2 text-sm text-like hover:bg-like/10"
        >
          注销账号
        </button>
      </section>

      {/* 注销确认弹层（设计稿：请输入「确认注销」→ 永久注销 / 再想想） */}
      {showDelete && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-6"
          role="dialog"
          aria-modal="true"
          aria-label="注销账号"
          onClick={() => setShowDelete(false)}
        >
          <div
            className="w-full max-w-[420px] rounded-xl border border-line bg-elevated p-6"
            onClick={(e) => e.stopPropagation()}
          >
            <h2 className="font-display text-lg font-semibold text-ink">注销账号</h2>
            <p className="mt-2 text-sm text-ink-2">
              注销后，你的帖子、草稿与关注关系将被永久删除，且无法恢复。
            </p>
            <p className="mt-3 text-xs text-ink-3">请输入「确认注销」以继续</p>
            <input
              type="text"
              value={deleteConfirm}
              onChange={(e) => setDeleteConfirm(e.target.value)}
              placeholder="确认注销"
              className="mt-2 h-10 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink focus:border-accent focus:outline-none"
            />
            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setShowDelete(false)}
                className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink"
              >
                再想想
              </button>
              <button
                type="button"
                disabled={deleteConfirm !== "确认注销" || deleting}
                onClick={() => void handleDeactivate()}
                className="rounded-full bg-like px-5 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
              >
                {deleting ? "注销中…" : "永久注销"}
              </button>
            </div>
          </div>
        </div>
      )}
    </SettingsLayout>
  );
}
