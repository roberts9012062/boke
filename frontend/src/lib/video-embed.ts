// src/lib/video-embed.ts
// 视频外链解析（纯函数）：把用户粘贴的视频链接解析为可内嵌的 iframe src。
// 支持平台白名单：bilibili / YouTube / 腾讯视频 / Vimeo（含常见链接形态）。
// 不支持或无法解析时返回 null，调用方降级为普通外链。

// VideoEmbed 解析结果。
export interface VideoEmbed {
  platform: "bilibili" | "youtube" | "tencent" | "vimeo"; // 平台标识
  embedUrl: string; // 可内嵌的 iframe src（已编码）
}

// youtubeEmbed 解析 YouTube 链接（watch / shorts / youtu.be）。
function youtubeEmbed(url: string): VideoEmbed | null {
  let id = "";
  const m = url.match(/youtube\.com\/watch\?.*v=([A-Za-z0-9_-]{6,})/);
  if (m) {
    id = m[1];
  } else {
    const shorts = url.match(/youtube\.com\/shorts\/([A-Za-z0-9_-]{6,})/);
    if (shorts) {
      id = shorts[1];
    } else {
      const youtu = url.match(/youtu\.be\/([A-Za-z0-9_-]{6,})/);
      if (youtu) {
        id = youtu[1];
      }
    }
  }
  if (!id) {
    return null;
  }
  return { platform: "youtube", embedUrl: `https://www.youtube.com/embed/${id}` };
}

// bilibiliEmbed 解析 bilibili 链接（BV / av 号）。
function bilibiliEmbed(url: string): VideoEmbed | null {
  const bv = url.match(/bilibili\.com\/video\/(BV[0-9A-Za-z]+)/);
  if (bv) {
    return { platform: "bilibili", embedUrl: `https://player.bilibili.com/player.html?bvid=${bv[1]}&page=1` };
  }
  const av = url.match(/bilibili\.com\/video\/av(\d+)/);
  if (av) {
    return { platform: "bilibili", embedUrl: `https://player.bilibili.com/player.html?aid=${av[1]}&page=1` };
  }
  return null;
}

// tencentEmbed 解析腾讯视频链接（v.qq.com/x/cover/.../VID.html 或 ?vid=）。
function tencentEmbed(url: string): VideoEmbed | null {
  const pathVid = url.match(/v\.qq\.com\/.*\/([A-Za-z0-9]{11})\.html/);
  if (pathVid) {
    return { platform: "tencent", embedUrl: `https://v.qq.com/txp/iframe/player.html?vid=${pathVid[1]}` };
  }
  const queryVid = url.match(/[?&]vid=([A-Za-z0-9]{11})/);
  if (queryVid) {
    return { platform: "tencent", embedUrl: `https://v.qq.com/txp/iframe/player.html?vid=${queryVid[1]}` };
  }
  return null;
}

// vimeoEmbed 解析 Vimeo 链接（vimeo.com/数字ID）。
function vimeoEmbed(url: string): VideoEmbed | null {
  const m = url.match(/vimeo\.com\/(\d+)/);
  if (!m) {
    return null;
  }
  return { platform: "vimeo", embedUrl: `https://player.vimeo.com/video/${m[1]}` };
}

// parseVideoEmbed 解析视频链接 → 内嵌信息（不支持返回 null）。
export function parseVideoEmbed(raw: string): VideoEmbed | null {
  const url = raw.trim();
  if (!url) {
    return null;
  }
  if (url.includes("youtube.com") || url.includes("youtu.be")) {
    return youtubeEmbed(url);
  }
  if (url.includes("bilibili.com")) {
    return bilibiliEmbed(url);
  }
  if (url.includes("v.qq.com")) {
    return tencentEmbed(url);
  }
  if (url.includes("vimeo.com")) {
    return vimeoEmbed(url);
  }
  return null;
}
