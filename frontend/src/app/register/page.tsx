// src/app/register/page.tsx
// 注册页（设计稿 D/冷月/注册 1400×900 + M/冷月/注册 390）：
// 返回首页 → 月言 → 创建账号 → 留下你的月色痕迹 → 昵称/邮箱/密码 → 注册
// → 注册即表示同意 服务条款 与 隐私政策 → 已有账号？登录
"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";

import { ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";

// RegisterPage 注册页（双端共用布局：居中卡片，移动端自适应）。
export default function RegisterPage() {
  const router = useRouter();
  const { register } = useAuth();

  // 表单状态（受控输入）
  const [nickname, setNickname] = useState<string>("");
  const [email, setEmail] = useState<string>("");
  const [password, setPassword] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [submitting, setSubmitting] = useState<boolean>(false);

  // 客户端校验（需求 3.1：昵称 1-20 字符、邮箱格式、密码 ≥8 位含字母数字）
  const validate = (): string => {
    const name = nickname.trim();
    if (!name || name.length > 20) {
      return "昵称需为 1-20 个字符";
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) {
      return "邮箱格式不正确";
    }
    if (password.length < 8 || !/[a-zA-Z]/.test(password) || !/\d/.test(password)) {
      return "密码至少 8 位，且包含字母与数字";
    }
    return "";
  };

  // 提交注册
  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (submitting) {
      return;
    }
    const validationError = validate();
    if (validationError) {
      setError(validationError);
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      await register(nickname.trim(), email.trim(), password);
      router.push("/"); // 注册成功（即已登录）回首页
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "注册失败，请稍后再试");
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

      {/* 注册卡片 */}
      <section className="w-full max-w-[420px]">
        {/* 品牌 + 欢迎语（设计稿文案） */}
        <h1 className="text-center font-display text-4xl font-bold text-ink">月言</h1>
        <p className="mt-2 text-center font-display text-xl text-ink">创建账号</p>
        <p className="mt-1 text-center text-sm text-ink-2">留下你的月色痕迹</p>

        {/* 注册表单 */}
        <form onSubmit={handleSubmit} className="mt-8 space-y-4">
          {/* 昵称（设计稿占位 你的名字） */}
          <div>
            <label htmlFor="nickname" className="mb-1.5 block text-sm text-ink-2">
              昵称
            </label>
            <input
              id="nickname"
              type="text"
              value={nickname}
              onChange={(e) => setNickname(e.target.value)}
              placeholder="你的名字"
              autoComplete="nickname"
              className="h-11 w-full rounded-lg border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </div>

          {/* 邮箱（设计稿占位 you@moon.light） */}
          <div>
            <label htmlFor="email" className="mb-1.5 block text-sm text-ink-2">
              邮箱
            </label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@moon.light"
              autoComplete="email"
              className="h-11 w-full rounded-lg border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </div>

          {/* 密码（设计稿提示 至少 8 位） */}
          <div>
            <label htmlFor="password" className="mb-1.5 block text-sm text-ink-2">
              密码
            </label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="至少 8 位"
              autoComplete="new-password"
              className="h-11 w-full rounded-lg border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </div>

          {/* 错误提示 */}
          {error && (
            <p className="rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
              {error}
            </p>
          )}

          {/* 注册按钮 */}
          <button
            type="submit"
            disabled={submitting}
            className="h-11 w-full rounded-lg bg-accent text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
          >
            {submitting ? "注册中…" : "注册"}
          </button>
        </form>

        {/* 协议声明（设计稿文案） */}
        <p className="mt-4 text-center text-xs text-ink-3">
          注册即表示同意
          <Link href="/terms" className="mx-0.5 text-glow hover:underline">
            服务条款
          </Link>
          与
          <Link href="/privacy" className="mx-0.5 text-glow hover:underline">
            隐私政策
          </Link>
        </p>

        {/* 登录入口（设计稿文案） */}
        <p className="mt-6 text-center text-sm text-ink-2">
          已有账号？
          <Link href="/login" className="ml-1 text-glow hover:underline">
            登录
          </Link>
        </p>
      </section>
    </main>
  );
}
