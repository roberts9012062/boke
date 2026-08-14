// src/components/compose/music-embed.tsx
// 音乐内嵌 Node 扩展（Tiptap）：把音乐链接/网易云歌曲渲染为播放器。
// 两种形态：
//   - 第三方 iframe（QQ 音乐/旧网易云链接）：div 包裹 iframe（平台播放器）
//   - 网易云歌曲引用（M7 插件）：div 携带 data-music-id/标题/歌手/封面，渲染自研播放器
// 注意：React 节点视图必须用 NodeViewWrapper 包裹，序列化用 div 包裹 iframe，
//       否则 ProseMirror DOM 映射错乱（Position out of range）。
"use client";

import { Node } from "@tiptap/core";
import { NodeViewWrapper, ReactNodeViewRenderer, type ReactNodeViewProps } from "@tiptap/react";

import { MusicRefPlayer } from "@/components/music-ref-player";

// MusicEmbedAttrs 音乐嵌入节点属性（两种形态共用）。
interface MusicEmbedAttrs {
  src: string; // 第三方 iframe src（网易云引用为空）
  platform: string; // 平台：qq / netease
  kind: string; // 类型：song / playlist / album
  songId: string; // 网易云歌曲 ID（引用形态）
  title: string; // 歌名
  artist: string; // 歌手
  cover: string; // 封面
}

// MusicEmbedComponent 音乐内嵌节点视图（网易云引用→自研播放器；否则→iframe）。
function MusicEmbedComponent({ node }: ReactNodeViewProps) {
  const attrs = node.attrs as MusicEmbedAttrs;
  // 音乐引用（有 songId；网易云存 song_id / QQ 存 songmid）用自研播放器
  if (attrs.songId) {
    return (
      <NodeViewWrapper>
        <MusicRefPlayer
          songId={attrs.songId}
          title={attrs.title}
          artist={attrs.artist}
          coverUrl={attrs.cover}
          platform={attrs.platform === "qq" ? "qq" : "netease"}
        />
      </NodeViewWrapper>
    );
  }
  // 网易云单曲 66px；歌单/专辑 430px；QQ 音乐播放器 110px
  const height = attrs.platform === "netease" && attrs.kind === "song" ? 66 : attrs.platform === "netease" ? 430 : 110;
  return (
    <NodeViewWrapper className="my-3">
      <div className="overflow-hidden rounded-lg border border-line bg-elevated" data-music-embed={attrs.platform}>
        <iframe
          src={attrs.src}
          title={`内嵌音乐（${attrs.platform}）`}
          allow="autoplay"
          allowFullScreen
          style={{ width: "100%", height, border: 0, display: "block" }}
        />
      </div>
    </NodeViewWrapper>
  );
}

// MusicEmbed 音乐内嵌节点（div 包裹 iframe；序列化与渲染侧 sanitize 对齐）。
export const MusicEmbed = Node.create({
  name: "musicEmbed",

  group: "block",

  atom: true,

  addAttributes() {
    return {
      src: { default: "" },
      platform: { default: "" },
      kind: { default: "song" },
      songId: { default: "" },
      title: { default: "" },
      artist: { default: "" },
      cover: { default: "" },
    };
  },

  parseHTML() {
    return [
      {
        tag: "div[data-music-embed]",
        getAttrs: (el) => {
          const div = el as HTMLElement;
          const iframe = div.querySelector("iframe");
          return {
            src: iframe?.getAttribute("src") ?? div.getAttribute("data-music-src") ?? "",
            platform: div.getAttribute("data-music-embed") ?? "",
            kind: div.getAttribute("data-music-kind") ?? "song",
            songId: div.getAttribute("data-music-id") ?? "",
            title: div.getAttribute("data-music-title") ?? "",
            artist: div.getAttribute("data-music-artist") ?? "",
            cover: div.getAttribute("data-music-cover") ?? "",
          };
        },
      },
    ];
  },

  renderHTML({ node }) {
    const attrs = node.attrs as MusicEmbedAttrs;
    // 网易云引用形态：div 携带数据属性（无 iframe，渲染侧挂自研播放器）
    if (attrs.songId) {
      return [
        "div",
        {
          "data-music-embed": attrs.platform,
          "data-music-kind": attrs.kind,
          "data-music-id": attrs.songId,
          "data-music-title": attrs.title,
          "data-music-artist": attrs.artist,
          "data-music-cover": attrs.cover,
        },
      ];
    }
    return [
      "div",
      { "data-music-embed": attrs.platform, "data-music-kind": attrs.kind },
      [
        "iframe",
        {
          src: attrs.src,
          title: `内嵌音乐（${attrs.platform}）`,
        },
      ],
    ];
  },

  addNodeView() {
    return ReactNodeViewRenderer(MusicEmbedComponent);
  },
});
