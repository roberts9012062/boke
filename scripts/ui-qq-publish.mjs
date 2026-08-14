// scripts/ui-qq-publish.mjs
// 完整链路验证：登录 → 写一帖（正文 + QQ 音乐内嵌）→ 发布 → 帖子详情页播放器渲染检查。
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
  // 登录拿令牌
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
  const tokens = loginBody.data;

  const browser = await chromium.launch({ channel: "chrome" });
  const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });
  await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
  await page.evaluate((t) => {
    localStorage.setItem(
      "yueyan-tokens",
      JSON.stringify({ access_token: t.access_token, refresh_token: t.refresh_token, expires_in: t.expires_in }),
    );
  }, tokens);

  // 进入写一帖
  await page.goto(`${BASE}/compose`, { waitUntil: "domcontentloaded" });
  await page.locator(".prose-editor").waitFor({ state: "visible", timeout: 15000 });

  // 输入正文
  await page.locator(".prose-editor").click();
  await page.keyboard.type("QQ音乐内嵌播放器修复验证（songid 链路）");

  // 插入 QQ 音乐（songDetail 链接）
  await page.getByRole("button", { name: "♪ 音乐" }).click();
  const input = page.locator('.modal input, [role="dialog"] input').last();
  await input.waitFor({ state: "visible", timeout: 5000 });
  await input.fill("https://y.qq.com/n/ryqq/songDetail/0039MnYb0qxYhV");
  await page.getByRole("button", { name: "插入", exact: true }).click();
  // 等待解析完成（按钮从「解析中…」回到「插入」且弹窗关闭）
  await page.waitForTimeout(4000);

  // 检查编辑器中 iframe
  const editorIframe = await page
    .locator('div[data-music-embed="qq"] iframe')
    .getAttribute("src")
    .catch(() => null);

  // 发布
  await page.getByRole("button", { name: "发布", exact: true }).click();
  await page.waitForURL(/publish-success|posts/, { timeout: 15000 });
  const urlAfterPublish = page.url();
  // 发布成功页 → 详情页
  await page.waitForTimeout(2000);
  const detailUrl = urlAfterPublish.includes("publish-success")
    ? urlAfterPublish.replace("publish-success", "posts")
    : urlAfterPublish;
  await page.goto(detailUrl, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(4000);

  // 检查详情页播放器
  const detail = {
    url: page.url(),
    iframeSrc: await page.locator('div[data-music-embed="qq"] iframe').getAttribute("src").catch(() => null),
    playerText: await page
      .frameLocator('div[data-music-embed="qq"] iframe')
      .locator("body")
      .innerText()
      .catch(() => ""),
  };
  detail.playerText = detail.playerText.replace(/\s+/g, " ").trim().slice(0, 120);
  await page.screenshot({ path: join(OUT_DIR, "访问-QQ音乐帖子详情.png"), fullPage: false });

  await browser.close();
  console.log(JSON.stringify({ editorIframe, detail }, null, 2));
};

main().catch((err) => {
  console.error("[错误]", err.message);
  process.exit(1);
});
