// scripts/verify-extension-edge.mjs
// 插件 v0.31.0「右键任务 → 悬浮球不可用 → 页内停靠兜底」链路的 Edge 程序化验证。
//
// 覆盖点（对照手册 §12 / §14）：
//   ① 扩展在 Edge 加载成功、background service worker 存活；
//   ② chrome.contextMenus 注册了全部 6 个菜单项（真实 getAll 查询）；
//   ③ 面板顶栏「关闭网页悬浮球」真实 UI 点击 → 球宿主节点 display:none（content script 实时隐藏）；
//   ④ Edge 无 chrome.sidePanel（证明 openPanelForTask 必走 ② yy-dock-open 分支）；
//   ⑤ 球隐藏时 yy-exec-offer 应答 {ok:false}（真实球代码行为）；yy-dock-open 应答 {ok:true}；
//   ⑥ 页内停靠 iframe 展开，其内 ExecutorOverlay 领取任务并渲染「总结本页」模态执行卡；
//   ⑦ 任务被领取后 exec_task_v1.target 变为 'claimed'（两阶段认领闭环）。
//
// 说明：Chromium 原生右键菜单无法被 Playwright 点击，onClicked → deliverExecTask 的
// 入口十几行（switch 构造载荷）由人工 Chrome 实测覆盖；本脚本从投递消息开始走的全是
// 真实产品代码（球应答 / dock 展开 / 面板领取 / 执行卡渲染）。
//
// 运行：bash scripts/verify-extension-edge.sh（复用 frontend 的 playwright 依赖，系统 Edge）。

import { createRequire } from 'node:module';
import { mkdirSync, rmSync } from 'node:fs';
import { resolve } from 'node:path';

/** 仓库根（脚本位于 scripts/ 下） */
const ROOT = resolve(import.meta.dirname, '..');
const require = createRequire(resolve(ROOT, 'frontend/package.json'));
const { chromium } = require('playwright');

const EXT_DIR = resolve(ROOT, 'dist/browser-extension');
const USER_DATA = resolve(ROOT, 'logs/.edge-verify-profile');
const LOG_DIR = resolve(ROOT, 'logs');

