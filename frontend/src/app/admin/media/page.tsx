// src/app/admin/media/page.tsx
// 后台媒体库（设计稿 D/冷月/后台媒体 1400×1000）：
// 统计条（全部文件/图片/音频/视频）+ 搜索 + 类型筛选
// + 文件表格（文件/类型/大小/引用/上传/操作：预览 · 删除）+ 预览弹层（图片大图/音视频播放）。
"use client";

import { useEffect, useState } from "react";

import {
  apiAdminDeleteMedia,
  apiAdminMedia,
  apiAdminMediaStats,
  ApiError,
  type AdminMediaItem,
} from "@/lib/api";
import { formatDateTime } from "@/lib/utils";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

// 类型文案（设计稿：图片/音频/视频）
const TYPE_LABEL: Record<string, string> = { image: "图片", audio: "音频", video: "视频" };

// formatBytes 字节数格式化为可读大小（设计稿：2.4 MB）。
function formatBytes(bytes: number): string {
  if (bytes >= 1 << 30) return `${(bytes / (1 << 30)).toFixed(1)} GB`;
  if (bytes >= 1 << 20) return `${(bytes / (1 << 20)).toFixed(1)} MB`;
  if (bytes >= 1 << 10) return `${Math.round(bytes / (1 << 10))} KB`;
  return `${bytes} B`;
}

