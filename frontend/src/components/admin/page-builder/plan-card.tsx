// src/components/admin/page-builder/plan-card.tsx
// AI 页面构建器 · 设计方案卡片：第一阶段（规划）输出以卡片形式展示，
// 最新待执行方案带「按此方案生成页面」按钮（历史方案只读展示）。
"use client";

import { Markdown } from "@/components/markdown";

// PlanCardProps 方案卡片参数。
interface PlanCardProps {
  content: string; // 方案 markdown 文本（流式期间持续追加）
  streaming: boolean; // 方案是否仍在生成中
  actionable: boolean; // 是否显示「按此方案生成」按钮（仅最新未执行的方案）
  executing: boolean; // 是否正在执行生成（按钮转禁用态）
  onExecute: () => void; // 点击「按此方案生成页面」
}

// PlanCard 设计方案卡片。
export function PlanCard({ content, streaming, actionable, executing, onExecute }: PlanCardProps) {
  return (
    <div className="max-w-[95%] rounded-lg border border-accent/40 bg-accent-soft/40 px-4 py-3">
      {/* 卡片头：标识 + 生成状态 */}
      <div className="mb-2 flex items-center gap-2">
        <span className="text-xs font-medium text-glow">✎ 设计方案</span>
        {streaming && <span className="text-[10px] text-ink-3">正在制定方案…</span>}
      </div>

      {/* 方案内容（markdown 渲染） */}
      <div className="text-sm text-ink-2">
        <Markdown content={content || "…"} />
      </div>

      {/* 执行入口：最新方案可确认执行，历史方案不显示 */}
      {actionable && !streaming && (
        <div className="mt-3 flex items-center gap-2 border-t border-accent/20 pt-3">
          <button
            type="button"
            onClick={onExecute}
            disabled={executing}
            className="rounded-full bg-accent px-5 py-1.5 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-50"
          >
            {executing ? "生成中…" : "按此方案生成页面"}
          </button>
          <span className="text-xs text-ink-3">或继续输入调整方案</span>
        </div>
      )}
    </div>
  );
}
