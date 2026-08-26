// src/components/compose/image-uploader.tsx
// 发帖中心 · 图片上传区（需求 3.4）：多图上传（≤9 张）、预览、删除、
// 前端 canvas 压缩（≤1MB/张）后上传到 /api/v1/media。
"use client";

import { useRef, useState } from "react";

import { apiUploadMedia } from "@/lib/api";
import { compressImage } from "@/lib/image-compress";
import type { MediaDTO } from "@/types/api";

// 图片数量上限（需求 3.4：≤9 张）
const MAX_IMAGES = 9;

// isImageFile 图片类型判定（纯函数）。
function isImageFile(file: File): boolean {
  return /^image\/(jpeg|png|gif|webp)$/i.test(file.type || "");
}

// collectDroppedFiles 从拖放事件收集图片文件（含文件夹：webkitGetAsEntry 递归遍历）。
async function collectDroppedFiles(dt: DataTransfer): Promise<File[]> {
  const out: File[] = [];
  const entries = Array.from(dt.items ?? [])
    .map((it) => (it.webkitGetAsEntry ? it.webkitGetAsEntry() : null))
    .filter((e): e is FileSystemEntry => e !== null);
  if (entries.length === 0) {
    return Array.from(dt.files ?? []).filter(isImageFile);
  }
  const walk = async (entry: FileSystemEntry): Promise<void> => {
    if (entry.isFile) {
      const file = await new Promise<File>((resolve, reject) =>
        (entry as FileSystemFileEntry).file(resolve, reject),
      );
      if (isImageFile(file)) {
        out.push(file);
      }
      return;
    }
    const reader = (entry as FileSystemDirectoryEntry).createReader();
    const children = await new Promise<FileSystemEntry[]>((resolve, reject) =>
      reader.readEntries(resolve, reject),
    );
    for (const child of children) {
      await walk(child);
    }
  };
  for (const entry of entries) {
    await walk(entry);
  }
  return out;
}

// ImageUploader 图片上传区。
// 参数：value 已上传媒体；onChange 变化回调（上传中状态由父组件处理）。
export function ImageUploader({
  value,
  onChange,
}: {
  value: MediaDTO[];
  onChange: (medias: MediaDTO[]) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [dragOver, setDragOver] = useState<boolean>(false);

  // 选择/拖入文件后逐个压缩上传（统一入口：点击选择与拖拽共用）
  const handleFilesFromList = async (files: File[]) => {
    if (files.length === 0) {
      return;
    }
    // 数量上限校验（含已有）
    const remain = MAX_IMAGES - value.length;
    if (files.length > remain) {
      setError(`最多上传 ${MAX_IMAGES} 张图片`);
      return;
    }
    setError("");
    setUploading(true);
    try {
      const uploaded: MediaDTO[] = [];
      for (const file of files) {
        const compressed = await compressImage(file);
        const result = await apiUploadMedia(compressed);
        uploaded.push({
          id: result.id,
          type: result.type,
          url: result.url,
          mime_type: result.mime_type,
          size_bytes: result.size_bytes,
          width: 0,
          height: 0,
        });
      }
      onChange([...value, ...uploaded]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "图片上传失败");
    } finally {
      setUploading(false);
      if (inputRef.current) {
        inputRef.current.value = "";
      }
    }
  };

  // 删除单张图片
  const removeImage = (mediaID: number) => {
    onChange(value.filter((m) => m.id !== mediaID));
  };

  return (
    <div>
      {/* 图片网格（已上传预览） */}
      {value.length > 0 && (
        <div className="grid grid-cols-3 gap-2">
          {value.map((media) => (
            <div key={media.id} className="group relative aspect-square overflow-hidden rounded-lg">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={media.url} alt="已上传图片" className="h-full w-full object-cover" />
              {/* 删除按钮（hover 显示） */}
              <button
                type="button"
                onClick={() => removeImage(media.id)}
                className="absolute right-1 top-1 hidden h-6 w-6 items-center justify-center rounded-full bg-black/60 text-xs text-white group-hover:flex"
                aria-label="删除图片"
              >
                ✕
              </button>
            </div>
          ))}
        </div>
      )}

      {/* 上传入口（未达上限时显示：点击选择 + 拖拽文件/文件夹） */}
      {value.length < MAX_IMAGES && (
        <div
          onDragOver={(e) => {
            e.preventDefault();
            setDragOver(true);
          }}
          onDragLeave={() => setDragOver(false)}
          onDrop={async (e) => {
            e.preventDefault();
            setDragOver(false);
            const files = await collectDroppedFiles(e.dataTransfer);
            await handleFilesFromList(files);
          }}
        >
          <button
            type="button"
            onClick={() => inputRef.current?.click()}
            disabled={uploading}
            className={`mt-3 flex h-24 w-full items-center justify-center rounded-lg border border-dashed text-sm transition-colors disabled:opacity-60 ${
              dragOver ? "border-accent bg-accent-soft text-glow" : "border-line text-ink-3 hover:border-accent hover:text-ink-2"
            }`}
          >
            {uploading ? "上传中…" : dragOver ? "松开即可上传（支持多张/文件夹）" : `+ 添加图片（${value.length}/${MAX_IMAGES}，可拖拽）`}
          </button>
        </div>
      )}
      {/* 隐藏文件选择 */}
      <input
        ref={inputRef}
        type="file"
        accept="image/*"
        multiple
        className="hidden"
        onChange={(e) => void handleFilesFromList(Array.from(e.target.files ?? []))}
      />
      {error && <p className="mt-2 text-xs text-like">{error}</p>}
    </div>
  );
}
