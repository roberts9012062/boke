// src/components/admin/page-builder/code-drawer.tsx
// AI 页面构建器代码抽屉：查看/手动微调当前页面 HTML 源码（textarea），
// 「应用」后更新预览（AI 生成代码允许人工修正后再保存）。
"use client";

import { useEffect, useState } from "react";

// CodeDrawerProps 代码抽屉参数。
interface CodeDrawerProps {
  open: boolean; // 是否显示
  html: string; // 当前页面代码
  onApply: (html: string) => void; // 应用修改
  onClose: () => void; // 关闭
}

// CodeDrawer 代码查看/编辑抽屉（覆盖在预览区上方的面板）。
export function CodeDrawer({ open, html, onApply, onClose }: CodeDrawerProps) {
  const [draft, setDraft] = useState<string>("");

  // 打开时同步当前代码为草稿（后续外部更新以最后一次打开为准）
  useEffect(() => {
    if (open) {
      setDraft(html);
    }
  }, [open, html]);

  if (!open) {
    return null;
  }

  return (
    <div className="absolute inset-0 z-20 flex flex-col rounded-lg border border-line bg-elevated">
      <div className="flex items-center justify-between border-b border-line px-4 py-2.5">
        <span className="text-sm font-medium text-ink">页面源码（可手动微调）</span>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => onApply(draft)}
            className="rounded-full bg-accent px-4 py-1.5 text-xs font-medium text-on-accent transition-opacity hover:opacity-90"
          >
            应用到预览
          </button>
          <button
            type="button"
            onClick={onClose}
            className="rounded-full border border-line px-4 py-1.5 text-xs text-ink-2 transition-colors hover:text-ink"
          >
            关闭
          </button>
        </div>
      </div>
      <textarea
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        spellCheck={false}
        className="flex-1 resize-none rounded-b-lg bg-muted p-4 font-mono text-xs leading-relaxed text-ink focus:outline-none"
      />
    </div>
  );
}
