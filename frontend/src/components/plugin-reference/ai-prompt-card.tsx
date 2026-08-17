// src/components/plugin-reference/ai-prompt-card.tsx
// 手册首页顶部「AI 开发提示词」卡片：使用说明 + 提示词预览（可展开）+ 一键复制。
// 用途：用户让 AI 开发插件前，先把提示词发给 AI——使其先完整阅读开发手册、
// 明确能力边界（八枚举）与禁止事项，再动手写代码。
"use client";

import { useState } from "react";

// AiPromptCardProps 卡片参数。
interface AiPromptCardProps {
  prompt: string; // 提示词全文（markdown 原文，复制即此文本）
}

// AiPromptCard AI 开发提示词卡片。
export function AiPromptCard({ prompt }: AiPromptCardProps) {
  const [expanded, setExpanded] = useState<boolean>(false);
  const [copied, setCopied] = useState<boolean>(false);

  // 一键复制（剪贴板 API；localhost/https 可用，失败提示手动复制）
  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(prompt);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  // 预览截取（折叠时只显示开头几行，完整内容展开查看）
  const preview = expanded ? prompt : `${prompt.split("\n").slice(0, 6).join("\n")}\n…`;

  return (
    <section className="mb-6 rounded-xl border border-accent/40 bg-accent-soft/30 p-5 sm:p-6">
      {/* 标题 + 一键复制 */}
      <div className="flex flex-wrap items-center gap-3">
        <h2 className="font-display text-lg font-semibold text-ink">AI 开发提示词</h2>
        <button
          type="button"
          onClick={() => void handleCopy()}
          className="ml-auto rounded-full bg-accent px-5 py-1.5 text-sm font-medium text-on-accent transition-opacity hover:opacity-90"
        >
          {copied ? "已复制 ✓" : "一键复制"}
        </button>
      </div>

      {/* 使用说明 */}
      <p className="mt-2 text-xs leading-relaxed text-ink-2">
        让 AI（任意对话模型）帮你开发插件前，先把这段提示词完整发给它：
        AI 会先通读本开发手册、明确插件体系「哪些能用、哪些不能用」的边界，再开始写代码——避免凭猜测发明不存在的接口。
      </p>

      {/* 提示词预览（等宽原文；折叠显示开头，展开看全文） */}
      <pre className="mt-3 max-h-[420px] overflow-y-auto whitespace-pre-wrap rounded-lg border border-line bg-elevated p-4 font-mono text-xs leading-relaxed text-ink-2">
        {preview}
      </pre>
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className="mt-2 text-xs text-glow hover:underline"
      >
        {expanded ? "收起预览 ▴" : "展开完整提示词 ▾"}
      </button>
    </section>
  );
}
