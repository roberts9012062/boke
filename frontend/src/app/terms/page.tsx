// src/app/terms/page.tsx
// 用户协议（设计稿《用户协议》画板 #153/#154；走查纠偏补，修复注册页链接 404）。
// 静态页：文案与设计稿逐字一致。
import Link from "next/link";

// 协议章节（设计稿五章）
const SECTIONS: readonly { title: string; body: string }[] = [
  {
    title: "一、服务说明",
    body: "月言（以下简称「本站」）是一个以文字、图片、音频与视频分享为主的创作社区。使用本站即表示你同意本协议全部条款。",
  },
  {
    title: "二、账号与安全",
    body: "你应对账号下的全部行为负责。请妥善保管密码，发现异常请立即修改密码并联系我们。未成年人使用本站应取得监护人同意。",
  },
  {
    title: "三、内容规范",
    body: "禁止发布违法、侵权、骚扰、广告垃圾或侵犯他人隐私的内容。本站有权对违规内容进行删除、限流或封禁处理。",
  },
  {
    title: "四、知识产权",
    body: "你保留原创内容的知识产权，同时授予本站在展示、传播与改进服务范围内的非独占许可。引用他人内容须注明来源。",
  },
  {
    title: "五、免责与终止",
    body: "因不可抗力或第三方原因导致的服务中断，本站不承担责任。严重违反协议的，我们可终止向你提供服务。",
  },
];

// TermsPage 用户协议页。
export default function TermsPage() {
  return (
    <main className="mx-auto max-w-[720px] px-6 py-10">
      <h1 className="font-display text-2xl font-semibold text-ink">用户协议</h1>
      <p className="mt-1 text-xs text-ink-3">最近更新：2026 年 6 月 1 日</p>
      <div className="mt-6 space-y-6">
        {SECTIONS.map((s) => (
          <section key={s.title}>
            <h2 className="text-base font-semibold text-ink">{s.title}</h2>
            <p className="mt-2 text-sm leading-relaxed text-ink-2">{s.body}</p>
          </section>
        ))}
      </div>
      <p className="mt-10 text-sm">
        <Link href="/" className="text-glow hover:underline">
          ← 返回首页
        </Link>
      </p>
    </main>
  );
}
