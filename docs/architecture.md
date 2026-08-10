# 月言博客平台 — 架构设计文档

| 项目 | 内容 |
|---|---|
| 文档版本 | v1.0 |
| 编写日期 | 2026-08-10 |
| 状态 | 已评审待开发（M1 依据） |
| 关联设计稿 | `boke.pen` / `UI设计/`（冷月、薄雾双主题，PC + 移动端） |

---

## 1. 项目概述

### 1.1 项目背景与目标

月言是一个**多功能博客平台**，目标是在 Go 生态中提供一套可扩展、可插拔、带 AI 能力的现代博客系统。与 WordPress（PHP）、Halo（Java）、Ghost（Node）不同，本项目采用 **Gin + Hashicorp go-plugin** 的组合，在保证性能与部署简单性的同时，提供**运行时热加载的第三方插件体系**与**多供应商 AI 能力**。

核心目标：

1. **多功能**：文章、评论（楼中楼）、私信、关注/收藏、举报审核、管理后台、审计、SEO、备份导出等完整闭环。
2. **可扩展**：通过 go-plugin 子进程实现运行时插件热加载（跨平台，含 Windows），配套插件市场（免费/付费 + 许可证校验）。
3. **AI 原生**：内置 AI SDK 模块，OpenAI 兼容协议统一接入 DeepSeek / 通义千问 / Kimi / 智谱 / OpenAI 等供应商，并向插件开放 AI 钩子。
4. **多主题**：冷月、薄雾双主题（各自含 PC 与移动端布局），主题以包形式分发与切换。

### 1.2 功能范围（依据设计稿）

**前台**
- 文章列表 / 详情（视频、音频、图片灯箱、目录）、草稿箱、发布编辑
- 评论：楼中楼嵌套回复、点赞
- 话题标签、搜索
- 私信（DM 会话）、关注 / 收藏 / 黑名单
- 举报（帖子 / 评论 / 用户）

**管理后台**
- Dashboard 数据看板
- 内容管理（文章 / 评论 / 媒体）、标签管理
- 用户管理、角色权限
- 站点设置、敏感词管理、封禁管理
- 审核队列（帖子 / 评论 / 用户）、审核日志
- 插件市场（GitHub 集成：连接账号 / 同步列表 / 安装 / 卸载 / 免费 / 付费）
- SEO 模块（SEO 设置、健康度评分、SERP 预览、批量修复）
- 数据报表、备份 / 导出

**基础**
- 登录 / 注册 / 找回密码、404 / 500 / 维护页、隐私 / 用户协议
- 双主题（冷月 / 薄雾）× PC / 移动端响应式

### 1.3 非功能需求

| 维度 | 要求 |
|---|---|
| 性能 | 单机 P95 API 延迟 < 200ms（不含 AI 流式调用）；文章列表页缓存命中率 > 90% |
| 可扩展性 | 插件可在不停机情况下安装 / 启用 / 停用 / 卸载；单插件崩溃不影响主进程 |
| 安全 | RBAC 细粒度权限；全量审计日志；敏感词过滤；付费插件许可证防伪造 |
| 可运维 | Docker Compose 一键部署；数据定期备份；插件进程资源受限与崩溃自愈 |
| 可移植 | 开发环境为 Windows，插件体系必须跨平台（macOS / Linux / Windows） |

---

## 2. 技术选型总览

| 领域 | 选型 | 理由 | 替代方案 |
|---|---|---|---|
| Web 框架 | **Gin** | 生态最大、中间件丰富、性能好；分层架构（controller/service/repository）控制力强 | GoFrame、chi |
| 插件机制 | **Hashicorp go-plugin** | 子进程 + gRPC 通信；运行时热加载；跨平台含 Windows；崩溃隔离；被 Terraform / Vault 等生产验证 | Go 原生 `plugin` 包（**不支持 Windows，排除**）、独立 HTTP 服务 |
| 数据库 | **PostgreSQL 15+** | JSONB 灵活、内置全文检索、并发与事务能力强，适配审核 / 报表 / 搜索场景 | MySQL、SQLite |
| ORM | **GORM** | 国内资料多、迁移工具完善 | sqlc、ent |
| 权限 | **Casbin** | RBAC 标准实现，gorm 适配器，策略可后台热更新 | 自研权限中间件 |
| 缓存 / 队列 | **Redis 7** | 缓存、会话、限流、异步任务（文章渲染、SEO 检查） | 内存缓存 + PG LISTEN/NOTIFY |
| 鉴权 | **golang-jwt/jwt v5** | 标准 JWT + Refresh Token；插件间也可复用 | 会话 Cookie |
| 日志 | **zap + lumberjack** | 结构化日志、按大小滚动；插件日志按插件分文件 | slog |
| 配置 | **viper** | YAML + 环境变量覆盖 | envconfig |
| AI SDK | **自研 OpenAI 兼容层 + langchaingo（可选）** | 国内主流模型全部兼容 OpenAI 协议，统一一层即可全接入；langchaingo 用于 RAG / Agent 高级能力 | 各厂商官方 SDK 逐个接入 |
| 前端 | **Vue 3 + Vite + TypeScript** | 设计稿为 PC + 移动端双端 SPA；生态成熟 | React、Nuxt |
| 部署 | **Docker Compose** | 主程序 + Postgres + Redis + Nginx + 插件挂载目录 | K8s（后期） |

> 决策记录：插件机制最终选择 go-plugin 子进程而非编译期注册，原因是设计稿明确包含**插件市场**（第三方插件、付费流程），需要运行时动态加载与故障隔离；编译期注册作为 M1 阶段的简化过渡方案保留在路线图中（见第 13 章）。

---

## 3. 整体架构

### 3.1 逻辑分层

