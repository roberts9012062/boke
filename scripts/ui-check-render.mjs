// scripts/ui-check-render.mjs
// 检查帖子详情页与写一贴页面的音乐嵌入渲染：是否有代码文本泄漏 / iframe 是否正常。
import { mkdirSync } from "node:fs";
import { createRequire } from "node:module";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const frontendRequire = createRequire(join(ROOT, "frontend", "package.json"));
const { chromium } = frontendRequire("playwright");

const BASE = "http://localhost:3000";
const OUT_DIR = join(ROOT, "screenshots");

const main = async () => {
  mkdirSync(OUT_DIR, { recursive: true });
  const browser = await chromium.launch({ channel: "chrome" });

  // 1. 详情页 /posts/71（已发布的 QQ 音乐测试帖）
  const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });
  await page.goto(`${BASE}/posts/71`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(4000);
  const body = page.locator("body");
  const bodyText = (await body.innerText().catch(() => "")).replace(/\s+/g, " ");
  // 检查是否有 HTML 代码文本泄漏（<div、<iframe 等标签文本）
  const codeLeak = bodyText.match(/<(div|iframe|p|br)[^>]*>/g) ?? [];
  // 内容区文本
  const contentText = (await page.locator(".prose, article, [class*='content']").first().innerText().catch(() => ""))
    .replace(/\s+/g, " ")
    .slice(0, 300);
  // iframe 与播放器
  const iframeCount = await page.locator('div[data-music-embed="qq"] iframe').count();
  const iframeSrc = await page.locator('div[data-music-embed="qq"] iframe').getAttribute("src").catch(() => null);
  const playerText = await page
    .frameLocator('div[data-music-embed="qq"] iframe')
    .locator("body")
    .innerText()
    .catch(() => "");
  await page.screenshot({ path: join(OUT_DIR, "检查-详情页71.png") });

  console.log(
    JSON.stringify(
      {
        页面: "详情页 /posts/71",
        页面可见代码标签: codeLeak.slice(0, 5),
        内容区文本: contentText.slice(0, 200),
        iframe数量: iframeCount,
        iframeSrc,
        播放器文本: playerText.replace(/\s+/g, " ").trim().slice(0, 120),
      },
      null,
      2,
    ),
  );
  await page.close();
  await browser.close();
};

main().catch((err) => {
  console.error("[错误]", err.message);
  process.exit(1);
});
