// src/components/ui/avatar.tsx
// 统一头像组件（M1.7）：有头像地址显示图片，否则首字占位。
// 设计稿：圆形头像；尺寸由调用方传入（如 "h-10 w-10 text-sm"）。
// 动效：真实头像加载完成后淡入（避免图片晚到时突兀闪现）。
"use client";

import { useState } from "react";

// AvatarProps 头像组件参数。
interface AvatarProps {
  name: string; // 昵称（首字占位用）
  url: string; // 头像地址（空 = 无头像）
  className?: string; // 尺寸与圆角样式（默认 h-10 w-10 圆形）
  alt?: string; // 图片替代文本
}

// Avatar 统一头像：优先显示真实头像，加载失败/无头像回退首字。
export function Avatar({ name, url, className = "h-10 w-10 text-sm", alt = "" }: AvatarProps) {
  const [failed, setFailed] = useState<boolean>(false);
  const [loaded, setLoaded] = useState<boolean>(false);
  const base = `flex shrink-0 items-center justify-center overflow-hidden rounded-full bg-muted font-display text-ink-2 ${className}`;

  // 无头像或加载失败：首字占位（与 M1.2 一致形态）
  if (!url || failed) {
    return <span className={base}>{name.charAt(0) || "月"}</span>;
  }

  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={url}
      alt={alt || `${name}的头像`}
      loading="lazy"
      onError={() => setFailed(true)}
      onLoad={() => setLoaded(true)}
      className={`${base} transition-opacity duration-[var(--yy-duration-base)] ease-[var(--yy-ease-out)] ${
        loaded ? "opacity-100" : "opacity-0"
      }`}
    />
  );
}
