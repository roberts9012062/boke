// src/components/compose/image-library-picker.tsx
// 发帖 · 图床图库选择器（数据驱动多源）：按图床插件 tab 切换，列出已有图片，点选插入正文。
// 数据源：插件代理 API /api/v1/plugins/{id}/manage/list（登录用户可调，图床插件契约同构——
// 条目取 url/size 通用字段；插件未启用/未配对时返回空列表——组件内提示引导）。
"use client";

import { useCallback, useEffect, useState } from "react";

import { apiPluginCall } from "@/lib/api";

// LibraryImage 图库图片（插件 /manage/list 条目；url/size 为各图床同构字段，url 兼作键）。
interface LibraryImage {
  url: string; // 公开访问 URL
  size: number; // 字节数
}

// ImageBedSource 图库源（图床插件通道）。
interface ImageBedSource {
  id: string; // 插件 ID
  name: string; // 插件名（tab 文案）
}

// ImageLibraryPicker 图库选择器。
// 参数：plugins 图床源列表（空=无可用图床，显示占位）；onPick 选中回调（传图片 URL）；onClose 关闭回调。
export function ImageLibraryPicker({ plugins, onPick, onClose }: { plugins: ImageBedSource[]; onPick: (url: string) => void; onClose: () => void }) {
  const [activeId, setActiveId] = useState<string>(plugins[0]?.id ?? "");
  const [images, setImages] = useState<LibraryImage[]>([]);
  const [cursor, setCursor] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string>("");
  const [hasMore, setHasMore] = useState<boolean>(false);

  // load 拉取当前源图库列表（append=true 追加下一页）
  const load = useCallback(async (append: boolean) => {
    setLoading(true);
    setError("");
    try {
      const r = await apiPluginCall<{ objects?: LibraryImage[]; cursor?: string; error?: string }>(
        activeId,
        "/manage/list",
        append ? { cursor } : {},
      );
      if (r.error) {
        setError(r.error);
        return;
      }
      setImages((prev) => (append ? [...prev, ...(r.objects ?? [])] : (r.objects ?? [])));
      setCursor(r.cursor ?? "");
      setHasMore(Boolean(r.cursor));
    } catch {
      setError("图库加载失败（插件未启用或未配对）");
    } finally {
      setLoading(false);
    }
  }, [activeId, cursor]);

  useEffect(() => {
    if (activeId === "") {
      return;
    }
    setImages([]);
    setCursor("");
    setHasMore(false);
    void load(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeId]);

  // 无可用图床源：占位提示（弹窗仍有本地/外链通道）
  if (plugins.length === 0) {
    return <p className="py-2 text-center text-xs text-ink-3">未安装图床插件——可从插件商城安装图床后在此选图</p>;
  }

  const activeName = plugins.find((p) => p.id === activeId)?.name ?? "";

  return (
    <div className="space-y-3">
      {/* 多源 tab（单源不渲染——与历史行为一致） */}
      {plugins.length > 1 && (
        <div className="flex gap-2">
          {plugins.map((p) => (
            <button
              key={p.id}
              type="button"
              onClick={() => setActiveId(p.id)}
              className={`rounded-full border px-3 py-1 text-xs transition-colors ${
                p.id === activeId ? "border-accent bg-accent-soft text-glow" : "border-line text-ink-2 hover:text-ink"
              }`}
            >
              {p.name}
            </button>
          ))}
        </div>
      )}
      <p className="text-xs text-ink-3">
        从{activeName}图库选择已有图片插入正文；新图可在上方直传通道或后台图库上传。
      </p>
      {error && (
        <p className="rounded-md bg-muted px-3 py-2 text-xs text-ink-3" role="alert">
          {error}
        </p>
      )}
      {loading && images.length === 0 ? (
        <div className="grid grid-cols-4 gap-2">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="aspect-square animate-pulse rounded-lg bg-muted" aria-hidden />
          ))}
        </div>
      ) : images.length === 0 && !error ? (
        <p className="py-6 text-center text-xs text-ink-3">图库还是空的——先上传几张图片吧</p>
      ) : (
        <div className="grid max-h-[320px] grid-cols-4 gap-2 overflow-y-auto">
          {images.map((img) => (
            <button
              key={img.url}
              type="button"
              onClick={() => {
                onPick(img.url);
                onClose();
              }}
              className="group relative aspect-square overflow-hidden rounded-lg border border-line transition-colors hover:border-accent"
              aria-label={"插入图片 " + img.url}
            >
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={img.url} alt={img.url} loading="lazy" className="h-full w-full object-cover" />
              <span className="absolute inset-x-0 bottom-0 hidden bg-black/60 px-1 py-0.5 text-[10px] text-white group-hover:block">
                {(img.size / 1024).toFixed(0)} KB
              </span>
            </button>
          ))}
        </div>
      )}
      {hasMore && (
        <button
          type="button"
          disabled={loading}
          onClick={() => void load(true)}
          className="w-full rounded-lg border border-line py-2 text-xs text-ink-2 hover:border-accent hover:text-ink disabled:opacity-60"
        >
          {loading ? "加载中…" : "加载更多"}
        </button>
      )}
    </div>
  );
}
