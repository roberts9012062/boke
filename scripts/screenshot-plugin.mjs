// scripts/screenshot-plugin.mjs
// M3.6 插件前端扩展 截图验证（Playwright）：
//   前置：后端 :8080 + 前端 :3000 运行中；demo 插件已安装且 running（含 frontend/ 扩展）。
//   流程：取一篇已发布帖子 → 打开详情页 → 等待 post.footer 槽位渲染插件内容 → 断言 + 截图。
//
// 用法：
//   node scripts/screenshot-plugin.mjs
import { mkdirSync } from "node:fs";
import { createRequire } from "node:module";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const frontendRequire = createRequire(join(ROOT, "frontend", "package.json"));
const { chromium } = frontendRequire("playwright");

const OUT_DIR = join(ROOT, "screenshots");
// 端口环境变量（生产模式验证 next start -p 3100 用；默认 dev 3000）
const FRONT = `http://localhost:${process.env.PLUGIN_SCREENSHOT_PORT ?? "3000"}`;
const BACK = "http://localhost:8080";

// 取一篇已发布帖子（时间线第一页第一条）。
async function pickPostId() {
  const res = await fetch(`${BACK}/api/v1/posts?page=1&page_size=5`);
  if (!res.ok) {
    return null;
  }
  const body = await res.json();
  const items = body?.data?.items ?? [];
  return items.length > 0 ? String(items[0].id) : null;
}

async function main() {
  const postId = await pickPostId();
  if (!postId) {
    console.error("[失败] 未找到已发布帖子（时间线为空）");
    process.exit(1);
  }
  console.log(`[步骤 1] 使用帖子 /posts/${postId} 验证 post.footer 插件扩展`);

  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });
  const errors = [];
  page.on("pageerror", (err) => errors.push(String(err)));

  // 打开帖子详情页（公开页面无需登录）
  await page.goto(`${FRONT}/posts/${postId}`, { waitUntil: "networkidle", timeout: 30000 });

  // 等待 post.footer 槽位渲染插件内容（demo 插件 .demo-plugin-card）
  try {
    await page.waitForSelector('[data-plugin-slot="post.footer"] .demo-plugin-card', { timeout: 15000 });
  } catch {
    console.error("[失败] post.footer 槽位未渲染插件内容（插件未运行或前端扩展加载失败）");
    console.error("  页面错误:", errors.slice(0, 5));
    await browser.close();
    process.exit(1);
  }

  // 断言内容
  const text = await page.textContent('[data-plugin-slot="post.footer"]');
  const ok = text.includes("演示插件") && text.includes("文章页脚扩展");
  console.log(`[${ok ? "PASS" : "FAIL"}] post.footer 渲染插件内容：${text?.trim()}`);

  // 截图（含插件扩展区域）
  mkdirSync(OUT_DIR, { recursive: true });
  await page.screenshot({ path: join(OUT_DIR, "plugin-post-footer.png"), fullPage: false });
  console.log(`[完成] 截图已保存：${join(OUT_DIR, "plugin-post-footer.png")}`);
  await browser.close();
  process.exit(ok ? 0 : 1);
}

main().catch((err) => {
  console.error("[失败]", err);
  process.exit(1);
});
