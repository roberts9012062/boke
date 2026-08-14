// scripts/ui-qq-demo.mjs
// 发布一篇 QQ 音乐演示帖（正文含歌名，一眼可辨），验证播放器端到端。
import { createRequire } from "node:module";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const frontendRequire = createRequire(join(ROOT, "frontend", "package.json"));
const { chromium } = frontendRequire("playwright");

const BASE = "http://localhost:3000";

const main = async () => {
  const login = await fetch(`${BASE}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ account: "admin@yueyan.site", password: "Yueyan2026" }),
  });
  const loginBody = await login.json();
  if (loginBody.code !== 0) {
    console.log(JSON.stringify({ fatal: `登录失败: ${loginBody.message}` }, null, 2));
    process.exit(1);
  }

  const browser = await chromium.launch({ channel: "chrome" });
  const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });
  await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
  await page.evaluate((t) => {
    localStorage.setItem(
      "yueyan-tokens",
      JSON.stringify({ access_token: t.access_token, refresh_token: t.refresh_token, expires_in: t.expires_in }),
    );
  }, loginBody.data);

  await page.goto(`${BASE}/compose`, { waitUntil: "domcontentloaded" });
  await page.locator(".prose-editor").waitFor({ state: "visible", timeout: 15000 });
  // 正文：歌名开头（卡片摘要可见）
  await page.locator(".prose-editor").click();
  await page.keyboard.type("《晴天》- 周杰伦（QQ音乐播放测试）");

  // 插入 QQ 音乐
  await page.getByRole("button", { name: "♪ 音乐" }).click();
  const input = page.locator('.modal input, [role="dialog"] input').last();
  await input.waitFor({ state: "visible", timeout: 5000 });
  await input.fill("https://y.qq.com/n/ryqq/songDetail/0039MnYb0qxYhV");
  await page.getByRole("button", { name: "插入", exact: true }).click();
  await page.waitForTimeout(4000);

  // 发布
  await page.getByRole("button", { name: "发布", exact: true }).click();
  await page.waitForURL(/publish-success|posts/, { timeout: 15000 });
  await page.waitForTimeout(2500);
  const afterUrl = page.url();
  const detailUrl = afterUrl.includes("publish-success") ? afterUrl.replace("publish-success", "posts") : afterUrl;
  await page.goto(detailUrl, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(4000);
  const playerText = await page
    .frameLocator('div[data-music-embed="qq"] iframe')
    .locator("body")
    .innerText()
    .catch(() => "");
  const postId = detailUrl.split("/").pop();
  console.log(
    JSON.stringify(
      {
        新帖子链接: detailUrl,
        postId,
        详情页播放器: playerText.replace(/\s+/g, " ").trim().slice(0, 120),
      },
      null,
      2,
    ),
  );
  await browser.close();
};

main().catch((err) => {
  console.error("[错误]", err.message);
  process.exit(1);
});
