// frontend/src/components/image-gallery.tsx
// 图片展示风格渲染器：发帖编辑器可选择风格，帖子列表/详情按风格渲染。
// 风格：grid 网格（默认）/ carousel 轮播 / flip 卡片翻转 / filmstrip 胶片带 / masonry 瀑布流 / polaroid 拍立得。
"use client";

import { useEffect, useState } from "react";

import type { MediaDTO } from "@/types/api";

// GalleryStyle 图片展示风格。
export type GalleryStyle = "" | "carousel" | "flip" | "filmstrip" | "masonry" | "polaroid";

// GALLERY_STYLES 风格清单（选择器用；key 存库）。
export const GALLERY_STYLES: { key: GalleryStyle; label: string; icon: string }[] = [
  { key: "", label: "网格", icon: "▦" },
  { key: "carousel", label: "轮播", icon: "▣" },
  { key: "flip", label: "卡片翻转", icon: "▤" },
  { key: "filmstrip", label: "胶片带", icon: "▥" },
  { key: "masonry", label: "瀑布流", icon: "▧" },
  { key: "polaroid", label: "拍立得", icon: "▢" },
];

// imagesOf 过滤图片媒体（媒体列表可能混入音频/视频）。
function imagesOf(media: MediaDTO[]): MediaDTO[] {
  return media.filter((m) => m.type === "image" || !m.type);
}

// ImageGallery 按风格渲染图片组。
export function ImageGallery({ media, style }: { media: MediaDTO[]; style: GalleryStyle }) {
  const images = imagesOf(media);
  if (images.length === 0) return null;
  if (images.length === 1) {
    return (
      <img src={images[0].url} alt="" className="max-h-[480px] w-full rounded-xl object-cover" />
    );
  }
  switch (style) {
    case "carousel":
      return <CarouselGallery images={images} />;
    case "flip":
      return <FlipGallery images={images} />;
    case "filmstrip":
      return <FilmstripGallery images={images} />;
    case "masonry":
      return <MasonryGallery images={images} />;
    case "polaroid":
      return <PolaroidGallery images={images} />;
    default:
      return <GridGallery images={images} />;
  }
}

// GridGallery 网格（默认）：三列方格。
function GridGallery({ images }: { images: MediaDTO[] }) {
  return (
    <div className="grid grid-cols-3 gap-1.5">
      {images.map((m) => (
        // eslint-disable-next-line @next/next/no-img-element
        <img key={m.id} src={m.url} alt="" className="aspect-square w-full rounded-lg object-cover" />
      ))}
    </div>
  );
}

// CarouselGallery 轮播：自动播放 + 指示点 + 左右箭头。
function CarouselGallery({ images }: { images: MediaDTO[] }) {
  const [idx, setIdx] = useState(0);
  useEffect(() => {
    const t = setInterval(() => setIdx((i) => (i + 1) % images.length), 3500);
    return () => clearInterval(t);
  }, [images.length]);
  const prev = () => setIdx((i) => (i - 1 + images.length) % images.length);
  const next = () => setIdx((i) => (i + 1) % images.length);
  return (
    <div className="group relative aspect-[4/3] overflow-hidden rounded-xl">
      {images.map((m, i) => (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          key={m.id}
          src={m.url}
          alt=""
          className={`absolute inset-0 h-full w-full object-cover transition-opacity duration-500 ${i === idx ? "opacity-100" : "opacity-0"}`}
        />
      ))}
      {/* 左右箭头（hover 显示） */}
      <button type="button" onClick={prev} aria-label="上一张"
        className="absolute left-2 top-1/2 hidden -translate-y-1/2 h-8 w-8 items-center justify-center rounded-full bg-black/50 text-white group-hover:flex">‹</button>
      <button type="button" onClick={next} aria-label="下一张"
        className="absolute right-2 top-1/2 hidden -translate-y-1/2 h-8 w-8 items-center justify-center rounded-full bg-black/50 text-white group-hover:flex">›</button>
      {/* 指示点 */}
      <div className="absolute bottom-2.5 left-1/2 flex -translate-x-1/2 gap-1.5">
        {images.map((m, i) => (
          <span key={m.id} className={`h-1.5 rounded-full transition-all ${i === idx ? "w-4 bg-white" : "w-1.5 bg-white/50"}`} />
        ))}
      </div>
    </div>
  );
}

