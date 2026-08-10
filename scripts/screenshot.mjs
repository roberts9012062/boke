// scripts/screenshot.mjs
// 视觉比对截图脚本（M1.7，Playwright 1.62+）：
// 对关键页面 × 双主题（冷月/薄雾）截图到 screenshots/，供与 UI设计/ 导出 PNG 人工比对。
//
// 用法：
//   ./scripts/screenshot.sh           # 默认截图（需先启动双端）
//   ./scripts/screenshot.sh --mobile  # 仅移动端视口
//
// 说明：
//   - 依赖 npx playwright（按需自动安装 chromium）
//   - 截图命名：screenshots/{页面}-{冷月|薄雾}-{desktop|mobile}.png
//   - 登录类页面（收藏/通知）通过 API 登录注入 localStorage（保证截图完整）
import { mkdirSync } from "node:fs";
import { createRequire } from "node:module";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

// playwright 从 frontend/node_modules 解析（ESM 不走 NODE_PATH，用 createRequire 指向项目依赖）
const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const frontendRequire = createRequire(join(ROOT, "frontend", "package.json"));
const { chromium } = frontendRequire("playwright");

// 项目根目录（本文件在 scripts/ 下）
const OUT_DIR = join(ROOT, "screenshots");
const BASE = "http://localhost:3000";

// 仅移动端模式（--mobile 参数）
const mobileOnly = process.argv.includes("--mobile");

// 截图任务清单（页面路径 + 名称 + 视口）
const TASKS = [
  { path: "/", name: "首页", desktop: true, mobile: true },
  { path: "/settings/theme", name: "主题设置", desktop: true, mobile: true },
  { path: "/login", name: "登录", desktop: true, mobile: false },
  { path: "/me/favorites", name: "收藏", desktop: true, mobile: true, auth: true },
  { path: "/users/1/followers", name: "粉丝", desktop: true, mobile: false, auth: true },
  { path: "/admin", name: "后台仪表盘", desktop: true, mobile: false, auth: true },
  { path: "/this-page-not-exist", name: "404", desktop: true, mobile: true },
  { path: "/onboarding", name: "引导", desktop: true, mobile: true },
];

// 登录（后台截图需要 admin 会话）：调用后端登录接口一次，返回令牌对。
// 说明：登录接口限流 5 次/分，因此主流程只登录一次，全部页面复用同一令牌注入。
async function loginAdminOnce() {
  const res = await fetch("http://localhost:8080/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ account: "admin@yueyan.site", password: "Yueyan2026" }),
  });
  if (res.status !== 200) {
    return null;
  }
  const body = await res.json();
  return body.data;
}

// 注入令牌到页面 localStorage（与前端 auth.tsx 的键一致）。
async function injectTokens(page, tokens) {
  await page.evaluate((t) => {
    localStorage.setItem("yueyan-tokens", JSON.stringify({
      access_token: t.access_token,
      refresh_token: t.refresh_token,
      expires_in: t.expires_in,
    }));
  }, tokens);
}

// 设置主题（localStorage yueyan-theme；默认走跟随系统解析）
async function setTheme(page, theme) {
  await page.evaluate((t) => {
    localStorage.setItem("yueyan-theme", t);
    document.documentElement.setAttribute("data-theme", t);
  }, theme);
}

// 主流程：双主题 × 双视口截图
async function main() {
  mkdirSync(OUT_DIR, { recursive: true });
  // 复用系统 Chrome（免下载 chromium；无 Chrome 环境可改 channel 或安装 playwright 浏览器）
  const browser = await chromium.launch({ channel: "chrome" });
  // admin 会话令牌（登录一次，全部 auth 页面复用；限流 5 次/分约束）
  const adminTokens = await loginAdminOnce();
  if (!adminTokens) {
    console.warn("[警告] admin 登录失败（登录限流或凭据错误），auth 页面将显示未登录态");
  }
  const desktopViewport = { width: 1400, height: 900 };
  const mobileViewport = { width: 390, height: 844 };

  for (const task of TASKS) {
    if (mobileOnly && !task.mobile) {
      continue;
    }
    for (const theme of ["cool-moon", "mist"]) {
      const viewports = [];
      if (!mobileOnly && task.desktop) viewports.push(["desktop", desktopViewport]);
      if (task.mobile) viewports.push(["mobile", mobileViewport]);

      for (const [device, viewport] of viewports) {
        const page = await browser.newPage({ viewport });
        await page.goto(`${BASE}${task.path}`, { waitUntil: "domcontentloaded" });
        await setTheme(page, theme);
        if (task.auth && adminTokens) {
          await injectTokens(page, adminTokens);
          // 登录后重新加载（会话生效）
          await page.reload({ waitUntil: "domcontentloaded" });
        }
        // 等待客户端数据加载（骨架 → 内容）
        await page.waitForTimeout(1800);
        const file = join(OUT_DIR, `${task.name}-${theme === "cool-moon" ? "冷月" : "薄雾"}-${device}.png`);
        await page.screenshot({ path: file, fullPage: true });
        console.log(`[截图] ${file}`);
        await page.close();
      }
    }
  }
  await browser.close();
  console.log(`[完成] 截图已输出到 ${OUT_DIR}`);
}

main().catch((err) => {
  console.error("[错误]", err.message);
  process.exit(1);
});
