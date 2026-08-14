// scripts/ui-netease-publish.mjs
// 端到端验证：发帖搜索选歌插入 → 发布 → 详情页/首页卡片自研播放器渲染。
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
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message.slice(0, 160)));
  await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
  await page.evaluate((t) => {
    localStorage.setItem("yueyan-tokens", JSON.stringify({ access_token: t.access_token, refresh_token: t.refresh_token, expires_in: t.expires_in }));
  }, loginBody.data);

  // 发帖
  await page.goto(`${BASE}/compose`, { waitUntil: "domcontentloaded" });
  await page.locator(".prose-editor").waitFor({ state: "visible", timeout: 15000 });
  await page.locator(".prose-editor").click();
  await page.keyboard.type("网易云音乐自研播放器测试");

  // 插入网易云歌曲
  await page.getByRole("button", { name: "♪ 音乐" }).click();
  await page.getByRole("button", { name: "网易云搜索" }).click();
  await page.locator('[role="dialog"] input[placeholder*="输入歌名"]').fill("海阔天空");
  await page.getByRole("button", { name: "搜索", exact: true }).click();
  await page.waitForTimeout(4000);
  await page.locator('[role="dialog"] button:has-text("插入")').first().click();
  await page.waitForTimeout(3000);

  // 编辑器内自研播放器（audio + 歌名）
  const editorText = (await page.locator(".prose-editor").innerText().catch(() => "")).replace(/\s+/g, " ");
  const editorHasAudio = (await page.locator(".prose-editor audio").count()) > 0;
  const editorHasSong = editorText.includes("海阔天空") && editorText.includes("Beyond");

  // 发布
  await page.getByRole("button", { name: "发布", exact: true }).click();
  await page.waitForURL(/publish-success/, { timeout: 15000 });
  const publishUrl = page.url();
  const postId = publishUrl.match(/publish-success\/(\d+)/)?.[1] ?? "";

  // 详情页
  await page.goto(`${BASE}/posts/${postId}`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(6000);
  const detailText = (await page.locator(".rich-content").innerText().catch(() => "")).replace(/\s+/g, " ");
  const detailHasAudio = (await page.locator(".rich-content audio").count()) > 0;
  const detailHasSong = detailText.includes("海阔天空") && detailText.includes("Beyond");
  const detailIframeCount = await page.locator(".rich-content iframe").count();

  // 首页卡片
  await page.goto(`${BASE}/`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(4000);
  const cardText = (await page.locator("article").first().innerText().catch(() => "")).replace(/\s+/g, " ");
  const cardHasAudio = (await page.locator("article").first().locator("audio").count()) > 0;
  const cardHasSong = cardText.includes("海阔天空") && cardText.includes("Beyond");

  console.log(
    JSON.stringify(
      {
        帖子ID: postId,
        编辑器播放器: { 有audio: editorHasAudio, 显示歌曲: editorHasSong },
        详情页: { 有audio: detailHasAudio, 显示歌曲: detailHasSong, iframe数: detailIframeCount },
        首页卡片: { 有audio: cardHasAudio, 显示歌曲: cardHasSong },
        页面错误: errors.slice(0, 3),
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
