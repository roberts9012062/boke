// frontend/src/plugins/block-registry.ts
// 内容块注册表（B4 keyed renderer，对齐 dsh ConversationNodeDefinition 模式）：
//   - 协议：正文 HTML 中的 <div data-plugin-block="{type}" data-props="{...json}"></div>
//   - 声明：插件 frontend/manifest.json 的 blocks: [{type, entry}]
//   - 分发：宿主按 type 查注册表 → 加载插件 ESM → register(ctx) 渲染（槽位契约复用）
//   - 与后端 content.render（waterfall 链式改写）闭环：插件改写正文注入块标记，
//     前端注册表分发渲染——任何插件可注册新内容块类型（开集，不再硬编码）。
// 安全：块标记属性 data-* 经 DOMPurify 默认放行（与存量音乐标记一致）；
//       块内渲染走受限插件 API 客户端（路径锁定 /api/v1/plugins/{id}）。
import { apiPluginExtensions } from "@/lib/api";
import { fetchManifest } from "@/plugins/loader";

// BlockEntry 单个块类型的注册项（提供方插件 + ESM 入口）。
export interface BlockEntry {
  pluginId: string; // 提供方插件 ID（受限 API 客户端与模块加载均锁定到该插件）
  entry: string; // ESM 入口文件（默认导出 register(ctx)，与槽位契约一致）
}

// BlockRegistry 块类型 → 注册项映射（fetchBlockRegistry 的返回结构）。
export type BlockRegistry = Map<string, BlockEntry>;

// 注册表缓存（30 秒过期，对齐 plugin-slot 的扩展清单缓存粒度）。
let cachedRegistry: { registry: BlockRegistry; at: number } | null = null;
const REGISTRY_TTL = 30_000;

// fetchBlockRegistry 收集 running 插件声明的内容块（type → 注册项）。
// 说明：块类型约定全局唯一，冲突时首个生效并保持稳定（后声明不覆盖先声明）。
export async function fetchBlockRegistry(): Promise<BlockRegistry> {
  if (cachedRegistry && Date.now() - cachedRegistry.at < REGISTRY_TTL) {
    return cachedRegistry.registry;
  }
  const registry: BlockRegistry = new Map();
  try {
    const r = await apiPluginExtensions();
    await Promise.all(
      r.items.map(async (item) => {
        try {
          const manifest = await fetchManifest(item.plugin_id);
          for (const block of manifest.blocks ?? []) {
            if (!registry.has(block.type)) {
              registry.set(block.type, { pluginId: item.plugin_id, entry: block.entry });
            }
          }
        } catch {
          /* 单插件清单拉取失败静默（其余插件的块不受影响） */
        }
      }),
    );
  } catch {
    /* 扩展清单接口失败：返回空注册表（正文块渲染为占位） */
  }
  cachedRegistry = { registry, at: Date.now() };
  return registry;
}

// clearBlockRegistry 清理注册表缓存（插件停用/卸载后下次请求重新收集）。
export function clearBlockRegistry(): void {
  cachedRegistry = null;
}

// parseBlockProps 解析块节点的 data-props JSON（非法 JSON 容错为空对象；纯函数）。
export function parseBlockProps(raw: string | null): Record<string, unknown> {
  if (!raw) {
    return {};
  }
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (parsed !== null && typeof parsed === "object" && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
    return {};
  } catch {
    return {};
  }
}
