// src/lib/music-embed.ts
// 音乐外链解析（纯函数）：把用户粘贴的音乐链接解析为可内嵌的 iframe src。
// 支持平台：网易云音乐 / QQ音乐（均用官方 iframe 播放器）。
// 不支持或无法解析时返回 null，调用方提示。
//
// 注意（M7 修复）：QQ 音乐外链播放器已不再支持 songmid 参数（2026-08 实测），
// 只支持 songid（数字 ID）。因此 QQ 链接解析只提取 songmid，由调用方通过后端
// /api/v1/music/qq-resolve 异步换取 songid 后再生成播放器 URL。

// MusicEmbed 解析结果。
export interface MusicEmbed {
  platform: "netease" | "qq"; // 平台标识
  embedUrl: string; // 可内嵌的 iframe src（网易云直接生成；QQ 需异步解析后生成）
  kind: "song" | "playlist" | "album"; // 网易云类型（QQ 仅有单曲）
  songmid?: string; // QQ 音乐歌曲 MID（QQ 平台才有，供后端解析 songid）
  songId?: string; // 网易云歌曲 ID（单曲，走自研播放器拿真实地址）
}

// neteaseEmbed 解析网易云音乐链接（单曲/歌单/专辑）。
function neteaseEmbed(url: string): MusicEmbed | null {
  const idMatch =
    url.match(/music\.163\.com\/#\/song\?id=(\d+)/) ||
    url.match(/music\.163\.com\/song\?id=(\d+)/) ||
    url.match(/music\.163\.com\/song\/(\d+)/);
  if (idMatch) {
    return {
      platform: "netease",
      kind: "song",
      embedUrl: `https://music.163.com/outchain/player?type=2&id=${idMatch[1]}&auto=0&height=66`,
      songId: idMatch[1], // 单曲 ID：调用方走自研播放器（插件取真实地址）
    };
  }
  const playlist = url.match(/music\.163\.com\/#\/playlist\?id=(\d+)/) || url.match(/music\.163\.com\/playlist\?id=(\d+)/);
  if (playlist) {
    return {
      platform: "netease",
      kind: "playlist",
      embedUrl: `https://music.163.com/outchain/player?type=0&id=${playlist[1]}&auto=0&height=430`,
    };
  }
  const album = url.match(/music\.163\.com\/#\/album\?id=(\d+)/) || url.match(/music\.163\.com\/album\?id=(\d+)/);
  if (album) {
    return {
      platform: "netease",
      kind: "album",
      embedUrl: `https://music.163.com/outchain/player?type=1&id=${album[1]}&auto=0&height=430`,
    };
  }
  return null;
}

// qqEmbed 解析 QQ 音乐链接（歌曲详情页 / 分享页，取 songmid；播放器 URL 需 songid，异步解析）。
function qqEmbed(url: string): MusicEmbed | null {
  // songDetail/{songmid} 或 share song.html?songmid={songmid}
  const songmid =
    url.match(/y\.qq\.com\/n\/ryqq\/songDetail\/([A-Za-z0-9]+)/)?.at(1) ||
    url.match(/[?&]songmid=([A-Za-z0-9]+)/)?.at(1) ||
    url.match(/i\.y\.qq\.com\/.*song\.html\?songmid=([A-Za-z0-9]+)/)?.at(1);
  if (!songmid) {
    return null;
  }
  // embedUrl 留空：播放器仅认 songid，由调用方经后端 /api/v1/music/qq-resolve 换取
  return {
    platform: "qq",
    kind: "song",
    embedUrl: "",
    songmid,
  };
}

// parseMusicEmbed 解析音乐链接 → 内嵌信息（不支持返回 null）。
export function parseMusicEmbed(raw: string): MusicEmbed | null {
  const url = raw.trim();
  if (!url) {
    return null;
  }
  if (url.includes("music.163.com")) {
    return neteaseEmbed(url);
  }
  if (url.includes("y.qq.com") || url.includes("i.y.qq.com")) {
    return qqEmbed(url);
  }
  return null;
}

// qqPlayerURL 用 songid 生成 QQ 音乐外链播放器 URL（纯函数；M7：播放器仅认 songid）。
export function qqPlayerURL(songid: number | string): string {
  return `https://i.y.qq.com/n2/m/outchain/player/index.html?songid=${songid}`;
}
