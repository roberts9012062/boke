// scripts/ui-netease-final.mjs
// 最终验证：访客态 + 登录态详情页自研播放器渲染与音频实际加载（canplay）。
import { createRequire } from "node:module";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const frontendRequire = createRequire(join(ROOT, "frontend", "package.json"));
const { chromium } = frontendRequire("playwright");

const BASE = "http://localhost:3000";

async function checkPlay(browser, label) {
  const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });
  const logs = [];
  page.on("console", (m) => { if (m.type() === "error") logs.push(m.text().slice(0, 150)); });
  page.on("pageerror", (e) => logs.push(`pageerror: ${e.message.slice(0, 150)}`));
  await page.goto(`${BASE}/posts/75`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(5000);

  const playBtn = page.locator(".rich-content button[aria-label='播放'], .rich-content button[aria-label='暂停']").first();
  const btnExists = (await playBtn.count()) > 0;
  const btnDisabled = await playBtn.isDisabled().catch(() => true);
  let audioState = null;
  if (btnExists && !btnDisabled) {
    await playBtn.click();
    await page.waitForTimeout(6000);
    audioState = await page.evaluate(() => {
      const a = document.querySelector(".rich-content audio");
      if (!a) return { noAudio: true };
      return {
        src前50: a.src?.slice(0, 50),
        readyState: a.readyState,
        error: a.error ? { code: a.error.code, message: a.error.message } : null,
        duration: a.duration,
      };
    });
  }
  const text = (await page.locator(".rich-content").innerText().catch(() => "")).replace(/\s+/g, " ");
  await page.close();
  return { label, btnExists, btnDisabled, 显示歌曲: text.includes("海阔天空"), audioState, 错误: logs.slice(0, 4) };
}

const main = async () => {
  const browser = await chromium.launch({ channel: "chrome" });
  const guest = await checkPlay(browser, "访客态");

  // 登录态
  const login = await fetch(`${BASE}/api/v1/auth/login`, {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ account: "admin@yueyan.site", password: "Yueyan2026" }),
  });
  const lb = await login.json();
  const page = await browser.newPage();
  await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
  await page.evaluate((t) => {
    localStorage.setItem("yueyan-tokens", JSON.stringify({ access_token: t.access_token, refresh_token: t.refresh_token, expires_in: t.expires_in }));
  }, lb.data);
  await page.close();
  const loggedIn = await checkPlay(browser, "登录态");

  await browser.close();
  console.log(JSON.stringify({ 访客态: guest, 登录态: loggedIn }, null, 2));
};

main().catch((err) => {
  console.error("[错误]", err.message);
  process.exit(1);
});
