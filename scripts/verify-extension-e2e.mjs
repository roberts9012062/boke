// scripts/verify-extension-e2e.mjs
// 插件 v0.31.0 右键任务「真实发布」E2E：以本地 mock 站点承载开放网关契约（真实后端为
// 生产数据库，不适合投放测试内容），插件侧全链路（连接 UI → AI 流式总结 → 文章发布 →
// 说说草稿篮文字+图片组合 → 转存 → 发布）均走真实产品代码与真实网络请求。
//
// 覆盖点：
//   E1 连接站点：真实填写欢迎页表单 → /open/me + /open/meta 校验通过进入 ready；
//   E2 球可见时 yy-exec-offer 应答 ok:true，球旁执行框 iframe 挂载（mode=exec&nonce）；
//   E3 总结任务：dock 抓正文 → mock SSE 流式总结填充编辑区（执行过程 → 交互）；
//   E4 发布文章载荷：post_kind=article / status=published / content 含总结与原文出处；
//   E5 完成态含「查看文章」链接（/posts/{id}）；
//   E6 说说组合流：文字任务注入选中段落 → 图片任务追加缩略图（跨右键累积草稿篮）；
//   E7 发布说说载荷：post_kind=moment / media_ids 含转存 ID / 正文含 <img> 与选中文字；
//   E8 发布成功后草稿篮清空（exec_moment_draft_v1 移除）。
//
// 运行：bash scripts/verify-extension-e2e.sh（复用 frontend 的 playwright，系统 Chrome）。

import { createRequire } from 'node:module';
import { mkdirSync, rmSync } from 'node:fs';
import { createServer } from 'node:http';
import { deflateSync } from 'node:zlib';
import { resolve } from 'node:path';

const ROOT = resolve(import.meta.dirname, '..');
const require = createRequire(resolve(ROOT, 'frontend/package.json'));
const { chromium } = require('playwright');

const EXT_DIR = resolve(ROOT, 'dist/browser-extension');
const USER_DATA = resolve(ROOT, 'logs/.e2e-profile');
const LOG_DIR = resolve(ROOT, 'logs');
const MOCK_PORT = 18787;
const MOCK_IMG_PORT = 18789;
const MOCK_IMG_BASE = `http://127.0.0.1:${MOCK_IMG_PORT}`;
const MOCK_BASE = `http://127.0.0.1:${MOCK_PORT}`;
const MOCK_KEY = 'oa_mock_e2e_key';
const QUOTE_TEXT = '山有木兮木有枝，心悦君兮君不知。';

/** mock 站点捕获的发布载荷（断言用） */
const captured = { posts: [], transfers: [], tgUploads: [], assists: [] };

/** 1×1 PNG（base64，说说缩略图展示用） */
const PNG_1PX = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
  'base64',
);

/** 生成纯色 PNG（zlib.crc32 + 最小 chunk 拼装；尺寸须 ≥250 才不会被插件的小图过滤规则丢弃） */
function makePng(width, height) {
  const chunk = (type, data) => {
    const out = Buffer.alloc(8 + data.length + 4);
    out.writeUInt32BE(data.length, 0);
    out.write(type, 4, 'ascii');
    data.copy(out, 8);
    out.writeUInt32BE(zlibCrc32(Buffer.concat([Buffer.from(type, 'ascii'), data])), 8 + data.length);
    return out;
  };
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8; // 位深
  ihdr[9] = 2; // 真彩色 RGB
  const row = Buffer.alloc(1 + width * 3);
  for (let x = 0; x < width; x += 1) {
    row[1 + x * 3] = 0x66;
    row[2 + x * 3] = 0x88;
    row[3 + x * 3] = 0xcc;
  }
  const raw = Buffer.concat(Array.from({ length: height }, () => row));
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr),
    chunk('IDAT', deflateSync(raw)),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

