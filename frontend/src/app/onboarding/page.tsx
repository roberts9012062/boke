// src/app/onboarding/page.tsx
// 引导页（设计稿《引导》画板，D/M 双端）：
// 「把夜里的声音，写成可以慢慢读的句子」→ 书写/相遇/氛围 三卡片 → 开始使用 / 已有账号·登录。
"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";

// 引导卡片配置（设计稿文案）
const FEATURES = [
  { icon: "✍️", title: "书写", desc: "文字、影像、声音，同一条时间线" },
  { icon: "🌙", title: "相遇", desc: "关注同频的人，在话题里轻声对话" },
  { icon: "🎨", title: "氛围", desc: "冷月与薄雾，随心切换阅读的夜色" },
] as const;

// OnboardingPage 引导页（登录/注册前首次进入展示；不强制拦截，可跳过）。
export default function OnboardingPage() {
  const router = useRouter();

  return (
    <div className="flex min-h-screen flex-col bg-bg">
      {/* 品牌 */}
      <header className="flex items-center justify-between px-6 pt-6">
        <p className="font-display text-2xl font-bold tracking-wide text-ink">月言</p>
        {/* 移动端「跳过」（设计稿 M：跳过） */}
        <button
          type="button"
          onClick={() => router.push("/")}
          className="rounded-full border border-line px-4 py-1.5 text-xs text-ink-2 hover:text-ink md:hidden"
        >
          跳过
        </button>
      </header>

      <main className="mx-auto flex w-full max-w-[560px] flex-1 flex-col justify-center px-6 py-10">
        {/* 主文案（设计稿：把夜里的声音，写成可以慢慢读的句子） */}
        <p className="font-display text-2xl font-semibold leading-snug text-ink md:text-3xl">
          把夜里的声音，
          <br />
          写成可以慢慢读的句子
        </p>

        {/* 三卡片（设计稿：书写/相遇/氛围） */}
        <div className="mt-8 grid gap-4 sm:grid-cols-3">
          {FEATURES.map((f) => (
            <div key={f.title} className="rounded-lg border border-line bg-elevated p-5">
              <span className="text-2xl" aria-hidden>
                {f.icon}
              </span>
              <p className="mt-3 text-sm font-semibold text-ink">{f.title}</p>
              <p className="mt-1 text-xs leading-relaxed text-ink-2">{f.desc}</p>
            </div>
          ))}
        </div>

        {/* 操作（设计稿：开始使用 / 已有账号 · 登录） */}
        <div className="mt-10 flex flex-col items-center gap-3">
          <Link
            href="/register"
            className="w-full rounded-full bg-accent py-3 text-center text-sm font-medium text-on-accent transition-opacity hover:opacity-90"
          >
            开始使用
          </Link>
          <Link href="/login" className="text-sm text-ink-2 transition-colors hover:text-glow">
            已有账号 · 登录
          </Link>
          {/* 稍后再说（桌面，设计稿） */}
          <button
            type="button"
            onClick={() => router.push("/")}
            className="hidden text-xs text-ink-3 hover:text-ink md:block"
          >
            稍后再说
          </button>
        </div>
      </main>
    </div>
  );
}
