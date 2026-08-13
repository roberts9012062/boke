// src/components/post-card-skeleton.tsx
// 帖子卡片骨架屏（设计稿《加载骨架》画板：头像/标题/正文占位）。
// M1.1 首页无数据时展示；M1.3 接入数据后仅加载过程显示。
// 动效：skeleton-shimmer 渐变扫光（animations.css），减少动效时退化为静态底色。
import type { ReactNode } from "react";

// SkeletonRow 单行占位条（宽度百分比控制长度，扫光动画）。
function SkeletonRow({ widthClass }: { widthClass: string }) {
  return (
    <div
      className={`h-3 skeleton-shimmer rounded-full ${widthClass}`}
      aria-hidden
    />
  );
}

// PostCardSkeleton 帖子卡片骨架：头像 + 作者行 + 两行正文 + 标签行 + 互动条。
export function PostCardSkeleton() {
  return (
    <article className="rounded-lg border border-line bg-elevated p-5">
      {/* 作者行：头像 + 昵称 + 时间 */}
      <div className="flex items-center gap-3">
        <div className="h-10 w-10 skeleton-shimmer rounded-full" aria-hidden />
        <div className="flex-1 space-y-2">
          <SkeletonRow widthClass="w-24" />
          <SkeletonRow widthClass="w-16" />
        </div>
      </div>
      {/* 正文两行 */}
      <div className="mt-4 space-y-2">
        <SkeletonRow widthClass="w-full" />
        <SkeletonRow widthClass="w-4/5" />
      </div>
      {/* 标签行 */}
      <div className="mt-3 flex gap-2">
        <SkeletonRow widthClass="w-14" />
        <SkeletonRow widthClass="w-14" />
      </div>
      {/* 互动条 */}
      <div className="mt-4 flex gap-6 border-t border-line pt-3">
        {["w-10", "w-10", "w-10", "w-8"].map((width, index) => (
          <SkeletonRow key={`${width}-${index}`} widthClass={width} />
        ))}
      </div>
    </article>
  );
}

// PostCardSkeletonList 帖子骨架列表（默认 3 条，供首页/详情加载使用）。
export function PostCardSkeletonList({ count = 3 }: { count?: number }) {
  const items: ReactNode[] = [];
  for (let i = 0; i < count; i++) {
    items.push(<PostCardSkeleton key={i} />);
  }
  return <div className="space-y-4">{items}</div>;
}
