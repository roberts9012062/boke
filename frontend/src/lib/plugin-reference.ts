// src/lib/plugin-reference.ts
// 插件参考手册网页化（服务端工具）：分组导航数据 + markdown 文件读取 + 站内链接改写。
// 文档源：仓库根 docs/（手写 markdown 唯一事实源，网页按需读取渲染——不做复制同步）。
// 仅服务端组件导入（依赖 node:fs）；客户端侧栏组件经 props 接收导航数据，不导入本文件。
import { readFile } from "node:fs/promises";
import path from "node:path";

// DocNavItem 手册导航项（slug 同时是 URL 段与文件名主干）。
export interface DocNavItem {
  slug: string; // URL 段（/plugin-reference/{slug}；index 为 /plugin-reference）
  title: string; // 侧栏展示名
}

// DocNavGroup 手册导航分组（对齐 docs/plugin-reference/index.md 的「文档导航」表）。
export interface DocNavGroup {
  label: string; // 分组名（概念 / 目录参考 / 开发手册）
  items: DocNavItem[]; // 组内页面
}

// DOC_NAV 手册侧栏导航（分组与顺序与手册首页一致；development 为配套教程入口）。
export const DOC_NAV: DocNavGroup[] = [
  {
    label: "总览",
    items: [{ slug: "index", title: "架构总览" }],
  },
  {
    label: "概念",
    items: [
      { slug: "concepts", title: "核心概念" },
      { slug: "lifecycle", title: "插件生命周期" },
    ],
  },
  {
    label: "目录参考",
    items: [
      { slug: "hooks-catalog", title: "钩子目录" },
      { slug: "seams", title: "能力服务接缝" },
    ],
  },
  {
    label: "开发手册",
    items: [
      { slug: "frontend-extensions", title: "前端扩展" },
      { slug: "capabilities-and-security", title: "能力与安全" },
      { slug: "packaging", title: "打包与分发" },
      { slug: "development", title: "开发教程（plugin-development）" },
    ],
  },
];

// DOC_FILES slug → docs 目录下文件路径（白名单；越界 slug 一律 404，防路径注入）。
const DOC_FILES: Record<string, string> = {
  index: "plugin-reference/index.md",
  concepts: "plugin-reference/concepts.md",
  lifecycle: "plugin-reference/lifecycle.md",
  "hooks-catalog": "plugin-reference/hooks-catalog.md",
  seams: "plugin-reference/seams.md",
  "frontend-extensions": "plugin-reference/frontend-extensions.md",
  "capabilities-and-security": "plugin-reference/capabilities-and-security.md",
  packaging: "plugin-reference/packaging.md",
  development: "plugin-development.md",
};

// docsRoot docs 目录绝对路径（next dev/start 的 cwd 均为 frontend/，docs 在仓库根）。
function docsRoot(): string {
  return path.join(process.cwd(), "..", "docs");
}

// slugToDocHref 手册内链接改写目标（markdown 相对链接 → 站内路由）。
const docHrefByFile: Record<string, string> = {
  "index.md": "/plugin-reference",
  "concepts.md": "/plugin-reference/concepts",
  "lifecycle.md": "/plugin-reference/lifecycle",
  "hooks-catalog.md": "/plugin-reference/hooks-catalog",
  "seams.md": "/plugin-reference/seams",
  "frontend-extensions.md": "/plugin-reference/frontend-extensions",
  "capabilities-and-security.md": "/plugin-reference/capabilities-and-security",
  "packaging.md": "/plugin-reference/packaging",
  "../plugin-development.md": "/plugin-reference/development",
};

// transformDocLinks 改写 markdown 站内链接为网页路由（纯函数）。
// 规则：手册间相对链接（含 ../plugin-development.md）→ /plugin-reference/{slug}；
//       页内锚点（#xxx）依赖 heading id，react-markdown 未注入——统一去锚点指向目标页；
//       外部链接（http/https）与其他路径（源码引用等）保持原样。
export function transformDocLinks(md: string): string {
  return md.replace(/\]\(([^)]+)\)/g, (matched: string, href: string): string => {
    if (/^https?:\/\//.test(href)) {
      return matched; // 外部链接不动
    }
    const [pathPart] = href.split("#");
    const target = docHrefByFile[pathPart];
    if (target) {
      return `](${target})`;
    }
    if (href.startsWith("#")) {
      return "](/plugin-reference)"; // 纯锚点无 id 可依，回首页
    }
    return matched; // 其他（不识别的路径）保持原样
  });
}

// readDocPage 读取手册页 markdown（slug 白名单校验 + 站内链接改写；不存在返回 null → 404）。
export async function readDocPage(slug: string): Promise<string | null> {
  const file = DOC_FILES[slug];
  if (!file) {
    return null;
  }
  try {
    const raw = await readFile(path.join(docsRoot(), file), "utf8");
    return transformDocLinks(raw);
  } catch {
    return null;
  }
}

// readAiPrompt 读取「AI 开发提示词」原文（docs/plugin-reference/ai-prompt.md，
// 不在 DOC_FILES 白名单内——不作为手册章节路由访问，仅供首页复制卡片读取）。
// 复制场景需要原文（含 markdown 标记），不做链接改写；失败返回 null（卡片隐藏）。
export async function readAiPrompt(): Promise<string | null> {
  try {
    return await readFile(path.join(docsRoot(), "plugin-reference", "ai-prompt.md"), "utf8");
  } catch {
    return null;
  }
}

// docNeighbors 相邻页（上一篇/下一篇；slug 展平顺序推导，首尾为 null）。
export function docNeighbors(slug: string): { prev: DocNavItem | null; next: DocNavItem | null } {
  const flat = DOC_NAV.flatMap((g) => g.items);
  const idx = flat.findIndex((item) => item.slug === slug);
  if (idx < 0) {
    return { prev: null, next: null };
  }
  return {
    prev: idx > 0 ? flat[idx - 1] : null,
    next: idx < flat.length - 1 ? flat[idx + 1] : null,
  };
}

// docTitle 手册页标题（导航数据派生；未知 slug 返回占位）。
export function docTitle(slug: string): string {
  const hit = DOC_NAV.flatMap((g) => g.items).find((item) => item.slug === slug);
  return hit ? hit.title : "插件参考手册";
}

// docHref 导航项 → URL（index 为手册首页，其余为 /plugin-reference/{slug}）。
export function docHref(slug: string): string {
  return slug === "index" ? "/plugin-reference" : `/plugin-reference/${slug}`;
}
