// frontend/src/components/compose/gallery-style-picker.tsx
// 图片展示风格选择器：发帖图片区下方，点击选择风格并实时预览效果。
"use client";

import { GALLERY_STYLES, ImageGallery, type GalleryStyle } from "@/components/image-gallery";
import type { MediaDTO } from "@/types/api";

// GalleryStylePicker 风格选择器（含预览）。
export function GalleryStylePicker({
  value,
  onChange,
  images,
}: {
  value: GalleryStyle;
  onChange: (style: GalleryStyle) => void;
  images: MediaDTO[];
}) {
  return (
    <div className="mt-4">
      <p className="text-sm font-medium text-ink">展示效果</p>
      <p className="mt-0.5 text-xs text-ink-3">选择图片在帖子中的展示风格（点击即可预览）</p>

      {/* 风格按钮组 */}
      <div className="mt-2.5 flex flex-wrap gap-2">
        {GALLERY_STYLES.map((s) => (
          <button
            key={s.key || "grid"}
            type="button"
            onClick={() => onChange(s.key)}
            className={
              "rounded-full border px-4 py-1.5 text-sm transition-colors " +
              (value === s.key
                ? "border-accent bg-accent-soft font-medium text-glow"
                : "border-line text-ink-2 hover:border-accent/60 hover:text-ink")
            }
          >
            <span className="mr-1.5">{s.icon}</span>
            {s.label}
          </button>
        ))}
      </div>

      {/* 预览区（有图才渲染；无图提示） */}
      {images.length > 0 ? (
        <div className="mt-3 rounded-xl border border-line bg-muted/40 p-3">
          <p className="mb-2 text-xs text-ink-3">预览 · {GALLERY_STYLES.find((s) => s.key === value)?.label ?? "网格"}</p>
          <ImageGallery media={images} style={value} />
        </div>
      ) : (
        <p className="mt-3 rounded-lg border border-dashed border-line px-3 py-5 text-center text-xs text-ink-3">
          上传图片后可预览展示效果
        </p>
      )}
    </div>
  );
}