```
┌─────────────────────────────────────────────────────────────┐
│                        客户端                               │
│     浏览器 (Vue3 SPA)         手机端 (响应式)    API 客户端    │
└──────────────────────────┬──────────────────────────────────┘
                           │ HTTPS
┌──────────────────────────▼──────────────────────────────────┐
│                      Nginx (静态资源 / 反向代理 / TLS)        │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                     Gin HTTP 层 (internal/server)           │
│  中间件链: 恢复 → 请求ID → 日志 → CORS → 限流 → JWT → RBAC    │
│  路由注册: /api/v1/*  (前台)   /api/v1/admin/*  (后台)        │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                  Handler 层 (控制器, 参数绑定/校验)           │
├─────────────────────────────────────────────────────────────┤
│                  Service 层 (业务逻辑 / 事务边界)             │
├─────────────────────────────────────────────────────────────┤
│        Repository 层 (GORM 数据访问, 缓存读写封装)            │
└──────────────┬──────────────────────────────┬───────────────┘
               │                              │
   ┌───────────▼───────────┐     ┌────────────▼────────────┐
   │   PostgreSQL 15+      │     │   Redis 7               │
   │  业务数据/全文检索     │     │  缓存/会话/限流/任务队列  │
   └───────────────────────┘     └─────────────────────────┘
```

### 3.2 插件子系统架构（go-plugin）

```
┌──────────────────────────── 主进程 (blog-server) ────────────────────────────┐
│                                                                            │
│  ┌────────────┐   调用钩子   ┌─────────────────────────────┐                │
│  │  业务模块   │ ──────────► │   Plugin Manager            │                │
│  │ (post/… )  │             │  · 实例注册表 (id→进程)      │                │
│  └────────────┘             │  · 生命周期状态机            │                │
│        ▲                    │  · 崩溃重启(退避)            │                │
│        │ 返回结果            │  · 资源限制 / 超时熔断        │                │
│        │                    └──────────┬──────────────────┘                │
│        │                               │ go-plugin Client                  │
│        │                               │ (gRPC over TCP, AutoMTLS)         │
├────────┼───────────────────────────────┼──────────────────────────────────┤
│        │          插件进程 1 (seo-plugin 二进制)          │                │
│        └───────────────►               │                                   │
│                          插件进程 2 (ai-assistant 二进制)  │                │
│                                        插件进程 N …         │                │
└────────────────────────────────────────┴───────────────────────────────────┘
     ▲ 安装包(.bpk)来源: GitHub Release / 本地上传 / 官方内置
```

要点：

- 每个插件是**独立可执行文件**，由主进程通过 go-plugin 拉起，gRPC 通信（默认 AutoMTLS 加密）。
- 插件崩溃 → 管理器按退避策略自动重启；连续失败 → 置为 `crashed` 状态并通知后台。
- 插件通过 **Hook（钩子）** 与主进程业务解耦：主进程在固定钩子点广播事件，插件订阅后异步处理。
- 插件可注册**自己的 API 路由**（前缀 `/api/plugins/{id}/...`）与 **管理后台页面扩展点**。

### 3.3 部署拓扑（概览，详见第 12 章）

```
┌────────────────────── Docker Host ──────────────────────┐
│  nginx ──► blog-server ──► postgres / redis              │
│                │                                         │
│                └── 插件挂载目录 (plugins/) 持久化          │
└──────────────────────────────────────────────────────────┘
```

---

## 4. 目录结构设计

```
boke/
├── cmd/
│   └── server/
│       └── main.go                     # 主程序入口（装配 + 启动）
├── internal/
│   ├── config/                         # viper 配置加载与校验
│   ├── server/                         # Gin 引擎、中间件链、优雅退出
│   ├── router/                         # 路由注册（前台 / 后台 / 插件路由挂载点）
│   ├── middleware/                     # 请求ID / 恢复 / CORS / 限流 / JWT / RBAC / 审计
│   ├── handler/                        # 控制器层（HTTP 参数绑定、响应组装）
│   ├── service/                        # 业务逻辑层（事务边界）
│   ├── repository/                     # 数据访问层（GORM + Redis 缓存封装）
│   ├── model/                          # GORM 数据模型 + DTO
│   ├── auth/                           # JWT 签发 / 刷新 / 会话
│   ├── plugin/                         # ★ 插件管理器（生命周期、钩子调度、进程治理）
│   ├── ai/                             # ★ AI 供应商抽象与管理（详见第 7 章）
│   ├── casbin/                         # 权限策略加载与同步
│   ├── search/                         # 全文检索封装（PG FTS + 中文分词）
│   ├── seo/                            # SEO 模块（设置 / 健康度 / SERP 预览 / 批量修复）
│   ├── audit/                          # 审计日志采集
│   ├── backup/                         # 备份 / 导出（pg_dump + 媒体打包）
│   ├── notify/                         # 站内信 / 私信通知
│   └── bootstrap/                      # 启动装配（依赖注入、插件拉起顺序）
├── pkg/                                # 可复用公共包（不依赖 internal）
│   ├── errs/                           # 统一错误码定义
│   ├── resp/                           # 统一响应结构
│   └── paginate/                       # 分页参数与序列化
├── plugin-sdk/                         # ★ 独立 Go module（第三方插件唯一依赖）
│   ├── sdk.go                          # Plugin / Hook 接口定义
│   ├── proto/                          # plugin.proto（gRPC 契约，含版本号）
│   ├── grpc/                           # 生成代码
│   ├── server/                         # 插件侧运行时（握手、服务注册、优雅退出）
│   └── client/                         # 主进程侧客户端封装
├── plugins/                            # 官方内置插件（各自独立 module，编译为独立二进制）
│   ├── seo/                            # 示例：SEO 增强插件
│   └── ai-assistant/                   # 示例：AI 助手插件（消费 AI SDK 钩子）
├── frontend/                           # Vue3 SPA（独立仓库亦可）
│   ├── src/
│   └── themes/                         # 主题包（冷月 / 薄雾）
│       ├── cool-moon/                  # theme.json + assets
│       └── mist/
├── license-service/                    # 可选：付费插件许可证签发服务（Ed25519，独立进程）
├── deploy/
│   ├── docker-compose.yml
│   └── nginx.conf
└── docs/
    ├── architecture.md                 # 本文档
    └── plugin-dev-guide.md             # ★ 插件开发手册（面向第三方作者，GitHub 发布必读）
```

