// src/components/admin/backup/backup-form.tsx
// 备份导出表单（设计稿《备份导出》#237/#244）：
// 备份类型（全站数据/媒体库）+ 范围（内容+用户+媒体）+ 保留天数（30）+ 导出格式（JSON/CSV/ZIP）+ 立即备份。
"use client";

import { useState } from "react";

import { apiCreateBackup, type BackupDTO } from "@/lib/api-report";

// 备份类型选项（设计稿：全站数据 · 媒体库）。
const TYPE_OPTIONS = [
  { key: "all", label: "全站数据", desc: "按范围导出内容/用户/媒体数据" },
  { key: "media", label: "媒体库", desc: "打包本地媒体文件（data/media）" },
] as const;

// 范围选项（设计稿：内容 + 用户 + 媒体）。
const SCOPE_OPTIONS = [
  { key: "content", label: "内容", desc: "帖子/评论/标签" },
  { key: "users", label: "用户", desc: "用户账号与资料" },
  { key: "media", label: "媒体", desc: "媒体资源元数据" },
] as const;

// 导出格式选项（设计稿：JSON / CSV / ZIP；媒体库固定 ZIP）。
const FORMAT_OPTIONS = [
  { key: "json", label: "JSON", desc: "单文件" },
  { key: "csv", label: "CSV", desc: "每表一个 CSV" },
  { key: "zip", label: "ZIP", desc: "打包压缩" },
] as const;

// BackupForm 备份表单（提交成功后回调 onCreated 刷新列表）。
export function BackupForm({ onCreated }: { onCreated: (dto: BackupDTO) => void }) {
  const [backupType, setBackupType] = useState<string>("all");
  const [scope, setScope] = useState<string[]>(["content", "users", "media"]);
  const [format, setFormat] = useState<string>("json");
  const [retentionDays, setRetentionDays] = useState<number>(30);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  // 范围复选切换
  const toggleScope = (key: string) => {
    setScope((prev) => (prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key]));
  };

  // 提交备份（媒体库时格式锁定 ZIP）
  const handleSubmit = async () => {
    setBusy(true);
    setError("");
    setSuccess("");
    try {
      const dto = await apiCreateBackup({
        backup_type: backupType,
        scope: backupType === "media" ? [] : scope,
        format: backupType === "media" ? "zip" : format,
        retention_days: retentionDays,
      });
      setSuccess("备份已创建：" + dto.file_name);
      onCreated(dto);
    } catch (err) {
      setError(err instanceof Error ? err.message : "备份失败");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="rounded-lg border border-line bg-elevated p-5">
      <h2 className="text-sm font-semibold text-ink">备份与导出</h2>
      <p className="mt-0.5 text-xs text-ink-3">创建一次手动备份，完成后可在下方列表下载</p>

      <div className="mt-4 space-y-4">
        {/* 备份类型（设计稿：全站数据 · 媒体库） */}
        <div>
          <p className="mb-1.5 text-sm text-ink-2">备份类型</p>
          <div className="grid grid-cols-2 gap-2">
            {TYPE_OPTIONS.map((opt) => (
              <button
                key={opt.key}
                type="button"
                onClick={() => {
                  setBackupType(opt.key);
                  if (opt.key === "media") {
                    setFormat("zip");
                  }
                }}
                className={`rounded-lg border p-3 text-left transition-colors ${
                  backupType === opt.key ? "border-accent bg-accent-soft" : "border-line hover:border-accent/50"
                }`}
              >
                <p className={`text-sm font-medium ${backupType === opt.key ? "text-glow" : "text-ink"}`}>{opt.label}</p>
                <p className="mt-0.5 text-xs text-ink-3">{opt.desc}</p>
              </button>
            ))}
          </div>
        </div>

        {/* 范围（设计稿：内容 + 用户 + 媒体；媒体库备份时隐藏） */}
        {backupType === "all" && (
          <div>
            <p className="mb-1.5 text-sm text-ink-2">范围</p>
            <div className="flex flex-wrap gap-2">
              {SCOPE_OPTIONS.map((opt) => (
                <button
                  key={opt.key}
                  type="button"
                  onClick={() => toggleScope(opt.key)}
                  className={`rounded-full border px-4 py-1.5 text-sm transition-colors ${
                    scope.includes(opt.key)
                      ? "border-accent bg-accent-soft text-glow"
                      : "border-line text-ink-2 hover:text-ink"
                  }`}
                >
                  {opt.label}
                </button>
              ))}
            </div>
            <p className="mt-1 text-xs text-ink-3">{SCOPE_OPTIONS.filter((o) => scope.includes(o.key)).map((o) => o.desc).join(" / ")}</p>
          </div>
        )}

        {/* 导出格式（设计稿：JSON / CSV / ZIP；媒体库锁定 ZIP） */}
        <div>
          <p className="mb-1.5 text-sm text-ink-2">导出格式</p>
          <div className="flex flex-wrap gap-2">
            {FORMAT_OPTIONS.map((opt) => {
              const disabled = backupType === "media" && opt.key !== "zip";
              return (
                <button
                  key={opt.key}
                  type="button"
                  disabled={disabled}
                  onClick={() => setFormat(opt.key)}
                  className={`rounded-full border px-4 py-1.5 text-sm transition-colors disabled:opacity-40 ${
                    format === opt.key
                      ? "border-accent bg-accent-soft text-glow"
                      : "border-line text-ink-2 hover:text-ink"
                  }`}
                >
                  {opt.label}
                  <span className="ml-1 text-xs text-ink-3">{opt.desc}</span>
                </button>
              );
            })}
          </div>
          {backupType === "media" && <p className="mt-1 text-xs text-ink-3">媒体库备份固定 ZIP 格式</p>}
        </div>

        {/* 保留天数（设计稿：30 天） */}
        <div>
          <label htmlFor="backup-retention" className="mb-1.5 block text-sm text-ink-2">
            保留天数
          </label>
          <input
            id="backup-retention"
            type="number"
            min={1}
            max={365}
            value={retentionDays}
            onChange={(e) => setRetentionDays(Number(e.target.value) || 30)}
            className="h-10 w-32 rounded-lg border border-line bg-muted px-3 text-sm text-ink focus:border-accent focus:outline-none"
          />
          <p className="mt-1 text-xs text-ink-3">超过保留天数的同类型旧备份会自动清理</p>
        </div>

        {/* 提交反馈 */}
        {error && <p className="rounded-md bg-like/10 px-3 py-2 text-sm text-like">{error}</p>}
        {success && <p className="rounded-md bg-accent-soft px-3 py-2 text-sm text-glow">{success}</p>}

        <button
          type="button"
          onClick={() => void handleSubmit()}
          disabled={busy || (backupType === "all" && scope.length === 0)}
          className="w-full rounded-full bg-accent px-5 py-2.5 text-sm font-medium text-on-accent hover:opacity-90 disabled:opacity-50"
        >
          {busy ? "备份中…" : "立即备份"}
        </button>
      </div>
    </div>
  );
}
