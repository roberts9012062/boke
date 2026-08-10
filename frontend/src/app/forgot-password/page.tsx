// src/app/forgot-password/page.tsx
// 忘记密码页（设计稿《忘记密码》《邮件已发送》画板）：
// 返回登录 → 月言 → 重置密码 → 输入注册邮箱，我们会发送重置链接 → 发送重置链接
// 提交后切换「邮件已发送」态：重置链接已发至 xxx / 请在 30 分钟内完成验证。 / 打开邮箱 / 没收到？
"use client";

import Link from "next/link";
import { useState } from "react";

import { apiForgotPassword, ApiError } from "@/lib/api";

// ForgotPasswordPage 忘记密码（M2 找回密码；未配置 SMTP 时后端降级日志输出链接，开发可验证）。
export default function ForgotPasswordPage() {
  const [email, setEmail] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [submitting, setSubmitting] = useState<boolean>(false);
  const [sent, setSent] = useState<boolean>(false); // 邮件已发送态

  // 发送重置链接
  const handleSubmit = async () => {
    setError("");
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) {
      setError("请输入有效的邮箱地址");
      return;
    }
    setSubmitting(true);
    try {
      await apiForgotPassword(email.trim());
      setSent(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "发送失败，请稍后再试");
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

        {/* 品牌（设计稿 D 端有「月言」；M 端 390 无品牌 → 移动端隐藏） */}
        <p className="mt-8 hidden text-center font-display text-3xl font-bold tracking-wide text-ink md:block">
          月言
        </p>

        {!sent ? (
          <>
            {/* 表单（设计稿：重置密码 / 输入注册邮箱，我们会发送重置链接） */}
            <h1 className="mt-8 text-center font-display text-xl font-semibold text-ink">重置密码</h1>
            <p className="mt-2 text-center text-sm text-ink-2">
              输入注册邮箱，我们会发送重置链接
            </p>

            <label htmlFor="email" className="mt-8 block text-sm text-ink-2">
              邮箱
            </label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  void handleSubmit();
                }
              }}
              placeholder="name@example.com"
              className="mt-2 h-12 w-full rounded-lg border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />

            {error && <p className="mt-3 text-sm text-like">{error}</p>}

            <button
              type="button"
              onClick={() => void handleSubmit()}
              disabled={submitting}
              className="mt-6 w-full rounded-full bg-accent py-3 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
            >
              {submitting ? "发送中…" : "发送重置链接"}
            </button>

            {/* 收不到提示（设计稿：请检查垃圾箱，或 60 秒后重新发送。链接 30 分钟内有效。） */}
            <p className="mt-6 text-center text-xs leading-relaxed text-ink-3">
              收不到？请检查垃圾箱，或 60 秒后重新发送。
              <br />
              链接 30 分钟内有效。
            </p>
          </>
        ) : (
          <>
            {/* 邮件已发送态（设计稿《邮件已发送》） */}
            <span className="mt-8 text-center text-4xl" aria-hidden>
              📮
            </span>
            <h1 className="mt-4 text-center font-display text-xl font-semibold text-ink">邮件已发送</h1>
            <p className="mt-2 text-center text-sm leading-relaxed text-ink-2">
              重置链接已发至 {email}
              <br />
              请在 30 分钟内完成验证。
            </p>
            <a
              href={`https://${email.split("@")[1] ?? "mail.example.com"}`}
              target="_blank"
              rel="noreferrer"
              className="mt-6 w-full rounded-full bg-accent py-3 text-center text-sm font-medium text-on-accent transition-opacity hover:opacity-90"
            >
              打开邮箱
            </a>
            <p className="mt-6 text-center text-xs text-ink-3">
              没收到？检查垃圾邮件，或 60 秒后重新发送。
            </p>
          </>
        )}
      </main>
    </div>
  );
}
