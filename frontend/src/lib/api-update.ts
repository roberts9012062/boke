// frontend/src/lib/api-update.ts
// 站点更新 API 客户端：版本检查 / 触发更新 / 进度轮询（后台左下角更新徽标数据源）。

import { get, post } from "@/lib/api";

// UpdateStatus 更新执行进度（宿主机更新代理写回，后端转发）。
export interface UpdateStatus {
  state: "idle" | "running" | "done" | "failed";
  stage?: string;
  percent: number;
  version?: string;
  message?: string;
  updated_at?: string;
}

// UpdateCheck 版本检查结果。
export interface UpdateCheck {
  current_version: string;
  latest_version: string;
  has_update: boolean;
  release_notes: string;
  repo_url: string;
  running_update: UpdateStatus;
}

// UpdateProgress 更新进度查询结果（含最新当前版本，完成后用于确认版本切换）。
export interface UpdateProgress {
  status: UpdateStatus;
  current_version: string;
}

// checkUpdate 检查版本（当前版本 vs 仓库最新 Release，附更新日志与进行中进度）。
export function checkUpdate(): Promise<UpdateCheck> {
  return get<UpdateCheck>("/api/v1/admin/update/check");
}

// startUpdate 触发更新（写入任务文件，宿主机代理约 1 分钟内调度执行）。
export function startUpdate(version: string): Promise<{ started: boolean; version: string }> {
  return post<{ started: boolean; version: string }>("/api/v1/admin/update/start", { version });
}

// fetchUpdateProgress 查询更新进度（前端轮询；服务重启期间请求失败由调用方容错重试）。
export function fetchUpdateProgress(): Promise<UpdateProgress> {
  return get<UpdateProgress>("/api/v1/admin/update/status");
}
