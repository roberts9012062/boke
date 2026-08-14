// src/lib/rich-text.ts
// 富文本辅助（纯函数，客户端侧 DOM API 用于字数统计）。
// htmlToText：将富文本 HTML 转为纯文本（DOM 解析去标签），与后端 plainText 语义对齐。

// htmlToText 从 HTML 提取纯文本（用于前端字数统计；img/iframe 等不产生可见文本）。
export function htmlToText(html: string): string {
  if (typeof document === "undefined") {
    // SSR 兜底：正则去标签 + 反转义
    return html
      .replace(/<[^>]*>/g, "")
      .replace(/&nbsp;/g, " ")
      .replace(/&amp;/g, "&")
      .replace(/&lt;/g, "<")
      .replace(/&gt;/g, ">");
  }
  const el = document.createElement("div");
  el.innerHTML = html;
  return (el.textContent ?? "").replace(/\u00a0/g, " ");
}