// AdminMedia 媒体库。
export default function AdminMedia() {
  const [items, setItems] = useState<AdminMediaItem[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [type, setType] = useState<string>("");
  const [keyword, setKeyword] = useState<string>("");
  const [stats, setStats] = useState<{ total: number; image: number; audio: number; video: number }>({
    total: 0,
    image: 0,
    audio: 0,
    video: 0,
  });
  const [loading, setLoading] = useState<boolean>(true);
  // 预览弹层
  const [preview, setPreview] = useState<AdminMediaItem | null>(null);
  const [error, setError] = useState<string>("");
  // 删除确认弹窗（取代 window.confirm——设计稿「确认弹窗」风格）
  const [confirmItem, setConfirmItem] = useState<AdminMediaItem | null>(null);
  const [deleting, setDeleting] = useState<boolean>(false);

  // 加载统计条（设计稿：全部文件/图片/音频/视频）
  useEffect(() => {
    apiAdminMediaStats().then(setStats).catch(() => undefined);
  }, []);

  // 加载列表（筛选变化时）
  useEffect(() => {
    setLoading(true);
    apiAdminMedia({ type: type || undefined, q: keyword || undefined })
      .then((r) => {
        setItems(r.items);
        setTotal(r.total);
      })
      .catch(() => setItems([]))
      .finally(() => setLoading(false));
  }, [type, keyword]);

  // 删除（二次确认弹窗；被引用时提示影响）
  const handleDelete = async (item: AdminMediaItem) => {
    setConfirmItem(item); // 打开确认弹窗（设计稿「确认弹窗」风格）
  };

  // 确认删除（弹窗确认后执行）
  const confirmDelete = async () => {
    if (!confirmItem) return;
    setDeleting(true);
    setError("");
    try {
      await apiAdminDeleteMedia(confirmItem.id);
      setItems((prev) => prev.filter((x) => x.id !== confirmItem.id));
      apiAdminMediaStats().then(setStats).catch(() => undefined);
      setConfirmItem(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "删除失败");
      setConfirmItem(null);
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div>
      <h1 className="font-display text-xl font-semibold text-ink">媒体库</h1>
      <p className="mt-0.5 text-xs text-ink-3">管理图片、音频与视频资源</p>

      {/* 统计条（设计稿：全部文件/图片/音频/视频） */}
      <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
        {[
          { key: "全部文件", value: stats.total },
          { key: "图片", value: stats.image },
          { key: "音频", value: stats.audio },
          { key: "视频", value: stats.video },
        ].map((s) => (
          <div key={s.key} className="rounded-lg border border-line bg-elevated p-4">
            <p className="text-2xl font-semibold text-ink">{s.value}</p>
            <p className="mt-1 text-xs text-ink-3">{s.key}</p>
          </div>
        ))}
      </div>

      {/* 搜索 + 类型筛选 */}
      <div className="mt-4 flex flex-wrap items-center gap-3">
        <input
          type="text"
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          placeholder="搜索媒体…"
          className="h-9 w-56 rounded-full border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
        <select
          value={type}
          onChange={(e) => setType(e.target.value)}
          className="h-9 rounded-full border border-line bg-elevated px-3 text-sm text-ink-2"
        >
          <option value="">全部类型</option>
          <option value="image">图片</option>
          <option value="audio">音频</option>
          <option value="video">视频</option>
        </select>
        <span className="text-xs text-ink-3">共 {total} 个文件</span>
      </div>

      {error && (
        <p className="mt-3 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
          {error}
        </p>
      )}

      {/* 文件表格（设计稿：文件/类型/大小/引用/上传/操作） */}
      <div className="mt-4 overflow-x-auto rounded-lg border border-line bg-elevated">
        <table className="w-full min-w-[760px] text-left text-sm">
          <thead>
            <tr className="border-b border-line text-xs text-ink-3">
              <th className="px-4 py-3 font-normal">文件</th>
              <th className="px-4 py-3 font-normal">类型</th>
              <th className="px-4 py-3 font-normal">大小</th>
              <th className="px-4 py-3 font-normal">引用</th>
              <th className="px-4 py-3 font-normal">上传</th>
              <th className="px-4 py-3 font-normal">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {items.map((item) => (
              <tr key={item.id} className="hover:bg-muted/40">
                <td className="px-4 py-3">
                  <p className="max-w-[220px] truncate text-ink">{item.file_name}</p>
                  <p className="text-xs text-ink-3">
                    {item.mime_type}
                    {item.width > 0 && item.height > 0 ? ` · ${item.width}×${item.height}` : ""}
                  </p>
                </td>
                <td className="px-4 py-3 text-ink-2">{TYPE_LABEL[item.type] ?? item.type}</td>
                <td className="px-4 py-3 text-ink-2">{formatBytes(item.size_bytes)}</td>
                <td className="px-4 py-3 text-ink-2">
                  {item.ref_count > 0 ? `${item.ref_count} 篇` : "未引用"}
                </td>
                <td className="px-4 py-3 text-xs text-ink-3">{formatDateTime(item.created_at)}</td>
                <td className="px-4 py-3">
                  <div className="flex gap-2 text-xs">
                    <button
                      type="button"
                      onClick={() => setPreview(item)}
                      className="text-glow hover:underline"
                    >
                      预览
                    </button>
                    <button
                      type="button"
                      onClick={() => void handleDelete(item)}
                      className="text-like hover:underline"
                    >
                      删除
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!loading && items.length === 0 && (
          <p className="py-12 text-center text-sm text-ink-3">没有匹配的媒体文件</p>
        )}
      </div>

      {/* 预览弹层（图片大图 / 音频播放 / 视频播放） */}
      {preview && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 px-6"
          role="dialog"
          aria-modal="true"
          aria-label="预览媒体"
          onClick={() => setPreview(null)}
        >
          <div
            className="w-full max-w-[640px] rounded-xl border border-line bg-elevated p-6"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between">
              <p className="max-w-[380px] truncate text-sm text-ink">{preview.file_name}</p>
              <button
                type="button"
                onClick={() => setPreview(null)}
                className="rounded-full px-2 py-0.5 text-ink-3 hover:text-ink"
                aria-label="关闭预览"
              >
                ✕
              </button>
            </div>
            <div className="mt-4 flex max-h-[420px] items-center justify-center overflow-hidden rounded-lg bg-black/20">
              {preview.type === "image" ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={preview.url} alt={preview.file_name} className="max-h-[420px] max-w-full object-contain" />
              ) : preview.type === "audio" ? (
                // eslint-disable-next-line jsx-a11y/media-has-caption
                <audio src={preview.url} controls className="w-full px-4 py-6" />
              ) : (
                // eslint-disable-next-line jsx-a11y/media-has-caption
                <video src={preview.url} controls className="max-h-[420px] w-full object-contain" />
              )}
            </div>
          </div>
        </div>
      )}

      {/* 删除确认弹窗（设计稿「确认弹窗」：问句标题 + 影响提示 + 取消/删除） */}
      <ConfirmDialog
        open={confirmItem !== null}
        title={`删除「${confirmItem?.file_name ?? ""}」？`}
        description={`${confirmItem && confirmItem.ref_count > 0 ? `该文件被 ${confirmItem.ref_count} 篇帖子引用，删除后将自动解除引用。` : ""}删除后无法恢复。`}
        confirmText="删除"
        danger
        loading={deleting}
        onConfirm={() => void confirmDelete()}
        onClose={() => setConfirmItem(null)}
      />
    </div>
  );
}
