// src/components/compose/bilibili-embed.tsx
// B站视频内嵌 Node 扩展（Tiptap · bilibili-video 插件）：
//   编辑器内以封面卡片预览；序列化为插件内容块协议
//   <div data-plugin-block="bilibili" data-props='{...}'></div>，
//   渲染侧经块注册表分发到插件 player.js 播放器（清晰度可选）。
// 注意：React 节点视图必须用 NodeViewWrapper 包裹（ProseMirror DOM 映射约束，同 music-embed）。
"use client";

import { Node } from "@tiptap/core";
import { NodeViewWrapper, ReactNodeViewRenderer, type ReactNodeViewProps } from "@tiptap/react";

// BilibiliQuality 清晰度档位（插件 /resolve 返回）。
export interface BilibiliQuality {
  qn: number; // 档位值（16/32/64/80）
  desc: string; // 展示名（360P~1080P）
  need_login: boolean; // 是否需要登录 B 站
}

// BilibiliEmbedAttrs B站视频块节点属性。
export interface BilibiliEmbedAttrs {
  bvid: string; // BV 号
  cid: number; // 第一 P cid（playurl 必需）
  title: string; // 标题
  cover: string; // 封面 URL
  author: string; // UP 主
  duration: number; // 时长（秒）
  quality: number; // 作者所选默认清晰度 qn
  qualitiesJson: string; // 清晰度档位表 JSON（序列化存储用）
}

// attrsToProps 节点属性 → 块协议 data-props 对象（纯函数）。
function attrsToProps(attrs: BilibiliEmbedAttrs): Record<string, unknown> {
  let qualities: BilibiliQuality[] = [];
  try {
    qualities = JSON.parse(attrs.qualitiesJson || "[]") as BilibiliQuality[];
  } catch {
    qualities = [];
  }
  return {
    bvid: attrs.bvid,
    cid: attrs.cid,
    title: attrs.title,
    cover: attrs.cover,
    author: attrs.author,
    duration: attrs.duration,
    quality: attrs.quality,
    qualities,
  };
}

// propsToAttrs 块协议 data-props → 节点属性（纯函数；字段缺失回退默认值）。
function propsToAttrs(raw: string | null): Partial<BilibiliEmbedAttrs> {
  if (!raw) {
    return {};
  }
  try {
    const p = JSON.parse(raw) as Partial<{
      bvid: string; cid: number; title: string; cover: string;
      author: string; duration: number; quality: number; qualities: BilibiliQuality[];
    }>;
    return {
      bvid: p.bvid ?? "",
      cid: p.cid ?? 0,
      title: p.title ?? "",
      cover: p.cover ?? "",
      author: p.author ?? "",
      duration: p.duration ?? 0,
      quality: p.quality ?? 32,
      qualitiesJson: JSON.stringify(p.qualities ?? []),
    };
  } catch {
    return {};
  }
}

// qualityDesc 当前清晰度展示名（纯函数）。
function qualityDesc(attrs: BilibiliEmbedAttrs): string {
  let qualities: BilibiliQuality[] = [];
  try {
    qualities = JSON.parse(attrs.qualitiesJson || "[]") as BilibiliQuality[];
  } catch {
    qualities = [];
  }
  const hit = qualities.find((q) => q.qn === attrs.quality);
  return hit ? hit.desc : String(attrs.quality);
}

// BilibiliEmbedComponent 节点视图（封面卡片预览）。
function BilibiliEmbedComponent({ node }: ReactNodeViewProps) {
  const attrs = node.attrs as BilibiliEmbedAttrs;
  return (
    <NodeViewWrapper>
      <div className="my-3 overflow-hidden rounded-lg border border-line bg-elevated">
        <div className="relative">
          {attrs.cover ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={attrs.cover} alt={attrs.title} referrerPolicy="no-referrer"
              className="block aspect-video w-full object-cover" />
          ) : (
            <div className="flex aspect-video w-full items-center justify-center bg-muted text-3xl text-ink-3">▶</div>
          )}
          <span className="absolute bottom-2 right-2 rounded bg-black/60 px-2 py-0.5 text-xs text-white">
            {Math.floor((attrs.duration || 0) / 60)}:{String((attrs.duration || 0) % 60).padStart(2, "0")}
          </span>
          <span className="absolute left-2 top-2 rounded bg-[#FB7299] px-2 py-0.5 text-xs font-medium text-white">
            B站 · {qualityDesc(attrs)}
          </span>
        </div>
        <div className="px-3 py-2">
          <p className="truncate text-sm font-medium text-ink">{attrs.title || attrs.bvid}</p>
          <p className="mt-0.5 text-xs text-ink-3">UP：{attrs.author || "未知"} · 哔哩哔哩（高清需 B 站插件启用）</p>
        </div>
      </div>
    </NodeViewWrapper>
  );
}

// BilibiliEmbed B站视频块节点（序列化与插件块协议对齐）。
export const BilibiliEmbed = Node.create({
  name: "bilibiliEmbed",

  group: "block",

  atom: true,

  addAttributes() {
    return {
      bvid: { default: "" },
      cid: { default: 0 },
      title: { default: "" },
      cover: { default: "" },
      author: { default: "" },
      duration: { default: 0 },
      quality: { default: 32 },
      qualitiesJson: { default: "[]" },
    };
  },

  parseHTML() {
    return [
      {
        tag: "div[data-plugin-block=bilibili]",
        getAttrs: (el) => propsToAttrs((el as HTMLElement).getAttribute("data-props")),
      },
    ];
  },

  renderHTML({ node }) {
    const attrs = node.attrs as BilibiliEmbedAttrs;
    return [
      "div",
      {
        "data-plugin-block": "bilibili",
        "data-props": JSON.stringify(attrsToProps(attrs)),
      },
    ];
  },

  addNodeView() {
    return ReactNodeViewRenderer(BilibiliEmbedComponent);
  },
});