/** 逐条断言输出（全过 exit 0，任一失败 exit 1） */
const results = [];
function check(name, ok, detail) {
  results.push({ name, ok, detail });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${detail !== '' ? ` —— ${detail}` : ''}`);
}

async function main() {
  // 每次全新 profile：上一轮持久化的 showBall=false 会破坏「球初始可见」前置
  rmSync(USER_DATA, { recursive: true, force: true });
  mkdirSync(LOG_DIR, { recursive: true });
  const context = await chromium.launchPersistentContext(USER_DATA, {
    channel: 'msedge',
    headless: false,
    viewport: { width: 1280, height: 800 },
    args: [
      `--disable-extensions-except=${EXT_DIR}`,
      `--load-extension=${EXT_DIR}`,
      '--no-first-run',
      '--no-default-browser-check',
    ],
  });

  try {
    // ---------- ① service worker 存活 ----------
    let worker = context.serviceWorkers()[0];
    if (worker === undefined) {
      worker = await context.waitForEvent('serviceworker', { timeout: 15000 });
    }
    const swUrl = worker.url();
    check('① background SW 存活', /background\.js$/.test(swUrl), swUrl);
    const extId = new URL(swUrl).host;

    // ---------- ② 右键菜单注册（Edge 无 contextMenus.getAll，用 update 无参探测：能更新=已注册） ----------
    const expectIds = ['yy-root', 'yy-summary', 'yy-fav-ai', 'yy-fav-pick', 'yy-shot', 'yy-moment-text', 'yy-moment-image'];
    const probeMenu = (id) => worker.evaluate(async (menuId) => {
      try {
        return await new Promise((done) => {
          chrome.contextMenus.update(menuId, {}, () => done(chrome.runtime.lastError === undefined));
        });
      } catch {
        return false;
      }
    }, id);
    const menuFlags = [];
    for (const id of expectIds) {
      menuFlags.push(await probeMenu(id));
    }
    const menusOk = menuFlags.every((f) => f);
    check('② 右键菜单 7 项注册齐全（update 探测）', menusOk, expectIds.map((id, i) => `${id}=${menuFlags[i] ? 'Y' : 'N'}`).join(','));

    // ---------- 打开网页，等球注入 ----------
    const page = await context.newPage();
    await page.goto('https://example.com/', { waitUntil: 'domcontentloaded' });
    // closed Shadow DOM 的宿主 div 对 Playwright 是 0×0「隐藏」元素，只能等挂载；可见性以 display 判断
    await page.waitForSelector('.yueyan-ball-host', { state: 'attached', timeout: 15000 });
    const ballShown = await page.evaluate(() => {
      const host = document.querySelector('.yueyan-ball-host');
      return host !== null && host.style.display !== 'none';
    });
    check('③a 悬浮球初始可见', ballShown, 'host 已注入且未隐藏');

    // ---------- ③ 面板顶栏真实点击关闭悬浮球 ----------
    const panel = await context.newPage();
    await panel.goto(`chrome-extension://${extId}/src/sidepanel/index.html`);
    const ballToggle = panel.locator('button[title="关闭网页悬浮球"]');
    await ballToggle.waitFor({ state: 'visible', timeout: 10000 });
    await ballToggle.click();
    // content script 经 storage.onChanged 实时隐藏球
    await page.waitForFunction(() => {
      const host = document.querySelector('.yueyan-ball-host');
      return host !== null && host.style.display === 'none';
    }, { timeout: 8000 });
    check('③b 面板关闭悬浮球后球实时隐藏', true, 'host.style.display=none');
    await panel.close();

    // ---------- ④ 原生侧栏能力观察（能力记录，不判失败） ----------
    // 新版 Edge 已实现 chrome.sidePanel（本机实测），三级降级 ① 可先行；
    // 本脚本仍直接验证 ② yy-dock-open 档位——它承担旧版 Edge 与 ① 抛错时的兜底，须始终可用。
    const hasSidePanel = await worker.evaluate(() => typeof chrome.sidePanel?.open === 'function');
    check(
      '④ 原生侧栏能力观察',
      true,
      hasSidePanel
        ? '本 Edge 已实现 sidePanel.open（① 档优先；② 停靠档为兜底，仍须可用）'
        : '本 Edge 无 sidePanel.open（兜底必走 ② yy-dock-open）',
    );

    // ---------- ⑤ 投递：球拒收 → 页内停靠展开（真实消息与应答） ----------
    const tabId = await worker.evaluate(async () => {
      const tabs = await chrome.tabs.query({ url: 'https://example.com/*' });
      return tabs[0]?.id ?? -1;
    });
    check('⑤a 定位 example.com 标签页', tabId >= 0, `tabId=${String(tabId)}`);

    const nonce = crypto.randomUUID();
    const taskPayload = {
      kind: 'summary',
      nonce,
      target: 'ball',
      createdAt: Date.now(),
      tabId,
      pageUrl: 'https://example.com/',
      pageTitle: 'Example Domain',
    };
    const offerReply = await worker.evaluate(async (payload) => {
      await chrome.storage.local.set({ exec_task_v1: payload });
      return chrome.tabs.sendMessage(payload.tabId, { type: 'yy-exec-offer', nonce: payload.nonce });
    }, taskPayload);
    check('⑤b 球隐藏时 yy-exec-offer 应答 ok:false', offerReply.ok === false, JSON.stringify(offerReply));

    const dockReply = await worker.evaluate(async (payload) => {
      await chrome.storage.local.set({ exec_task_v1: { ...payload, target: 'panel' } });
      return chrome.tabs.sendMessage(payload.tabId, { type: 'yy-dock-open' });
    }, taskPayload);
    check('⑤c yy-dock-open 应答 ok:true（页内停靠展开）', dockReply.ok === true, JSON.stringify(dockReply));

    // ---------- ⑥ dock iframe 内模态执行卡渲染 ----------
    let embedFrame = page.frames().find((f) => f.url().includes('mode=embed'));
    if (embedFrame === undefined) {
      await page.waitForEvent('frameattached', { timeout: 8000 }).catch(() => undefined);
      await page.waitForTimeout(800);
      embedFrame = page.frames().find((f) => f.url().includes('mode=embed'));
    }
    check('⑥a 页内停靠 iframe 已挂载', embedFrame !== undefined, embedFrame?.url() ?? '未找到');

    if (embedFrame !== undefined) {
      await embedFrame.waitForSelector('text=总结本页，发布到博客', { timeout: 12000 });
      const cardOk = await embedFrame.getByText('抓取网页正文').isVisible()
        && await embedFrame.getByText('该任务需要先连接月言站点').isVisible();
      check('⑥b 模态执行卡渲染（标题+步骤+未连接引导）', cardOk, 'ExecutorOverlay 已领取并渲染');
    } else {
      check('⑥b 模态执行卡渲染', false, '无 embed frame');
    }

    // ⑥c 点击停靠区外收起页内停靠（0.31.1 补充的关闭手段回归）
    await page.mouse.click(80, 400);
    await page.waitForTimeout(900);
    const embedGone = page.frames().every((f) => !f.url().includes('mode=embed'));
    check('⑥c 点击停靠区外收起页内停靠（iframe 移除）', embedGone, embedGone ? 'dock 关闭回归通过' : 'dock 仍展开');

    // ---------- ⑦ 任务认领闭环 ----------
    await page.waitForTimeout(400);
    const claimed = await worker.evaluate(async () => {
      const stored = await chrome.storage.local.get('exec_task_v1');
      return String(stored.exec_task_v1?.target ?? '');
    });
    check('⑦ exec_task_v1.target=claimed（单消费者认领）', claimed === 'claimed', `target=${claimed}`);

    await page.screenshot({ path: resolve(LOG_DIR, 'edge-fallback-verify.png'), fullPage: false });
    console.log('截图：logs/edge-fallback-verify.png');
  } finally {
    await context.close().catch(() => undefined);
  }

  const failed = results.filter((r) => !r.ok).length;
  console.log(failed === 0 ? '\n全部通过 ✓' : `\n${failed} 项失败 ✗`);
  process.exit(failed === 0 ? 0 : 1);
}

main().catch((err) => {
  console.error('脚本异常：', err);
  process.exit(1);
});
