# 插件架构 B 路线实施计划

> 依据：《插件架构研究-deepseek-harness对比汇报.md》结论——保留进程隔离壳，移植 Cordis 四件核心语义：
> 类型化事件（waterfall）、ctx 服务接缝（三角色）、配置分层叠加、注册即可逆副作用。
> 原则：每步独立可验收；现有插件（demo/seo/netease/qq）零改动兼容；不动 gRPC 进程模型与安全体系。

---

## B1：钩子分发模式标注 + waterfall 调度器

**目标**：把 11 个钩子从「同步/异步」二值升级为显式分发模式（对齐 Cordis `@mode` 事件目录），改写型钩子从「扁平覆盖」升级为「链式改写」（waterfall 语义的 Go 化落地）。

### 设计

**三种分发模式**（Cordis 四模式中，parallel 无真实场景，YAGNI 不实现）：

| 模式 | 语义 | 现有对应 |
|---|---|---|
| `serial` | 顺序执行，任一拒绝即短路返回（拦截型） | 现同步拦截钩子 |
| `waterfall` | 链式改写：下游处理器收到上游改写后的载荷（改写型） | 现同步改写钩子（行为升级） |
| `emit` | 异步观察，事后通知，不阻塞 | 现异步钩子 |

**waterfall 的 Go 化适配**：Cordis 的洋葱模型要求监听器持有 `next()`（可在下游返回后包装结果），我们的 Handler 签名 `func(ctx, Event) (Result, error)` 是扁平的、且进程外插件经 gRPC 也只能扁平往返。因此落地为**管道语义**：调度器维护 `currentPayload`，每步把它作为 `ev.Payload` 传给下一个处理器，处理器的 `Modify` 更新 `currentPayload`，拒绝即短路。这已解决核心问题——多改写者**基于彼此的结果组合**而非互相覆盖（如 content.render：插件A插目录 + 插件B美化代码块，可叠加生效）。

**载荷类型已核实天然兼容链式传递**：
- `search.query`：`string` → `string`
- `content.render`：`map[string]any{post_id, content}` → 同构
- `ai.before_generate`：`map[string]any{task, input, model}` → 同构

### 改动清单

| 文件 | 改动 |
|---|---|
| `internal/plugin/hook.go` | `hookSpec{sync}` → `hookSpec{mode}`；`DispatchMode` 枚举；`HookMode()` 查询函数；`IsSyncHook` 改为 mode 派生（`mode != emit`，签名不变） |
| `internal/plugin/dispatcher.go` | `Dispatch` 按 mode 三分支；新增 waterfall 管道执行（复用 `dispatchOne` 超时/panic 隔离） |
| `internal/plugin/dispatcher_test.go` | 新增 waterfall 组：链式改写传递、拒绝短路、单处理器行为等价 |
| `docs/plugin-development.md` | 第 5 章钩子表加「模式」列 + waterfall 多插件改写说明 |

**兼容性**：单处理器行为完全不变；拦截/异步钩子行为不变；唯一行为变化是改写型钩子多处理器时从「后者覆盖前者」变为「链式组合」——这是目标改进，发布说明标注。

---

## B2：ctx 服务注册表 + music seam 三角色

**目标**：建立 Cordis「ctx 服务容器」的 Go 对应物——泛型服务注册表；把音乐源桥接（现有最真实的双 Provider seam）迁移为完整三角色落地，形成可复制的新 seam 模式。

### 设计

**ServiceRegistry**（`internal/plugin/service_registry.go` 新文件）：
- 键 `string`（命名空间形式 `"music.netease"`），值为服务实例（Go interface）
- `Register(key, id, svc)` / `Unregister(key, id)` / `UnregisterAll(id)`（插件停用时按 id 清理全部贡献——**可逆副作用**）/ `LookupService[T](registry, key)`（泛型保类型安全）
- 与现有钩子 `Registry` 并列，同为进程内基础设施

**music seam 三角色**（`internal/plugin/seam_music.go` 新文件）：

| 角色 | 落地 |
|---|---|
| Service Definition | `MusicSource` 接口（`ResolveURL` / `ResolveBGM`）+ 键构造 `MusicSourceKey(provider)` |
| Provider | `NewMusicSourceAdapter(pluginID, callAPI)`：包装 `PluginService.CallAPI` 为接口实现（进程外插件的桥接提供方） |
| Consumer | `internal/handler/music.go` 改为经注册表查找；未命中时走现有发现逻辑（市场清单→静态表）**懒注册**后返回 |

**兼容策略**：现有 resolveMusicPlugin 逻辑保留为懒注册兜底（注册表未命中→发现→注册适配器→返回），行为完全向后兼容；插件停用时 `PluginService.deactivate` 统一 `UnregisterAll(pluginID)` 清理。

