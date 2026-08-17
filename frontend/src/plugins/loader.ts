// frontend/src/plugins/loader.ts
// 插件前端模块加载器（M3.6）：从 /plugin-assets/{id}/frontend/ 拉取资源并动态执行。
// 说明：
//   - 资源安装时已 checksums 全量校验（落盘即可信，加载时不再重复校验）
//   - 模块用 Blob URL + 动态 import 执行（webpackIgnore 跳过打包器静态解析）
//   - 契约对齐 docs/plugin-dev-guide.md 8.1：ESM 默认导出 register(ctx) 返回清理函数
export interface PluginRegister {
  (ctx: PluginCtx): (() => void) | void;
}

// PluginCtx 注册上下文（register 入参）。
export interface PluginCtx {
  slot: string; // 槽位名（theme.header/post.footer/comment.footer/admin.menu/comment.item）
  el: HTMLElement; // 挂载点 DOM（插件渲染目标）
  api: PluginApi; // 受限 API 客户端（仅插件自定义 API，带主站凭证）
  user: PluginUser | null; // 用户信息（脱敏，不含密钥；未登录为 null）
  props?: Record<string, unknown>; // 槽位透传参数（M3.9：评论对象/页面参数等）
}

// PluginApi 受限 API 客户端（路径为插件自定义 API，如 "/ping"）。
export interface PluginApi {
  get<T>(path: string): Promise<T>;
  post<T>(path: string, body?: unknown): Promise<T>;
}

// PluginUser 用户信息（脱敏）。
export interface PluginUser {
  id: number;
  name: string;
  role: string;
}

// PluginManifest 扩展点声明（包内 frontend/manifest.json；pages 为独立页面声明，M3.9；
// blocks 为内容块声明，B4 keyed renderer；siteNav 为前台导航项声明，site.page 扩展配套）。
export interface PluginManifest {
  extensionPoints: ExtensionPoint[];
  pages?: PluginPage[];
  blocks?: PluginBlock[]; // 内容块注册（B4：正文 data-plugin-block 节点按 type 分发渲染）
  siteNav?: PluginSiteNav[]; // 前台头部导航项注册（running 插件自动并入前台导航）
}

// PluginSiteNav 前台导航项声明（插件注册到前台头部导航；path 仅允许站内路径）。
export interface PluginSiteNav {
  label: string; // 显示文案（≤30 字符）
  path: string; // 站内路径（/ 开头，如 /plugins/{id}/{route}）
  icon?: string; // 图标 key（保留；前台导航暂不渲染）
}

// PluginBlock 内容块声明（正文嵌入协议：宿主扫描 div[data-plugin-block="type"] 节点，
// 按 type 查本声明分发到插件 entry 渲染——对齐 dsh keyed renderer 思想）。
export interface PluginBlock {
  type: string; // 块类型（全局唯一小写，如 "vote"；正文标记 data-plugin-block 的取值）
  entry: string; // ESM 入口文件（默认导出 register(ctx)，与槽位契约一致）
}

// ExtensionPoint 扩展点（slot 槽位 + entry ESM 文件 + 可选 props + 挂载模式）。
export interface ExtensionPoint {
  slot: string;
  entry: string;
  props?: Record<string, unknown>;
  mode?: "append" | "replace"; // M3.9：append=追加共存（默认）；replace=替换槽位默认内容
}

// PluginPage 插件独立页面声明（M3.9：admin.page 能力——壳路由 /admin/plugin-pages/{id}/{route}；
// scope=site 时为前台公开页面（site.page 能力）——壳路由 /plugins/{id}/{route}，访客可访问）。
export interface PluginPage {
  route: string; // 页面路由名（如 "demo"）
  entry: string; // 页面入口（sandbox=false 为 ESM 模块文件；sandbox=true 为 HTML 页面）
  sandbox?: boolean; // E1 沙箱模式：true 经 iframe 强隔离加载（第三方插件推荐），缺省同源 ESM
  scope?: "admin" | "site"; // 页面作用域：admin（默认，后台权限守卫）/ site（前台公开）
}

// 资源基础路径（与后端 /plugin-assets/:id 对应）。
const ASSETS_BASE = "/plugin-assets";

// 模块缓存（"pluginId/entry" → 模块；同插件只加载一次）。
const moduleCache = new Map<string, PluginModule>();

// 扩展点声明缓存（pluginId → manifest）。
const manifestCache = new Map<string, PluginManifest>();

// PluginModule 动态导入的模块（默认导出为 register）。
export interface PluginModule {
  default: PluginRegister;
}

