// src/components/compose/video-uploader.tsx
// 发帖中心 · 视频上传区（M2 视频发帖，需求 3.4）：
// 上传视频文件（mp4/mov/webm，≤200MB），支持预览与替换/删除。
// 设计稿：后台编辑·视频画板「window-timelapse.mp4 · 20s · 1080p · 18.4 MB / 更换视频 / 更换封面」。
"use client";

import { useRef, useState } from "react";

import { apiUploadMedia } from "@/lib/api";
import type { MediaDTO } from "@/types/api";

// 视频大小上限（与后端 MaxVideoSize 一致）
const MAX_VIDEO_SIZE = 200 << 20;

// formatBytes 字节数格式化为可读大小（MB）。
function formatBytes(bytes: number): string {
  return `${(bytes / (1 << 20)).toFixed(1)} MB`;
}

// VideoUploader 视频上传区（单选，支持预览/替换/删除）。
// 参数：value 已上传视频；onChange 变化回调。
export function VideoUploader({
  value,
  onChange,
}: {
  value: MediaDTO | null;
  onChange: (media: MediaDTO | null) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // 选择视频文件并上传
  const handleFile = async (file: File | undefined) => {
    if (!file) {
      return;
    }
    setError("");
    // 前端大小预检（≤200MB，避免浪费上传流量）
    if (file.size > MAX_VIDEO_SIZE) {
      setError("视频不能超过 200MB");
      return;
    }
    setUploading(true);
    try {
      const result = await apiUploadMedia(file);
      onChange({
        id: result.id,
        type: result.type,
        url: result.url,
        mime_type: result.mime_type,
        size_bytes: result.size_bytes,
        width: 0,
        height: 0,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "视频上传失败");
    } finally {
      setUploading(false);
    }
  };

  // 已上传：视频预览 + 替换/删除（设计稿「更换视频」）
  if (value) {
    return (
      <div className="rounded-lg border border-line bg-muted/40 p-4">
        <video src={value.url} controls preload="metadata" className="aspect-video w-full rounded-md bg-black" />
        <p className="mt-2 text-xs text-ink-3">
          已上传 · {formatBytes(value.size_bytes)}
        </p>
        <div className="mt-2 flex gap-2 text-xs">
          <button
            type="button"
            onClick={() => inputRef.current?.click()}
            className="rounded-full border border-line px-3 py-1 text-ink-2 hover:text-ink"
          >
            更换视频
          </button>
          <button
            type="button"
            onClick={() => onChange(null)}
            className="rounded-full border border-line px-3 py-1 text-like hover:opacity-80"
          >
            删除
          </button>
        </div>
        <input
          ref={inputRef}
          type="file"
          accept=".mp4,.mov,.webm,video/*"
          className="hidden"
          onChange={(e) => void handleFile(e.target.files?.[0])}
        />
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-dashed border-line p-6 text-center">
      <p className="text-sm text-ink-2">上传一段夜色（mp4/mov/webm，≤200MB）</p>
      <button
        type="button"
        onClick={() => inputRef.current?.click()}
        disabled={uploading}
        className="mt-4 rounded-full bg-accent px-6 py-2 text-sm text-on-accent disabled:opacity-60"
      >
        {uploading ? "上传中…" : "选择视频"}
      </button>
      <input
        ref={inputRef}
        type="file"
        accept=".mp4,.mov,.webm,video/*"
        className="hidden"
        onChange={(e) => void handleFile(e.target.files?.[0])}
      />
      {error && <p className="mt-3 text-xs text-like">{error}</p>}
    </div>
  );
}
