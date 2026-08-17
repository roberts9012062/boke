"use client";

// src/components/doc-sidebar.tsx
// 插件参考手册侧栏（client）：分组导航 + 当前页高亮 + 移动端下拉切换。
// 导航数据由服务端 layout 经 props 注入（源 lib/plugin-reference.ts 含 node:fs，client 不导入）。
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";

// DocNavItem / DocNavGroup 类型对齐 lib/plugin-reference.ts（客户端仅用类型，重新声明避免连带导入）。
interface DocNavItem {
  slug: string;
  title: string;
}
interface DocNavGroup {
  label: string;
  items: DocNavItem[];
}

// slugToHref 导航项 → URL（index 为手册首页；与 lib 侧 docHref 一致）。
function slugToHref(slug: string): string {
  return slug === "index" ? "/plugin-reference" : `/plugin-reference/${slug}`;
}

// DocSidebar 手册侧栏。
// 参数：groups 分组导航（props 注入）；variant 桌面侧栏 / 移动端下拉。
export function DocSidebar({ groups }: { groups: DocNavGroup[] }) {
  const pathname = usePathname();
  // 当前 slug：/plugin-reference → index；/plugin-reference/xxx → xxx
  const currentSlug = pathname === "/plugin-reference" ? "index" : (pathname.split("/")[2] ?? "");

  return (
    <nav aria-label="手册导航">
      {groups.map((group) => (
        <div key={group.label} className="mb-4">
          <p className="mb-1.5 px-3 text-[11px] font-semibold uppercase tracking-wider text-ink-3">
            {group.label}
          </p>
          {group.items.map((item) => {
            const active = currentSlug === item.slug;
            return (
              <Link
                key={item.slug}
                href={slugToHref(item.slug)}
                aria-current={active ? "page" : undefined}
                className={`mb-0.5 block rounded-lg px-3 py-1.5 text-sm transition-colors ${
                  active
                    ? "bg-accent-soft font-medium text-glow"
                    : "text-ink-2 hover:bg-muted hover:text-ink"
                }`}
              >
                {item.title}
              </Link>
            );
          })}
        </div>
      ))}
    </nav>
  );
}

// DocSidebarMobile 移动端手册切换（下拉选择当前页；桌面侧栏的替代形态）。
export function DocSidebarMobile({ groups }: { groups: DocNavGroup[] }) {
  const pathname = usePathname();
  const router = useRouter();
  const currentSlug = pathname === "/plugin-reference" ? "index" : (pathname.split("/")[2] ?? "");
  return (
    <select
      aria-label="切换手册页"
      value={currentSlug}
      onChange={(e) => router.push(slugToHref(e.target.value))}
      className="w-full rounded-lg border border-line bg-elevated px-3 py-2 text-sm text-ink"
    >
      {groups.map((group) => (
        <optgroup key={group.label} label={group.label}>
          {group.items.map((item) => (
            <option key={item.slug} value={item.slug}>
              {item.title}
            </option>
          ))}
        </optgroup>
      ))}
    </select>
  );
}