分层约定（强约束）：

- `handler` → `service` → `repository` 单向依赖，禁止反向。
- `service` 是事务与业务规则唯一归属处；`handler` 不做业务判断。
- `internal/plugin` 与 `internal/ai` 对其他模块只暴露接口，通过接口注入调用（便于插件化演进）。

---

## 5. 核心模块划分

| 模块 | 职责 | 关键接口（示意） |
|---|---|---|
| 用户 / 认证 | 注册、登录、找回密码、JWT 签发/刷新、个人信息 | `POST /api/v1/auth/register`、`/login` |
| 用户关系 | 关注、收藏、黑名单、粉丝统计 | `PUT /api/v1/users/{id}/follow` |
| 文章 | 列表 / 详情 / 发布 / 编辑 / 草稿 / 删除、标签关联 | `GET/POST /api/v1/posts` |
| 媒体 | 图片 / 视频 / 音频上传（分片）、OBS 直传可选、缩略图 | `POST /api/v1/media` |
| 评论 | 楼中楼（parent_id 树）、点赞、置顶、软删除 | `GET /api/v1/posts/{id}/comments` |
| 私信 | 会话（DM）、消息分页、已读、未读数 | `GET /api/v1/messages` |
| 举报 | 举报提交、举报单流转、处理结果通知 | `POST /api/v1/reports` |
| 审核 | 帖子 / 评论 / 用户审核队列、敏感词命中提示、封禁 | `/api/v1/admin/audit/*` |
| 审计 | 管理操作全量留痕（操作者 / IP / 前后值）、查询检索 | `/api/v1/admin/audit-logs` |
| 敏感词 | 词库 CRUD、命中检测（多模式匹配，如 Aho-Corasick） | `internal/service/sensitive` |
| 标签 | 话题标签、计数、热度排行 | `/api/v1/tags` |
| 搜索 | 全文检索（PG FTS + 中文分词）、搜索建议、插件扩展钩子 | `GET /api/v1/search` |
| SEO | SEO 设置、健康度评分、SERP 预览、批量修复任务 | `/api/v1/admin/seo/*` |
| 报表 | 访问量、用户增长、内容统计、导出 | `/api/v1/admin/reports/*` |
| 备份 | 定时备份（pg_dump + 媒体打包）、手动导出、恢复 | `/api/v1/admin/backup/*` |
| 站点设置 | 全局 key-value 配置、敏感配置加密存储 | `/api/v1/admin/settings` |
| **插件系统** | 插件市场、生命周期、钩子调度、前端扩展点（第 6 章） | `/api/v1/admin/plugins/*` |
| **AI 模块** | 供应商管理、统一推理接口、任务路由（第 7 章） | `/api/v1/admin/ai/*` |

---

## 6. 插件系统设计（重点章节）

### 6.1 设计目标与约束

- **运行时热加载**：安装、启用、停用、卸载均不重启主进程。
- **故障隔离**：插件子进程崩溃 / 死循环 / OOM 不影响主进程。
- **跨平台**：开发与部署覆盖 Windows / Linux / macOS —— 因此排除 Go 原生 `plugin` 包。
- **安全**：插件二进制校验、许可证防伪造、插件进程最小权限。
- **生态友好（GitHub 原生）**：插件开发只需依赖 `plugin-sdk` 一个 module；源码托管 GitHub、清单 `yueyan-plugin.json` 放仓库根目录、产物作为 Release 资产分发，无需自建中央仓库（详见 6.5）。

### 6.2 go-plugin 协议设计

采用 Hashicorp go-plugin 的标准握手 + gRPC：

```go
// internal/plugin/manager.go（主进程侧）
var handshake = plugin.HandshakeConfig{
    ProtocolVersion:  3,                       // 协议版本，升级不兼容时协商
    MagicCookieKey:   "YUEYAN_PLUGIN_COOKIE",
    MagicCookieValue: "yueyan-blog-plugin-v1",
}

client := plugin.NewClient(&plugin.ClientConfig{
    HandshakeConfig:  handshake,
    Plugins:          map[string]plugin.Plugin{"core": &coreGRPCPlugin{}},
    Cmd:              exec.Command(pluginBinPath), // 插件二进制
    AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
    AutoMTLS:         true,                        // 自动 TLS 加密
})
rpcClient, err := client.Client()
```

插件侧入口（由 `plugin-sdk/server` 封装，插件作者只写业务）：

```go
// plugins/seo/main.go（插件侧，作者视角）
func main() {
    server.Serve(&seo.Plugin{}) // sdk 内部完成握手、注册、优雅退出
}
```

协议契约 `plugin-sdk/proto/plugin.proto`（版本号随 sdk 升级）：

```proto
service PluginService {
    rpc Info(Empty) returns (PluginInfo);            // 名称/版本/作者/依赖
    rpc Activate(Empty) returns (Status);            // 启用
    rpc Deactivate(Empty) returns (Status);          // 停用（含保存状态）
}
service HookService {
    rpc Execute(HookRequest) returns (HookResponse); // 同步钩子（可阻塞）
    rpc Stream(HookRequest) returns (stream HookEvent);// 异步/流式钩子
}
service PluginAPI {                                   // 插件暴露的自定义 API
    rpc Call(APICall) returns (APICallResult);        // 由主进程按前缀路由代理
}
```

### 6.3 插件 SDK 接口与钩子点

```go
// plugin-sdk/sdk.go（核心接口）
type Plugin interface {
    Info() Info                       // 名称、版本、作者、入口页面等
    OnActivate(ctx context.Context) error
    OnDeactivate(ctx context.Context) error
    Hooks() []Hook                    // 声明订阅的钩子
}

type Hook struct {
    Name     string                   // 钩子名（见下表）
    Sync     bool                     // true=同步(阻塞请求链)，false=异步
    Priority int                      // 执行优先级（小先执行）
    Handler  func(ctx context.Context, e Event) (Result, error)
}
```

