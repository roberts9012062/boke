# 插件架构 B 路线验收报告

> 依据：《插件架构B路线-实施计划.md》。本批次将 Cordis（deepseek-harness）四件核心语义
> 移植进现有进程外插件架构（gRPC 隔离壳与安全体系不动）：类型化事件 + waterfall、
> ctx 服务接缝、配置分层叠加、注册可逆。B1~B4 四步全部落地并验收通过。

---

## 验收总览

| 步骤 | 交付 | 验收结果 |
|---|---|---|
| B1 分发模式 + waterfall | 代码 + 7 个新测试 + 文档 | ✅ `go build` + `go test ./internal/plugin/...` 全绿 |
| B2 服务注册表 + music seam | 代码 + 6 个新测试 + seam 目录文档 | ✅ build + vet + test 全绿 |
| B3 配置默认值分层合并 | 代码 + 6 个新测试 + 文档 | ✅ build + test 全绿 |
| B4 前端内容块注册表 | 代码（3 文件）+ 文档 | ✅ `scripts/build-frontend.sh` 生产构建成功 |
| **总验收** | — | ✅ 全仓 `go build ./...` + `go vet ./...` + `go test ./...` 退出码 0 |

## B1：钩子分发模式 + waterfall 调度器

- `internal/plugin/hook.go`：`hookSpec{sync}` → `hookSpec{mode}`，新增 `DispatchMode`
  （`serial` 串行拦截 / `waterfall` 链式改写 / `emit` 异步通知——Cordis 四模式裁剪，
  parallel 无场景不引入）；`HookMode()` 查询 API；`IsSyncHook` 由模式派生（签名兼容）。
- `internal/plugin/dispatcher.go`：`Dispatch` 按模式三分支；`dispatchWaterfall` 管道执行
  ——下游处理器收到上游改写后的载荷（多改写者**链式组合**而非覆盖），任一拒绝短路，
  超时/panic 隔离复用 `dispatchOne`；全程无改写时 `Modify=nil`（对齐旧行为）。
- **行为兼容**：单处理器与拦截/异步钩子行为完全不变；唯一变化是改写型钩子
  （`content.render`/`search.query`/`ai.before_generate`）多插件时从「后者覆盖」变「链式组合」。
- 测试：`TestRegistryWaterfallChained`（链式传递）、`RejectShortCircuit`（拒绝短路）、
  `NoModifyNil`（无改写 nil）、`TestHookModeTable`（11 钩子模式目录断言）。
- 文档：`docs/plugin-development.md` 第 5 章钩子表「模式」列 + waterfall 说明。

## B2：ctx 服务注册表 + music capability seam

- `internal/plugin/service_registry.go`（新）：泛型 `ServiceRegistry`（键→服务）；
  `Register`（同 id 幂等覆盖）/ `Unregister` / `UnregisterAll(id)`（插件停用统一清理——
  **注册可逆**）/ `LookupService[T]`（泛型类型安全查找）。
- `internal/plugin/seam_music.go`（新）：music seam 三角色——`MusicSource` 接口（定义）+
  `MusicSourceKey`（`music.{provider}` 键）+ `MusicSourceAdapter`（提供方：包装 CallAPI）。
- `internal/service/plugin_seam.go`（新）：消费门面 `PluginService.MusicSource()`——
  注册表直达 → 未命中走原发现（市场清单→静态兜底，静态表自 handler 迁入）懒注册；
  `seamRegistry()` 懒初始化（构造签名与装配零改动）。
- `internal/handler/music.go`：MusicHandler 改为纯 seam 消费方（不再感知插件 ID/gRPC/
  发现逻辑）；`deactivate` 增加 `UnregisterAll`（停用即回滚）。
- 测试：注册/查找/类型不符/幂等覆盖/按键注销/按 id 全清理（6 项）。
- 文档：第 19 章新增 19.0 seam 目录（三角色表 + 新增 seam 检查单；ai/search 预留——
  无第二 Provider 不强行抽象）。

