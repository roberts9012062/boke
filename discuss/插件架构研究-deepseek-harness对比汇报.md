# 插件架构研究报告：对标 DeepSeek Harness（Cordis）

> 目的：研究 deepseek-harness（下称 dsh）的插件架构，评估将本站（boke）插件系统按其重构的可行性，并给出路线建议。
> 参考资料：
> - 架构参考文档：https://deepseek-harness.github.io/deepseek-harness/reference/
> - 宿主仓库：https://github.com/deepseek-ai/deepseek-harness
> - 本报告同时基于对本仓库 `internal/plugin/`、`pkg/plugin-sdk/`、`internal/service/plugin*.go` 的全量调查。

---

## 一、dsh 插件架构是什么

dsh 是 DeepSeek 开源的智能体宿主框架（developer preview，迭代快、明确会有破坏性变更），技术栈为 TypeScript/Node pnpm monorepo，底层是 **Cordis 框架**（设计源自论文《A Programming Paradigm for Spatiotemporal Composability》）。核心口号是 **"Everything is a Plugin"（一切皆插件）**。

### 1.1 Cordis 五个支柱

1. **插件 = 实现 Service 的对象**：一个带可选 `inject` 与 `apply(ctx)` 的函数，或一个 Service 子类，生命周期由框架挂载到当前上下文。
2. **Context = 服务容器**：每个服务占一个稳定的 `ctx.<key>`（`ctx.llm`、`ctx.tools`、`ctx.sessions`……），消费方按键查找，不导入具体实现——这就是解耦核心。
3. **`inject` 声明依赖**：插件声明所需服务，等就绪才启动。**加载顺序由依赖图自动推导**，无需手动编排。
4. **类型化事件**：事件名经 TS 声明合并注册，四种分发模式（见 1.4）。
5. **注册 = 可逆副作用**：工具 schema、监听器、适配器等经 `ctx.effect()` / `ctx.on()` 安装，插件卸载/热重载时自动撤销。

### 1.2 一切皆插件、无特权内核

模型适配器、工具注册表、会话日志、**连 agent loop 本身都是插件**。"没有需要打补丁的特权核心"——扩展方式是把插件挂到其他插件旁边，而不是修改核心。注册本身即副作用，卸载即回滚。

### 1.3 配置驱动的插件树：profile + bundle + patch 分层叠加

- **Profile**：Harness home 里的命名组装（`web`、`headless` 随发行版交付），列出叠加的 bundle、树外插件、用户 patch。
- **Bundle**：Cordis 配置行 + 挂载代码的发行格式，均声明在 package.json 的 `dsh` 字段。
- **叠加顺序**：空列表 → profile 列出的各 bundle（按序）→ profile 的 cordis.patch.yml → home 级配置 → `--patch` overlay。patch 按 id 定位配置行，整体替换或插入。
- `dsh --profile web --dump-config` 可查看最终启动的插件树。

### 1.4 事件分发的四种模式

| 模式 | await | 顺序 | 返回值 |
|---|---|---|---|
| emit | 否 | 注册序观察 | 无 |
| waterfall | 否 | 注册序**洋葱式环绕**（koa 中间件） | 有，可层层包装/短路 |
| parallel | 是 | 并行扇出 | 无 |
| serial | 是 | 注册序执行 | 有 |

分发模式是事件的公开约定（`@mode` 标注），目录与调用点交叉校验。

### 1.5 能力 seam（接缝）三角色

每个 `ctx` 键按三角色描述，**加一个能力必须三件套一起设计**：

- **Service Definition（声明包）**：定义接口契约（如 `packages/llm/llm` 声明 `ctx.llm`）；
- **Provider（实现包）**：并列注册多个实现（`llm-deepseek`、`llm-replay`……）；
- **Consumer（消费方）**：只依赖抽象调用（agent-loop 不绑定具体 LLM）。

服务分三类：**core**（核心主干）、**seam**（可替换扩展点）、**bundle**（组合包）。

### 1.6 安全与沙箱的定位（关键！）

dsh 的 `ctx.sandbox` / `ctx.approval` / `ctx.permissionPresets` 是**"工具执行"层面**的沙箱（管住模型跑的 bash/fs），而**不是"插件代码"层面的隔离**——插件本身是进程内受信任代码，与宿主同权限。这是理解本报告结论的钥匙。

