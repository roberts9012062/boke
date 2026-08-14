// src/lib/sanitize.ts
// 富文本 HTML 消毒（渲染侧安全）：DOMPurify 清除危险标签/属性，
// 并用 hook 限制 iframe 仅允许白名单视频平台域名（防 XSS 内嵌任意 iframe）。
// 说明：DOMPurify.addHook 为全局累积副作用，模块加载时仅注册一次。
"use client";

import DOMPurify from "dompurify";

// 视频平台 iframe 域名白名单（与 video-embed.ts 解析结果一致）。
const ALLOWED_IFRAME_HOSTS = [
  "player.bilibili.com",
  "www.youtube.com",
  "www.youtube-nocookie.com",
  "v.qq.com",
  "player.vimeo.com",
  "music.163.com", // 网易云音乐外链播放器
  "i.y.qq.com", // QQ音乐外链播放器
];

// iframeHostAllowed 判断 iframe src 域名是否在白名单内（纯函数）。
function iframeHostAllowed(value: string): boolean {
  return ALLOWED_IFRAME_HOSTS.some((host) => {
    const idx = value.indexOf(host);
    return idx !== -1 && (idx === 0 || value[idx - 1] === "/" || value[idx - 1] === "." || value[idx - 1] === ":" || value[idx - 1] === "?");
  });
}

// 模块加载时注册一次 iframe 白名单 hook（幂等）。
let hookRegistered = false;
function ensureHook(): void {
  if (hookRegistered) {
    return;
  }
  hookRegistered = true;
  DOMPurify.addHook("uponSanitizeAttribute", (node, data) => {
    if (data.attrName === "src" && node.tagName === "IFRAME") {
      if (!iframeHostAllowed(String(data.attrValue ?? ""))) {
        node.remove(); // 非白名单 iframe 整体移除
      }
    }
  });
}

// sanitizeHtml 消毒富文本 HTML：去危险内容 + iframe 域名白名单。
export function sanitizeHtml(html: string): string {
  ensureHook();
  return DOMPurify.sanitize(html, {
    // 允许 iframe（视频内嵌）；其余按默认白名单
    ADD_TAGS: ["iframe"],
    ADD_ATTR: ["allow", "allowfullscreen", "frameborder", "scrolling", "title"],
  });
}