**ai / search seam**：本批**只做目录登记不做代码**——两者当前均无第二 Provider 需求（无真实 seam），强行抽象属过度设计。seam 目录文档写明新增 seam 的流程与三角色检查单。

### 改动清单

| 文件 | 改动 |
|---|---|
| `internal/plugin/service_registry.go` | 新增：泛型服务注册表 |
| `internal/plugin/seam_music.go` | 新增：MusicSource 接口 + 键 + 适配器 |
| `internal/service/plugin.go` | PluginService 持有 ServiceRegistry；`deactivate` 增加 UnregisterAll |
| `internal/handler/music.go` | 消费方迁移（注册表优先 + 懒注册兜底） |
| `internal/server/server.go` | 装配 ServiceRegistry |
| `internal/plugin/service_registry_test.go` | 新增：注册/查找/幂等/按 id 全清理 |
| `docs/plugin-development.md` | 新增 seam 目录章节（三角色表 + music 样例 + 新增流程） |

---

## B3：插件配置分层叠加（默认值层 ⊕ 实例层）

**目标**：对齐 Cordis「bundle 默认 → patch 叠加」思想。现状缺陷：`PluginConfigProvider` 只下发已保存的实例配置，未设置的 key 插件拿到空串，插件需自行处理默认值（每个插件重复实现——冗余坏味道）。

### 设计

两层叠加（不加更多层——站点级覆盖无真实需求，YAGNI）：

```
生效配置 = schema Default 层 ⊕ 实例配置层（实例 key 存在即覆盖，未声明 key 丢弃）
```

- `MergeConfigDefaults(schema, values)` 纯函数：values 中**存在**的 key 用实例值（含显式空串——用户显式清空即空，语义可预测），不存在的 key 回退 schema Default
- 下发（`PluginConfigProvider`）与回显（`Detail`/`GetConfig`）均改为合并后的生效配置
- 保存仍整体覆盖（PUT 语义，前端全量表单提交，无需 PATCH）

### 改动清单

| 文件 | 改动 |
|---|---|
| `internal/service/plugin_config.go` | `MergeConfigDefaults` + 三处接入点改造 |
| `internal/service/plugin_config_test.go` | 新增：默认回退/实例覆盖/未声明丢弃/空 map |
| `docs/plugin-development.md` | 设置章节补「生效配置 = 默认值 ⊕ 实例配置」说明 |

---

## B4：前端内容块注册表（keyed renderer）

**目标**：对齐 dsh ConversationNodeDefinition 的「类型注册表 + 按键分发渲染」模式。现状 `post-content.tsx` 的分段渲染（音乐卡片）是硬编码类型，闭集；升级为开放协议：**任何插件可注册新内容块类型**。

### 设计

**块协议**（对现有 `data-music-embed` 音乐标记的泛化，存量保留不动）：

```html
<div data-plugin-block="vote" data-props='{"id": 123}'></div>
```

- **manifest 扩展**：`blocks?: [{type, entry}]`（frontend/manifest.json 静态文件，后端零改动）
- **BlockRegistry**（`frontend/src/plugins/block-registry.ts` 新文件）：收集 running 插件 blocks 声明 → `type → {pluginId, entry}` 映射；`<PluginBlock>` React 组件内部 ref + `loadModule` + `register(ctx)`（复用现有槽位契约，`slot="block:{type}"`）
- **post-content 接入**：`splitContent` 扫描 `div[data-plugin-block]` 切段；未注册类型渲染占位提示（「插件未启用」）
- **安全**：`data-*` 属性 DOMPurify 默认放行（`ALLOW_DATA_ATTR`，现有音乐标记已依赖）；块内渲染仍走插件受限 API 客户端（路径锁定 `/api/v1/plugins/{id}`）
- 与 B1 的 `content.render` 链式改写形成闭环：后端插件改写正文注入块标记 → 前端注册表分发渲染

### 改动清单

| 文件 | 改动 |
|---|---|
| `frontend/src/plugins/loader.ts` | manifest 类型加 `blocks` |
| `frontend/src/plugins/block-registry.ts` | 新增：块注册表 + `<PluginBlock>` 组件（文件 < 300 行） |
| `frontend/src/components/post-content.tsx` | Segment 加 block 类型；扫描与渲染接入 |
| `docs/plugin-development.md` | 前端章节补内容块协议 |

---

## 实施顺序与验收

| 步骤 | 依赖 | 验收命令（scripts/） |
|---|---|---|
| B1 | 无 | `go build ./...` + `go test ./internal/plugin/...` |
| B2 | 无（与 B1 并行安全） | `go build ./...` + `go test ./internal/plugin/... ./internal/service/...` |
| B3 | 无 | `go test ./internal/service/...` |
| B4 | 建议在 B1 后（文档闭环） | `scripts/build-frontend.sh` |

全程每步完成后跑对应验收，全部通过后出总验收报告。