内置钩子点（v1.0 全集）：

| 钩子 | 触发时机 | 同步/异步 | 典型插件用途 |
|---|---|---|---|
| `post.before_publish` | 文章发布前（可改写/拦截） | 同步 | 敏感词、合规检查、AI 标题润色 |
| `post.after_publish` | 文章发布后 | 异步 | 自动建索引、SEO 推送、AI 摘要生成 |
| `content.render` | 文章/评论渲染前 | 同步 | 内容改写、外链处理、代码高亮 |
| `comment.before_save` | 评论保存前（可拒绝） | 同步 | AI 评论审核、垃圾评论识别 |
| `search.query` | 搜索执行时 | 同步 | 扩展搜索范围（标签、私密内容） |
| `api.middleware` | 插件自注册路由的中间件 | 同步 | 统计、限流、白名单 |
| `ai.chat` / `ai.embedding` | AI 推理调用（见 7.4） | 同步/流式 | 自定义模型路由、提示词注入 |
| `notification.send` | 通知发送前 | 异步 | 邮件、Webhook 转发 |
| `admin.page` | 后台扩展点渲染 | 同步 | 后台新页面（走前端扩展点） |

事件 `Event` 统一携带：`pluginID`、`actorID`、`payload(JSON)`、`traceID`（贯穿日志）。

### 6.4 插件生命周期管理

```
                      ┌──────────┐
       安装包校验失败   │          │ 校验通过(哈希+许可证)
      ┌───────────────►│  installed │────────────────────┐
      │                │          │                      ▼
      │                └──────────┘                 ┌─────────┐
      │                                             │ verified │──► 许可证过期 → degraded
      │                                             └────┬────┘
      │                                                  │ 启用(拉起进程)
      │                ┌──────────┐                 ┌────▼────┐
      │                │ uninstalled│◄──卸载─────────│ running  │
      │                └──────────┘                 └────┬────┘
      │                ▲          ▲                      │
      │                │          └────────── 停用 ◄──────┤
      │                │                                 │ 崩溃(退避重启, 连续N次)
      │                │                           ┌─────▼─────┐
      └────────────────┴───────────────────────────│  crashed   │──► 通知后台
                                                  └───────────┘
```

管理要点：

- **状态机实现**：`internal/plugin/manager.go` 中 `ManagedPlugin{state, client, rpcClient, restartCount, lastErr}`，状态流转加锁，持久化到 `plugin_instances` 表。
- **崩溃自愈**：退避策略 `1s → 2s → 4s → … 上限 60s`；连续崩溃 ≥ 5 次置 `crashed`，通知后台并停止重启。
- **资源限制**：Linux 用 `rlimit` / cgroup 限制 CPU 内存；Windows 用 `Job Object`（开发阶段至少做超时熔断 + 内存监控）。
- **优雅退出**：主进程收到 SIGTERM 时先调用 `Deactivate` 再 `client.Kill()`，超时 10s 强制杀。
- **钩子调度**：同步钩子设置超时（默认 2s，可配），超时返回 `hook_timeout` 错误并熔断该插件；异步钩子进 Redis 队列，失败重试（指数退避）。
- **API 代理**：插件自定义 API 统一挂载 `/api/plugins/{id}/**`，主进程代理转发，附带插件身份上下文。

### 6.5 插件市场：GitHub 集成模式

**总体模式**：不设自建中央仓库，插件生态直接建立在 GitHub 上（Homebrew tap 模式）：

- 插件**源码托管在作者自己的 GitHub 仓库**，插件清单 `yueyan-plugin.json` 放在仓库根目录；
- 安装产物（`.bpk`）作为 **GitHub Release 资产**分发（作者用 CI 构建多平台产物后随 tag 发布）；
- 站点管理员在后台**连接 GitHub 账号** → 主进程拉取/同步插件列表 → 市场页面展示 → 一键安装 / 升级。

#### 6.5.1 连接 GitHub

- 管理员在后台「插件市场 → 设置」点击「连接 GitHub」，走 **OAuth App** 流程（scope：`read:user` + 公开仓库读取，可选私有仓库）。
- `access_token` 加密存储于 `settings`（AES），用于 API 同步与私有仓库下载。
- **匿名模式**：未连接时也能浏览——插件清单与 `.bpk` 直接走 `raw.githubusercontent.com` / Release 下载地址（走 CDN，**不受 GitHub API 限流影响**）；连接后可获得：API 同步加速、私有仓库支持、后续 webhook 能力。

#### 6.5.2 插件发现（多数据源合并）

| 数据源 | 说明 |
|---|---|
| 官方索引仓库（默认） | `yueyan/plugin-registry` 仓库根目录 `plugins.json` 收录官方精选 / 社区审核通过的插件（id、仓库地址、版本）。**PR 审核制**保证列表质量（Homebrew tap 模式） |
| 自定义仓库 / 组织 | 管理员可添加任意 GitHub 仓库或整个 org，从各仓库根目录 `yueyan-plugin.json` 发现插件 |
| 手动仓库 URL | 添加单仓库 URL；公开仓库无需连接 GitHub |

合并展示，按（免费 / 付费、分类）过滤排序。

#### 6.5.3 插件清单 yueyan-plugin.json（仓库根目录）

```json
{
  "id": "seo-helper",
  "name": "SEO 助手",
  "version": "1.4.2",
  "description": "SEO 元信息管理、健康度检查、批量修复",
  "author": { "name": "月言官方", "github": "yueyan" },
  "license": "BSL-1.1",
  "pricing": { "model": "paid", "edition": "pro", "features": ["meta_auto", "batch_fix"] },
  "sdk": ">=1.0.0",
  "platforms": ["linux", "darwin", "windows"],
  "assets": { "pattern": "seo-helper-{version}-{os}-{arch}.bpk" },
  "hooks": ["post.after_publish", "content.render", "seo.meta"],
  "frontend": true,
  "screenshots": ["docs/screenshot.png"]
}
```

