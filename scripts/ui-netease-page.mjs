// scripts/ui-netease-page.mjs
// 验证网易云音乐插件后台页：登录页渲染 + 搜索试播（拿地址 → audio 播放）。
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
  const consoleMsgs = [];
  page.on("pageerror", (e) => errors.push(e.message.slice(0, 160)));
  page.on("console", (m) => {
    if (["error", "warning"].includes(m.type())) consoleMsgs.push(`${m.type()}: ${m.text().slice(0, 160)}`);
  });
  await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
  await page.evaluate((t) => {
    localStorage.setItem(
      "yueyan-tokens",
      JSON.stringify({ access_token: t.access_token, refresh_token: t.refresh_token, expires_in: t.expires_in }),
    );
  }, loginBody.data);

  // 访问插件后台页
  await page.goto(`${BASE}/admin/plugin-pages/netease-music/settings`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(3000);

  // 页面文本（登录表单 / 状态）
  const pageText = (await page.locator("body").innerText().catch(() => "")).replace(/\s+/g, " ").trim();
  const hasLoginForm = pageText.includes("手机号") && pageText.includes("登录");
  const hasSearch = pageText.includes("搜索试播");

  // 搜索试播
  let searchResultText = "";
  let playBtnText = "";
  if (hasSearch) {
    const input = page.locator('[data-search-q]');
    await input.fill("海阔天空");
    await page.locator("[data-search-btn]").click();
    await page.waitForTimeout(4000);
    searchResultText = (await page.locator("[data-search-result]").innerText().catch(() => "")).replace(/\s+/g, " ").slice(0, 200);
    // 点击第一个「试播」按钮，验证拿地址 + 播放
    const playBtn = page.locator("[data-search-result] button").first();
    if ((await playBtn.count()) > 0) {
      await playBtn.click();
      await page.waitForTimeout(4000);
      playBtnText = await playBtn.innerText();
    }
  }

  console.log(
    JSON.stringify(
      {
        页面URL: page.url(),
        页面文本: pageText.slice(0, 300),
        有登录表单: hasLoginForm,
        有搜索试播: hasSearch,
        搜索结果: searchResultText,
        试播按钮状态: playBtnText,
        页面错误: errors.slice(0, 3),
        console: consoleMsgs.slice(0, 5),
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
