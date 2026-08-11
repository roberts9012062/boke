// src/app/admin/backup/page.tsx
// 备份导出页（设计稿《备份导出》#237/#244）：
// 左：备份表单（类型/范围/保留天数/格式/立即备份）+ 右：备份记录列表（下载/删除）。
"use client";

import { useEffect, useState } from "react";

import { BackupForm } from "@/components/admin/backup/backup-form";
import { apiBackupDownload, apiBackups, apiDeleteBackup, type BackupDTO } from "@/lib/api-report";
import { formatDateTime } from "@/lib/utils";

// 备份类型文案（设计稿：全站数据 · 媒体库）。
const TYPE_LABEL: Record<string, string> = { all: "全站数据", media: "媒体库" };

// formatSize 文件大小人性化（B/KB/MB）。
function formatSize(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

// BackupPage 备份导出页。
export default function BackupPage() {
  const [items, setItems] = useState<BackupDTO[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState("");

  // 加载备份记录列表
  const loadList = () => {
    apiBackups()
      .then((r) => setItems(r.items))
      .catch(() => setItems([]))
      .finally(() => setLoaded(true));
  };
  useEffect(loadList, []);

  // 下载备份文件
  const handleDownload = async (item: BackupDTO) => {
    try {
      await apiBackupDownload(item.id, item.file_name);
    } catch (err) {
      setError(err instanceof Error ? err.message : "下载失败");
    }
  };

  // 删除备份（二次确认）
  const handleDelete = async (item: BackupDTO) => {
    if (!confirm(`确认删除备份「${item.file_name}」？删除后不可恢复。`)) {
      return;
    }
    try {
      await apiDeleteBackup(item.id);
      setItems((prev) => prev.filter((x) => x.id !== item.id));
    } catch (err) {
      setError(err instanceof Error ? err.message : "删除失败");
    }
  };

  return (
    <div>
      {/* 标题（设计稿：备份与导出） */}
      <div>
        <h1 className="font-display text-xl font-semibold text-ink">备份与导出</h1>
        <p className="mt-0.5 text-xs text-ink-3">全站数据与媒体库的导出、下载与清理</p>
      </div>

      {error && <p className="mt-3 rounded-lg bg-like/10 px-3 py-2 text-sm text-ink">{error}</p>}

      {/* 两栏：左表单 / 右记录列表 */}
      <div className="mt-4 grid gap-6 lg:grid-cols-[360px_1fr]">
        <BackupForm
          onCreated={() => {
            setError("");
            loadList();
          }}
        />

        {/* 备份记录列表 */}
        <div className="rounded-lg border border-line bg-elevated p-5">
          <h2 className="text-sm font-semibold text-ink">备份记录</h2>
          <p className="mt-0.5 text-xs text-ink-3">手动备份记录（自动定时备份后置）</p>

          {!loaded && <div className="mt-4 h-32 animate-pulse rounded-lg bg-muted" aria-hidden />}
          {loaded && items.length === 0 && (
            <p className="py-14 text-center text-sm text-ink-3">暂无备份记录，使用左侧表单创建</p>
          )}
          {items.length > 0 && (
            <div className="mt-3 overflow-x-auto">
              <table className="w-full min-w-[520px] text-left text-sm">
                <thead>
                  <tr className="border-b border-line text-xs text-ink-3">
                    <th className="px-3 py-2.5 font-normal">类型</th>
                    <th className="px-3 py-2.5 font-normal">状态</th>
                    <th className="px-3 py-2.5 font-normal">文件</th>
                    <th className="px-3 py-2.5 font-normal">大小</th>
                    <th className="px-3 py-2.5 font-normal">时间</th>
                    <th className="px-3 py-2.5 font-normal">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-line">
                  {items.map((item) => (
                    <tr key={item.id} className="hover:bg-muted/40">
                      <td className="px-3 py-3 text-ink-2">{TYPE_LABEL[item.type] ?? item.type}</td>
                      <td className="px-3 py-3">
                        {item.status === "success" ? (
                          <span className="rounded-full bg-accent-soft px-2 py-0.5 text-xs text-glow">成功</span>
                        ) : (
                          <span className="rounded-full bg-like/15 px-2 py-0.5 text-xs text-like">失败</span>
                        )}
                      </td>
                      <td className="max-w-[180px] px-3 py-3 text-xs text-ink-2">
                        <span className="line-clamp-1">{item.file_name || "—"}</span>
                      </td>
                      <td className="px-3 py-3 text-xs text-ink-3">{formatSize(item.file_size)}</td>
                      <td className="px-3 py-3 text-xs text-ink-3">{formatDateTime(item.created_at)}</td>
                      <td className="px-3 py-3">
                        {item.status === "success" ? (
                          <div className="flex gap-2 text-xs">
                            <button type="button" onClick={() => void handleDownload(item)} className="text-glow hover:underline">
                              下载
                            </button>
                            <button type="button" onClick={() => void handleDelete(item)} className="text-like hover:underline">
                              删除
                            </button>
                          </div>
                        ) : (
                          <button type="button" onClick={() => void handleDelete(item)} className="text-xs text-like hover:underline">
                            删除
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
