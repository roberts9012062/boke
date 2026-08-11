// src/components/admin/ai/ai-summary.tsx
// 后台编辑页 · AI 生成摘要（M4-AI 场景 1）：调用 post.summary 任务 → 写入
// seo_meta.summary → 展示；失败展示错误（如未配 API Key 提示去 AI 设置）。
"use client";

import { useState } from "react";

import { apiAiGenSummary } from "@/lib/api-ai";
import { ApiError } from "@/lib/api";

// AiSummary 生成并展示帖子摘要。
// 参数：postId 帖子 ID；initial 已存摘要（首次加载回填）。
export function AiSummary({ postId, initial }: { postId: number; initial: string }) {
  const [summary, setSummary] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  // 生成摘要（成功回填；失败提示原因，未配 Key 引导去 AI 设置）
  const handleGen = async () => {
    setBusy(true);
    setError("");
    try {
      const r = await apiAiGenSummary(postId);
      setSummary(r.summary);
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : "生成失败";
      setError(msg);
      if (msg.includes("AI 设置") || msg.includes("API Key")) {
        setError(`${msg}（可前往侧栏「AI 设置」配置）`);
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="rounded-lg border border-line/70 bg-muted/40 p-3">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-ink-2">AI 摘要</span>
        <button
          type="button"
          onClick={() => void handleGen()}
          disabled={busy}
          className="rounded-full bg-accent/10 px-3 py-1 text-xs text-glow hover:bg-accent/20 disabled:opacity-50"
        >
          {busy ? "生成中…" : summary ? "重新生成" : "AI 生成摘要"}
        </button>
      </div>
      {summary && <p className="mt-2 text-xs leading-relaxed text-ink-2">{summary}</p>}
      {!summary && !error && <p className="mt-2 text-xs text-ink-3">摘要写入 SEO 元数据，供搜索与分享使用</p>}
      {error && <p className="mt-2 text-xs text-like">{error}</p>}
    </div>
  );
}
