// src/app/about/page.tsx
// 关于页（设计稿《关于》画板 #155/#156；走查纠偏补）。
// 静态页：品牌信息 + 统计 + 协议入口 + 联系我们，文案与设计稿一致。
import Link from "next/link";

// AboutPage 关于页。
export default function AboutPage() {
  return (
    <main className="mx-auto max-w-[560px] px-6 py-14 text-center">
      {/* 品牌（设计稿：月 / 月言 / 在月光下慢慢写，不必急着被人听懂） */}
      <span className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-accent font-display text-2xl font-bold text-on-accent">
        月
      </span>
      <h1 className="mt-4 font-display text-2xl font-semibold text-ink">月言</h1>
      <p className="mt-2 text-sm text-ink-2">在月光下慢慢写，不必急着被人听懂。</p>

      {/* 统计（设计稿：v2.4.0 版本 / 2024 创立 / 12.4k 创作者） */}
      <div className="mt-8 grid grid-cols-3 gap-3">
        {[
          { label: "版本", value: "v2.4.0" },
          { label: "创立", value: "2024" },
          { label: "创作者", value: "12.4k" },
        ].map((s) => (
          <div key={s.label} className="rounded-lg border border-line bg-elevated p-4">
            <p className="font-display text-lg font-semibold text-ink">{s.value}</p>
            <p className="mt-0.5 text-xs text-ink-3">{s.label}</p>
          </div>
        ))}
      </div>

      {/* 协议入口 + 联系我们（设计稿：用户协议/隐私政策/联系我们） */}
      <div className="mt-8 flex flex-wrap justify-center gap-3 text-sm">
        <Link href="/terms" className="text-glow hover:underline">
          用户协议
        </Link>
        <span className="text-ink-3">·</span>
        <Link href="/privacy" className="text-glow hover:underline">
          隐私政策
        </Link>
        <span className="text-ink-3">·</span>
        <a href="mailto:contact@yueyan.site" className="text-glow hover:underline">
          联系我们
        </a>
      </div>

      <p className="mt-10 text-xs text-ink-3">© 2026 月言 Yueyan · 保留所有权利</p>
    </main>
  );
}
