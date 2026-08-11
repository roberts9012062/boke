// src/app/admin-login/page.tsx
// 后台登录页（设计稿 D/冷月/后台登录 1400×900）：
// 月言 · 管理后台 + 在月光下照管每一篇未说完的话 + 仅限授权管理员进入
// + 管理员登录 + 账号/密码 + 进入后台。
"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";

import { ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { canAccessAdmin } from "@/lib/rbac";

// AdminLoginPage 后台登录（仅 admin 角色可进）。
export default function AdminLoginPage() {
  const router = useRouter();
  const { user, login } = useAuth();

  const [account, setAccount] = useState<string>("");
  const [password, setPassword] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [submitting, setSubmitting] = useState<boolean>(false);

  // 已登录 admin：直接进后台
  if (user && canAccessAdmin(user.role)) {
    router.replace("/admin");
    return null;
  }

  // 提交：登录成功后进 /admin（AdminLayout 会校验角色，非 admin 重定向回本页）
  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (submitting) {
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      await login(account.trim(), password);
      router.push("/admin");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "登录失败，请稍后再试");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <main className="flex min-h-screen items-center justify-center px-6">
      <section className="w-full max-w-[480px]">
        {/* 品牌（设计稿：月言 · 管理后台） */}
        <div className="flex items-center gap-3">
          <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-accent font-display text-lg font-bold text-on-accent">
            月
          </span>
          <div>
            <p className="font-display text-xl font-semibold text-ink">月言 · 管理后台</p>
            <p className="text-xs text-ink-3">在月光下照管每一篇未说完的话</p>
          </div>
        </div>
        <p className="mt-1 text-xs text-ink-3">内容审核 · 用户治理 · 站点配置 · 仅限授权管理员进入</p>

        {/* 登录表单（设计稿：管理员登录） */}
        <form onSubmit={handleSubmit} className="mt-8 rounded-lg border border-line bg-elevated p-6">
          <p className="font-display text-lg font-semibold text-ink">管理员登录</p>
          <p className="mt-0.5 text-xs text-ink-3">使用后台账号，建议开启二次验证</p>

          <div className="mt-5">
            <label htmlFor="admin-account" className="mb-1.5 block text-sm text-ink-2">
              账号
            </label>
            <input
              id="admin-account"
              type="text"
              value={account}
              onChange={(e) => setAccount(e.target.value)}
              placeholder="admin@yueyan.site"
              className="h-11 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </div>
          <div className="mt-4">
            <label htmlFor="admin-password" className="mb-1.5 block text-sm text-ink-2">
              密码
            </label>
            <input
              id="admin-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              className="h-11 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </div>

          {error && (
            <p className="mt-3 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
              {error}
            </p>
          )}

          <button
            type="submit"
            disabled={submitting}
            className="mt-6 h-11 w-full rounded-lg bg-accent text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
          >
            {submitting ? "登录中…" : "进入后台"}
          </button>
          <p className="mt-4 text-center text-xs text-ink-3">登录即表示你同意管理员行为准则</p>
        </form>

        <p className="mt-6 text-center">
          <Link href="/" className="text-sm text-ink-2 hover:text-ink">
            ← 返回站点
          </Link>
        </p>
      </section>
    </main>
  );
}
