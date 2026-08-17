// src/app/plugin-reference/layout.tsx
// 插件参考手册布局（公开访问，无需登录）：顶栏 + 左侧分组导航 + 内容区。
// markdown 源为仓库根 docs/plugin-reference/（服务端按需读取，改文档刷新即生效）。
import Link from "next/link";

import { DocSidebar, DocSidebarMobile } from "@/components/doc-sidebar";
import { DOC_NAV } from "@/lib/plugin-reference";

// 布局元信息（浏览器标签页标题后缀）。
export const metadata = {
  title: "插件参考手册 · 月言",
  description: "月言博客插件系统参考手册：架构、钩子目录、能力接缝、前端扩展与安全边界",
};

// 手册布局（服务端组件：导航数据直传，client 侧栏不接触 node:fs）。
export default function PluginReferenceLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen bg-bg">
      {/* 顶栏：站点标识 + 返回 + GitHub 文档源 */}
      <header className="sticky top-0 z-20 border-b border-line bg-elevated/95 backdrop-blur-md">
        <div className="mx-auto flex h-13 max-w-6xl items-center gap-3 px-4 py-2.5">
          <Link href="/plugin-reference" className="flex items-center gap-2">
            <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-accent font-display text-xs font-bold text-on-accent">
              月
            </span>
            <span className="font-display text-sm font-semibold text-ink">月言 · 插件参考手册</span>
          </Link>
          <Link
            href="/"
            className="ml-auto text-xs text-ink-2 transition-colors hover:text-ink"
          >
            ← 返回站点
          </Link>
        </div>
      </header>

      <div className="mx-auto flex max-w-6xl gap-6 px-4 py-6">
        {/* 桌面侧栏（分组导航 + 高亮当前页） */}
        <aside className="sticky top-[61px] hidden h-fit w-56 shrink-0 lg:block">
          <div className="rounded-xl border border-line bg-elevated p-2">
            <DocSidebar groups={DOC_NAV} />
          </div>
        </aside>

        {/* 内容区：移动端带下拉切换 */}
        <main className="min-w-0 flex-1">
          <div className="mb-4 lg:hidden">
            <DocSidebarMobile groups={DOC_NAV} />
          </div>
          {children}
        </main>
      </div>
    </div>
  );
}