## B3：插件配置分层叠加（默认值层 ⊕ 实例层）

- `internal/service/plugin_config.go`：`MergeConfigDefaults` 纯函数（实例 key 存在即覆盖
  ——含显式空串=用户清空；否则回退 `Default`；schema 外 key 丢弃；结果覆盖全部 schema key）；
  schema 聚合抽取 `aggregateSchema`（消除三处内联重复）；读取侧统一出口 `effectiveConfig`。
- 接入点：**下发**（`PluginConfigProvider`，插件不再自行处理默认值）、**回显**
  （`Detail`/`GetConfig` 设置页显示生效值）、**推送**（`SetConfig` 推合并后的生效配置）。
- 测试：默认回退/实例覆盖/显式空串/未声明丢弃/nil 容错/空 schema（6 项）。
- 文档：第 8 章补「生效配置 = 默认值 ⊕ 实例配置」契约说明。

## B4：前端内容块注册表（keyed renderer）

- 协议：正文（html）中 `<div data-plugin-block="{type}" data-props='{...}'></div>` →
  按 `type` 分发到声明该块的插件 entry 渲染——任何插件可注册新内容块类型（开集），
  宿主零改动；与 B1 `content.render` 链式改写形成「后端注入 → 前端分发」闭环。
- `frontend/src/plugins/loader.ts`：manifest 类型加 `blocks?: PluginBlock[]`。
- `frontend/src/plugins/block-registry.ts`（新）：注册表收集（running 插件清单，30s 缓存，
  type 全局唯一首个生效）+ `parseBlockProps` 容错解析 + `clearBlockRegistry`。
- `frontend/src/components/plugin-block.tsx`（新）：`<PluginBlock>` 分发组件（槽位契约复用，
  `slot="block:{type}"`；未注册类型占位提示；受限 API 客户端路径锁定）。
- `frontend/src/components/post-content.tsx`：Segment 增加 `block` 类型，`splitContent`
  扫描块节点切段（存量音乐标记逻辑不变）；`registry.unmountPlugin` 联动清块缓存。
- 安全：`data-*` 属性 DOMPurify 默认放行（`ALLOW_DATA_ATTR`，存量音乐标记同机制）。
- 验收：`scripts/build-frontend.sh` 生产构建成功（含 TS 类型检查与 lint）。
- 文档：第 7 章补内容块协议与示例。

## Cordis 语义映射总结（本批次后）

| Cordis 概念 | 本仓落地 |
|---|---|
| 类型化事件四模式 | `DispatchMode` 三模式（serial/waterfall/emit；parallel 无场景） |
| waterfall 洋葱环绕 | `dispatchWaterfall` 管道（扁平 Handler 签名下的 Go 化等价物） |
| ctx 服务容器 | `ServiceRegistry`（泛型查找） |
| capability seam 三角色 | `seam_music.go` 样例 + 19.0 目录与检查单 |
| bundle 默认 → patch 叠加 | `MergeConfigDefaults`（默认值层 ⊕ 实例层） |
| 注册即可逆副作用 | 钩子 `UnregisterWithID`（既有）+ 服务 `UnregisterAll`（新增统一出口） |

## 兼容性声明

- 现有 4 个进程外插件（demo/seo/netease/qq）与 1 个内置插件：**零改动、零重打包**。
- 对外 HTTP API、gRPC 契约（plugin.proto）、bpkg 包格式、市场清单格式：均未变更。
- 唯一行为变化：改写型钩子多插件同时订阅时改写结果链式组合（目标改进，已在文档标注）。

## 建议后续（不在本批次）

1. 出一个使用 `blocks` + `content.render` 的官方示例插件（验证闭环 DX）；
2. 事件目录测试升级为代码生成守卫（对齐 dsh gen-doc-graphs，防止表与文档漂移）；
3. `ai`/`search` seam 在出现第二 Provider 需求时按 19.0 检查单落地。