// fetchManifest 拉取插件扩展点声明（缓存）。
export async function fetchManifest(pluginId: string): Promise<PluginManifest> {
  const cached = manifestCache.get(pluginId);
  if (cached) {
    return cached;
  }
  const res = await fetch(`${ASSETS_BASE}/${pluginId}/frontend/manifest.json`, { cache: "no-store" });
  if (!res.ok) {
    throw new Error(`插件 ${pluginId} 前端清单拉取失败`);
  }
  const manifest = (await res.json()) as PluginManifest;
  manifestCache.set(pluginId, manifest);
  return manifest;
}

// loadModule 加载插件 ESM 模块（真实同源 URL + 原生 import；模块缓存）。
// 说明：不用 webpackIgnore 动态 import（打包器运行时走 eval，被 CSP 'unsafe-eval' 拦截）——
//       经内联 <script type="module"> 桥接执行原生 import(同源 URL)，不受 CSP eval 限制。
//       必须用真实 URL 而非 Blob URL：blob: 为非层级 scheme，模块内 import 说明符
//       （如共享 SDK "/plugin-sdk/shared.js"、相对路径 "./x.js"）一律无法解析
//       （历史 bug：E2 共享 SDK 上线即崩）；真实同源 URL 下浏览器原生解析整个模块图。
//       权衡：同一页面会话内模块图由浏览器永久缓存——停用再启用（不刷新页面）复用
//       旧模块闭包，插件前端升级需刷新页面生效（可接受，管理操作后通常伴随刷新）。
export async function loadModule(pluginId: string, entry: string): Promise<PluginModule> {
  const key = `${pluginId}/${entry}`;
  const cached = moduleCache.get(key);
  if (cached) {
    return cached;
  }
  // 模块 URL 附加周期指纹（30 秒粒度）：Chromium 模块图按 URL 进程级缓存，
  // 固定 URL 在 webview 不重启时永远复用旧模块实例（文档刷新也不重取）——
  // 周期性变更 query 强制重新 fetch，插件前端升级约 30 秒内自动生效；
  // 周期内 URL 稳定，正常浏览仍走浏览器 HTTP 缓存协商
  const bust = `_ts=${Math.floor(Date.now() / 30_000)}`;
  const sep = entry.includes("?") ? "&" : "?";
  const url = `${ASSETS_BASE}/${pluginId}/frontend/${entry}${sep}${bust}`;
  try {
    const mod = await importViaNativeEsm(url);
    moduleCache.set(key, mod);
    return mod;
  } catch (e) {
    throw new Error(`插件 ${pluginId} 前端模块加载失败（${entry}）：${String(e)}`);
  }
}

// 内联模块桥接的全局注册表（并发加载按唯一 id 区分；模块执行后清理）。
declare global {
  interface Window {
    __yueyanPluginModules?: Record<string, { resolve: (m: PluginModule) => void; reject: (e: unknown) => void }>;
  }
}

// importViaNativeEsm 经内联 <script type="module"> 执行原生 import(url)。
// url 必须是同源绝对/相对 URL（真实资源地址；模块内 import 由浏览器按此基址解析）。
async function importViaNativeEsm(url: string): Promise<PluginModule> {
  return new Promise<PluginModule>((resolve, reject) => {
    // 注册回调（按唯一 id，支持并发加载多个插件模块）
    const id = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    window.__yueyanPluginModules = window.__yueyanPluginModules ?? {};
    window.__yueyanPluginModules[id] = { resolve, reject };

    const script = document.createElement("script");
    script.type = "module";
    // 原生 ESM 动态导入（非打包器运行时，不触发 CSP unsafe-eval）
    script.textContent =
      `import("${url}").then(m => { ` +
      `window.__yueyanPluginModules["${id}"]?.resolve(m); ` +
      `delete window.__yueyanPluginModules["${id}"]; ` +
      `}).catch(e => { ` +
      `window.__yueyanPluginModules["${id}"]?.reject(e); ` +
      `delete window.__yueyanPluginModules["${id}"]; ` +
      `})`;
    document.head.appendChild(script);
    // 脚本自身执行失败（语法错误等）时兜底清理
    script.onerror = () => {
      delete window.__yueyanPluginModules?.[id];
      reject(new Error(`插件模块脚本执行失败（${url}）`));
    };
  });
}

// clearPlugin 清理插件缓存（停用/卸载时调用，下次启用重新加载）。
export function clearPlugin(pluginId: string): void {
  for (const key of [...moduleCache.keys()]) {
    if (key.startsWith(`${pluginId}/`)) {
      moduleCache.delete(key);
    }
  }
  manifestCache.delete(pluginId);
}
