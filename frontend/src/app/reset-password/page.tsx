// src/app/reset-password/page.tsx
// 重置密码页（设计稿《重置成功》画板 + 新密码表单）：
// 返回登录 → 月言 → 新密码 + 确认 → 提交 → 密码已更新 / 你可以使用新密码登录月言。 / 前往登录
// 说明：token 从 URL ?token= 读取（邮件重置链接带入）。
"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";

import { apiResetPassword, ApiError } from "@/lib/api";

// ResetInner 重置密码主体（Suspense 内使用 useSearchParams）。
function ResetInner() {
  const searchParams = useSearchParams();
  const token = searchParams.get("token") ?? "";
  const [password, setPassword] = useState<string>("");
  const [confirm, setConfirm] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [submitting, setSubmitting] = useState<boolean>(false);
  const [done, setDone] = useState<boolean>(false); // 成功态（设计稿《重置成功》）

  // 提交新密码
  const handleSubmit = async () => {
    setError("");
    if (!token) {
      setError("重置链接无效，请重新发起找回密码");
      return;
    }
    if (password.length < 8 || !/[a-zA-Z]/.test(password) || !/\d/.test(password)) {
      setError("密码至少 8 位，且包含字母与数字");
      return;
    }
    if (password !== confirm) {
      setError("两次输入的密码不一致");
      return;
    }
    setSubmitting(true);
    try {
      await apiResetPassword(token, password);
      setDone(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "重置失败，请稍后再试");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="flex min-h-screen flex-col bg-bg">
      <main className="mx-auto flex w-full max-w-[420px] flex-1 flex-col justify-center px-6 py-10">
        {/* 返回登录（设计稿） */}
        <Link href="/login" className="text-sm text-ink-3 transition-colors hover:text-ink">
          ← 返回登录
        </Link>

        {/* 品牌（设计稿 D 端有「月言」；M 端无 → 移动端隐藏） */}
        <p className="mt-8 hidden text-center font-display text-3xl font-bold tracking-wide text-ink md:block">月言</p>

        {!done ? (
          <>
            {/* 新密码表单 */}
            <h1 className="mt-8 text-center font-display text-xl font-semibold text-ink">设置新密码</h1>
            <p className="mt-2 text-center text-sm text-ink-2">至少 8 位，且包含字母与数字</p>

            <label htmlFor="new-password" className="mt-8 block text-sm text-ink-2">
              新密码
            </label>
            <input
              id="new-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              className="mt-2 h-12 w-full rounded-lg border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />

            <label htmlFor="confirm-password" className="mt-4 block text-sm text-ink-2">
              确认新密码
            </label>
            <input
              id="confirm-password"
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  void handleSubmit();
                }
              }}
              placeholder="••••••••"
              className="mt-2 h-12 w-full rounded-lg border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />

            {error && <p className="mt-3 text-sm text-like">{error}</p>}

            <button
              type="button"
              onClick={() => void handleSubmit()}
              disabled={submitting}
              className="mt-6 w-full rounded-full bg-accent py-3 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
            >
              {submitting ? "提交中…" : "确认重置"}
            </button>
          </>
        ) : (
          <>
            {/* 成功态（设计稿《重置成功》：密码已更新 / 前往登录 / 其他设备已退出） */}
            <span className="mt-8 text-center text-4xl" aria-hidden>
              ✅
            </span>
            <h1 className="mt-4 text-center font-display text-xl font-semibold text-ink">密码已更新</h1>
            <p className="mt-2 text-center text-sm leading-relaxed text-ink-2">
              你可以使用新密码登录月言。
            </p>
            <Link
              href="/login"
              className="mt-6 w-full rounded-full bg-accent py-3 text-center text-sm font-medium text-on-accent transition-opacity hover:opacity-90"
            >
              前往登录
            </Link>
            <p className="mt-6 text-center text-xs text-ink-3">
              为了账号安全，其他设备上的登录会话已退出。
            </p>
          </>
        )}
      </main>
    </div>
  );
}

// ResetPasswordPage 重置密码页（Suspense 包裹 useSearchParams，Next 静态预渲染要求）。
export default function ResetPasswordPage() {
  return (
    <Suspense fallback={null}>
      <ResetInner />
    </Suspense>
  );
}