// FlipGallery 卡片翻转：3D 翻转，每 3 秒只翻一次（翻面展示下一张，翻回时正面已换新图），循环。
// 说明：turn 每次 +1，rotateY 在 0°/180° 交替——每 3 秒仅一次翻转动画，指示进度同步走一格。
function FlipGallery({ images }: { images: MediaDTO[] }) {
  const [turn, setTurn] = useState(0);
  useEffect(() => {
    const timer = setInterval(() => setTurn((n) => n + 1), 3000);
    return () => clearInterval(timer);
  }, [images.length]);

  const pair = Math.floor(turn / 2);
  const front = images[(pair * 2) % images.length];
  const back = images[(pair * 2 + 1) % images.length];
  const rot = (turn % 2) * 180;
  const shown = turn % images.length;

  return (
    <div className="[perspective:1600px]">
      <div
        className="relative aspect-[4/3] transition-transform duration-700 [transform-style:preserve-3d]"
        style={{ transform: `rotateY(${rot}deg)` }}
      >
        {/* 正面：偶数序图；eslint-disable-next-line @next/next/no-img-element */}
        <img src={front.url} alt="" className="absolute inset-0 h-full w-full rounded-xl object-cover shadow-lg [backface-visibility:hidden]" />
        {/* 背面：奇数序图（预翻转 180°）；eslint-disable-next-line @next/next/no-img-element */}
        <img src={back.url} alt=""
          className="absolute inset-0 h-full w-full rounded-xl object-cover shadow-lg [backface-visibility:hidden] [transform:rotateY(180deg)]" />
      </div>
      {/* 指示点 + 序号（与显示图同步） */}
      <div className="mt-2.5 flex items-center justify-center gap-1.5">
        {images.map((m, i) => (
          <span key={m.id} className={`h-1.5 rounded-full transition-all ${i === shown ? "w-4 bg-accent" : "w-1.5 bg-line"}`} />
        ))}
        <span className="ml-2 text-xs text-ink-3">{shown + 1}/{images.length}</span>
      </div>
    </div>
  );
}

// FilmstripGallery 胶片带：横向滚动的电影胶片条（底片底 + 胶片孔，snap 逐格吸附）。
function FilmstripGallery({ images }: { images: MediaDTO[] }) {
  return (
    <div className="flex snap-x gap-3 overflow-x-auto pb-2">
      {images.map((m, i) => (
        <figure
          key={m.id}
          className="w-60 shrink-0 snap-center rounded-lg bg-[#15181d] p-2.5 shadow-lg shadow-black/30"
        >
          {/* 上排胶片孔 */}
          <div className="mb-2 flex justify-between px-1">
            {Array.from({ length: 7 }).map((_, k) => (
              <span key={k} className="h-1.5 w-1.5 rounded-[2px] bg-white/25" />
            ))}
          </div>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={m.url} alt="" className="aspect-[4/3] w-full rounded object-cover" />
          {/* 下排胶片孔 + 格数编号 */}
          <div className="mt-2 flex items-center justify-between px-1">
            {Array.from({ length: 7 }).map((_, k) => (
              <span key={k} className="h-1.5 w-1.5 rounded-[2px] bg-white/25" />
            ))}
            <span className="ml-2 shrink-0 font-mono text-[10px] text-white/40">{i + 1}/{images.length}</span>
          </div>
        </figure>
      ))}
    </div>
  );
}

// MasonryGallery 瀑布流：CSS 双列按原始高度错落。
function MasonryGallery({ images }: { images: MediaDTO[] }) {
  return (
    <div className="columns-2 gap-2">
      {images.map((m) => (
        // eslint-disable-next-line @next/next/no-img-element
        <img key={m.id} src={m.url} alt="" className="mb-2 w-full break-inside-avoid rounded-lg object-cover" />
      ))}
    </div>
  );
}

// PolaroidGallery 拍立得：白边相纸 + 微旋转 + 手写风编号。
function PolaroidGallery({ images }: { images: MediaDTO[] }) {
  return (
    <div className="flex flex-wrap justify-center gap-4 py-2">
      {images.map((m, i) => (
        <figure
          key={m.id}
          className="w-40 rounded-[3px] bg-white p-2 pb-7 shadow-lg shadow-black/20 transition-transform duration-200 hover:scale-105 hover:rotate-0"
          style={{ transform: `rotate(${(i % 5) - 2}deg)` }}
        >
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={m.url} alt="" className="aspect-square w-full rounded-[2px] object-cover" />
          <figcaption className="mt-1.5 text-center font-serif text-xs italic text-stone-500">yueyan · {i + 1}</figcaption>
        </figure>
      ))}
    </div>
  );
}

