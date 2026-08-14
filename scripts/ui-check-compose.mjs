// scripts/ui-check-compose.mjs
// 检查 /compose 编辑器：插入 QQ 音乐后的实际显示 + 前端 console 报错。
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
  page.on("console", (m) => {
    if (["error", "warning"].includes(m.type())) errors.push(`${m.type()}: ${m.text().slice(0, 160)}`);
  });
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message.slice(0, 160)}`));

  await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
  await page.evaluate((t) => {
    localStorage.setItem(
      "yueyan-tokens",
      JSON.stringify({ access_token: t.access_token, refresh_token: t.refresh_token, expires_in: t.expires_in }),
    );
  }, loginBody.data);
  await page.goto(`${BASE}/compose`, { waitUntil: "domcontentloaded" });
  await page.locator(".prose-editor").waitFor({ state: "visible", timeout: 15000 });

  // 插入 QQ 音乐
  await page.getByRole("button", { name: "♪ 音乐" }).click();
  const input = page.locator('.modal input, [role="dialog"] input').last();
  await input.waitFor({ state: "visible", timeout: 5000 });
  await input.fill("https://y.qq.com/n/ryqq/songDetail/0039MnYb0qxYhV");
  await page.getByRole("button", { name: "插入", exact: true }).click();
  await page.waitForTimeout(4000);

  // 编辑器可见文本（若播放器正常，此处应为播放器交互内容而非代码；若 ReactNodeView 失败会显示 HTML 源码文本）
  const editorText = (await page.locator(".prose-editor").innerText().catch(() => "")).replace(/\s+/g, " ").trim();
  const embedHtml = await page.locator(".prose-editor").innerHTML().catch(() => "");
  const hasIframe = await page.locator('.prose-editor div[data-music-embed="qq"] iframe').count();
  const iframeSrc = await page
    .locator('.prose-editor div[data-music-embed="qq"] iframe')
    .getAttribute("src")
    .catch(() => null);
  const playerText = await page
    .frameLocator('.prose-editor div[data-music-embed="qq"] iframe')
    .locator("body")
    .innerText()
    .catch(() => "");

  console.log(
    JSON.stringify(
      {
        编辑器可见文本: editorText.slice(0, 200),
        iframe数量: hasIframe,
        iframeSrc,
        播放器文本: playerText.replace(/\s+/g, " ").trim().slice(0, 120),
        嵌入节点HTML片段: embedHtml.slice(0, 300),
        console报错: errors.slice(0, 6),
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
