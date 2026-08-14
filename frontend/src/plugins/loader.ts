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

// PluginManifest 扩展点声明（包内 frontend/manifest.json；pages 为独立页面声明，M3.9）。
export interface PluginManifest {
  extensionPoints: ExtensionPoint[];
  pages?: PluginPage[];
}

// ExtensionPoint 扩展点（slot 槽位 + entry ESM 文件 + 可选 props + 挂载模式）。
export interface ExtensionPoint {
  slot: string;
  entry: string;
  props?: Record<string, unknown>;
  mode?: "append" | "replace"; // M3.9：append=追加共存（默认）；replace=替换槽位默认内容
}

// PluginPage 插件独立页面声明（M3.9：admin.page.* 能力——壳路由 /admin/plugin-pages/{id}/{route}）。
export interface PluginPage {
  route: string; // 页面路由名（如 "demo"）
  entry: string; // 页面 ESM 文件（registerPage 契约）
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

// loadModule 加载插件 ESM 模块（Blob URL + 原生 ESM 桥接；模块缓存）。
// 说明：不用 webpackIgnore 动态 import（打包器运行时走 eval，被 CSP 'unsafe-eval' 拦截）——
//       改为内联 <script type="module"> 桥接：模块内 import(blobUrl) 为原生 ESM，不受 CSP eval 限制。
export async function loadModule(pluginId: string, entry: string): Promise<PluginModule> {
  const key = `${pluginId}/${entry}`;
  const cached = moduleCache.get(key);
  if (cached) {
    return cached;
  }
  const res = await fetch(`${ASSETS_BASE}/${pluginId}/frontend/${entry}`, { cache: "no-store" });
  if (!res.ok) {
    throw new Error(`插件 ${pluginId} 前端模块拉取失败`);
  }
  const code = await res.text();
  const blobUrl = URL.createObjectURL(new Blob([code], { type: "text/javascript" }));
  try {
    const mod = await importViaNativeEsm(blobUrl);
    moduleCache.set(key, mod);
    return mod;
  } finally {
    URL.revokeObjectURL(blobUrl);
  }
}

// 内联模块桥接的全局注册表（并发加载按唯一 id 区分；模块执行后清理）。
declare global {
  interface Window {
    __yueyanPluginModules?: Record<string, { resolve: (m: PluginModule) => void; reject: (e: unknown) => void }>;
  }
}

// importViaNativeEsm 经内联 <script type="module"> 执行原生 import(blobUrl)。
async function importViaNativeEsm(blobUrl: string): Promise<PluginModule> {
  return new Promise<PluginModule>((resolve, reject) => {
    // 注册回调（按唯一 id，支持并发加载多个插件模块）
    const id = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    window.__yueyanPluginModules = window.__yueyanPluginModules ?? {};
    window.__yueyanPluginModules[id] = { resolve, reject };

    const script = document.createElement("script");
    script.type = "module";
    // 原生 ESM 动态导入 blob URL（非打包器运行时，不触发 CSP unsafe-eval）
    script.textContent =
      `import("${blobUrl}").then(m => { ` +
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
      reject(new Error(`插件模块脚本执行失败（${blobUrl}）`));
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
