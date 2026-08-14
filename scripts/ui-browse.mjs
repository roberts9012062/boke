// scripts/ui-browse.mjs
// 浏览器访问检查脚本：用 Playwright 打开真实浏览器访问项目页面，
// 抓取页面标题、关键文本与截图，输出结构化 JSON 供人工核对。
//
// 用法：
//   node scripts/ui-browse.mjs [--admin]     # 默认访问首页；--admin 额外登录访问后台仪表盘
//
// 说明：
//   - 复用 frontend/node_modules 的 playwright（--no-save 安装）与系统 Chrome
//   - 不修改任何业务代码，仅读取页面渲染结果
import { mkdirSync } from "node:fs";
import { createRequire } from "node:module";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

// playwright 从 frontend/node_modules 解析
const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const frontendRequire = createRequire(join(ROOT, "frontend", "package.json"));
const { chromium } = frontendRequire("playwright");

const BASE = "http://localhost:3000";
const OUT_DIR = join(ROOT, "screenshots");
const withAdmin = process.argv.includes("--admin");

// 抓取页面关键信息：标题、描述、h1、可见文本前若干字
const snapshotPage = async (page) => {
  const title = await page.title();
  const description = await page
    .locator('meta[name="description"]')
    .getAttribute("content")
    .catch(() => "");
  const h1List = await page.locator("h1").allTextContents();
  const bodyText = (await page.locator("body").innerText()).replace(/\s+/g, " ").trim();
  return {
    url: page.url(),
    title,
    description,
    h1: h1List.map((t) => t.trim()).filter(Boolean).slice(0, 5),
    text: bodyText.slice(0, 600),
  };
};

// 调用后端登录一次，返回令牌对（与前端 auth 存储键一致）
const loginAdmin = async () => {
  const res = await fetch(`${BASE}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ account: "admin@yueyan.site", password: "Yueyan2026" }),
  });
  if (res.status !== 200) return null;
  const body = await res.json();
  return body.data;
};

const main = async () => {
  mkdirSync(OUT_DIR, { recursive: true });
  const browser = await chromium.launch({ channel: "chrome" });
  const result = { pages: [] };

  // 首页
  const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });
  await page.goto(`${BASE}/`, { waitUntil: "networkidle" });
  await page.waitForTimeout(1500);
  result.pages.push(await snapshotPage(page));
  await page.screenshot({ path: join(OUT_DIR, "访问-首页.png"), fullPage: false });
  await page.close();

  // 后台仪表盘（--admin 时）
  if (withAdmin) {
    const tokens = await loginAdmin();
    if (!tokens) {
      result.adminLogin = "失败（限流或凭据错误）";
    } else {
      result.adminLogin = "成功";
      const admin = await browser.newPage({ viewport: { width: 1400, height: 900 } });
      await admin.goto(`${BASE}/admin-login`, { waitUntil: "domcontentloaded" });
      await admin.evaluate((t) => {
        localStorage.setItem(
          "yueyan-tokens",
          JSON.stringify({
            access_token: t.access_token,
            refresh_token: t.refresh_token,
            expires_in: t.expires_in,
          }),
        );
      }, tokens);
      await admin.goto(`${BASE}/admin`, { waitUntil: "networkidle" });
      await admin.waitForTimeout(1500);
      result.pages.push(await snapshotPage(admin));
      await admin.screenshot({ path: join(OUT_DIR, "访问-后台仪表盘.png"), fullPage: false });
      await admin.close();
    }
  }

  await browser.close();
  console.log(JSON.stringify(result, null, 2));
};

main().catch((err) => {
  console.error("[错误]", err.message);
  process.exit(1);
});
