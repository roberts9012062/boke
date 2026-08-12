// src/components/compose/config.ts
// 发帖中心配置常量与媒体辅助（从 compose 页抽出，保持页面行数 ≤300 规范）。
import type { MediaDTO, PostContentType } from "@/types/api";

// 正文上限（需求 3.4：0/2000）
export const MAX_CONTENT = 2000;
// 标签上限
export const MAX_TAGS = 5;

// 可见性选项（设计稿《可见性》弹层：公开/仅关注者/仅自己）
export const VISIBILITY_OPTIONS = [
  { key: "public", label: "公开", desc: "所有人可见，可被推荐" },
  { key: "followers", label: "仅关注者", desc: "互相关注的人可见" },
  { key: "private", label: "仅自己", desc: "草稿箱式私密，不出现在主页" },
] as const;

// 类型 Tab（文案与设计稿一致；文字/图片/音频/视频四类型 M2 全开放）
export const TYPE_TABS: readonly { key: PostContentType; label: string }[] = [
  { key: "text", label: "文字" },
  { key: "image", label: "图片" },
  { key: "audio", label: "音频" },
  { key: "video", label: "视频" },
];

// collectMediaIds 组装媒体 ID（图片多选 / 音频单选 / 视频单选；纯函数）。
export function collectMediaIds(
  contentType: PostContentType,
  images: MediaDTO[],
  audio: MediaDTO | null,
  video: MediaDTO | null,
): number[] {
  if (contentType === "image") {
    return images.map((m) => m.id);
  }
  if (contentType === "audio") {
    return audio ? [audio.id] : [];
  }
  if (contentType === "video") {
    return video ? [video.id] : [];
  }
  // 文字等其他类型：不携带媒体
  // （历史兜底：text 分支返回 audio 会导致先传音频再切回「文字」时纯文字帖附带残留音频）
  return [];
}
