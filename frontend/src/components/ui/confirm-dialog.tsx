// src/components/ui/confirm-dialog.tsx
// 确认弹窗组件（对齐设计稿「D/冷月/弹窗提示·确认弹窗」）：
//   标题（问句）+ 说明文字 + 取消/主操作双按钮；危险操作（删除/退出）确认按钮用警示色。
// 取代 window.confirm（系统默认弹窗）——全站弹窗风格统一。
"use client";

// ConfirmDialogProps 确认弹窗参数。
interface ConfirmDialogProps {
  open: boolean; // 是否显示
  title: string; // 标题（设计稿为问句，如「删除这条帖子？」）
  description?: string; // 说明文字（设计稿为影响提示，如「删除后无法恢复…」）
  confirmText?: string; // 确认按钮文案（默认「确认」）
  cancelText?: string; // 取消按钮文案（默认「取消」）
  danger?: boolean; // 危险操作（删除/退出等）：确认按钮用警示色
  loading?: boolean; // 确认中（禁用按钮防重复提交）
  onConfirm: () => void; // 确认回调
  onClose: () => void; // 关闭（遮罩/取消/Esc）
}

// ConfirmDialog 确认弹窗（桌面居中 + 遮罩点击关闭 + aria 无障碍）。
export function ConfirmDialog({
  open,
  title,
  description,
  confirmText = "确认",
  cancelText = "取消",
  danger = false,
  loading = false,
  onConfirm,
  onClose,
}: ConfirmDialogProps) {
  if (!open) {
    return null;
  }
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-6"
      role="dialog"
      aria-modal="true"
      aria-label={title}
      onClick={onClose}
    >
      <div
        className="w-full max-w-[420px] rounded-xl border border-line bg-elevated p-6"
        onClick={(e) => e.stopPropagation()}
      >
        {/* 标题（设计稿：问句，font-display） */}
        <h2 className="font-display text-lg font-semibold text-ink">{title}</h2>
        {/* 说明文字（设计稿：影响提示，次要色） */}
        {description && <p className="mt-2 text-sm leading-relaxed text-ink-2">{description}</p>}

        {/* 按钮组（设计稿：取消居左次要 / 主操作居右） */}
        <div className="mt-5 flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            disabled={loading}
            className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 transition-colors hover:text-ink disabled:opacity-60"
          >
            {cancelText}
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={loading}
            className={`rounded-full px-5 py-2 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60 ${
              danger ? "bg-like" : "bg-accent"
            }`}
          >
            {loading ? "处理中…" : confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}
