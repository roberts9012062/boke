// src/lib/share.ts
// 分享工具（#17 完整实现）：二维码生成（qrcode 库）+ 分享海报 canvas 绘制 + 图片下载。
// 说明：海报样式为冷月夜色风（品牌/标题/正文/作者·话题/二维码+链接），设计稿仅提供入口无结果页，样式为合理补充。
import QRCode from "qrcode";

// 海报画布尺寸（800×1120 竖版，适合社交分享）。
const POSTER_W = 800;
const POSTER_H = 1120;
// 海报配色（冷月令牌：bg #0b0f1a / 文字 #e8ecf4 / 强调 #a8b8d8 / 次文字 #8a94ab）。
const POSTER_BG_TOP = "#0b0f1a";
const POSTER_BG_BOTTOM = "#1a2233";
const POSTER_TEXT = "#e8ecf4";
const POSTER_TEXT_SUB = "#8a94ab";
const POSTER_ACCENT = "#a8b8d8";
const POSTER_BORDER = "#2a3348";

// ShareContent 海报内容（与分享面板入参一致）。
export interface ShareContent {
  title: string; // 标题（可空）
  content: string; // 正文摘要
  author: string; // 作者昵称
  tags: string; // 话题（如「#月色 #夜读」）
  link: string; // 分享链接
  media?: string[]; // 图片 URL（图片帖；最多取前 3 张，加载失败自动跳过）
}

// qrDataUrl 生成二维码图片（dataURL PNG）。
// 参数：text 编码内容；size 边长（px）；dark/light 前景/背景色。
export async function qrDataUrl(text: string, size: number, dark = "#0b0f1a", light = "#ffffff"): Promise<string> {
  return QRCode.toDataURL(text, {
    width: size,
    margin: 2,
    errorCorrectionLevel: "M",
    color: { dark, light },
  });
}

// wrapText canvas 文字按宽度换行截断（返回行数组，最多 maxLines 行，超出加省略号）。
function wrapText(ctx: CanvasRenderingContext2D, text: string, maxWidth: number, maxLines: number): string[] {
  const lines: string[] = [];
  let current = "";
  for (const char of text) {
    const test = current + char;
    if (ctx.measureText(test).width > maxWidth && current !== "") {
      lines.push(current);
      current = char;
      if (lines.length === maxLines - 1) {
        break;
      }
    } else {
      current = test;
    }
  }
  // 收尾：最后一行（超长加省略号）
  if (lines.length < maxLines) {
    if (ctx.measureText(current + "…").width > maxWidth && current !== "") {
      // 逐字回退至可容纳省略号
      while (current !== "" && ctx.measureText(current + "…").width > maxWidth) {
        current = current.slice(0, -1);
      }
      current += "…";
    }
    lines.push(current);
  }
  return lines;
}

// loadImage 加载图片（跨域匿名请求；失败返回 null 由调用方跳过）。
async function loadImage(url: string): Promise<HTMLImageElement | null> {
  return new Promise((resolve) => {
    const img = new Image();
    img.crossOrigin = "anonymous";
    img.onload = () => resolve(img);
    img.onerror = () => resolve(null);
    img.src = url;
  });
}

