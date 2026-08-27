// frontend/src/components/admin/update-badge.tsx
// 后台左下角站点更新徽标：静默检查新版本 → 有更新时绿色徽标自动出现 →
// 点击弹窗查看版本与更新日志 → 一键更新（下载/构建/重启进度条轮询，完成自动收尾）。
// 旁边附 GitHub 源码仓库链接（项目自有更新通道指向主仓库）。

"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { Modal } from "@/components/ui/modal";
import {
  checkUpdate,
  fetchUpdateProgress,
  startUpdate,
  type UpdateCheck,
  type UpdateStatus,
} from "@/lib/api-update";

// 版本展示格式化（dev 显示开发版；纯函数）。
function formatVersion(v: string): string {
  if (!v || v === "dev") return "开发版";
  return v.startsWith("v") ? v : `v${v}`;
}

// UpdateBadge 更新徽标（挂后台侧栏底部）。
export function UpdateBadge() {
  const [check, setCheck] = useState<UpdateCheck | null>(null);
  const [open, setOpen] = useState<boolean>(false);
  const [updating, setUpdating] = useState<boolean>(false);
  const [progress, setProgress] = useState<UpdateStatus>({ state: "idle", percent: 0 });
  const [error, setError] = useState<string>("");
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // 静默检查（进入后台时一次；10 分钟内不重复）
  useEffect(() => {
    let cancelled = false;
    const run = async (): Promise<void> => {
      try {
        const result = await checkUpdate();
        if (!cancelled) setCheck(result);
      } catch {
        // 检测失败静默（离线/GitHub 不可达）：徽标仅显示当前版本
      }
    };
    void run();
    return () => {
      cancelled = true;
    };
  }, []);

  // 停止进度轮询
  const stopPolling = useCallback((): void => {
    if (timerRef.current !== null) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  // 启动进度轮询（3 秒一次；服务重启期间请求失败按「重启中」容错，恢复后继续）
  const startPolling = useCallback((): void => {
    stopPolling();
    timerRef.current = setInterval(() => {
      void fetchUpdateProgress()
        .then((result) => {
          setProgress(result.status);
          if (result.status.state === "done" || result.status.state === "failed") {
            stopPolling();
            setUpdating(false);
            // 完成后刷新版本检查（徽标转为新版本号）
            void checkUpdate().then(setCheck).catch(() => undefined);
          }
        })
        .catch(() => {
          // 容器重启中：视为进行中的过渡阶段（进度保持，阶段提示重启）
          setProgress((prev) =>
            prev.state === "running"
              ? { ...prev, stage: "服务重启中，即将恢复…", percent: Math.max(prev.percent, 92) }
              : prev,
          );
        });
    }, 3000);
  }, [stopPolling]);

  // 组件卸载清理定时器
  useEffect(() => stopPolling, [stopPolling]);

  // 弹窗打开时若更新已在进行（其他窗口触发过），直接进入轮询
  useEffect(() => {
    if (open && check?.running_update.state === "running") {
      setUpdating(true);
      setProgress(check.running_update);
      startPolling();
    }
  }, [open, check, startPolling]);

  // handleStart 一键更新
  const handleStart = useCallback(async (): Promise<void> => {
    if (!check || updating) return;
    setError("");
    setUpdating(true);
    setProgress({ state: "running", stage: "更新任务已提交，等待执行", percent: 1, version: check.latest_version });
    try {
      await startUpdate(check.latest_version);
      startPolling();
    } catch (err) {
      setUpdating(false);
      setError(err instanceof Error ? err.message : "触发更新失败");
    }
  }, [check, updating, startPolling]);

  const hasUpdate: boolean = check?.has_update ?? false;
  const badge = hasUpdate ? `新版本 ${formatVersion(check?.latest_version ?? "")}` : formatVersion(check?.current_version ?? "dev");

  return (
    // 紧凑底栏：内边距与菜单对齐（px-2），高度收敛，hover 圆角块与 NavList 视觉一致
    <div className="border-t border-line px-2 py-1.5">
      <div className="flex items-center gap-1">
        <button
          type="button"
          onClick={() => setOpen(true)}
          className="flex min-w-0 flex-1 items-center gap-1.5 rounded-lg px-3 py-1.5 text-left text-[11px] text-ink-2 transition-colors hover:bg-muted hover:text-ink"
          aria-label="站点版本与更新"
        >
          <span className={`inline-block h-1.5 w-1.5 shrink-0 rounded-full ${hasUpdate ? "animate-pulse bg-emerald-500" : "bg-line-strong"}`} />
          <span className={`truncate ${hasUpdate ? "font-medium text-emerald-600" : ""}`}>{badge}</span>
        </button>
        <a
          href="https://github.com/roberts9012062/boke"
          target="_blank"
          rel="noreferrer"
          className="shrink-0 rounded-lg px-2 py-1.5 text-ink-3 transition-colors hover:bg-muted hover:text-ink"
          aria-label="GitHub 源码仓库"
          title="GitHub 源码仓库"
        >
          <svg viewBox="0 0 16 16" className="h-3.5 w-3.5" fill="currentColor" aria-hidden="true">
            <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z" />
          </svg>
        </a>
      </div>

      {/* 版本与更新弹窗（宽度 600px；Modal 的 maxWidth 为 Tailwind 类） */}
      <Modal open={open} title="站点版本与更新" onClose={() => setOpen(false)} maxWidth="max-w-[600px]">
        <div className="space-y-4">
          {/* 版本信息 */}
          <div className="flex items-center justify-between rounded-lg border border-line px-4 py-3">
            <div>
              <p className="text-xs text-ink-3">当前版本</p>
              <p className="text-sm font-semibold text-ink">{formatVersion(check?.current_version ?? "dev")}</p>
            </div>
            <div className="text-right">
              <p className="text-xs text-ink-3">最新版本</p>
              <p className={`text-sm font-semibold ${hasUpdate ? "text-emerald-600" : "text-ink"}`}>
                {formatVersion(check?.latest_version ?? "dev")}
              </p>
            </div>
          </div>

          {/* 更新日志（有新版本且 Release 有说明时） */}
          {hasUpdate && check?.release_notes && (
            <div className="max-h-40 overflow-y-auto rounded-lg bg-muted px-4 py-3">
              <p className="mb-1 text-xs font-medium text-ink">更新日志</p>
              <pre className="whitespace-pre-wrap break-words font-sans text-xs leading-5 text-ink-2">
                {check.release_notes}
              </pre>
            </div>
          )}

          {/* 更新进度（执行中） */}
          {updating && (
            <div className="space-y-2">
              <div className="h-2 overflow-hidden rounded-full bg-muted">
                <div
                  className={`h-full rounded-full transition-all duration-500 ${progress.state === "failed" ? "bg-red-500" : "bg-emerald-500"}`}
                  style={{ width: `${Math.min(100, Math.max(0, progress.percent))}%` }}
                />
              </div>
              <p className="text-center text-xs text-ink-2">
                {progress.state === "failed"
                  ? `更新失败：${progress.message || "未知原因"}`
                  : `${progress.stage || "执行中…"}（${progress.percent}%）`}
              </p>
              {progress.state === "running" && (
                <p className="text-center text-[11px] text-ink-3">拉取代码 → 构建镜像 → 重启服务，全程自动完成</p>
              )}
            </div>
          )}

          {/* 完成提示 */}
          {progress.state === "done" && !updating && (
            <p className="rounded-lg bg-emerald-500/10 px-4 py-2.5 text-center text-sm text-emerald-700">
              已更新到 {formatVersion(progress.version ?? "")} ✓
            </p>
          )}

          {error && <p className="rounded-lg bg-red-500/10 px-4 py-2.5 text-sm text-red-600">{error}</p>}

          {/* 操作按钮 */}
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={() => setOpen(false)}
              className="rounded-lg border border-line px-4 py-2 text-sm text-ink-2 transition-colors hover:text-ink"
            >
              关闭
            </button>
            {hasUpdate && !updating && progress.state !== "done" && (
              <button
                type="button"
                onClick={() => void handleStart()}
                className="rounded-lg bg-emerald-600 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-emerald-600/85"
              >
                立即更新
              </button>
            )}
          </div>
        </div>
      </Modal>
    </div>
  );
}
