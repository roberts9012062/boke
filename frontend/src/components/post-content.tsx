// src/components/post-content.tsx
// 帖子正文渲染（M5 富文本）：按 content_format 分支——
//   html → DOMPurify 消毒后渲染（支持图片/视频内嵌/链接）
//   markdown/空 → 复用 Markdown 组件（旧帖兼容）
// M7 增强：html 正文中的「网易云音乐引用」节点（div[data-music-embed="netease"][data-music-id]）
//   拆分为 React 组件渲染（自研播放器，实时经公开端点取地址），避免 createRoot 与
//   dangerouslySetInnerHTML 混用导致 React 渲染冲突（历史 bug：渲染期间 unmount）。
"use client";

import { useMemo } from "react";

import { Markdown } from "@/components/markdown";
import { MusicRefPlayer } from "@/components/music-ref-player";
import { sanitizeHtml } from "@/lib/sanitize";

// Segment 正文分段（html 文本段 / 网易云音乐引用）。
type Segment =
  | { type: "html"; html: string }
  | { type: "music"; songId: string; title: string; artist: string; cover: string; platform: string };

// escapeHtml 转义文本节点（纯函数；作为 __html 时防 < > & 被当标签解析）。
function escapeHtml(text: string): string {
  return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

// splitContent 将 sanitize 后的 HTML 按网易云音乐引用节点拆分为段落（纯函数）。
function splitContent(html: string): Segment[] {
  const doc = new DOMParser().parseFromString(html, "text/html");
  const segments: Segment[] = [];
  let htmlBuf = "";
  const flush = () => {
    if (htmlBuf.trim()) {
      segments.push({ type: "html", html: htmlBuf });
      htmlBuf = "";
    }
  };
  for (const child of Array.from(doc.body.childNodes)) {
    if (child.nodeType === Node.ELEMENT_NODE) {
      const el = child as HTMLElement;
      const platform = el.getAttribute("data-music-embed") ?? "";
      if ((platform === "netease" || platform === "qq") && el.getAttribute("data-music-id")) {
        flush();
        segments.push({
          type: "music",
          songId: el.getAttribute("data-music-id") ?? "",
          title: el.getAttribute("data-music-title") ?? "",
          artist: el.getAttribute("data-music-artist") ?? "",
          cover: el.getAttribute("data-music-cover") ?? "",
          platform,
        });
      } else {
        htmlBuf += el.outerHTML;
      }
    } else if (child.nodeType === Node.TEXT_NODE) {
      htmlBuf += escapeHtml(child.textContent ?? "");
    }
  }
  flush();
  return segments;
}

// PostContent 帖子正文渲染。
// 参数：content 正文；format 格式（html / markdown，空=markdown）。
export function PostContent({ content, format }: { content: string; format?: string }) {
  const segments = useMemo(
    () => (format === "html" ? splitContent(sanitizeHtml(content)) : []),
    [content, format],
  );

  if (format === "html") {
    return (
      <div className="rich-content break-words text-[15px] leading-relaxed text-ink">
        {segments.map((seg, i) =>
          seg.type === "music" ? (
            <MusicRefPlayer
              key={`music-${seg.songId}-${i}`}
              songId={seg.songId}
              title={seg.title}
              artist={seg.artist}
              coverUrl={seg.cover}
              platform={seg.platform === "qq" ? "qq" : "netease"}
            />
          ) : (
            // 已过 DOMPurify 消毒（iframe 白名单）
            <div key={`html-${i}`} dangerouslySetInnerHTML={{ __html: seg.html }} />
          ),
        )}
      </div>
    );
  }
  return (
    <div className="break-words text-[15px] leading-relaxed text-ink">
      <Markdown content={content} />
    </div>
  );
}
