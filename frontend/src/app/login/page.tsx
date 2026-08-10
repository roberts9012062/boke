// src/app/login/page.tsx
// 登录页（设计稿 D/冷月/登录 1400×900 + M/冷月/登录 390）：
// 返回首页 → 月言 → 欢迎回来 → 在月光下继续未说完的话 → 邮箱/密码 → 登录
// → 或 → 以访客继续浏览 → 没有账号？注册
"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";

import { ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";

// LoginPage 登录页（双端共用布局：居中卡片，移动端自适应）。
export default function LoginPage() {
  const router = useRouter();
  const { login } = useAuth();

  // 表单状态（受控输入）
  const [account, setAccount] = useState<string>("");
  const [password, setPassword] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [submitting, setSubmitting] = useState<boolean>(false);

  // 提交登录
  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (submitting) {
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      await login(account.trim(), password);
      router.push("/"); // 登录成功回首页
    } catch (err) {
      // 展示后端提示文案（如「邮箱或密码不正确」/限流提示）
      setError(err instanceof ApiError ? err.message : "登录失败，请稍后再试");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <main className="flex min-h-screen flex-col items-center justify-center px-6 py-10">
      {/* 返回首页（设计稿左上角） */}
      <Link
        href="/"
        className="absolute left-6 top-6 text-sm text-ink-2 transition-colors hover:text-ink"
      >
        ← 返回首页
      </Link>

      {/* 登录卡片 */}
      <section className="w-full max-w-[420px]">
        {/* 品牌 + 欢迎语（设计稿文案） */}
        <h1 className="text-center font-display text-4xl font-bold text-ink">月言</h1>
        <p className="mt-2 text-center font-display text-xl text-ink">欢迎回来</p>
        <p className="mt-1 text-center text-sm text-ink-2">在月光下继续未说完的话</p>

        {/* 登录表单 */}
        <form onSubmit={handleSubmit} className="mt-8 space-y-4">
          {/* 邮箱（设计稿占位 you@moon.light） */}
          <div>
            <label htmlFor="account" className="mb-1.5 block text-sm text-ink-2">
              邮箱
            </label>
            <input
              id="account"
              type="text"
              value={account}
              onChange={(e) => setAccount(e.target.value)}
              placeholder="you@moon.light"
              autoComplete="username"
              className="h-11 w-full rounded-lg border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </div>

          {/* 密码 + 忘记密码（M2 激活，占位链接） */}
          <div>
            <div className="mb-1.5 flex items-center justify-between">
              <label htmlFor="password" className="text-sm text-ink-2">
                密码
              </label>
              <Link href="/forgot-password" className="text-xs text-glow hover:underline">
                忘记密码？
              </Link>
            </div>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              autoComplete="current-password"
              className="h-11 w-full rounded-lg border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </div>

          {/* 错误提示（统一文案展示区） */}
          {error && (
            <p className="rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
              {error}
            </p>
          )}

          {/* 登录按钮 */}
          <button
            type="submit"
            disabled={submitting}
            className="h-11 w-full rounded-lg bg-accent text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
          >
            {submitting ? "登录中…" : "登录"}
          </button>
        </form>

        {/* 分隔线 + 访客入口（设计稿文案） */}
        <div className="mt-6 flex items-center gap-3 text-xs text-ink-3">
          <span className="h-px flex-1 bg-line" aria-hidden />
          <span>或</span>
          <span className="h-px flex-1 bg-line" aria-hidden />
        </div>
        <Link
          href="/"
          className="mt-4 block text-center text-sm text-ink-2 transition-colors hover:text-ink"
        >
          以访客继续浏览
        </Link>

        {/* 注册入口（设计稿文案） */}
        <p className="mt-6 text-center text-sm text-ink-2">
          没有账号？
          <Link href="/register" className="ml-1 text-glow hover:underline">
            注册
          </Link>
        </p>
      </section>
    </main>
  );
}