#### 6.5.4 拉取与同步

- 「同步」按钮 / 定时轮询（默认每 6h，可配）：按数据源拉取清单 → 解析校验（id 唯一、sdk 兼容、assets 模式合法）→ 合并入市场候选列表（新增 / 变更标记）。
- 同步结果本地缓存（etag / 本地副本），最小化 GitHub API 调用；失败退避重试，不影响已装插件。
- **更新检查**：对比各插件仓库 GitHub Release `latest` tag 与本地已装版本 → 市场页出现「可更新」角标。

#### 6.5.5 安装与更新

1. 点击安装 → 按平台匹配 Release 资产（命名模式 `{id}-{version}-{os}-{arch}.bpk`）→ 下载。
2. **双重 SHA-256 校验**（清单声明 + 下载后实算），不匹配拒绝安装。
3. 解包到 `plugins/{id}/` → 读取 `manifest.json` → 校验 sdk 兼容范围 → 校验许可证（付费插件）→ 状态 `installed`。
4. 启用 → 拉起进程（状态机见 6.4）。
5. 更新：检测到新 tag → 一键升级（下载新 `.bpk` → 停用 → 替换 → 校验 → 启用）。
6. **源码构建模式**（可选）：克隆仓库 → 检出 tag → 本地编译 → 打包安装（适合无 Release 产物的插件与开发调试；生产推荐 Release 资产）。

#### 6.5.6 许可证：公开源码 + 激活模式

付费插件采用「**代码公开 + 功能锁定**」：

- 源码与 Release 二进制公开（**fair-code 许可**，推荐 BSL 1.1 或 Commons Clause：代码可读、个人自用免费，商业部署需许可）。
- 安装后处于 **demo 模式**（基础功能可用）；购买后由作者 / 官方许可证服务签发 `license.jwt`（Ed25519 签名）：

  ```json
  {
    "sub": "plugin:seo-helper",
    "licensee": "站点ID",
    "edition": "pro",
    "features": ["meta_auto", "batch_fix"],
    "exp": 1752537600,
    "signature": "base64(ed25519)"
  }
  ```

- 主进程内置作者公钥（随插件包附带 `pubkey.pem`，安装时登记），验签通过 → 全功能；**离线宽限期 7 天**，到期未续 → 自动降级 demo 模式（核心保留、增值停用）。
- **防绕过**：二进制是 Go 编译产物无法完全防逆向，策略 = 签名校验 + 功能降级 + fair-code 许可约束（违反可依法追责），并写入插件协议。

**付费变现路径**：免费插件积累生态 → 付费插件（demo → 激活）形成收入；许可证签发由插件作者自持（独立小服务）或委托官方 `license-service`（可选组件，见 12.1）。

**插件包格式 `.bpk`**（zip 封装，作者侧构建，详见《插件开发手册》）：

```
seo-helper-1.4.2-linux-amd64.bpk
├── manifest.json        # id / 名称 / 版本 / 作者 / 描述 / 兼容的 sdk 版本范围
├── plugin.bin           # 插件二进制（按平台分别构建）
├── pubkey.pem           # 作者许可证公钥（付费插件必带）
├── frontend/            # 前端扩展资产（js / css / manifest）
│   └── manifest.json    # 扩展点声明（见 6.6）
└── checksums.json       # 各文件 SHA-256
```

### 6.6 前端插件扩展点

插件的前端能力通过**扩展点注册 + 运行时加载**实现：

```
frontend/src/plugins/
├── loader.ts        # 运行时从 /api/plugins/{id}/assets 加载 js/css
├── registry.ts      # 扩展点注册表（插件声明 → 已注册组件）
└── sandbox.ts       # iframe 沙箱（复杂插件）
```

- **轻量插件**：`frontend/manifest.json` 声明注入点（`theme.header`、`post.footer`、`admin.menu`、`admin.page.*`），loader 在路由挂载时动态加载组件（ESM + 类型化 Props 约定）。
- **复杂插件**（如独立管理页）：iframe 挂载，通过 `postMessage` + 短期 token 调用主站 API。
- 扩展点与主题系统共存：主题决定布局插槽，插件填充槽位内容；插件停用时其前端资产同步卸载。

### 6.7 插件开发示例（SDK 使用）

```go
// 第三方插件: hello-plugin/main.go —— 完整代码约 40 行
package main

import (
    "context"
    "github.com/yueyan/plugin-sdk"   // 唯一依赖
)

type HelloPlugin struct{}

func (p *HelloPlugin) Info() sdk.Info {
    return sdk.Info{ID: "hello", Name: "示例插件", Version: "1.0.0"}
}
func (p *HelloPlugin) OnActivate(ctx context.Context) error { return nil }
func (p *HelloPlugin) OnDeactivate(ctx context.Context) error { return nil }
func (p *HelloPlugin) Hooks() []sdk.Hook {
    return []sdk.Hook{{
        Name:     "post.after_publish",
        Sync:     false,
        Priority: 100,
        Handler:  func(ctx context.Context, e sdk.Event) (sdk.Result, error) {
            // 文章发布后给作者发个私信
            return sdk.Result{OK: true}, nil
        },
    }}
}

func main() { sdk.Serve(&HelloPlugin{}) }
```

> 📖 完整开发手册见《[插件开发手册](./plugin-dev-guide.md)》：环境准备、快速开始、钩子全解、前端插件、打包与 GitHub 发布、调试排障、最佳实践。

---

## 7. AI SDK 模块设计

### 7.1 设计原则

