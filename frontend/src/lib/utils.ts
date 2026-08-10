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
