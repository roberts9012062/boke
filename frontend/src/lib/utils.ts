// src/lib/utils.ts
// 通用工具函数（纯函数，无副作用）。
// 说明：相对时间语义与设计稿一致（刚刚 / N 分钟前 / N 小时前 / 昨天 / N 天前）。

// timeAgo 将 ISO8601 时间转为相对时间文案。
// 参数：iso 时间字符串；返回相对时间（如「2 小时前」）。
export function timeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  const days = Math.floor(hours / 24);
  if (days === 1) return "昨天";
  if (days < 7) return `${days} 天前`;
  return new Date(iso).toLocaleDateString("zh-CN");
}

// excerpt 取字符串前 N 字符（按 Unicode 码点，避免截断中文）。
// 参数：text 原文；max 上限；返回截断文本（超长加省略号）。
export function excerpt(text: string, max: number): string {
  const runes = Array.from(text);
  if (runes.length <= max) {
    return text;
  }
  return runes.slice(0, max).join("") + "…";
}

// formatDateTime 格式化日期时间为「YYYY-MM-DD HH:mm」（后台发布信息面板，设计稿格式）。
// 参数：iso 时间字符串；解析失败时原样返回（避免页面崩溃）。
export function formatDateTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return iso;
  }
  const pad = (n: number) => n.toString().padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

// formatCompact 数字缩写（设计稿互动数据：128 赞 / 24 评 / 1.2k 览）。
// 参数：n 数值；≥1000 缩写为 x.xk（去掉末尾 .0），其余原样。
export function formatCompact(n: number): string {
  if (n >= 1000) {
    return (n / 1000).toFixed(1).replace(/\.0$/, "") + "k";
  }
  return String(n);
}

// formatDuration 秒数格式化为可读时长（设计稿审核队列「平均耗时 4m」）。
// 参数：seconds 秒数；<60s → Ns；<3600s → Nm；<86400s → Nh；其余 → Nd。
export function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.max(0, Math.round(seconds))}s`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
  if (seconds < 86400) return `${Math.round(seconds / 3600)}h`;
  return `${Math.round(seconds / 86400)}d`;
}
