// scripts/ui-music-test.mjs
// QQ 音乐链接发帖播放测试（Playwright）：
// 在 /compose 写一贴，用「♪ 音乐」按钮插入不同类型 QQ 音乐链接，
// 验证解析结果与 iframe 播放器加载情况。
//
// 用法：
//   node scripts/ui-music-test.mjs
//
// 输出：结构化 JSON（每类链接的解析结果 + iframe 加载状态 + 编辑器 HTML 片段）
import { mkdirSync } from "node:fs";
import { createRequire } from "node:module";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const frontendRequire = createRequire(join(ROOT, "frontend", "package.json"));
const { chromium } = frontendRequire("playwright");

const BASE = "http://localhost:3000";
const OUT_DIR = join(ROOT, "screenshots");

// 待测 QQ 音乐链接（songmid 来自 QQ 音乐公开搜索接口的真实数据）
const LINKS = [
  {
    label: "新版网页版 songDetail（真实 songmid）",
    url: "https://y.qq.com/n/ryqq/songDetail/0039MnYb0qxYhV",
  },
  {
    label: "旧版分享 playsong?songmid=（真实 songmid）",
    url: "https://i.y.qq.com/v8/playsong.html?songmid=0039MnYb0qxYhV&songname=%E6%99%B4%E5%A4%A9",
  },
  {
    label: "短链 c.y.qq.com（预期解析失败）",
    url: "https://c.y.qq.com/base/fcgi-bin/u?__=abc123",
  },
];

// 登录拿令牌（与前端 auth 存储键一致）
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

// 检查 iframe 播放器加载状态：等待框架出现并读取其文档内容
const inspectIframe = async (page, timeoutMs) => {
  const embed = page.locator('div[data-music-embed]').last();
  const count = await embed.count();
  if (count === 0) return { node: false };
  const attrs = await embed.evaluate((el) => ({
    platform: el.getAttribute("data-music-embed"),
    kind: el.getAttribute("data-music-kind"),
    iframes: Array.from(el.querySelectorAll("iframe")).map((f) => f.src),
  }));
  const frame = page.frameLocator('div[data-music-embed] iframe').last();
  let frameState = "pending";
  let frameText = "";
  try {
    const body = frame.locator("body");
    await body.waitFor({ state: "visible", timeout: timeoutMs });
    frameState = "loaded";
    frameText = (await body.innerText().catch(() => "")).replace(/\s+/g, " ").slice(0, 200);
  } catch {
    frameState = "timeout-or-blocked";
  }
  return { node: true, ...attrs, frameState, frameText };
};

const main = async () => {
  mkdirSync(OUT_DIR, { recursive: true });
  const tokens = await loginAdmin();
  const browser = await chromium.launch({ channel: "chrome" });
  const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });

  // 注入登录态并进入写一帖
  if (!tokens) {
    console.log(JSON.stringify({ fatal: "admin 登录失败（限流或凭据错误）" }, null, 2));
    await browser.close();
    process.exit(1);
  }
  await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
  await page.evaluate((t) => {
    localStorage.setItem(
      "yueyan-tokens",
      JSON.stringify({
        access_token: t.access_token,
        refresh_token: t.refresh_token,
        expires_in: t.expires_in,
      }),
    );
  }, tokens);
  await page.goto(`${BASE}/compose`, { waitUntil: "domcontentloaded" });
  // 等待编辑器就绪（Tiptap 渲染）
  await page.locator(".prose-editor").waitFor({ state: "visible", timeout: 15000 });

  const results = [];
  for (const link of LINKS) {
    const caseResult = { label: link.label, url: link.url };
    // 打开「插入音乐」弹窗
    await page.getByRole("button", { name: "♪ 音乐" }).click();
    // 输入链接（弹窗内唯一输入框，placeholder 为网易云示例）
    const input = page.locator('.modal input, [role="dialog"] input').last();
    await input.waitFor({ state: "visible", timeout: 5000 });
    await input.fill(link.url);
    // 点击「插入」
    await page.getByRole("button", { name: "插入", exact: true }).click();
    // 稍等渲染
    await page.waitForTimeout(800);
    // 弹窗是否已关闭：仍开着说明解析失败（错误提示留在弹窗内）
    const modalOpen = await page.locator(".modal, [role='dialog']").count();
    if (modalOpen > 0) {
      const errText = await page.locator(".modal, [role='dialog']").first().innerText().catch(() => "");
      caseResult.parse = "失败";
      caseResult.error = errText.replace(/\s+/g, " ").slice(0, 160);
      // 关闭弹窗继续（force：Modal 动画期间元素不稳定）
      await page.getByRole("button", { name: "取消" }).click({ force: true }).catch(() => {});
      await page.waitForTimeout(500);
    } else {
      caseResult.parse = "成功";
      caseResult.embed = await inspectIframe(page, 15000);
      // 每个用例独立：重新加载写一帖页，避免 Tiptap ReactNodeView 状态残留
      await page.goto(`${BASE}/compose`, { waitUntil: "domcontentloaded" });
      await page.locator(".prose-editor").waitFor({ state: "visible", timeout: 15000 });
      await page.waitForTimeout(300);
    }
    results.push(caseResult);
  }

  // 编辑器 HTML 快照（确认序列化格式）
  const editorHtml = await page.locator(".prose-editor").innerHTML().catch(() => "");
  await page.screenshot({ path: join(OUT_DIR, "访问-写一帖QQ音乐.png"), fullPage: false });

  await browser.close();
  console.log(JSON.stringify({ results, editorHtmlSample: editorHtml.slice(0, 400) }, null, 2));
};

main().catch((err) => {
  console.error("[错误]", err.message);
  process.exit(1);
});
