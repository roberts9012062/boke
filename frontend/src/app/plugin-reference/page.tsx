// src/app/plugin-reference/page.tsx
// 手册首页（架构总览）：顶部「AI 开发提示词」复制卡片 + 渲染 docs/plugin-reference/index.md。
import { notFound } from "next/navigation";

import { Markdown } from "@/components/markdown";
import { AiPromptCard } from "@/components/plugin-reference/ai-prompt-card";
import { readAiPrompt, readDocPage } from "@/lib/plugin-reference";

// 首页元信息。
export const metadata = {
  title: "架构总览 · 插件参考手册",
};

// 手册首页（服务端读取 markdown 渲染；提示词原文传入复制卡片）。
export default async function PluginReferenceIndexPage() {
  const [md, aiPrompt] = await Promise.all([readDocPage("index"), readAiPrompt()]);
  if (md === null) {
    notFound();
  }
  return (
    <div>
      {/* AI 开发提示词（一键复制）：开发前先发给 AI，使其读完整手册、明确能力边界 */}
      {aiPrompt !== null && <AiPromptCard prompt={aiPrompt} />}
      <article className="rounded-xl border border-line bg-elevated p-5 sm:p-8">
        <Markdown content={md} />
      </article>
    </div>
  );
}