- **协议统一**：DeepSeek、通义千问（DashScope 兼容模式）、Kimi（Moonshot）、智谱 GLM、OpenAI 均提供 OpenAI 兼容 `/chat/completions` 与 `/embeddings` 接口 —— 核心层只维护一套 HTTP 客户端，供应商差异收敛为「base_url + api_key + 模型映射」配置。
- **流式优先**：内容生成场景一律 SSE 流式返回（对话、文章续写），减少首字节延迟。
- **供应商可插拔**：后台可配置多个供应商、按任务路由（如「摘要用 DeepSeek、嵌入用通义」）、随时切换与限流。
- **成本可控**：按任务维度统计 token 用量与费用；单任务超时 / 重试 / 熔断。

### 7.2 统一接口（internal/ai）

```go
package ai

type Provider interface {
    Name() string
    Chat(ctx context.Context, req ChatRequest, stream bool) (ChatStream, error) // SSE
    Embedding(ctx context.Context, req EmbeddingRequest) ([]float64, error)
}

type ChatRequest struct {
    Model    string            // 供应商内模型名（后台映射）
    Messages []Message
    MaxTokens int
    Temperature float32
}

type Manager struct { /* providers map[string]Provider; routing 策略; usage 统计 */ }

func (m *Manager) Chat(ctx, req) (ChatStream, error) // 按路由策略选供应商
```

任务注册表（`ai_tasks` 表）：任务名（`post.summary`、`post.tags`、`comment.review`、`seo.advice`）→ 供应商 + 模型 + 提示词模板 + 限流配额。提示词模板可后台编辑（多语言）。

### 7.3 内置 AI 应用场景（M4 落地）

| 场景 | 触发 | 说明 |
|---|---|---|
| 文章摘要 | 发布后异步 | 生成摘要入 `seo_meta`，用于列表页与分享卡片 |
| 自动标签 | 发布后异步 | 从标题/正文提取 3~5 个标签，供作者确认 |
| 智能评论审核 | 评论保存时同步 | 垃圾 / 广告 / 人身攻击分类，命中高危直接拦截 |
| 智能回复助手 | 编辑态 | 作者侧「AI 续写 / 润色 / 翻译」流式接口 |
| SEO 建议 | 后台 | 对文章给出标题/描述/关键词建议，接入 SEO 健康度 |

### 7.4 AI 与插件系统的关系

AI 能力既服务内置模块，也开放给插件：

```
插件(ai-assistant) ──► 钩子 ai.chat ──► internal/ai Manager ──► 供应商 A / B
```

- 插件可通过 `ai.chat` / `ai.embedding` 钩子**复用**主进程的供应商配置、配额与密钥（密钥不出主进程）。
- 插件的自定义模型路由（如「先摘要后问答」）通过钩子链实现；SDK 提供 `sdk.AI()` 便捷客户端。
- 内置 `plugins/ai-assistant` 即以此模式实现的官方参考插件。

---

## 8. 数据库设计概要

### 8.1 核心表清单（PostgreSQL 15+）

| 表 | 说明 | 关键字段 / 索引 |
|---|---|---|
| `users` | 用户 | `email, username, status`；唯一索引 email/username |
| `user_relations` | 关注 / 收藏 / 黑名单 | `(user_id, target_id, type)` 联合唯一 |
| `posts` | 文章 | `author_id, status(草稿/待审/已发布/已删), published_at`；`gin(status, published_at)` |
| `post_versions` | 文章版本历史 | `post_id, content_diff` |
| `tags` / `post_tags` | 标签 | `post_tags(post_id, tag_id)` 联合唯一 |
| `media_assets` | 媒体文件 | `owner_id, type, storage_key, size` |
| `comments` | 评论（楼中楼） | `post_id, parent_id, floor`；`(post_id, parent_id)` 索引 |
| `messages` | 私信 | `(conversation_id, created_at)` 索引；`conversations` 表 |
| `reports` / `report_tickets` | 举报单流转 | 状态机字段 `status` |
| `audit_logs` | 审计日志 | `(actor_id, action, created_at)`；按月分区 |
| `sensitive_words` | 敏感词库 | 加载进内存（Aho-Corasick 匹配） |
| `ban_records` | 封禁记录 | `user_id, reason, until` |
| `settings` | 站点设置 key-value | JSONB 值 |
| `plugin_instances` | 插件实例状态 | `plugin_id, version, state, config` |
| `plugin_licenses` | 许可证缓存 | `plugin_id, licensee, edition, exp` |
| `ai_providers` | AI 供应商配置 | 密钥字段 AES 加密存储 |
| `ai_tasks` | AI 任务路由配置 | 任务名 → 供应商/模型/提示词 |
| `ai_usage` | token 用量与费用 | 按日聚合视图 |
| `seo_meta` | 文章 SEO 元信息 | `post_id` 唯一；`seo_settings`、`seo_health_checks` |
| `backup_records` | 备份记录 | 文件路径、大小、状态 |
| `notifications` | 站内通知 | `(user_id, read_at)` 索引 |

### 8.2 中文全文搜索方案

- 首选：**PostgreSQL 内置 FTS** + 中文分词扩展 `zhparser` / `pg_jieba`（生成 `tsvector`，`GIN` 索引）。
- 兜底：`pg_trgm` 三元组索引支持模糊搜索与「搜索建议」。
- 搜索流程中暴露 `search.query` 钩子，允许插件扩展（第 6.3 节）。
- 若规模扩大（> 百万文章），升级方案为独立 **Meilisearch**，搜索模块接口先行抽象以便替换。

---

## 9. 权限与安全设计

### 9.1 RBAC 角色模型（Casbin）

| 角色 | 权限范围 |
|---|---|
| 访客 guest | 读公开内容、注册 |
| 用户 user | 发文（按站点策略）、评论、私信、关注收藏、举报 |
| 作者 author | 用户 + 草稿管理、媒体库 |
| 编辑 editor | 作者 + 文章编辑（非本人）、标签管理、SEO 设置 |
| 审核员 moderator | 审核队列、举报处理、敏感词、封禁（限时） |
| 管理员 admin | 用户管理、角色分配、插件管理、备份 |
| 超级管理员 superadmin | 全部 + 站点设置 + 审计日志查看 |

