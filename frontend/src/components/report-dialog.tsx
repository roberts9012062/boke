// src/components/report-dialog.tsx
// 举报弹层（设计稿《举报》《举报成功》画板）：
// 举报内容 → 请选择举报原因，我们会尽快处理。→ 6 原因单选 + 补充说明（可选）→ 取消/提交举报
// 提交成功 → 已收到举报 / 感谢你的反馈。我们会在 24 小时内完成审核。 / 知道了。
"use client";

import { useState } from "react";

import { apiSubmitReport, ApiError } from "@/lib/api";

// 举报原因（设计稿预置选项）
const REASONS = ["垃圾广告", "骚扰辱骂", "色情低俗", "违法违规", "侵犯版权", "其他"] as const;

// ReportDialogProps 弹层参数。
interface ReportDialogProps {
  targetType: "post" | "comment" | "user"; // 举报对象类型
  targetId: number; // 对象 ID
  onClose: () => void; // 关闭回调
}

// ReportDialog 举报弹层（含提交成功态，设计稿《举报成功》）。
export function ReportDialog({ targetType, targetId, onClose }: ReportDialogProps) {
  const [reason, setReason] = useState<string>("");
  const [detail, setDetail] = useState<string>("");
  const [submitting, setSubmitting] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [done, setDone] = useState<boolean>(false);

  // 提交举报
  const handleSubmit = async () => {
    if (!reason) {
      setError("请选择举报原因");
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      await apiSubmitReport(targetType, targetId, reason, detail.trim());
      setDone(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "提交失败，请稍后再试");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-6"
      role="dialog"
      aria-modal="true"
      aria-label="举报内容"
      onClick={onClose}
    >
      <div
        className="w-full max-w-[420px] rounded-xl border border-line bg-elevated p-6"
        onClick={(e) => e.stopPropagation()}
      >
        {!done ? (
          <>
            {/* 表单（设计稿：举报内容 / 请选择举报原因，我们会尽快处理。） */}
            <h2 className="font-display text-lg font-semibold text-ink">举报内容</h2>
            <p className="mt-1 text-xs text-ink-3">请选择举报原因，我们会尽快处理。</p>

            {/* 原因选项（单选，设计稿 6 项） */}
            <div className="mt-4 grid grid-cols-2 gap-2">
              {REASONS.map((r) => (
                <button
                  key={r}
                  type="button"
                  onClick={() => setReason(r)}
                  aria-pressed={reason === r}
                  className={`rounded-lg border px-3 py-2 text-sm transition-colors ${
                    reason === r
                      ? "border-accent bg-accent-soft text-glow"
                      : "border-line text-ink-2 hover:text-ink"
                  }`}
                >
                  {r}
                </button>
              ))}
            </div>

            {/* 补充说明（可选） */}
            <textarea
              value={detail}
              onChange={(e) => setDetail(e.target.value)}
              maxLength={500}
              rows={3}
              placeholder="补充说明（可选）…"
              className="mt-4 w-full resize-none rounded-lg border border-line bg-muted px-3 py-2 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />

            {error && <p className="mt-3 text-xs text-like">{error}</p>}

            {/* 操作（设计稿：取消 / 提交举报） */}
            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={onClose}
                className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink"
              >
                取消
              </button>
              <button
                type="button"
                onClick={() => void handleSubmit()}
                disabled={submitting}
                className="rounded-full bg-accent px-5 py-2 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
              >
                {submitting ? "提交中…" : "提交举报"}
              </button>
            </div>
          </>
        ) : (
          <>
            {/* 成功态（设计稿《举报成功》：已收到举报 / 24 小时内完成审核 / 知道了） */}
            <div className="py-6 text-center">
              <span className="text-3xl" aria-hidden>
                ✅
              </span>
              <h2 className="mt-3 font-display text-lg font-semibold text-ink">已收到举报</h2>
              <p className="mt-2 text-sm leading-relaxed text-ink-2">
                感谢你的反馈。我们会在 24 小时内完成审核。
              </p>
            </div>
            <div className="flex justify-center">
              <button
                type="button"
                onClick={onClose}
                className="rounded-full bg-accent px-8 py-2 text-sm font-medium text-on-accent transition-opacity hover:opacity-90"
              >
                知道了
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
