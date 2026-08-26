// src/components/compose/visibility-select.tsx
// 可见性选择（设计稿 D/冷月/可见性 弹层：公开/仅关注者/仅自己）。
// 从 compose 页抽出（说说/文章两种表单共用；受控组件，状态由父级管理）。
"use client";

import { useState } from "react";

import { VISIBILITY_OPTIONS } from "@/components/compose/config";

// 可见性类型（与后端字典同步）
type Visibility = "public" | "followers" | "private";

// VisibilitySelect 可见性选择器（受控：value 当前值；onChange 选择回调）。
export function VisibilitySelect({
  value,
  onChange,
}: {
  value: Visibility;
  onChange: (value: Visibility) => void;
}) {
  const [open, setOpen] = useState<boolean>(false);

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-1.5 rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 transition-colors hover:text-ink"
      >
        {value === "public" ? "公开" : value === "followers" ? "仅关注者" : "仅自己"}
        <span className="text-xs text-ink-3" aria-hidden>
          ▾
        </span>
      </button>

      {/* 谁可以看 弹层（设计稿 D/冷月/可见性） */}
      {open && (
        <div className="absolute left-0 top-11 z-20 w-72 rounded-lg border border-line bg-elevated p-4 shadow-lg">
          <p className="font-display text-base font-semibold text-ink">谁可以看</p>
          <div className="mt-3 space-y-2">
            {VISIBILITY_OPTIONS.map((opt) => (
              <button
                key={opt.key}
                type="button"
                onClick={() => onChange(opt.key)}
                className={`w-full rounded-md border px-3 py-2.5 text-left transition-colors ${
                  value === opt.key ? "border-accent bg-accent-soft" : "border-line hover:bg-muted"
                }`}
              >
                <p className={`text-sm ${value === opt.key ? "text-glow" : "text-ink"}`}>{opt.label}</p>
                <p className="mt-0.5 text-xs text-ink-3">{opt.desc}</p>
              </button>
            ))}
          </div>
          <div className="mt-4 flex justify-end gap-2">
            <button
              type="button"
              onClick={() => setOpen(false)}
              className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink"
            >
              取消
            </button>
            <button
              type="button"
              onClick={() => setOpen(false)}
              className="rounded-full bg-accent px-5 py-1.5 text-sm text-on-accent"
            >
              完成
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
