// scripts/ui-check-home-music.mjs
// 验证首页时间线卡片渲染音乐迷你播放器（QQ/网易云 iframe）。
import { createRequire } from "node:module";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const frontendRequire = createRequire(join(ROOT, "frontend", "package.json"));
const { chromium } = frontendRequire("playwright");

const BASE = "http://localhost:3000";

const main = async () => {
  const browser = await chromium.launch({ channel: "chrome" });
  const page = await browser.newPage({ viewport: { width: 1400, height: 1200 } });
  await page.goto(`${BASE}/`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(6000);

  // 首页卡片上的音乐播放器 iframe
  const cards = await page.locator("article").count();
  const musicIframes = await page
    .locator('article div[data-music-embed] iframe, article iframe[title*="内嵌音乐"]')
    .evaluateAll((els) =>
      els.map((f) => ({
        src: f.getAttribute("src")?.slice(0, 120),
        width: f.getAttribute("width"),
        height: f.style.height,
      })),
    );
  // 尝试读取第一个 QQ 播放器的内容
  let playerText = "";
  const qqFrame = page.frameLocator('article iframe[src*="i.y.qq.com"]').first();
  try {
    await qqFrame.locator("body").waitFor({ state: "visible", timeout: 12000 });
    playerText = (await qqFrame.locator("body").innerText().catch(() => "")).replace(/\s+/g, " ").trim();
  } catch {
    playerText = "(无法读取)";
  }
  await page.screenshot({ path: join(ROOT, "screenshots", "检查-首页音乐播放器.png"), fullPage: false });
  await browser.close();
  console.log(JSON.stringify({ 卡片数: cards, 音乐播放器数: musicIframes.length, 播放器列表: musicIframes.slice(0, 3), QQ播放器内容: playerText.slice(0, 120) }, null, 2));
};

main().catch((err) => {
  console.error("[错误]", err.message);
  process.exit(1);
});