策略模型 `model.conf`（标准 RBAC + 资源域）：

```
[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[role_definition]
g = _, _
[matchers]
m = g(r.sub, p.sub) && (r.obj == p.obj || p.obj == "*") && r.act == p.act
```

Casbin 策略存 PostgreSQL（gorm adapter），后台修改即时生效（版本号 + 缓存失效）；路由级别做拦截，`obj` 使用资源模式如 `/api/v1/admin/posts*`。

### 9.2 审计日志

- 覆盖：登录、权限变更、内容删除/恢复、审核操作、封禁、敏感词命中处理、插件安装/卸载、AI 配置变更、备份操作。
- 记录：`actor_id, action, resource_type, resource_id, before/after(JSONB), ip, ua, created_at`。
- 保留策略：按月分区，默认保留 2 年；只允许 superadmin 查询。
- 插件操作同样入审计（插件 ID 作为 actor）。

### 9.3 其他安全措施

- **传输**：全站 HTTPS；插件 gRPC 走 AutoMTLS。
- **认证**：JWT access（15min）+ refresh（7d，可撤销）；后台操作强制二次校验（可选 TOTP）。
- **限流**：Redis 滑动窗口——登录 5 次/分/账号、API 300 次/分/IP、AI 按配额。
- **注入**：GORM 参数化 + 白名单排序字段；XSS 由前端渲染层转义 + `content.render` 钩子净化。
- **文件安全**：上传类型/魔数校验、随机文件名、存储隔离（用户媒体 / 插件资产）。
- **敏感词**：Aho-Corasick 多模式匹配，命中即拦截（帖子）/ 标记（评论）。
- **密钥管理**：AI 密钥、许可证私钥只存主进程侧加密存储与许可证签发服务，经环境变量注入，不进代码库。

---

## 10. 主题系统

### 10.1 渲染方案

前端为 Vue3 SPA，主题采用**「主题包 = 设计令牌 + 资源清单」**方案，服务端只做元数据管理：

| 层 | 职责 |
|---|---|
| 服务端 | `themes` 表维护主题（id/名称/版本/启用状态）；主题包上传与发布；启用切换下发 `theme.json` 与资源 CDN 前缀 |
| 前端 | 运行时按 `theme.json` 加载 CSS 变量（design token）与布局资源；PC / 移动端由断点响应式切换（设计稿已分别出图，可进一步做布局级差异） |
| 插件 | 前端扩展点挂载到主题槽位（6.6 节），主题与插件互不侵入 |

主题包结构：

```
theme-cool-moon/
├── theme.json      # id / 名称 / 版本 / 作者 / 兼容版本
├── tokens.css      # 色板 / 字号 / 圆角 / 阴影（冷月·暗色系）
├── layout/         # 布局组件（header / sidebar / post 卡片…）
└── assets/         # logo、背景、图标
```

- 冷月（暗色 / 冷色）、薄雾（浅色 / 雾感）两套主题同时维护 `Light/Dark` 风格指南（`UI设计/公共/月言 · Style Guide`）。
- 主题切换即时生效（本地持久化 + 服务端默认值），管理员可在后台预览 / 回滚。

---

## 11. API 设计约定

### 11.1 规范

- 版本前缀 `/api/v1`；后台 `/api/v1/admin/*`；插件 `/api/plugins/{id}/*`。
- 资源化命名（复数名词）、动词用 HTTP 方法；`PATCH` 做部分更新。
- 参数：查询用 query string，复杂过滤用 `?filter=`（JSON）；变更类一律 body JSON。
- 请求统一带 `X-Request-ID`（中间件生成或透传），日志与错误响应回显。

### 11.2 统一响应格式

```json
{ "code": 0, "message": "ok", "data": { }, "request_id": "req_xxx" }
```

### 11.3 错误码（pkg/errs）

| 段 | 范围 | 示例 |
|---|---|---|
| 成功 | 0 | — |
| 鉴权 | 1xxx | 1001 未登录、1002 token 过期、1003 无权限 |
| 校验 | 2xxx | 2001 参数错误、2002 资源不存在 |
| 资源冲突 | 3xxx | 3001 重名、3002 状态不允许该操作 |
| 插件 | 4xxx | 4001 插件不存在、4002 插件崩溃、4003 许可证无效/过期、4004 钩子超时 |
| AI | 5xxx | 5001 供应商不可用、5002 超时、5003 配额不足 |
| 系统 | 6xxx | 6001 内部错误、6002 上游不可用 |

### 11.4 分页

```json
{ "page": 1, "page_size": 20, "total": 100, "items": [] }
```

- `page_size` 上限 100；游标分页用于私信 / 评论（`?cursor=&limit=`）。

### 11.5 鉴权

- `Authorization: Bearer <access_token>`；`POST /api/v1/auth/refresh` 换取新对。
- 插件路由由主进程代理并注入插件身份，插件不得自行解析 JWT。

---

## 12. 部署架构

### 12.1 Docker Compose（deploy/docker-compose.yml 概要）

```yaml
services:
  postgres:      # image: postgres:15-alpine, 卷挂载持久化, 中文分词扩展 initdb
  redis:         # image: redis:7-alpine
  app:           # 构建 cmd/server; 环境变量注入 JWT/AI 密钥; 挂载 ./plugins 与 ./data
  license:       # 许可证签发服务（可选；付费插件作者亦可自持）
  nginx:         # 静态资源 + /api 反向代理 + TLS
```

- 插件二进制按平台分发：容器内为 Linux 版；Windows 开发环境直接跑本平台插件（go-plugin 跨平台支持即为此设计）。
- 健康检查：`/healthz`（含插件存活汇总）；日志统一 stdout 采集。

### 12.2 备份策略

