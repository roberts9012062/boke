// src/components/admin/media-picker.tsx
// 媒体库选择弹窗（M5 插件设置图片字段）：分页加载后台媒体库（仅图片），
// 网格缩略图点选回调 URL（复用后台媒体列表接口 apiAdminMedia）。
"use client";

import { useEffect, useState } from "react";

import { Modal } from "@/components/ui/modal";
import { apiAdminMedia, ApiError, type AdminMediaItem } from "@/lib/api";

// MediaPicker 媒体库选择器。
// 参数：open 是否打开；onClose 关闭回调；onSelect 选中回调（回填 URL）。
export function MediaPicker({
  open,
  onClose,
  onSelect,
}: {
  open: boolean;
  onClose: () => void;
  onSelect: (url: string) => void;
}) {
  const [items, setItems] = useState<AdminMediaItem[]>([]);
  const [page, setPage] = useState<number>(1);
  const [total, setTotal] = useState<number>(0);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // 打开时重新加载（重置分页）；关闭/切换时丢弃过期响应
  useEffect(() => {
    if (!open) {
      return;
    }
    let cancelled = false;
    setPage(1);
    setItems([]);
    setTotal(0);
    setError("");
    setLoading(true);
    apiAdminMedia({ type: "image", page: 1 })
      .then((r) => {
        if (cancelled) {
          return;
        }
        setItems(r.items);
        setTotal(r.total);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.message : "加载图片库失败");
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [open]);

  // 加载更多（下一页追加）
  const loadMore = async () => {
    const next = page + 1;
    setLoading(true);
    setError("");
    try {
      const r = await apiAdminMedia({ type: "image", page: next });
      setItems((prev) => [...prev, ...r.items]);
      setPage(next);
      setTotal(r.total);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "加载更多失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open={open} title="从图片库选择" onClose={onClose} maxWidth="max-w-[640px]">
      {error && (
        <p className="rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
          {error}
        </p>
      )}

      {loading && items.length === 0 ? (
        // 加载骨架
        <div className="grid grid-cols-4 gap-3">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="aspect-square animate-pulse rounded-lg bg-muted" aria-hidden />
          ))}
        </div>
      ) : items.length === 0 ? (
        <p className="py-10 text-center text-sm text-ink-3">图片库暂无图片，可先「点击上传」添加</p>
      ) : (
        <>
          {/* 网格缩略图（点选即回填） */}
          <div className="grid max-h-[55vh] grid-cols-4 gap-3 overflow-y-auto pr-1">
            {items.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => onSelect(item.url)}
                title={item.file_name}
                className="group overflow-hidden rounded-lg border border-line bg-muted transition-colors hover:border-accent"
              >
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={item.url}
                  alt={item.file_name}
                  className="aspect-square w-full object-cover"
                  loading="lazy"
                />
                <p className="truncate px-1.5 py-1 text-[10px] text-ink-3 group-hover:text-glow">
                  {item.file_name}
                </p>
              </button>
            ))}
          </div>

          {/* 加载更多（未拉完时显示） */}
          {items.length < total && (
            <div className="mt-3 flex justify-center">
              <button
                type="button"
                onClick={() => void loadMore()}
                disabled={loading}
                className="rounded-full border border-line px-5 py-1.5 text-xs text-ink-2 transition-colors hover:text-ink disabled:opacity-60"
              >
                {loading ? "加载中…" : `加载更多（${items.length}/${total}）`}
              </button>
            </div>
          )}
        </>
      )}
    </Modal>
  );
}