---

## 二、boke 现有插件系统现状（调查结论）

现有系统是 **HashiCorp go-plugin 风格的"子进程 + gRPC 契约"架构**：

- **进程模型**：`exec.Command` 拉起插件二进制 → stdio MagicCookie 握手 → AutoMTLS 加密的本地 gRPC（`internal/plugin/manager_start.go`）。
- **扩展点**：后端 **11 个硬编码钩子**（`internal/plugin/hook.go` 的 `hookSpecs` 单表：`post.before_publish`、`comment.before_save`、`search.query`、`content.render`、`api.middleware`、`ai.before_generate` 等，同步/异步是钩子固有属性）+ 前端 **5 个槽位**（`theme.header`、`post.footer`、`comment.footer`、`admin.menu`、`comment.item`）。
- **执行语义**：同步钩子串行 + 优先级 + 2s 超时 + panic/崩溃一律放行（故障隔离）；任一拒绝即阻断；`Modify` 回写改写类钩子。
- **安全供应链**：bpkg zip 包（manifest + plugin.bin + frontend/ + checksums）→ Ed25519 强制验签 → capabilities "登记 ∩ 自报"交集门控 → 脱敏只读 DataService → 调用者身份透传 → 许可证/付费体系。
- **市场**：GitHub 插件源仓库清单 + 版本钉扎 + 双重 SHA-256 + 一键升级。
- **规模**：手写后端约 5500 行（含前端运行时与 cmd 插件约 7300 行）；4 个进程外插件 + 1 个内置进程内插件（comment-anti-spam）+ 9 个市场条目。

---

## 三、本质对比

| 维度 | dsh / Cordis | boke 现状 |
|---|---|---|
| 进程模型 | 单进程、进程内 TS 模块 | 多进程、gRPC + AutoMTLS |
| 信任模型 | **本地可信**：用户自己装配 profile | **市场分发不可信**：签名+能力+隔离 |
| 扩展点 | **开集**：可替换任意服务乃至 agent loop | **闭集**：11 钩子 + 5 槽位，明确不支持替换核心 |
| 依赖模型 | `inject` 依赖图自动排启动序 | 无运行时依赖图，仅安装期 requires/conflicts |
| 事件语义 | 4 模式，waterfall 洋葱环绕 | 同步串行 / 异步扇出 两种 |
| 配置 | profile/bundle/patch 多层叠加 | 实例级单层 config |
| 注册可逆性 | 框架级保证（effect/disposer） | Deactivate 时反注册（已有雏形） |
| 通信成本 | 零（进程内函数调用） | JSON 序列化 + gRPC 往返 |
| 失败域 | 插件崩 = 宿主崩 | 进程隔离 + 退避重启 + 5 次熔断 |
| 插件语言 | 仅 TS/JS（npm 生态源码分发） | 语言中立（预编译二进制 bpkg） |

---

## 四、根本矛盾：为什么不能照搬

1. **信任模型冲突（最核心）**。dsh 的前提是"插件是产品的一部分，用户在自己 home 里装配"——插件与宿主同权限天经地义。boke 的前提是"第三方从市场下载的不可信代码"——进程内加载意味着恶意插件直接获得宿主全部权限（数据库连接、密钥、设置），现有的 capabilities 门控、脱敏 DataService、Ed25519 供应链**全部失效**。我们花在隔离上的约一半复杂度（签名、门控、broker、熔断）正是进程外架构的"补偿性复杂度"，但它买的是市场生态的立足之本。
2. **语言栈不匹配**。Cordis 是 TS/Node 框架，Go 生态没有等价物。要么引入 Node 副进程当插件容器（比现状更重、双运行时），要么用 Go 亲手仿造 Cordis（概念可仿、TS 的声明合并类型化事件仿不了）。
3. **分发单元不匹配**。Cordis 插件 = npm 源码包同进程加载；boke 插件 = 预编译二进制（`{os}-{arch}` 矩阵）+ 付费许可证。进程内化将迫使第三方插件用 Go 编写并与宿主同版本编译——市场模式基本终结。
4. **dsh 自身是 developer preview**，官方明确破坏性变更频繁，直接对标一个移动靶风险高。

---

## 五、三条路线

### 路线 A：全面 Cordis 化（不推荐）