// drawSharePoster 绘制分享海报（canvas → dataURL PNG）。
// 参数：content 海报内容（含图片 URL）；qr 二维码 dataURL（可空，空则海报不含二维码）。
// 返回：海报 dataURL；绘制失败抛错（调用方降级为「仅二维码」视图）。
export async function drawSharePoster(content: ShareContent, qr: string): Promise<string> {
  const canvas = document.createElement("canvas");
  canvas.width = POSTER_W;
  canvas.height = POSTER_H;
  const ctx = canvas.getContext("2d");
  if (!ctx) {
    throw new Error("canvas 不可用");
  }

  // ---------- 背景（冷月夜色垂直渐变） ----------
  const bg = ctx.createLinearGradient(0, 0, 0, POSTER_H);
  bg.addColorStop(0, POSTER_BG_TOP);
  bg.addColorStop(1, POSTER_BG_BOTTOM);
  ctx.fillStyle = bg;
  ctx.fillRect(0, 0, POSTER_W, POSTER_H);

  // ---------- 月亮装饰（右上角半透明圆环） ----------
  ctx.save();
  ctx.globalAlpha = 0.12;
  ctx.strokeStyle = POSTER_ACCENT;
  ctx.lineWidth = 3;
  ctx.beginPath();
  ctx.arc(660, 150, 96, 0, Math.PI * 2);
  ctx.stroke();
  ctx.globalAlpha = 0.08;
  ctx.beginPath();
  ctx.arc(660, 150, 130, 0, Math.PI * 2);
  ctx.stroke();
  ctx.restore();

  // ---------- 品牌（月言 · 月色微博客） ----------
  ctx.fillStyle = POSTER_ACCENT;
  ctx.font = "600 30px Georgia, 'Songti SC', serif";
  ctx.fillText("月言 · 月色微博客", 48, 72);

  // ---------- 标题（display 字体，最多 2 行） ----------
  ctx.fillStyle = POSTER_TEXT;
  ctx.font = "700 44px Georgia, 'Songti SC', serif";
  const title = content.title || "分享一段月色";
  const titleLines = wrapText(ctx, title, POSTER_W - 96, 2);
  let y = 190;
  for (const line of titleLines) {
    ctx.fillText(line, 48, y);
    y += 56;
  }

  // ---------- 图片区（图片帖：最多 3 张，网格布局；加载失败自动跳过） ----------
  const images = (content.media ?? []).slice(0, 3);
  const loaded: HTMLImageElement[] = [];
  for (const url of images) {
    const img = await loadImage(url);
    if (img) {
      loaded.push(img);
    }
  }
  if (loaded.length > 0) {
    const contentWidth = POSTER_W - 96; // 704
    const gap = 24;
    if (loaded.length === 1) {
      // 单图：宽 704，高 ≤380（保持比例）
      const ratio = Math.min(contentWidth / loaded[0].width, 380 / loaded[0].height, 1);
      const w = Math.round(loaded[0].width * ratio);
      const h = Math.round(loaded[0].height * ratio);
      ctx.drawImage(loaded[0], 48, y, w, h);
      y += h;
    } else {
      // 2-3 张：一行网格（2 张 340×240；3 张 3 列等宽方图）
      const cols = loaded.length;
      const cellW = Math.round((contentWidth - gap * (cols - 1)) / cols);
      const cellH = cols === 3 ? cellW : 240;
      loaded.forEach((img, i) => {
        const x = 48 + i * (cellW + gap);
        // 等比裁切填充（cover）
        const scale = Math.max(cellW / img.width, cellH / img.height);
        const sw = Math.min(img.width, cellW / scale);
        const sh = Math.min(img.height, cellH / scale);
        const sx = (img.width - sw) / 2;
        const sy = (img.height - sh) / 2;
        ctx.save();
        ctx.beginPath();
        ctx.rect(x, y, cellW, cellH);
        ctx.clip();
        ctx.drawImage(img, sx, sy, sw, sh, x, y, cellW, cellH);
        ctx.restore();
      });
      y += cellH;
    }
    y += 20;
  }

  // ---------- 正文（最多 8 行） ----------
  ctx.fillStyle = POSTER_TEXT_SUB;
  ctx.font = "26px 'PingFang SC', 'Microsoft YaHei', sans-serif";
  const contentLines = wrapText(ctx, content.content || "", POSTER_W - 96, 8);
  y += 16;
  for (const line of contentLines) {
    ctx.fillText(line, 48, y);
    y += 40;
  }

  // ---------- 分隔线 ----------
  ctx.strokeStyle = POSTER_BORDER;
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.moveTo(48, y + 24);
  ctx.lineTo(POSTER_W - 48, y + 24);
  ctx.stroke();

  // ---------- 作者 · 话题 ----------
  ctx.fillStyle = POSTER_ACCENT;
  ctx.font = "24px 'PingFang SC', 'Microsoft YaHei', sans-serif";
  ctx.fillText(`${content.author}${content.tags ? ` · ${content.tags}` : ""}`, 48, y + 76);

  // ---------- 底部：二维码 + 链接 ----------
  if (qr) {
    const qrSize = 168;
    const qrX = POSTER_W - 48 - qrSize;
    const qrY = POSTER_H - 48 - qrSize;
    const img = new Image();
    img.src = qr;
    await new Promise<void>((resolve, reject) => {
      img.onload = () => resolve();
      img.onerror = () => reject(new Error("二维码加载失败"));
    });
    ctx.drawImage(img, qrX, qrY, qrSize, qrSize);
  }
  ctx.fillStyle = POSTER_TEXT_SUB;
  ctx.font = "20px 'PingFang SC', 'Microsoft YaHei', sans-serif";
  ctx.fillText(content.link, 48, POSTER_H - 96);

  return canvas.toDataURL("image/png");
}

// downloadDataUrl 下载 dataURL 图片（a[download] 触发浏览器下载）。
export function downloadDataUrl(dataUrl: string, filename: string): void {
  const a = document.createElement("a");
  a.href = dataUrl;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
}
