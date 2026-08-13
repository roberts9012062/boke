// src/components/lightbox.tsx
// 图片灯箱（设计稿 D/冷月/灯箱 1400×900）：
// 全屏查看大图、左右切换（2/5 计数）、ESC/点击关闭。
// 动效：遮罩淡入/淡出 + 主图缩放入场；切换图片时重播入场。
"use client";

import { useEffect, useState } from "react";

import { useDismiss } from "@/components/motion/use-dismiss";

// Lightbox 图片灯箱。
// 参数：images 图片列表；index 初始索引；onClose 关闭回调。
export function Lightbox({
  images,
  index,
  onClose,
}: {
  images: { id: number; url: string }[];
  index: number;
  onClose: () => void;
}) {
  const [current, setCurrent] = useState<number>(index);
  const { closing, close } = useDismiss(onClose, 180);

  // 键盘操作：ESC 关闭 / ← → 切换
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        close();
      } else if (e.key === "ArrowLeft") {
        setCurrent((i) => (i - 1 + images.length) % images.length);
      } else if (e.key === "ArrowRight") {
        setCurrent((i) => (i + 1) % images.length);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [images.length, close]);

  // 点击背景关闭
  const handleBackdrop = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      close();
    }
  };

  return (
    <div
      className={`fixed inset-0 z-50 flex items-center justify-center bg-black/90 ${
        closing ? "animate-fade-out" : "animate-fade-in"
      }`}
      onClick={handleBackdrop}
      role="dialog"
      aria-label="图片灯箱"
    >
      {/* 关闭按钮 */}
      <button
        type="button"
        onClick={close}
        className="absolute right-5 top-5 flex h-10 w-10 items-center justify-center rounded-full bg-white/10 text-lg text-white transition-colors hover:bg-white/20"
        aria-label="关闭"
      >
        ✕
      </button>

      {/* 主图（key 随 current 变化：切换时重播缩放入场） */}
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        key={current}
        src={images[current].url}
        alt=""
        className="max-h-[85vh] max-w-[85vw] animate-scale-in rounded-lg object-contain"
      />

      {/* 切换按钮（多图时显示） */}
      {images.length > 1 && (
        <>
          <button
            type="button"
            onClick={() => setCurrent((i) => (i - 1 + images.length) % images.length)}
            className="absolute left-5 top-1/2 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-full bg-white/10 text-xl text-white transition-colors hover:bg-white/20"
            aria-label="上一张"
          >
            ‹
          </button>
          <button
            type="button"
            onClick={() => setCurrent((i) => (i + 1) % images.length)}
            className="absolute right-5 top-1/2 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-full bg-white/10 text-xl text-white transition-colors hover:bg-white/20"
            aria-label="下一张"
          >
            ›
          </button>
        </>
      )}

      {/* 计数（设计稿：2 / 5） */}
      <p className="absolute bottom-6 left-1/2 -translate-x-1/2 text-sm text-white/80">
        {current + 1} / {images.length}
      </p>
    </div>
  );
}
