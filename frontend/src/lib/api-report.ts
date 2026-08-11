// src/lib/api-report.ts
// 数据报表 + 备份导出 API（M4-报表，设计稿《数据报表》《备份导出》）。
// 说明：独立文件避免 api.ts 持续膨胀；附件下载走 fetch blob（带 Bearer，不走 JSON 封装）。
import { get, post, del } from "./api";

// ---------- 数据报表 ----------

// ReportTrendPoint 趋势点（四维：浏览/新帖/获赞/评论，按日）。
export interface ReportTrendPoint {
  date: string; // 日期 MM-DD
  views: number; // 浏览
  posts: number; // 新帖
  likes: number; // 获赞
  comments: number; // 评论
}

// ReportPending 待处理块（设计稿：评论待审/内容举报/敏感词命中）。
export interface ReportPending {
  comments: number; // 评论待审
  reports: number; // 内容举报
  sensitive: number; // 敏感词命中
}

// ReportOverview 报表页聚合数据。
export interface ReportOverview {
  views_7d: number; // 近 7 日浏览
  views_trend: number; // 环比（%）
  likes_7d: number; // 近 7 日获赞
  likes_trend: number; // 环比（%）
  comments_7d: number; // 近 7 日评论
  comments_trend: number; // 环比（%）
  posts_today: number; // 今日新帖
  pending_audit: number; // 待审需处理（统计卡徽标）
  trend: ReportTrendPoint[]; // 四维趋势
  type_counts: Record<string, number>; // 内容分布（环形图）
  activities: ActivityRow[]; // 最近动态（与仪表盘同构）
  pending: ReportPending; // 待处理块
}

// ActivityRow 最近动态（与仪表盘 /admin/dashboard 的 activities 同构）。
export interface ActivityRow {
  kind: string; // post / comment / user
  id: number; // 对象 ID
  actor: string; // 行为者昵称
  content: string; // 内容摘要
  created_at: string; // 时间
}

// 报表聚合（days=7|30，默认 30）。
export function apiReportOverview(days: number): Promise<ReportOverview> {
  return get<ReportOverview>(`/admin/reports/overview?days=${days}`);
}

// 导出趋势 CSV（附件下载：fetch blob → a.download 触发）。
export async function apiReportExportCsv(days: number): Promise<void> {
  await downloadFile(`/admin/reports/export.csv?days=${days}`, `trend-${days}d.csv`);
}

// ---------- 备份导出 ----------

// BackupDTO 备份记录（后台列表）。
export interface BackupDTO {
  id: number; // 记录 ID
  type: string; // all 全站数据 / media 媒体库
  status: string; // success / failed
  file_name: string; // 文件名
  file_size: number; // 大小（字节）
  created_at: string; // 备份时间
}

// BackupInput 创建备份输入（设计稿表单字段）。
export interface BackupInput {
  backup_type: string; // all / media
  scope: string[]; // content/users/media
  format: string; // json/csv/zip
  retention_days: number; // 保留天数
}

// 备份记录列表。
export function apiBackups(): Promise<{ items: BackupDTO[] }> {
  return get<{ items: BackupDTO[] }>("/admin/backups");
}

// 创建备份。
export function apiCreateBackup(input: BackupInput): Promise<BackupDTO> {
  return post<BackupDTO>("/admin/backups", input);
}

// 下载备份文件（附件下载）。
export async function apiBackupDownload(id: number, fileName: string): Promise<void> {
  await downloadFile(`/admin/backups/${id}/download`, fileName);
}

// 删除备份（文件 + 记录）。
export function apiDeleteBackup(id: number): Promise<void> {
  return del<void>(`/admin/backups/${id}`);
}

// ---------- 附件下载辅助 ----------

// downloadFile 带鉴权的附件下载（fetch blob → 临时 URL → a.download 触发）。
// 说明：后端附件接口不走统一 JSON 响应，需独立 fetch（带 Bearer，失败抛 ApiError 风格消息）。
async function downloadFile(path: string, fileName: string): Promise<void> {
  // 从 localStorage 读取令牌（键与 auth.tsx 一致）
  const raw = localStorage.getItem("yueyan-tokens");
  const accessToken = raw ? (JSON.parse(raw) as { access_token?: string }).access_token ?? "" : "";

  const res = await fetch(`/api/v1${path}`, {
    headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {},
  });
  if (!res.ok) {
    // 尝试解析统一错误体（后端错误走 JSON；附件成功为文件流）
    let message = "下载失败";
    try {
      const body = (await res.json()) as { message?: string };
      if (body.message) {
        message = body.message;
      }
    } catch {
      // 非 JSON（如 404 页面）：保留默认消息
    }
    throw new Error(message);
  }
  const blob = await res.blob();
  // 触发浏览器下载（临时对象 URL + a[download]）
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}
