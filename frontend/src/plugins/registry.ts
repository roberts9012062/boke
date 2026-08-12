// frontend/src/plugins/registry.ts
// 插件扩展点注册表（M3.6）：按插件挂载/卸载前端扩展（docs/architecture.md 6.6）。
// 职责：加载扩展点声明 → 动态加载模块 → register(ctx) 挂载 → 记录清理函数。
import { fetchManifest, loadModule, clearPlugin, type PluginUser } from "./loader";

// 受限 API 客户端（插件自定义 API，带主站凭证；路径如 "/ping"）。
function makePluginApi(pluginId: string) {
  return {
    async get<T>(path: string): Promise<T> {
      const res = await fetch(`/api/v1/plugins/${pluginId}${path}`, {
        headers: { "Content-Type": "application/json" },
      });
      return (await res.json()) as T;
    },
    async post<T>(path: string, body?: unknown): Promise<T> {
      const res = await fetch(`/api/v1/plugins/${pluginId}${path}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
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
  // 参数：pluginId 插件 ID；slot 槽位名；el 挂载点 DOM；user 用户信息。
  // 返回：清理函数（插件无该槽位声明时为空清理）。
  async mountSlot(pluginId: string, slot: string, el: HTMLElement, user: PluginUser | null): Promise<() => void> {
    const manifest = await fetchManifest(pluginId);
    const point = manifest.extensionPoints.find((p) => p.slot === slot);
    if (!point) {
      return () => undefined; // 插件未订阅该槽位
    }
    const mod = await loadModule(pluginId, point.entry);
    const cleanup = mod.default({ slot, el, api: makePluginApi(pluginId), user }) ?? (() => undefined);
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
  }
}

// registry 全局单例（页面内共享）。
export const registry = new PluginRegistry();
