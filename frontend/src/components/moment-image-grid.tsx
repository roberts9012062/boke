// src/components/moment-image-grid.tsx
// 说说图片网格（时间线列表形态）：按图片数量自适应布局。
// 1 张：显示原图（不裁剪，等比缩放限高）；2-4 张：横向一排方形小图；
// 5-9 张：三列九宫格（超出 9 张截断，角标提示总数）。
"use client";

import Link from "next/link";

import type { MediaDTO } from "@/types/api";

// 单张图片的最大展示高度（等比缩放，保持原图比例不裁剪）
const SINGLE_MAX_HEIGHT = "max-h-[28rem]";

// 按图片数量返回网格布局样式（纯函数）
// 1 张原图 / 2-4 张一行 / 5-9 张九宫格
function gridClassFor(count: number): string {
  if (count === 1) {
    return "flex justify-center";
  }
  if (count <= 4) {
    // 横向一排，每张固定 1/4 宽度（2/3 张时靠左不拉伸）
    return "flex gap-1";
  }
  return "grid grid-cols-3 gap-1";
}

// 图片列表（最多取 9 张）
export function MomentImageGrid({ media, postHref }: { media: MediaDTO[]; postHref: string }) {
  const images = media.slice(0, 9);
  const isSingle = images.length === 1;

  return (
    <Link href={postHref} className="mt-3 block">
      <div className={`group relative overflow-hidden rounded-lg ${gridClassFor(images.length)}`}>
        {images.map((m, i) => (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            key={`${m.id}-${i}`}
            src={m.url}
            alt=""
            loading="lazy"
            className={
              isSingle
                ? // 单图：等比缩放显示原图，不裁剪
                  `w-auto ${SINGLE_MAX_HEIGHT} rounded-lg object-contain transition-transform duration-[var(--yy-duration-slow)] ease-[var(--yy-ease-out)] group-hover:scale-[1.02]`
                : // 多图：方形缩略图统一裁剪，hover 微放大；2-4 张固定 1/4 宽度，5-9 张铺满网格
                  `aspect-square object-cover transition-transform duration-[var(--yy-duration-slow)] ease-[var(--yy-ease-out)] group-hover:scale-[1.03] ${
                    images.length <= 4 ? "w-1/4" : "w-full"
                  }`
            }
          />
        ))}
        {/* 超过 9 张时角标提示剩余数量 */}
        {media.length > 9 && (
          <span className="absolute bottom-1.5 right-1.5 rounded-md bg-black/55 px-1.5 py-0.5 text-[10px] font-medium text-white">
            +{media.length - 9}
          </span>
        )}
      </div>
    </Link>
  );
}