Go 宿主重写为"一切皆插件"，进程内加载。代价：放弃隔离/签名/能力体系（市场生态前提）、重写约 7300 行、第三方插件只能 Go + 与宿主同编译。收益只有零通信开销与无限扩展力，而后者对博客平台是过度设计（YAGNI）。

### 路线 B：概念移植，保留隔离壳（推荐）

**进程模型与安全体系不动，把 Cordis 的"扩展语义层"搬进来**——这也是 Cordis 精华所在：

1. **Dispatcher 升级为类型化事件总线**：给现有 11 个钩子标注分发模式（对齐 `@mode` 目录）；将"同步串行 + Modify 回写"的扁平模型升级为 **waterfall 洋葱链**（监听器 `(event, next)`，可包装结果、可短路、可 `prepend`）。`content.render`、`search.query`、`ai.before_generate` 三个改写型钩子立即受益（可组合多个改写者而非串行覆盖）。
2. **引入 ctx 命名空间服务注册表**：定义 `ctx.ai`、`ctx.music`、`ctx.search`、`ctx.notify` 等第一方服务键；内置服务与插件适配器注册到同一注册表（现有 `Registry` + 音乐源桥接已经是雏形，把它一般化）。
3. **能力 seam 三角色规范**：每个扩展点补齐"定义 / 提供方 / 消费方"三份契约与文档，用一张生成式目录表维护（对齐 capability-seams 的完整性守卫）。
4. **配置分层叠加**：插件配置支持 默认值 → 管理员层 → 实例层 的 patch 叠加（profile/overlay 思想），`PushConfig` 语义不变。
5. **可逆注册硬化**：现有 registerAdapters/Deactivate 已具备反注册，补齐"任何注册必有 disposer"的断言测试与守卫。
6. **前端槽位 → keyed renderer 注册表**：5 个闭集槽位升级为"节点类型注册表"（dsh ConversationNodeDefinition 模式），插件可注册新节点类型与渲染器，而不只能往既有槽里塞内容。

### 路线 C：双轨混合（可作为 B 的子集吸收）

第一方可信域插件进程内直连（AI、搜索、内容渲染走 Go interface，零开销），第三方插件仍走 gRPC 子进程——对应 dsh 的 dsh-base（可信底座）与树外插件（不可信扩展）之分。现有 `builtin.go`（comment-anti-spam）正是进程内插件的雏形，可顺势扩大为正式的"内置插件层"。

---

## 六、结论与建议

**建议采用路线 B，并吸收路线 C 的"内置插件层"思想。** 理由：dsh 架构的真正价值不在"进程内"（那是它信任模型的产物），而在四件可迁移的事——**类型化事件 + waterfall 语义、ctx 服务接缝与三角色契约、配置分层叠加、注册即可逆副作用**。这四件事在进程外架构上同样可以落地，且能立刻改善我们最弱的三个点：改写型钩子无法组合、扩展点是硬编码闭集、配置只有单层。

若确认走 B，建议分四步（每步独立可验收）：

- **B1**：钩子模式标注 + waterfall Dispatcher（改 `internal/plugin/dispatcher.go`、`hook.go`，gRPC 适配层 `grpc_bridge.go` 加 `next` 语义桥接）；
- **B2**：ctx 服务注册表 + 首批 seam（`ai` / `music` / `search`）三角色契约落地；
- **B3**：配置 patch 分层叠加（`plugin_config.go` + `plugin_instances.config` 结构升级）；
- **B4**：前端 keyed renderer 注册表（`frontend/src/plugins/registry.ts` + `plugin-slot.tsx` 改造）。

### 待决策问题

1. 是否接受 B1 中 waterfall 语义对现有插件 HookResult 的兼容成本（建议保留旧扁平语义为 waterfall 的糖）？
2. 内置插件层（路线 C）是否在 B2 一并正式化？
3. 事件目录是否采用代码生成（对齐 dsh 的 gen-doc-graphs 守卫）还是先手写单表？

---

## 附：信息来源

- reference 首页、cordis-primer、capability-seams、architecture.md（仓库 docs/）
- 本仓库代码调查：`internal/plugin/`（manager/dispatcher/hook/proxy/grpc_bridge/bpkg）、`pkg/plugin-sdk/`（sdk/serve/caller/proto）、`internal/service/plugin*.go`、`frontend/src/plugins/`、`docs/plugin-development.md`
