// src/components/compose/image-uploader.tsx
// 发帖中心 · 图片上传区（需求 3.4）：多图上传（≤9 张）、预览、删除、
// 前端 canvas 压缩（≤1MB/张）后上传到 /api/v1/media。
"use client";

import { useRef, useState } from "react";

import { apiUploadMedia } from "@/lib/api";
import type { MediaDTO } from "@/types/api";

// 图片数量上限（需求 3.4：≤9 张）
const MAX_IMAGES = 9;
// 压缩后目标大小（1MB）
const TARGET_SIZE = 1 << 20;

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

  // 压缩图片：canvas 重绘，质量逐步降低直至 ≤1MB。
  // 返回：压缩后的 File。
  async function compressImage(file: File): Promise<File> {
    // 已是小文件直接返回
    if (file.size <= TARGET_SIZE) {
      return file;
    }
    const bitmap = await createImageBitmap(file);
    // 等比缩放：长边不超过 1600px
    const maxSide = 1600;
    let { width, height } = bitmap;
    if (width > maxSide || height > maxSide) {
      const ratio = Math.min(maxSide / width, maxSide / height);
      width = Math.round(width * ratio);
      height = Math.round(height * ratio);
    }
    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext("2d");
    if (!ctx) {
      return file;
    }
    ctx.drawImage(bitmap, 0, 0, width, height);
    bitmap.close();

    // 质量 0.8 起，逐级降低直到达标（webp 压缩率高）
    const type = "image/webp";
    let quality = 0.8;
    let blob: Blob | null = null;
    for (let i = 0; i < 5; i++) {
      blob = await new Promise<Blob | null>((resolve) =>
        canvas.toBlob(resolve, type, quality),
      );
      if (blob && blob.size <= TARGET_SIZE) {
        break;
      }
      quality -= 0.15;
    }
    if (!blob) {
      return file;
    }
    return new File([blob], file.name.replace(/\.\w+$/, ".webp"), { type });
  }

  // 选择文件后逐个压缩上传
  const handleFiles = async (files: FileList | null) => {
    if (!files || files.length === 0) {
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
      for (const file of Array.from(files)) {
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

      {/* 上传入口（未达上限时显示） */}
      {value.length < MAX_IMAGES && (
        <button
          type="button"
          onClick={() => inputRef.current?.click()}
          disabled={uploading}
          className="mt-3 flex h-24 w-full items-center justify-center rounded-lg border border-dashed border-line text-sm text-ink-3 transition-colors hover:border-accent hover:text-ink-2 disabled:opacity-60"
        >
          {uploading ? "上传中…" : `+ 添加图片（${value.length}/${MAX_IMAGES}）`}
        </button>
      )}
      {/* 隐藏文件选择 */}
      <input
        ref={inputRef}
        type="file"
        accept="image/*"
        multiple
        className="hidden"
        onChange={(e) => void handleFiles(e.target.files)}
      />
      {error && <p className="mt-2 text-xs text-like">{error}</p>}
    </div>
  );
}
