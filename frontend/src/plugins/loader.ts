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
  slot: string; // 槽位名（theme.header/post.footer/comment.footer/admin.menu）
  el: HTMLElement; // 挂载点 DOM（插件渲染目标）
  api: PluginApi; // 受限 API 客户端（仅插件自定义 API，带主站凭证）
  user: PluginUser | null; // 用户信息（脱敏，不含密钥；未登录为 null）
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

// PluginManifest 扩展点声明（包内 frontend/manifest.json）。
export interface PluginManifest {
  extensionPoints: ExtensionPoint[];
}

// ExtensionPoint 扩展点（slot 槽位 + entry ESM 文件 + 可选 props）。
export interface ExtensionPoint {
  slot: string;
  entry: string;
  props?: Record<string, unknown>;
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
  const res = await fetch(`${ASSETS_BASE}/${pluginId}/frontend/manifest.json`);
  if (!res.ok) {
    throw new Error(`插件 ${pluginId} 前端清单拉取失败`);
  }
  const manifest = (await res.json()) as PluginManifest;
  manifestCache.set(pluginId, manifest);
  return manifest;
}

// loadModule 加载插件 ESM 模块（Blob URL + 动态 import；模块缓存）。
export async function loadModule(pluginId: string, entry: string): Promise<PluginModule> {
  const key = `${pluginId}/${entry}`;
  const cached = moduleCache.get(key);
  if (cached) {
    return cached;
  }
  const res = await fetch(`${ASSETS_BASE}/${pluginId}/frontend/${entry}`);
  if (!res.ok) {
    throw new Error(`插件 ${pluginId} 前端模块拉取失败`);
  }
  const code = await res.text();
  // Blob URL 动态 import（webpackIgnore 注释跳过打包器解析；模块为纯 ESM 即可执行）
  const blobUrl = URL.createObjectURL(new Blob([code], { type: "text/javascript" }));
  try {
    const mod = (await import(/* webpackIgnore: true */ blobUrl)) as PluginModule;
    moduleCache.set(key, mod);
    return mod;
  } finally {
    URL.revokeObjectURL(blobUrl);
  }
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
