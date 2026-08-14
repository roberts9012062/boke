// src/components/admin/image-setting-field.tsx
// 插件设置「图片」字段（M5：默认 OG 图等）：
//   展示当前图预览 + 值；「点击上传」→ 16:9 裁剪弹窗（拖动/缩放）→ 裁剪输出 1200×630
//   → 上传媒体库 → 回填 URL；「从图片库选择」→ 媒体库弹窗点选回填。附尺寸建议与「恢复默认」。
"use client";

import { useRef, useState } from "react";

import { apiUploadMedia, ApiError, type PluginSettingField } from "@/lib/api";

import { ImageCropDialog } from "./image-crop-dialog";
import { MediaPicker } from "./media-picker";

// SUGGESTION 尺寸建议文案（OG 分享标准）。
const SUGGESTION = "建议 1200×630（社交分享标准比例），上传后可拖动定位与缩放裁剪";

// ImageSettingField 图片设置项。
// 参数：field schema（key/label/default）；value 当前值；onChange 回填 URL。
export function ImageSettingField({
  field,
  value,
  onChange,
}: {
  field: PluginSettingField;
  value: string;
  onChange: (url: string) => void;
}) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [picked, setPicked] = useState<File | null>(null); // 待裁剪文件（非空=打开裁剪弹窗）
  const [pickerOpen, setPickerOpen] = useState<boolean>(false); // 图片库选择弹窗
  const [uploading, setUploading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // handleFileChange 选中文件后打开裁剪弹窗（同一文件再次选择时清空 value 重新触发）。
  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) {
      return;
    }
    setError("");
    setPicked(file);
  };

  // handleCropped 裁剪确认后上传媒体库并回填 URL。
  const handleCropped = async (cropped: File) => {
    setPicked(null);
    setUploading(true);
    setError("");
    try {
      const result = await apiUploadMedia(cropped);
      onChange(result.url);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "上传失败，请重试");
    } finally {
      setUploading(false);
    }
  };

  return (
    <div className="w-full">
      {/* 尺寸建议提示（对用户始终可见） */}
      <p className="text-[11px] text-ink-3">{SUGGESTION}</p>

      {/* 当前图预览 + 操作 */}
      <div className="mt-2 flex items-start gap-4">
        {value ? (
          <div className="w-44 shrink-0 overflow-hidden rounded-lg border border-line bg-muted">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={value} alt={field.label} className="aspect-video w-full object-cover" />
          </div>
        ) : (
          <div className="flex aspect-video w-44 shrink-0 items-center justify-center rounded-lg border border-dashed border-line bg-muted text-xs text-ink-3">
            暂无图片
          </div>
        )}
        <div className="flex min-w-0 flex-1 flex-col items-start gap-2">
          {/* 当前值（相对路径或媒体 URL） */}
          <p className="max-w-full truncate text-xs text-ink-3" title={value}>
            {value || "未设置"}
          </p>
          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={() => fileInputRef.current?.click()}
              disabled={uploading}
              className="rounded-full bg-accent px-4 py-1.5 text-xs font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
            >
              {uploading ? "上传中…" : "点击上传"}
            </button>
            <button
              type="button"
              onClick={() => setPickerOpen(true)}
              disabled={uploading}
              className="rounded-full border border-line px-4 py-1.5 text-xs text-ink-2 transition-colors hover:text-ink disabled:opacity-60"
            >
              从图片库选择
            </button>
            {field.default && value !== field.default && (
              <button
                type="button"
                onClick={() => onChange(field.default ?? "")}
                disabled={uploading}
                className="rounded-full border border-line px-4 py-1.5 text-xs text-ink-2 transition-colors hover:text-ink disabled:opacity-60"
              >
                恢复默认
              </button>
            )}
          </div>
          {error && (
            <p className="rounded-md bg-like/10 px-3 py-1.5 text-xs text-like" role="alert">
              {error}
            </p>
          )}
        </div>
      </div>

      {/* 隐藏文件选择（限制图片类型） */}
      <input
        ref={fileInputRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={(e) => void handleFileChange(e)}
      />

      {/* 裁剪弹窗（选中文件后打开） */}
      {picked && (
        <ImageCropDialog
          file={picked}
          onCancel={() => setPicked(null)}
          onConfirm={(cropped) => void handleCropped(cropped)}
        />
      )}

      {/* 图片库选择弹窗（点选回填 URL） */}
      <MediaPicker
        open={pickerOpen}
        onClose={() => setPickerOpen(false)}
        onSelect={(url) => {
          setPickerOpen(false);
          setError("");
          onChange(url);
        }}
      />
    </div>
  );
}