- 每日 `pg_dump`（custom 格式）+ 媒体目录增量同步（rclone / rsync）；保留 30 天。
- 手动「导出」= 数据库 dump + 媒体打包 zip（对应设计稿的备份/导出页面）；导入做全量校验 + 可回滚。
- 备份操作全程入审计日志。

---

## 13. 开发路线图

| 里程碑 | 内容 | 验收标准 |
|---|---|---|
| **M1** | 项目骨架、配置、DB 迁移、认证 JWT、用户、文章 CRUD、媒体上传、统一响应/错误码 | 前后台可跑通「注册→发文→列表→详情」 |
| **M2** | 评论楼中楼、关注/收藏/黑名单、私信、通知、管理后台（用户/内容/媒体/标签/设置）、审计日志、敏感词、举报 | 设计稿核心页面全部可用 |
| **M3** | ★ 插件系统：plugin-sdk v1、gRPC 契约、插件管理器（状态机/崩溃重启/钩子调度）、`.bpk` 打包工具、**GitHub 集成插件市场**（OAuth 连接 / 清单同步 / Release 安装 / 许可证激活）、前端扩展点 loader | 示例插件（hello/seo）经 GitHub Release 完成安装→启用→钩子生效→卸载全流程 |
| **M4** | AI 模块（供应商抽象、流式、配额）、内置场景（摘要/标签/评论审核/SEO 建议）、SEO 模块（健康度/SERP/批量修复）、报表、备份导出 | AI 任务在双供应商下可路由切换；SEO 检查报告可生成 |
| **M5** | 双主题包化与切换、前端插件 iframe 沙箱、性能打磨（缓存/索引/限流）、安全审计、灰度发布 | 双主题切换即时生效；压测达标；安全清单关闭 |

> 过渡说明：若 M3 前需要插件能力，先用「编译期注册 + 配置启停」的轻量机制（第 2 章决策记录），M3 切换到 go-plugin 时保持钩子语义不变。

---

## 14. 风险与权衡

| 风险 | 影响 | 缓解 |
|---|---|---|
| go-plugin 子进程复杂度（调试、进程治理） | 开发与排障成本高 | sdk 封装运行时细节；协议版本化；插件单独日志文件；`plugin.Reattach` 支持调试 |
| 插件进程资源失控（死循环 / 内存泄漏） | 拖垮宿主 | 超时熔断 + 退避重启 + 平台资源限制；同步钩子硬超时 |
| Windows 进程管理差异（信号 / Job Object） | 优雅退出与资源限制不完整 | 开发期以超时 + 主动 Kill 兜底；生产以 Linux 容器为准 |
| Go 插件无统一生态（无标准包格式） | 生态冷启动难 | 官方提供 SDK + 打包 CLI + 示例插件；官方索引仓库收录与推广 |
| 付费插件防绕过有限（Go 二进制可逆向） | 收益风险 | 签名校验 + 功能降级 + fair-code 许可约束；许可证密钥不出主进程 |
| GitHub 限流 / 不可用 / 仓库变更 | 列表同步失败、安装失败 | 清单走 raw CDN（无 API 限流）+ 本地缓存 + 退避重试；已装插件本地留存不受影响 |
| 供应链安全（恶意 / 被篡改插件） | 站点安全 | 下载双重 SHA-256 校验；索引仓库 PR 审核制；插件最小权限 + 崩溃隔离；安装时展示权限清单 |
| 中文全文搜索质量 | 搜索体验 | zhparser/pg_jieba + trgm 兜底；接口抽象预留 Meilisearch |
| 插件与主题扩展点 API 稳定性 | 破坏第三方生态 | 扩展点语义化版本（v1 冻结后只增不删）；钩子 payload 兼容策略 |
| AI 供应商不稳定 / 成本失控 | 功能与财务风险 | 多供应商路由 + 熔断；token 用量统计与配额告警 |

---

## 15. 附录

### 15.1 术语表

| 术语 | 含义 |
|---|---|
| .bpk | 插件打包格式（zip，含 manifest + 二进制 + 前端资产） |
| 钩子（Hook） | 主进程业务节点向插件开放的事件点 |
| 扩展点（Extension Point） | 前端主题槽位中可由插件填充的位置 |
| 许可证（License） | Ed25519 签名的 JWT，声明插件的版本/功能/有效期 |
| 设计令牌（Design Token） | 主题包的色板/字号等可切换变量 |
| 索引仓库（Registry Repo） | 官方维护的插件清单仓库（Homebrew tap 模式），如 `yueyan/plugin-registry` |
| fair-code | 代码可读、可自用，商业部署需许可的开源变体许可（BSL 1.1 / Commons Clause） |
| yueyan-plugin.json | 插件仓库根目录的清单文件，供市场拉取与展示 |

### 15.2 参考项目

| 项目 | 借鉴点 |
|---|---|
| **Halo**（Java 博客 CMS） | 插件市场流程、主题体系、管理后台形态（本项目 UI 设计稿与之同赛道） |
| **Grafana** | 前端插件扩展点 + 后端数据源插件模式 |
| **Hashicorp Terraform / Vault** | go-plugin 进程治理、协议版本协商、崩溃重启经验 |
| **WordPress** | 钩子（actions/filters）语义，`content.render` 即 filter 对应物 |
| **Homebrew** | 索引仓库（tap）聚合第三方软件清单的生态模式——本项目插件市场 GitHub 模式的原型 |

### 15.3 待定事项（后续评审）

1. GitHub OAuth App 注册与公开审核（应用名称、域名、回调 URL 备案）。
2. 官方索引仓库 `yueyan/plugin-registry` 的收录规则与 PR 审核流程细化。
3. 许可证签发服务形态：官方集中式 vs 插件作者自持（影响分成与结算）。
4. 支付渠道（支付宝 / 微信）接入阶段与分成比例。
5. 主题包是否开放第三方制作（需要主题 SDK 与预览沙箱）。
6. AI 任务是否开放用户侧付费（如按次计费的 AI 会员功能）。
