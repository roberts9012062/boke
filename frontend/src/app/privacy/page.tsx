// src/app/privacy/page.tsx
// 隐私政策（设计稿《隐私政策》画板 #161/#162；走查纠偏补，修复注册页链接 404）。
// 静态页：文案与设计稿逐字一致。
import Link from "next/link";

// 政策章节（设计稿五章）
const SECTIONS: readonly { title: string; body: string }[] = [
  {
    title: "一、我们收集的信息",
    body: "为提供服务，我们可能收集账号信息、设备信息、使用日志与你主动发布的内容。访客可在不登录的情况下浏览公开内容并发表开放评论。",
  },
  {
    title: "二、信息如何使用",
    body: "用于账号安全、内容分发、社区治理与产品改进。我们不会出售你的个人信息。仅在法律要求或取得同意时向第三方披露。",
  },
  {
    title: "三、Cookie 与本地存储",
    body: "我们使用 Cookie 与本地存储以保持登录状态、记住主题偏好（冷月/薄雾）并统计匿名流量。你可以在浏览器中清除它们。",
  },
  {
    title: "四、数据安全与保留",
    body: "我们采取合理的技术与管理措施保护数据。账号注销后，相关个人数据将在合理期限内删除或匿名化，法律另有规定的除外。",
  },
  {
    title: "五、你的权利",
    body: "你可访问、更正或删除个人资料，也可申请导出数据。如有疑问，请通过「关于」页的联系方式与我们沟通。",
  },
];

// PrivacyPage 隐私政策页。
export default function PrivacyPage() {
  return (
    <main className="mx-auto max-w-[720px] px-6 py-10">
      <h1 className="font-display text-2xl font-semibold text-ink">隐私政策</h1>
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
