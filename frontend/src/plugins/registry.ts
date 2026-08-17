// frontend/src/plugins/registry.ts
// 插件扩展点注册表（M3.6）：按插件挂载/卸载前端扩展（docs/architecture.md 6.6）。
// 职责：加载扩展点声明 → 动态加载模块 → register(ctx) 挂载 → 记录清理函数。
import { fetchManifest, loadModule, clearPlugin, type PluginUser } from "./loader";
import { clearBlockRegistry } from "./block-registry";

// authHeader 受限 API 客户端凭证头（插件自定义 API，带主站访问令牌；路径如 "/ping"）。
// 凭证：从 localStorage 读取访问令牌（与 auth.tsx 同键；插件 API 属 authed 组）。
// 导出供 sandbox.ts 的 postMessage 代理复用。
export function authHeader(): Record<string, string> {
  try {
    const raw = localStorage.getItem("yueyan-tokens");
    if (!raw) {
      return {};
    }
    const tokens = JSON.parse(raw) as { access_token?: string };
    return tokens.access_token ? { Authorization: `Bearer ${tokens.access_token}` } : {};
  } catch {
    return {};
  }
}

// PluginApiClient 插件受限 API 客户端（get/post 打到 /api/v1/plugins/{id}，带主站凭证）。
export interface PluginApiClient {
  get<T>(path: string): Promise<T>;
  post<T>(path: string, body?: unknown): Promise<T>;
}

// makePluginApi 构造插件受限 API 客户端（E2 去重：槽位挂载与独立页面壳共用；
// 路径前缀锁定到对应插件，插件无法访问其他后端接口）。
export function makePluginApi(pluginId: string): PluginApiClient {
  return {
    async get<T>(path: string): Promise<T> {
      const res = await fetch(`/api/v1/plugins/${pluginId}${path}`, {
        headers: { "Content-Type": "application/json", ...authHeader() },
      });
      return (await res.json()) as T;
    },
    async post<T>(path: string, body?: unknown): Promise<T> {
      const res = await fetch(`/api/v1/plugins/${pluginId}${path}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeader() },
        body: JSON.stringify(body ?? {}),
      });
      return (await res.json()) as T;
    },
  };
}

// 已挂载项（清理函数与挂载点配对，卸载时精确移除）。
interface MountedItem {
  pluginId: string;
  slot: string;
  el: HTMLElement;
  cleanup: () => void;
}

// PluginRegistry 槽位注册表（单例）。
class PluginRegistry {
  private mounted: MountedItem[] = [];

  // mountSlot 在指定挂载点渲染插件扩展。
  // 参数：pluginId 插件 ID；slot 槽位名；el 挂载点 DOM；user 用户信息；props 槽位透传参数（M3.9）。
  // 返回：清理函数（插件无该槽位声明时为空清理）。
  async mountSlot(
    pluginId: string,
    slot: string,
    el: HTMLElement,
    user: PluginUser | null,
    props?: Record<string, unknown>,
  ): Promise<() => void> {
    const manifest = await fetchManifest(pluginId);
    const point = manifest.extensionPoints.find((p) => p.slot === slot);
    if (!point) {
      return () => undefined; // 插件未订阅该槽位
    }
    const mod = await loadModule(pluginId, point.entry);
    const cleanup =
      mod.default({ slot, el, api: makePluginApi(pluginId), user, props }) ?? (() => undefined);
    this.mounted.push({ pluginId, slot, el, cleanup });
    return cleanup;
  }

  // unmountPlugin 卸载插件全部扩展（停用/卸载时调用，同步清理挂载内容与缓存）。
  unmountPlugin(pluginId: string): void {
    const remaining: MountedItem[] = [];
    for (const item of this.mounted) {
      if (item.pluginId === pluginId) {
        item.cleanup();
      } else {
        remaining.push(item);
      }
    }
    this.mounted = remaining;
    clearPlugin(pluginId);
    clearBlockRegistry(); // B4：块注册表含该插件声明，一并失效重收集
  }
}

// registry 全局单例（页面内共享）。
export const registry = new PluginRegistry();
