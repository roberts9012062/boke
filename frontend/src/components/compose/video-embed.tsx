// src/components/compose/video-embed.tsx
// 视频内嵌 Node 扩展（Tiptap）：把解析后的视频链接渲染为 iframe 播放器。
// 注意（修复 RangeError）：atom 节点的 renderHTML 不能输出裸 <iframe>，否则 ProseMirror
// 把 iframe 当节点视图根元素，点击时 posAtCoords 算出越界位置抛「Position out of range」。
// 必须用 div 包裹 iframe（对齐官方 youtube 扩展的 div[data-youtube-video] 模式）。
"use client";

import { Node } from "@tiptap/core";
import { NodeViewWrapper, ReactNodeViewRenderer, type ReactNodeViewProps } from "@tiptap/react";

// VideoEmbedComponent 视频内嵌节点视图（16:9 iframe 播放器）。
// 注意：React 节点视图必须用 NodeViewWrapper 包裹，否则 tiptap 报
// 「Please use the NodeViewWrapper component」且 ProseMirror DOM 映射错乱（Position out of range）。
function VideoEmbedComponent({ node }: ReactNodeViewProps) {
  const attrs = node.attrs as { src: string; platform: string };
  const { src, platform } = attrs;
  return (
    <NodeViewWrapper className="my-3">
      <div className="overflow-hidden rounded-lg border border-line bg-black" data-video-embed={platform}>
        <div className="relative aspect-video w-full">
          <iframe
            src={src}
            title={`内嵌视频（${platform}）`}
            allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; fullscreen"
            allowFullScreen
            className="absolute inset-0 h-full w-full border-0"
          />
        </div>
      </div>
    </NodeViewWrapper>
  );
}

// VideoEmbed 视频内嵌节点（div 包裹 iframe；序列化与渲染侧 sanitize 对齐）。
export const VideoEmbed = Node.create({
  name: "videoEmbed",

  group: "block",

  atom: true,

  addAttributes() {
    return {
      src: { default: "" },
      platform: { default: "" },
    };
  },

  parseHTML() {
    return [
      // 新版：div[data-video-embed] 包裹 iframe（编辑回填）
      {
        tag: "div[data-video-embed]",
        getAttrs: (el) => {
          const div = el as HTMLElement;
          const iframe = div.querySelector("iframe");
          return {
            src: iframe?.getAttribute("src") ?? div.getAttribute("data-video-src") ?? "",
            platform: div.getAttribute("data-video-embed") ?? "",
          };
        },
      },
      // 兼容旧版：裸 iframe[data-video-platform]（早期测试帖）
      {
        tag: "iframe[data-video-platform]",
        getAttrs: (el) => {
          const iframe = el as HTMLElement;
          return {
            src: iframe.getAttribute("src") ?? "",
            platform: iframe.getAttribute("data-video-platform") ?? "",
          };
        },
      },
    ];
  },

  renderHTML({ node }) {
    const attrs = node.attrs as { src: string; platform: string };
    return [
      "div",
      { "data-video-embed": attrs.platform },
      [
        "iframe",
        {
          src: attrs.src,
          title: `内嵌视频（${attrs.platform}）`,
          allowfullscreen: "true",
        },
      ],
    ];
  },

  addNodeView() {
    return ReactNodeViewRenderer(VideoEmbedComponent);
  },
});
