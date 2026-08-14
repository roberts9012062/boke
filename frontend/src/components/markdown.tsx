// src/components/markdown.tsx
// Markdown 渲染组件（插件商城「详情」README 等场景）：
// react-markdown + remark-gfm（表格/任务列表/删除线），默认不渲染原始 HTML（防 XSS），
// 元素映射到设计令牌工具类（双主题自适应）；code/pre 样式见 globals.css 的 .md-body 区块。
"use client";

import ReactMarkdown from "react-markdown";
import type { Components } from "react-markdown";
import remarkBreaks from "remark-breaks";
import remarkGfm from "remark-gfm";

// mdComponents 元素样式映射（纯配置；剥离 node 避免透传 DOM 警告）。
const mdComponents: Components = {
  h1: ({ node, ...props }) => (
    <h1 className="mb-2 mt-5 border-b border-line pb-1 font-display text-lg font-semibold text-ink" {...props} />
  ),
  h2: ({ node, ...props }) => (
    <h2 className="mb-2 mt-4 font-display text-base font-semibold text-ink" {...props} />
  ),
  h3: ({ node, ...props }) => <h3 className="mb-1.5 mt-3 text-sm font-semibold text-ink" {...props} />,
  h4: ({ node, ...props }) => <h4 className="mb-1 mt-2.5 text-sm font-medium text-ink-2" {...props} />,
  p: ({ node, ...props }) => <p className="my-2 text-sm leading-relaxed text-ink-2" {...props} />,
  ul: ({ node, ...props }) => <ul className="my-2 list-disc space-y-1 pl-5 text-sm text-ink-2" {...props} />,
  ol: ({ node, ...props }) => <ol className="my-2 list-decimal space-y-1 pl-5 text-sm text-ink-2" {...props} />,
  li: ({ node, ...props }) => <li className="leading-relaxed" {...props} />,
  blockquote: ({ node, ...props }) => (
    <blockquote className="my-2 border-l-2 border-accent-soft pl-3 text-sm text-ink-3" {...props} />
  ),
  a: ({ node, ...props }) => (
    <a
      className="text-glow underline-offset-2 hover:underline"
      target="_blank"
      rel="noreferrer noopener"
      {...props}
    />
  ),
  img: ({ node, ...props }) => <img className="my-2 max-w-full rounded-lg" {...props} />,
  hr: ({ node, ...props }) => <hr className="my-4 border-line" {...props} />,
  strong: ({ node, ...props }) => <strong className="font-semibold text-ink" {...props} />,
  table: ({ node, ...props }) => (
    <div className="my-2 overflow-x-auto">
      <table className="w-full border-collapse text-sm text-ink-2" {...props} />
    </div>
  ),
  thead: ({ node, ...props }) => <thead className="text-ink" {...props} />,
  tbody: ({ node, ...props }) => <tbody {...props} />,
  tr: ({ node, ...props }) => <tr className="border-b border-line last:border-b-0" {...props} />,
  th: ({ node, ...props }) => <th className="px-2 py-1.5 text-left text-xs font-semibold" {...props} />,
  td: ({ node, ...props }) => <td className="px-2 py-1.5 align-top" {...props} />,
  input: ({ node, ...props }) => <input className="mr-1.5 accent-[var(--yy-accent)]" {...props} />,
};

// Markdown 渲染内容（纯展示；原始 HTML 不渲染）。
// 说明：remarkBreaks 让单换行渲染为 <br>，兼容旧纯文本帖子的换行显示。
// 参数：content Markdown 原文。
export function Markdown({ content }: { content: string }) {
  return (
    <div className="md-body">
      <ReactMarkdown remarkPlugins={[remarkGfm, remarkBreaks]} components={mdComponents}>
        {content}
      </ReactMarkdown>
    </div>
  );
}
