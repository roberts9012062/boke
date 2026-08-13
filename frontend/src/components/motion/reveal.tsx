// src/components/motion/reveal.tsx
// 滚动进场组件：IntersectionObserver 观察元素，进入视口后加 is-in 类触发 CSS 过渡。
//
// 设计要点：
//   - 隐藏规则由 JS 添加的 html.motion-ready 类门控（animations.css），无 JS/爬虫时内容始终可见
//   - 首屏已可见元素直接标记 is-in（不播隐藏-再显示动画），仅滚动进入的元素播放进场过渡
//   - 减少动效（data-motion="reduced"）时隐藏规则不生效，动画天然关闭
"use client";

import { useEffect, useRef, type ReactNode } from "react";

// Reveal 滚动进场容器（仅负责触发一次，动画由 CSS 完成）。
export function Reveal({ children }: { children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const root = document.documentElement;
    const el = ref.current;
    if (!el) {
      return;
    }
    // 首屏已可见：直接标记，避免无谓的隐藏-再显示闪烁
    const rect = el.getBoundingClientRect();
    const inViewport = rect.top < window.innerHeight && rect.bottom > 0;
    if (inViewport) {
      el.classList.add("is-in");
      root.classList.add("motion-ready");
      return;
    }
    // 视口外：进入视口（带 40px 提前量）时触发进场，仅一次
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            el.classList.add("is-in");
            observer.disconnect();
          }
        }
      },
      { threshold: 0.1, rootMargin: "0px 0px -40px 0px" }
    );
    observer.observe(el);
    root.classList.add("motion-ready");
    return () => observer.disconnect();
  }, []);

  return (
    <div ref={ref} data-reveal>
      {children}
    </div>
  );
}
