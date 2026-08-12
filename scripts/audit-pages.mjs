// scripts/audit-pages.mjs
// 全页面巡检脚本（Playwright）：遍历前台/用户/后台全部页面，
// 捕获 console 错误/警告、未捕获异常、4xx/5xx 响应、请求失败，输出巡检报告。
//
// 用法：
//   ./scripts/audit-pages.sh            # 双端需已启动（:8080 + :3000）
//   AUDIT_FRONT_PORT=3100 ./scripts/audit-pages.sh   # 指定前端端口（生产模式验证）
//
// 说明：
//   - 登录一次（限流 5 次/分约束）注入 localStorage，用户/后台页面复用同一会话
//   - 动态路由 id 通过 API 预取（帖子/话题/用户/会话/插件实例）
//   - 每页等待客户端渲染 + 滚动到底触发懒加载后采集问题
import { createRequire } from "node:module";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

// playwright 从 frontend/node_modules 解析（与 screenshot.mjs 同一约定）
const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const frontendRequire = createRequire(join(ROOT, "frontend", "package.json"));
const { chromium } = frontendRequire("playwright");

const FRONT = `http://localhost:${process.env.AUDIT_FRONT_PORT ?? "3000"}`;
const BACK = "http://localhost:8080";

// —— 页面巡检清单（auth: none=公开；user=登录态用户页；admin=后台页）——
// 动态段：{post}=帖子 id、{topic}=话题名、{user}=用户 id、{conv}=会话 id、{plugin}=插件实例 id
const PAGE_SPECS = [
  // 前台公开页面
  { path: "/", auth: "none", name: "首页" },
  { path: "/about", auth: "none", name: "关于" },
  { path: "/login", auth: "none", name: "登录" },
  { path: "/register", auth: "none", name: "注册" },
  { path: "/forgot-password", auth: "none", name: "忘记密码" },
  { path: "/reset-password", auth: "none", name: "重置密码" },
  { path: "/privacy", auth: "none", name: "隐私政策" },
  { path: "/terms", auth: "none", name: "服务条款" },
  { path: "/search", auth: "none", name: "搜索" },
  { path: "/topics", auth: "none", name: "话题广场" },
  { path: "/topics/{topic}", auth: "none", name: "话题详情" },
  { path: "/posts/{post}", auth: "none", name: "帖子详情" },
  { path: "/users/{user}", auth: "none", name: "用户主页" },
  { path: "/users/{user}/followers", auth: "none", name: "粉丝列表" },
  { path: "/users/{user}/following", auth: "none", name: "关注列表" },
  { path: "/maintenance", auth: "none", name: "维护页" },
  { path: "/admin-login", auth: "none", name: "后台登录" },
  { path: "/onboarding", auth: "none", name: "引导页" },
  { path: "/this-page-not-exist", auth: "none", name: "404 页", expectDoc404: true },
  // 登录态用户页面
  { path: "/compose", auth: "user", name: "发布" },
  { path: "/drafts", auth: "user", name: "草稿箱" },
  { path: "/me", auth: "user", name: "我的" },
  { path: "/me/favorites", auth: "user", name: "我的收藏" },
  { path: "/messages", auth: "user", name: "消息列表" },
  { path: "/messages/{conv}", auth: "user", name: "会话详情" },
  { path: "/notifications", auth: "user", name: "通知" },
  { path: "/settings", auth: "user", name: "设置总览" },
  { path: "/settings/notifications", auth: "user", name: "通知设置" },
  { path: "/settings/privacy", auth: "user", name: "隐私设置" },
  { path: "/settings/profile", auth: "user", name: "资料设置" },
  { path: "/settings/security", auth: "user", name: "安全设置" },
  { path: "/settings/theme", auth: "user", name: "主题设置" },
  { path: "/publish-success/{post}", auth: "user", name: "发布成功" },
  // 后台页面
  { path: "/admin", auth: "admin", name: "后台仪表盘" },
  { path: "/admin/ai", auth: "admin", name: "AI 中心" },
  { path: "/admin/audit", auth: "admin", name: "审计日志" },
  { path: "/admin/backup", auth: "admin", name: "备份" },
  { path: "/admin/bans", auth: "admin", name: "封禁管理" },
  { path: "/admin/comments", auth: "admin", name: "评论管理" },
  { path: "/admin/media", auth: "admin", name: "媒体库" },
  { path: "/admin/plugin-market", auth: "admin", name: "插件市场" },
  { path: "/admin/plugins", auth: "admin", name: "我的插件" },
  { path: "/admin/plugins/{plugin}/settings", auth: "admin", name: "插件设置" },
  { path: "/admin/posts", auth: "admin", name: "帖子管理" },
  { path: "/admin/posts/{post}/edit", auth: "admin", name: "编辑帖子" },
  { path: "/admin/reports", auth: "admin", name: "举报工单" },
  { path: "/admin/roles", auth: "admin", name: "角色权限" },
  { path: "/admin/sensitive-words", auth: "admin", name: "敏感词" },
  { path: "/admin/seo", auth: "admin", name: "SEO 设置" },
  { path: "/admin/seo-health", auth: "admin", name: "SEO 健康" },
  { path: "/admin/serp", auth: "admin", name: "SERP 模拟" },
  { path: "/admin/settings", auth: "admin", name: "系统设置" },
  { path: "/admin/tags", auth: "admin", name: "标签管理" },
  { path: "/admin/under-construction", auth: "admin", name: "施工中" },
  { path: "/admin/users", auth: "admin", name: "用户管理" },
];