const zlibCrc32 = (buf) => {
  let c = ~0;
  for (const byte of buf) {
    c ^= byte;
    for (let k = 0; k < 8; k += 1) {
      c = (c >>> 1) ^ (0xedb88320 & -(c & 1));
    }
  }
  return ~c >>> 0;
};

/** data: 形式的 1×1 PNG（说说右键图片任务源；避开主机授权弹窗） */
const PNG_DATA_URL = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==';

/** 480×270 测试配图（真实尺寸，避免被内容区图片收集的小图过滤丢弃） */
const IMG_PNG = makePng(480, 270);

/** 统一信封 */
function envelope(res, data, status = 200) {
  res.writeHead(status, { 'Content-Type': 'application/json; charset=utf-8' });
  res.end(JSON.stringify({ code: 0, message: '', data }));
}

/** 起本地 mock 站点：CORS 放行扩展 Origin，承载开放网关契约（双端口：站点 / 外链图源） */
async function startMock() {
  const handler = (req, res) => {
    const origin = req.headers.origin ?? '*';
    res.setHeader('Access-Control-Allow-Origin', origin);
    res.setHeader('Access-Control-Allow-Headers', 'Content-Type, X-Api-Key');
    res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
    if (req.method === 'OPTIONS') {
      res.writeHead(204);
      res.end();
      return;
    }

    if (req.method === 'GET' && req.url === '/page') {
      res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
      res.end(`<!doctype html><html><head><title>测试文章页</title></head><body>
        <h1>Mock 测试文章</h1>
        <p>这是用于网页总结的测试正文段落，包含足够长度的文字以通过非空校验。</p>
        <p id="quote">${QUOTE_TEXT}</p>
        <img src="http://127.0.0.1:18789/img.png" alt="测试配图">
        <img src="http://127.0.0.1:18789/missing.png" alt="失效图" width="480" height="270">
      </body></html>`);
      return;
    }
    if (req.method === 'GET' && req.url === '/img.png') {
      res.writeHead(200, { 'Content-Type': 'image/png', 'Content-Length': IMG_PNG.length });
      res.end(IMG_PNG);
      return;
    }
    if (req.method === 'GET' && req.url === '/api/v1/open/meta') {
      envelope(res, {
        site_name: '月言测试站', site_description: 'E2E mock', default_theme: 'cool-moon',
        maintenance_mode: 'off', nav: [],
      });
      return;
    }
    if (req.method === 'GET' && req.url === '/api/v1/open/me') {
      envelope(res, {
        id: 1, username: 'tester', nickname: '测试博主', avatar_url: '', bio: '',
        role: 'admin', post_count: 0, like_count: 0, follower_count: 0, following_count: 0,
        created_at: '2026-01-01T00:00:00Z',
      });
      return;
    }
    if (req.method === 'GET' && req.url === '/api/v1/open/ai/models') {
      envelope(res, { providers: [{ id: 1, name: 'mock', models: ['mock-chat'], enabled: true }] });
      return;
    }
    if (req.method === 'POST' && req.url === '/api/v1/open/ai/chat/stream') {
      // 按调用场景分流：文章元信息返回标题/SEO/标签 JSON，书签分类返回推荐 JSON，
      // 其余返回 Markdown 总结
      let reqBody = '';
      req.on('data', (c) => { reqBody += c; });
      req.on('end', () => {
        res.writeHead(200, { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' });
        if (reqBody.includes('生成文章标题')) {
          res.write('data: {"text":"{\\"title\\":\\"Mock AI 标题\\",\\"seo_title\\":\\"Mock SEO 标题\\",\\"seo_description\\":\\"这是 mock 生成的 SEO 描述，用于验证右键总结发布的元信息装配链路。\\",\\"tags\\":[\\"mock\\",\\"测试\\",\\"总结\\"]}"}\n\n');
        } else if (reqBody.includes('选出最合适的收藏位置')) {
          res.write('data: {"text":"{\\"folder\\":\\"\\",\\"new_folder\\":\\"工具箱\\",\\"title\\":\\"Example 测试站\\"}"}\n\n');
        } else {
          res.write('data: {"text":"Mock 总结第一段。"}\n\n');
          res.write('data: {"text":"\\n- 要点 A\\n- 要点 B"}\n\n');
          res.write('data: {"text":"\\n\\n💡 点评：mock 测试点评"}\n\n');
        }
        res.write('data: [DONE]\n\n');
        res.end();
      });
      return;
    }
    if (req.method === 'POST' && req.url === '/api/v1/open/media/transfer') {
      let body = '';
      req.on('data', (c) => { body += c; });
      req.on('end', () => {
        captured.transfers.push(JSON.parse(body));
        envelope(res, { url: '/media/mock-img-1.png', media_id: 101, mime_type: 'image/png', size_bytes: IMG_PNG.length });
      });
      return;
    }
    if (req.method === 'POST' && req.url === '/api/v1/open/ai/assist') {
      let body = '';
      req.on('data', (c) => { body += c; });
      req.on('end', () => {
        captured.assists.push(JSON.parse(body));
        envelope(res, { action: 'recognize', text: '**Mock 识别结果**\n\n- 页面标题为「测试文章页」\n- 包含一段正文与一张配图' });
      });
      return;
    }
    if (req.method === 'POST' && req.url === '/api/v1/open/plugins/tg-image-bed/upload') {
      let body = '';
      req.on('data', (c) => { body += c; });
      req.on('end', () => {
        captured.tgUploads.push(JSON.parse(body));
        envelope(res, { url: 'https://tg-bed.example.com/f/mock-tg-1.png', storage_key: 'sk1', mime: 'image/png', size: 96, markdown: '' });
      });
      return;
    }
    if (req.method === 'POST' && req.url === '/api/v1/open/posts') {
      let body = '';
      req.on('data', (c) => { body += c; });
      req.on('end', () => {
        captured.posts.push(JSON.parse(body));
        envelope(res, { id: 4242 });
      });
      return;
    }
    res.writeHead(404, { 'Content-Type': 'text/plain' });
    res.end('not found');
  };
  // 双端口（两个 server 实例共享 handler）：18787 扮演站点、18789 扮演外链图源
  //（同源图会被转存逻辑按「本站图」跳过）
  const server = createServer(handler);
  const imgServer = createServer(handler);
  await new Promise((done) => server.listen(MOCK_PORT, '127.0.0.1', () => done()));
  await new Promise((done) => imgServer.listen(MOCK_IMG_PORT, '127.0.0.1', () => done()));
  return {
    close: () => {
      server.close();
      imgServer.close();
    },
  };
}

const results = [];
function check(name, ok, detail) {
  results.push({ name, ok });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${detail !== '' ? ` —— ${detail}` : ''}`);
}

/** 简易 sleep（SW 就绪轮询用） */
function page0Timeout(ms) {
  return new Promise((done) => setTimeout(done, ms));
}

async function main() {
  rmSync(USER_DATA, { recursive: true, force: true });
  mkdirSync(LOG_DIR, { recursive: true });
  const server = await startMock();
  void server; // 句柄仅用于关闭（finally 中 server.close()）
  const context = await chromium.launchPersistentContext(USER_DATA, {
    channel: 'msedge',
    headless: false,
    permissions: ['clipboard-read', 'clipboard-write'],
    viewport: { width: 1280, height: 800 },
    args: [
      `--disable-extensions-except=${EXT_DIR}`,
      `--load-extension=${EXT_DIR}`,
      '--no-first-run',
      '--no-default-browser-check',
    ],
  });

  try {
    // SW 就绪：轮询获取（Chrome 首启注册 SW 可能慢于事件监听挂载，事件会被错过）
    let worker = context.serviceWorkers()[0];
    for (let i = 0; i < 30 && worker === undefined; i += 1) {
      await page0Timeout(1000);
      worker = context.serviceWorkers()[0];
    }
    check('E-pre service worker 就绪', worker !== undefined, worker?.url() ?? '未启动');
    const extId = new URL(worker.url()).host;

    // 打开 mock 测试页（球注入）
    const page = await context.newPage();
    await page.goto(`${MOCK_BASE}/page`, { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('.yueyan-ball-host', { state: 'attached', timeout: 15000 });
    const tabId = await worker.evaluate(async (base) => {
      const tabs = await chrome.tabs.query({ url: `${base}/page` });
      return tabs[0]?.id ?? -1;
    }, MOCK_BASE);
    check('E0 测试页与球就绪', tabId >= 0, `tabId=${String(tabId)}`);

    // ---------- E1 真实 UI 连接站点 ----------
    const panel = await context.newPage();
    await panel.goto(`chrome-extension://${extId}/src/sidepanel/index.html`);
    await panel.getByPlaceholder('如 https://blog.example.com').fill(MOCK_BASE);
    await panel.getByPlaceholder('oa_ 开头，在后台「接口开放」生成').fill(MOCK_KEY);
    await panel.getByRole('button', { name: '连接站点' }).click();
    await panel.getByText('首页').first().waitFor({ timeout: 10000 });
    const connected = await worker.evaluate(async (expectKey) => {
      const stored = await chrome.storage.local.get('plugin_settings_v1');
      return stored.plugin_settings_v1?.apiKey === expectKey;
    }, MOCK_KEY);
    check('E1 连接站点（真实表单 → me/meta 校验通过）', connected, `apiKey 已持久化=${String(connected)}`);
    await panel.close();

    // ---------- E2~E5 总结发布 ----------
    const summaryTask = { kind: 'summary', pageUrl: `${MOCK_BASE}/page`, pageTitle: '测试文章页' };
    const nonce1 = crypto.randomUUID();
    const offer1 = await worker.evaluate(async (payload) => {
      await chrome.storage.local.set({ exec_task_v1: { ...payload.task, nonce: payload.nonce, target: 'ball', createdAt: Date.now() } });
      return chrome.tabs.sendMessage(payload.task.tabId, { type: 'yy-exec-offer', nonce: payload.nonce });
    }, { task: { ...summaryTask, tabId }, nonce: nonce1 });
    check('E2 球可见 offer ok:true 且执行框投递', offer1.ok === true, JSON.stringify(offer1));

    let execFrame = page.frames().find((f) => f.url().includes('mode=exec'));
    if (execFrame === undefined) {
      await page.waitForTimeout(1000);
      execFrame = page.frames().find((f) => f.url().includes('mode=exec'));
    }
    check('E2b 球旁执行框 iframe 挂载', execFrame !== undefined, execFrame?.url() ?? '未找到');

    // E3 等编辑态：流式总结完成 → markdown 渲染为富文本（原文图片内嵌）；
    //    并行 AI 元信息回填标题与标签（ArticlePanel 同款体验）
    const titleInput = execFrame.getByPlaceholder('文章标题');
    await titleInput.waitFor({ state: 'visible', timeout: 20000 });
    await execFrame.waitForFunction(() => {
      const body = document.body.innerText ?? '';
      const imgCount = document.querySelectorAll('[contenteditable="true"] img').length;
      return body.includes('Mock 总结第一段') && imgCount === 1; // 恰好 1 图：404 死图被过滤
    }, { timeout: 15000 });
    await execFrame.waitForFunction(() => {
      const el = document.querySelector('input[placeholder="文章标题"]');
      return el !== null && el.value === 'Mock AI 标题';
    }, { timeout: 15000 });
    const tagsVal = await execFrame.getByPlaceholder('标签（AI 已生成，可修改，逗号分隔 ≤5 个）').inputValue();
    const e3ok = tagsVal.includes('mock') && tagsVal.includes('总结');
    check('E3 富文本编辑态（渲染后正文含图 + AI 标题/标签回填）', e3ok, `tags=${tagsVal}`);

    // E4 发布文章（图床路由：默认 none → 站点转存并关联 media_ids；SEO/标签随发布提交）
    await execFrame.locator('button:has-text("发布到博客")').click();
    await execFrame.getByText('已发布到博客').waitFor({ timeout: 15000 });
    const articlePost = captured.posts.find((p) => p.post_kind === 'article');
    const e4ok = articlePost !== undefined
      && articlePost.status === 'published'
      && articlePost.title === 'Mock AI 标题'
      && articlePost.content.includes('Mock 总结第一段')
      && articlePost.content.includes('原文：')
      && articlePost.content.includes('/media/mock-img-1.png')
      && articlePost.media_ids.includes(101)
      && articlePost.tags.length === 3
      && articlePost.seo?.seo_title === 'Mock SEO 标题'
      && captured.transfers.some((t) => t.url === `${MOCK_IMG_BASE}/img.png`)
      && !captured.transfers.some((t) => t.url.includes('missing.png'));
    check(
      'E4 文章发布载荷正确（AI 标题/标签/SEO + 转存图 + media_ids）',
      e4ok,
      articlePost ? `title=${articlePost.title} tags=${JSON.stringify(articlePost.tags)} seo=${articlePost.seo?.seo_title ?? '无'}` : '未收到',
    );

    // E5 完成态链接
    const viewLink = execFrame.locator(`a[href="${MOCK_BASE}/posts/4242"]`);
    check('E5 完成态含查看文章链接', await viewLink.count() > 0, `${MOCK_BASE}/posts/4242`);
    await page.screenshot({ path: resolve(LOG_DIR, 'e2e-summary-done.png') });

    // ---------- E6~E8 说说组合流（文字 + 图片跨任务累积；发布图床预设 tg） ----------
    await worker.evaluate(async () => {
      const stored = await chrome.storage.local.get('plugin_settings_v1');
      const settings = stored.plugin_settings_v1;
      await chrome.storage.local.set({ plugin_settings_v1: { ...settings, publishImageBed: 'tg' } });
    });
    const nonce2 = crypto.randomUUID();
    const offer2 = await worker.evaluate(async (payload) => {
      await chrome.storage.local.set({ exec_task_v1: { ...payload.task, nonce: payload.nonce, target: 'ball', createdAt: Date.now() } });
      return chrome.tabs.sendMessage(payload.task.tabId, { type: 'yy-exec-offer', nonce: payload.nonce });
    }, { task: { kind: 'moment', tabId, pageUrl: `${MOCK_BASE}/page`, pageTitle: '测试文章页', addText: QUOTE_TEXT, addImage: '' }, nonce: nonce2 });
    check('E6a 文字任务 offer ok:true', offer2.ok === true, '执行框重载至新任务');

    await page.waitForTimeout(1200);
    execFrame = page.frames().find((f) => f.url().includes('mode=exec'));
    const momentEditor = execFrame.getByPlaceholder('要发布的文字（右键选中文字 / 图片可持续加入草稿）…');
    await momentEditor.waitFor({ state: 'visible', timeout: 15000 });
    const textInjected = await momentEditor.inputValue();
    check('E6b 选中文字注入草稿篮', textInjected.includes(QUOTE_TEXT), textInjected.slice(0, 24));

    const nonce3 = crypto.randomUUID();
    await worker.evaluate(async (payload) => {
      await chrome.storage.local.set({ exec_task_v1: { ...payload.task, nonce: payload.nonce, target: 'ball', createdAt: Date.now() } });
      return chrome.tabs.sendMessage(payload.task.tabId, { type: 'yy-exec-offer', nonce: payload.nonce });
    }, { task: { kind: 'moment', tabId, pageUrl: `${MOCK_BASE}/page`, pageTitle: '测试文章页', addText: '', addImage: PNG_DATA_URL }, nonce: nonce3 });
    await page.waitForTimeout(1200);
    execFrame = page.frames().find((f) => f.url().includes('mode=exec'));
    const thumb = execFrame.locator('img[src^="data:image"]');
    await thumb.waitFor({ state: 'visible', timeout: 10000 });
    check('E6c 图片任务追加缩略图（跨任务累积）', true, '草稿篮含 1 图');

    // E7 发布说说
    await execFrame.getByRole('button', { name: '发布', exact: true }).click();
    await execFrame.getByText('说说已发布').waitFor({ timeout: 15000 });
    const momentPost = captured.posts.find((p) => p.post_kind === 'moment');
    const e7ok = momentPost !== undefined
      && momentPost.content.includes('https://tg-bed.example.com/f/mock-tg-1.png')
      && momentPost.content.includes(QUOTE_TEXT)
      && momentPost.media_ids.length === 0
      && captured.tgUploads.filter((t) => typeof t.content_b64 === 'string' && t.content_b64.length > 50).length === 1;
    check(
      'E7 说说按图床设置发布（TG 直传 + 仅正文引用 + 文字）',
      e7ok,
      momentPost ? `有效上传=${String(captured.tgUploads.filter((t) => (t.content_b64 ?? '').length > 50).length)}（另含可用性探测） media_ids=${JSON.stringify(momentPost.media_ids)}` : '未收到',
    );

    // E8 草稿篮清空
    await page.waitForTimeout(300);
    const draftGone = await worker.evaluate(async () => {
      const stored = await chrome.storage.local.get('exec_moment_draft_v1');
      return stored.exec_moment_draft_v1 === undefined;
    });
    check('E8 发布成功后草稿篮清空', draftGone, `exec_moment_draft_v1 移除=${String(draftGone)}`);
    await page.screenshot({ path: resolve(LOG_DIR, 'e2e-moment-done.png') });

    // ---------- E9~E10 AI 自动收藏（连接态真实链路） ----------
    const nonce4 = crypto.randomUUID();
    const offer4 = await worker.evaluate(async (payload) => {
      await chrome.storage.local.set({ exec_task_v1: { ...payload.task, nonce: payload.nonce, target: 'ball', createdAt: Date.now() } });
      return chrome.tabs.sendMessage(payload.task.tabId, { type: 'yy-exec-offer', nonce: payload.nonce });
    }, { task: { kind: 'bookmark', mode: 'ai', tabId, pageUrl: `${MOCK_BASE}/page`, pageTitle: '测试文章页' }, nonce: nonce4 });
    check('E9a AI 收藏任务 offer ok:true', offer4.ok === true, '执行框重载至收藏任务');

    await page.waitForTimeout(1500);
    execFrame = page.frames().find((f) => f.url().includes('mode=exec'));
    // AI 推荐填充：标题输入框值变为 mock 推荐标题；下拉预选「新建顶层文件夹」并填名
    const bookmarkTitleInput = execFrame.getByPlaceholder('书签标题');
    await bookmarkTitleInput.waitFor({ state: 'visible', timeout: 15000 });
    await execFrame.waitForFunction(() => {
      const el = document.querySelector('input[placeholder="书签标题"]');
      return el !== null && el.value === 'Example 测试站';
    }, { timeout: 15000 });
    const newFolderFilled = await execFrame.getByPlaceholder('新文件夹名称').inputValue();
    check('E9b AI 推荐 JSON 解析并预填（新建「工具箱」+ 标题）', newFolderFilled === '工具箱', `newFolder=${newFolderFilled}`);

    await execFrame.getByRole('button', { name: '收藏', exact: true }).click();
    await execFrame.getByText('已收藏到「工具箱」').waitFor({ timeout: 10000 });
    const bookmarkSaved = await worker.evaluate(async (expectUrl) => {
      const stored = await chrome.storage.local.get('bookmarks_v2');
      const roots = stored.bookmarks_v2?.roots ?? [];
      const folder = roots.find((r) => r.kind === 'folder' && r.title === '工具箱');
      return folder !== undefined && folder.children[0]?.title === 'Example 测试站' && folder.children[0]?.url === expectUrl;
    }, `${MOCK_BASE}/page`);
    check('E10 收藏写入书签树（新建文件夹 + 链接节点）', bookmarkSaved, 'bookmarks_v2.roots 含「工具箱/Example 测试站」');

    // ---------- E11 执行框关闭（BUG2 回归：× / 完成 → background 转发 → 球收起并移除 iframe） ----------
    await execFrame.getByRole('button', { name: '完成', exact: true }).click();
    await page.waitForTimeout(1500);
    const frameGone = page.frames().every((f) => !f.url().includes('mode=exec'));
    check('E11 完成按钮收起执行框（iframe 已移除）', frameGone, frameGone ? 'yy-exec-close 转发链路工作正常' : '执行框仍在页面');

    // ---------- E12 截图分析直通：携截图与选区投递 → 直接分析出结果 ----------
    const nonce5 = crypto.randomUUID();
    const offer5 = await worker.evaluate(async (payload) => {
      await chrome.storage.local.set({ exec_task_v1: { ...payload.task, nonce: payload.nonce, target: 'ball', createdAt: Date.now() } });
      return chrome.tabs.sendMessage(payload.task.tabId, { type: 'yy-exec-offer', nonce: payload.nonce });
    }, { task: { kind: 'shot', tabId, pageUrl: `${MOCK_BASE}/page`, pageTitle: '测试文章页', imageDataUrl: PNG_DATA_URL, rect: { x: 0, y: 0, w: 10, h: 10, dpr: 1 } }, nonce: nonce5 });
    check('E12a 截图直通任务 offer ok:true', offer5.ok === true, '执行框打开（右键已框选完成的直通态）');

    await page.waitForTimeout(1500);
    execFrame = page.frames().find((f) => f.url().includes('mode=exec'));
    await execFrame.locator('.md-body strong', { hasText: 'Mock 识别结果' }).waitFor({ timeout: 15000 });
    await execFrame.locator('.md-body li', { hasText: '测试文章页' }).waitFor({ timeout: 5000 });
    const stepsOk = await execFrame.getByText('框选分析区域').isVisible() && await execFrame.getByText('AI 识图分析').isVisible();
    const e12b = captured.assists.length === 1
      && captured.assists[0].action === 'recognize'
      && typeof captured.assists[0].image_url === 'string'
      && captured.assists[0].image_url.startsWith('data:image');
    check('E12b 直通分析完成（跳过框选，识图请求载荷正确）', e12b && stepsOk, `assists=${String(captured.assists.length)} action=${String(captured.assists[0]?.action)}`);

    // E12d 复制文字：点击按钮 → 剪贴板含识别文本 + 按钮反馈
    await execFrame.getByRole('button', { name: '复制文字' }).click();
    await execFrame.getByRole('button', { name: '已复制 ✓' }).waitFor({ timeout: 5000 });
    const clipText = await page.evaluate(() => navigator.clipboard.readText());
    check('E12d 复制文字按钮（剪贴板含识别结果）', clipText.includes('Mock 识别结果'), `剪贴板 ${String(clipText.length)} 字`);

    // 关闭按钮收起（× 同走 yy-exec-close 转发链路）
    await execFrame.locator('button[aria-label="关闭"]').click();
    await page.waitForTimeout(1500);
    const shotFrameGone = page.frames().every((f) => !f.url().includes('mode=exec'));
    check('E12c 关闭按钮收起执行框', shotFrameGone, shotFrameGone ? '× 链路工作正常' : '执行框仍在页面');
    console.log('截图：logs/e2e-summary-done.png / logs/e2e-moment-done.png');
  } finally {
    await context.close().catch(() => undefined);
    server.close();
  }

  const failed = results.filter((r) => !r.ok).length;
  console.log(failed === 0 ? '\n全部通过 ✓' : `\n${failed} 项失败 ✗`);
  process.exit(failed === 0 ? 0 : 1);
}

main().catch((err) => {
  console.error('脚本异常：', err);
  process.exit(1);
});