// —— API 预取（动态路由数据源；失败返回 null，对应页面跳过）——

// 通用 API GET（可选携带 admin 会话 token）。
async function apiGet(path, token) {
  const headers = token ? { Authorization: `Bearer ${token}` } : {};
  const res = await fetch(`${BACK}/api/v1${path}`, { headers });
  if (!res.ok) {
    return null;
  }
  const body = await res.json();
  return body?.data ?? null;
}

// 登录 admin（全流程仅一次，规避登录限流 5 次/分）。
async function loginAdmin() {
  const res = await fetch(`${BACK}/api/v1/auth/login`, {
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

// 预取全部动态路由数据：{ post, topic, user, conv, plugin }。
async function fetchDynamicIds(token) {
  const ids = {};
  // 帖子：时间线第一篇（帖子详情/编辑/发布成功页共用）
  const posts = await apiGet("/posts?page=1&page_size=5", null);
  const post = posts?.items?.[0]?.id ?? null;
  ids.post = post;
  // 用户：优先时间线帖子的作者，兜底 id=1
  ids.user = posts?.items?.[0]?.author_id ?? null;
  if (!ids.user) {
    const me = await apiGet("/me", token);
    ids.user = me?.id ?? null;
  }
  // 话题：列表第一个
  const topics = await apiGet("/topics", null);
  ids.topic = topics?.items?.[0]?.name ?? topics?.[0]?.name ?? null;
  // 会话：登录态第一个（admin 可能无会话）
  const convs = await apiGet("/conversations?page=1&page_size=5", token);
  ids.conv = convs?.items?.[0]?.id ?? null;
  // 插件实例：后台插件列表第一个（InstalledPlugin.id 即实例 ID）
  const plugins = await apiGet("/admin/plugins", token);
  ids.plugin = plugins?.items?.[0]?.id ?? null;
  return ids;
}

// —— 单页面巡检 ——

// 挂接页面监听，收集五类问题：未捕获异常 / console 错误 / console 警告 / HTTP 4xx5xx / 请求失败。
// expectDoc404：404 测试页豁免——页面本身演示 404 态，其 document 级 404 属预期（不计入）。
function collectIssues(page, expectDoc404) {
  const issues = { pageError: [], consoleError: [], consoleWarn: [], httpError: [], reqFailed: [] };
  page.on("pageerror", (err) => issues.pageError.push(String(err)));
  page.on("console", (msg) => {
    if (msg.type() === "error") {
      issues.consoleError.push(msg.text());
    } else if (msg.type() === "warning") {
      issues.consoleWarn.push(msg.text());
    }
  });
  page.on("response", (res) => {
    if (res.status() >= 400) {
      const isDoc404 = res.status() === 404 && res.request().resourceType() === "document" && expectDoc404;
      if (!isDoc404) {
        issues.httpError.push(`${res.status()} ${res.url()}`);
      }
    }
  });
  page.on("requestfailed", (req) => {
    issues.reqFailed.push(`${req.method()} ${req.url()} :: ${req.failure()?.errorText ?? "?"}`);
  });
  return issues;
}

// 巡检单个页面：注入会话（如需）→ 打开 → 等待渲染 → 滚动触发懒加载 → 返回问题清单。
async function auditPage(browser, spec, ids, tokens) {
  const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });
  const issues = collectIssues(page, spec.expectDoc404 ?? false);
  const path = spec.path.replace(/\{(\w+)\}/g, (_, key) => (ids[key] != null ? String(ids[key]) : "0"));
  try {
    await page.goto(`${FRONT}${path}`, { waitUntil: "domcontentloaded", timeout: 30000 });
    if (spec.auth !== "none" && tokens) {
      await page.evaluate((t) => {
        localStorage.setItem("yueyan-tokens", JSON.stringify({
          access_token: t.access_token,
          refresh_token: t.refresh_token,
          expires_in: t.expires_in,
        }));
      }, tokens);
      await page.reload({ waitUntil: "domcontentloaded", timeout: 30000 });
    }
    // 等待客户端数据渲染（骨架 → 内容）
    await page.waitForTimeout(2500);
    // 滚动到底触发懒加载（时间线分页等），再等增量请求
    await page.mouse.wheel(0, 6000);
    await page.waitForTimeout(1500);
  } catch (err) {
    issues.pageError.push(`[导航失败] ${err.message}`);
  } finally {
    await page.close();
  }
  return { path, name: spec.name, auth: spec.auth, expectDoc404: spec.expectDoc404 ?? false, issues };
}

// —— 报告输出 ——

// 汇总单页问题：PASS 或 FAIL（附明细）。
function summarize(result) {
  // 404 测试页：document 404 触发的浏览器层「资源加载失败」日志属预期，不计入
  const consoleError = result.expectDoc404
    ? result.issues.consoleError.filter((t) => !t.startsWith("Failed to load resource"))
    : result.issues.consoleError;
  const total = result.issues.pageError.length + consoleError.length + result.issues.consoleWarn.length + result.issues.httpError.length + result.issues.reqFailed.length;
  if (total === 0) {
    return `[PASS] ${result.name} (${result.auth}) ${result.path}`;
  }
  const lines = [`[FAIL] ${result.name} (${result.auth}) ${result.path} — ${total} 个问题`];
  for (const [kind, list] of [
    ["pageError", result.issues.pageError],
    ["consoleError", consoleError],
    ["consoleWarn", result.issues.consoleWarn],
    ["httpError", result.issues.httpError],
    ["reqFailed", result.issues.reqFailed],
  ]) {
    for (const item of list) {
      lines.push(`   - ${kind}: ${item}`);
    }
  }
  return lines.join("\n");
}

// 主流程：登录 → 预取动态 id → 顺序巡检 → 打印报告与汇总。
async function main() {
  const browser = await chromium.launch({ channel: "chrome" });
  const tokens = await loginAdmin();
  if (!tokens) {
    console.warn("[警告] admin 登录失败（限流或凭据错误），用户/后台页面将以未登录态巡检");
  }
  const ids = await fetchDynamicIds(tokens?.access_token ?? null);
  console.log(`[数据] 动态 id：帖子=${ids.post} 用户=${ids.user} 话题=${ids.topic} 会话=${ids.conv} 插件=${ids.plugin}`);
  for (const [key, val] of Object.entries(ids)) {
    if (val == null) {
      console.warn(`[跳过] 含 {${key}} 的页面因数据缺失将不巡检`);
    }
  }

  const results = [];
  for (const spec of PAGE_SPECS) {
    // 动态段数据缺失 → 跳过（避免用 0 访问无效资源产生噪音）
    const need = [...spec.path.matchAll(/\{(\w+)\}/g)].map((m) => m[1]);
    if (need.some((key) => ids[key] == null)) {
      console.log(`[跳过] ${spec.name}（${need.filter((k) => ids[k] == null).join(",")} 数据缺失）`);
      continue;
    }
    const result = await auditPage(browser, spec, ids, tokens);
    results.push(result);
    console.log(summarize(result));
  }
  await browser.close();

  // 汇总统计（404 测试页的 document 404 日志豁免——与 summarize 一致）
  const countIssues = (r) => {
    const consoleError = r.expectDoc404
      ? r.issues.consoleError.filter((t) => !t.startsWith("Failed to load resource"))
      : r.issues.consoleError;
    return r.issues.pageError.length + consoleError.length + r.issues.consoleWarn.length + r.issues.httpError.length + r.issues.reqFailed.length;
  };
  const failed = results.filter((r) => countIssues(r) > 0);
  const keyWarnings = results.flatMap((r) => r.issues.consoleWarn).filter((t) => t.includes("key"));
  const httpIssues = results.flatMap((r) => r.issues.httpError);
  console.log("\n===== 巡检汇总 =====");
  console.log(`总页面：${results.length} | 有问题：${failed.length} | 干净：${results.length - failed.length}`);
  console.log(`React key 警告：${keyWarnings.length} 条`);
  if (keyWarnings.length > 0) {
    console.log(`  - ${keyWarnings.join("\n  - ")}`);
  }
  console.log(`HTTP 4xx/5xx：${httpIssues.length} 条`);
  for (const url of [...new Set(httpIssues)]) {
    console.log(`  - ${url}`);
  }
  if (failed.length === 0) {
    console.log("[结论] 全部页面无报错 ✓");
  } else {
    console.log(`[结论] ${failed.length} 个页面存在问题（明细见上）`);
  }
  process.exit(failed.length === 0 ? 0 : 1);
}

main().catch((err) => {
  console.error("[错误]", err);
  process.exit(1);
});
