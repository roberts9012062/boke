# 月言博客 · 插件开发手册

> 面向第三方插件作者：如何开发、打包、发布插件到月言博客平台。
> 适用版本：核心 ≥0.1.0（M3 插件系统全量：进程外化 + 能力授权 + 数据服务 + 设置 + 许可证 + 流式钩子 + 独立页面 + 支付渠道）
> 配套：`docs/architecture.md` 第 6 章（架构设计）；`discuss/插件能力授权-验收报告.md`（M3.8）；`discuss/插件后置七项-验收报告.md`（M3.9）

---

## 1. 架构与设计原则

插件是**独立进程**（go-plugin + gRPC），与博客主进程隔离运行：

```
┌───────────────────────── 博客主进程 ─────────────────────────┐
│  HTTP API / 业务服务 / 数据库 / 钩子调度器                     │
│         ▲  gRPC（契约 plugin.proto）        ▲ 只读数据（GRPCBroker）│
└─────────┼──────────────────────────────────┼──────────────────┘
    插件子进程（plugin.exe）◄─────────────────┘
    插件作者代码：Info/Hooks/RegisterAPI/OnActivate/...
```

**设计原则（解耦边界）**：

| 原则 | 说明 |
|------|------|
| **进程隔离** | 插件崩溃/卡死不影响主进程（熔断 + 超时 + 退避重启） |
| **契约通信** | 插件只能经 gRPC 契约与主进程交互，不共享内存 |
| **不直连数据库** | 插件**禁止**访问数据库——数据查询走只读数据服务（`data.read`），数据变更走钩子/主进程 API |
| **最小权限** | 能力声明制（capabilities）：插件声明要什么，主进程只给什么 |
| **单向数据流** | 主进程 → 插件：钩子事件/配置/许可证；插件 → 主进程：钩子响应/自定义 API/数据查询 |

---

## 2. 快速开始（30 秒骨架）

```go
// main.go
package main

import (
    "context"
    "github.com/roberts9012062/boke/pkg/plugin-sdk"
    "github.com/roberts9012062/boke/pkg/plugin-sdk/server"
)

type MyPlugin struct{}

func (p *MyPlugin) Info() sdk.Info {
    return sdk.Info{
        ID: "my-plugin", Name: "我的插件", Version: "1.0.0",
        Author: "你的名字", Description: "一句话描述",
        Capabilities: []string{"data.read"}, // 能力声明（可选）
        Settings: []sdk.SettingField{        // 设置项（可选）
            {Key: "greeting", Label: "问候语", Type: "text", Default: "你好"},
        },
    }
}

func (p *MyPlugin) OnActivate(ctx context.Context) error  { return nil } // 启用回调
func (p *MyPlugin) OnDeactivate(ctx context.Context) error { return nil } // 停用回调

func (p *MyPlugin) Hooks() []sdk.Hook { return nil } // 订阅钩子（见第 5 章）

func main() { server.Serve(&MyPlugin{}) }
```

编译出二进制 → 打包 `.bpk`（第 13 章）→ 后台「插件商城/本地上传」安装。

---

## 3. 插件清单（manifest.json）

`.bpk` 包内 `manifest.json`（与主进程校验 ID 一致性）：

```json
{
  "id": "my-plugin",
  "name": "我的插件",
  "version": "1.0.0",
  "author": "你的名字",
  "description": "一句话描述",
  "sdk": ">=1.0.0"
}
```

**市场清单**（GitHub 源 `plugins.json`，安装入口展示）补充字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 插件 ID（唯一，小写字母数字连字符） |
| `name` / `version` / `author` / `description` | string | 基础信息 |
| `category` | string | 类别：seo/security/performance/analytics/writing/ops/enhancement |
| `price` | int | 价格（0=免费） |
| `capabilities` | string[] | **能力声明**（见第 9 章；未知能力拒绝安装） |
| `core_version` | string | 兼容核心版本（如 `>=0.1.0`） |
| `requires` / `conflicts` | string[] | 依赖 / 冲突插件 ID |
| `platforms` | string[] | 支持平台（linux/darwin/windows） |
| `repo_url` | string | 源码仓库（Release 资产发布源） |
| `nav` | object | 后台侧栏入口（`{href, label, icon}`） |
| `settings_schema` | object[] | 设置项声明（`{key,label,type,default,options}`；type: text/switch/select） |

---

## 4. SDK API 全览

| API | 说明 |
|-----|------|
| `Info() sdk.Info` | 插件信息 + 设置项 + 能力声明 |
| `OnActivate(ctx)` / `OnDeactivate(ctx)` | 生命周期回调（初始化/释放资源；Activate 失败不进入 running） |
| `Hooks() []sdk.Hook` | 订阅钩子（`{Name, Sync, Priority, Handler}`） |
| `RegisterAPI(api *sdk.APIMux)` | 可选接口（APIProvider）：暴露自定义 HTTP API |
| `sdk.License(ctx)` | 读取许可证（付费功能开关；`FeatureEnabled("demo_pro")`） |
| `sdk.Config(ctx)` | 读取配置（设置页保存后下发；`cfg["greeting"]`） |
| `sdk.Data(ctx)` | 只读数据服务（声明 `data.read` 且被授权后非 nil；**未授权返回 nil 需判空**） |

---

## 5. 钩子契约（与主进程的扩展/替换点）

钩子是插件影响博客行为的主要通道。**同步钩子**可拦截（拒绝）或改写；**异步钩子**事后通知（不阻塞）。

| 钩子 | 同步 | 触发时机 | Payload | 用途 |
|------|------|---------|---------|------|
| `post.before_publish` | ✅ | 发布前 | 帖子对象 | **拦截**（OK=false 阻断发布） |
| `post.after_publish` | ❌ | 发布后 | 帖子对象 | 通知/统计 |
| `comment.before_save` | ✅ | 评论保存前 | 评论内容字符串 | **拦截**（反垃圾） |
| `comment.after_save` | ❌ | 评论保存后 | 评论对象 | 通知/统计 |
| `search.query` | ✅ | 搜索时 | 关键词字符串 | **改写**（Modify 回写关键词） |
| `notification.send` | ❌ | 通知发送后 | 通知对象 | 通知增强（如推送到外部） |
| `admin.page` | ✅ | 后台仪表盘 | 仪表盘数据 | 后台增强 |
| `content.render` | ✅ | 帖子详情返回 | `{post_id, content}` | **改写正文**（Modify 回写 content） |
| `api.middleware` | ✅ | 写请求（POST/PUT/DELETE） | `{method, path, user_id}` | **拦截**（OK=false → 403） |
| `ai.before_generate` | ✅ | AI 生成前 | `{task, input, model}` | **改写输入**（Modify 回写 input） |
| `ai.after_generate` | ❌ | AI 生成后 | `{task, result}` | 通知/统计 |

> **异步钩子传输**：异步钩子（❌）经 **流式通道**（`HookService.Stream`）推送——主进程建立长期连接持续发送；断连自动回退 Execute（进程重启后重建通道）。插件侧无需感知通道差异（`Hooks()` 声明即可）。

**拦截示例**（同步钩子拒绝）：

```go
Handler: func(ctx context.Context, ev sdk.Event) (sdk.Result, error) {
    if strings.Contains(string(ev.Payload), "广告") {
        return sdk.Result{OK: false, Reason: "内容含广告词，已拦截"}, nil
    }
    return sdk.Result{OK: true}, nil
}
```

**改写示例**（search.query 修改搜索词）：

```go
Handler: func(ctx context.Context, ev sdk.Event) (sdk.Result, error) {
    keyword := string(ev.Payload)
    return sdk.Result{OK: true, Modify: []byte(keyword + " 优质")}, nil
}
```

**语义与约束**：
- 同步钩子主进程侧 **2 秒超时**；超时/panic/网络失败 → **放行**（故障隔离，不拖垮核心）
- 插件进程崩溃 → 钩子自动跳过（退避重启恢复）
- 事件 `sdk.Event`：`TraceID`（请求追踪）、`ActorID`（操作者用户 ID，0=匿名/系统）、`Payload`（JSON bytes）

---

## 6. 自定义 API（扩展前台/后台能力）

实现 `APIProvider` 接口暴露 HTTP API，主进程统一挂载 `/api/v1/plugins/{插件ID}/**`（需登录）：

```go
func (p *MyPlugin) RegisterAPI(api *sdk.APIMux) {
    api.Handle("GET", "/ping", func(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
        return 200, []byte(`{"pong":true}`), nil
    })
    api.Handle("POST", "/notify", func(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
        // 读取配置/数据服务/许可证
        cfg := sdk.Config(ctx)
        return 200, []byte(`{"ok":true}`), nil
    })
}
```

---

## 7. 前端扩展（槽位渲染 + 独立页面）

打包时 `cmd/bp pack -frontend frontend/` 携带前端资产，声明槽位：

```json
// frontend/manifest.json
{
  "extensionPoints": [
    { "slot": "post.footer", "entry": "index.js", "mode": "append" }
  ]
}
```

```js
// frontend/index.js（原生 ESM，CSP 兼容）
export default function register(ctx) {
  const el = document.createElement("div");
  el.className = "my-plugin-card";
  el.textContent = "插件内容";
  ctx.el.appendChild(el);
  return () => el.remove(); // 清理函数（卸载/停用时调用）
}
```

**可用槽位**：

| 槽位 | 位置 | 透传 props |
|------|------|-----------|
| `theme.header` | 顶栏 | — |
| `post.footer` | 帖子详情页脚 | — |
| `comment.footer` | 评论区底部 | — |
| `admin.menu` | 后台侧栏 | — |
| `comment.item` | **每条顶层评论下方**（M3.9） | `{ comment }`（评论对象） |

**挂载模式**（`ExtensionPoint.mode`，M3.9）：
- `append`（默认）：多插件追加共存
- `replace`：隐藏槽位默认内容，仅渲染插件内容（声明该模式的插件存在时生效）

**独立页面**（`manifest.pages`，M3.9 后置规划落地）：
```json
{ "pages": [ { "route": "dashboard", "entry": "page.js" } ] }
```
页面模块默认导出 `registerPage(ctx)`（`ctx: { container, api, user, params }`），访问路径 `/admin/plugin-pages/{插件ID}/{route}`（后台权限守卫 + 插件 running 校验）。完整示例见第 18 章。

**安全**：渲染环境为 iframe 沙箱（CSP 严格，无 unsafe-eval）+ 短期 token（1 小时，插件可直调自身代理 API）；槽位/页面 API 客户端自动携带登录凭证。

---

## 8. 设置项（后台可配置参数）

在 `Info()` 声明 `Settings`，后台「我的插件 → 打开设置」自动生成表单：

```go
Settings: []sdk.SettingField{
    {Key: "greeting", Label: "页脚问候语", Type: "text", Default: "你好"},
    {Key: "enable_badge", Label: "显示徽章", Type: "switch", Default: "on"},
    {Key: "theme", Label: "主题", Type: "select", Default: "auto",
     Options: []string{"auto", "light", "dark"}},
}
```

- 保存后**即时生效**（主进程推送 `SetConfig`）；重启/升级后保持（激活时自动下发）
- 插件侧读取：`cfg := sdk.Config(ctx); cfg["greeting"]`
- 主进程按 schema 过滤：**未声明的键会被丢弃**（防注入）

---

## 9. 能力授权（capabilities）

插件能做什么，由**能力声明**决定（主进程安装校验 + 运行时门控）：

| 能力 | 类型 | 说明 |
|------|------|------|
| `hooks` | 基础 | 订阅钩子（默认可用） |
| `api` | 基础 | 自定义 API（默认可用） |
| `frontend` | 基础 | 前端槽位/独立页面扩展（默认可用） |
| `settings` | 基础 | 设置项（默认可用） |
| `data.read` | **扩展** | **只读数据服务**——声明后才在激活时获得数据通道（运行时门控） |
| `admin.page` | 扩展 | 后台独立页面（M3.9：manifest `pages` 声明 + 壳路由） |
| `ai` | 扩展 | AI 能力（M3.9：ai.before/after_generate 钩子） |

- 声明**未知能力** → 安装被拒绝（提示支持列表）
- `data.read` 为运行时门控：未声明 → 激活时不下发数据服务连接 → `sdk.Data(ctx)` 返回 nil
- 清单 `capabilities` 与代码 `Info().Capabilities` 建议保持一致（主进程以进程 Info 上报为准做门控）

---

## 10. 数据服务（只读访问博客数据）

声明 `data.read` 后，插件经 GRPCBroker 获得主进程只读数据通道：

```go
svc := sdk.Data(ctx)
if svc == nil {
    // 未授权（未声明 data.read）：降级处理
}
user, err := svc.GetUser(ctx, 123)        // 脱敏用户：ID/昵称/头像/角色/简介
post, err := svc.GetPost(ctx, 456)        // 脱敏帖子：ID/标题/状态/作者昵称
settings, err := svc.GetSettings(ctx)     // 站点公开设置（白名单键）
```

**安全边界**：
- **只读**：无任何写接口（数据变更走钩子/主进程 API）
- **脱敏**：不含邮箱/手机/密码/正文全文/私信
- **白名单**：设置仅返回 `site_name`/`site_description`/`site_keywords`/`site_logo`/`site_icp`/`site_announcement`（密钥类永不暴露）
- 查询不存在/失败返回空占位（不暴露内部细节）

---

## 11. 数据库策略（重要）

**插件不可直连数据库，也不可执行 SQL**。理由：隔离（崩溃不影响数据）、安全（防越权读写）、解耦（主进程升级不破坏插件）。

数据操作的正确姿势：

| 需求 | 方式 |
|------|------|
| 读取用户/帖子/设置 | 数据服务 `sdk.Data(ctx)`（第 10 章） |
| 发布/评论等业务动作 | 经钩子影响主进程流程（拦截/改写），或引导用户在主站操作 |
| 自定义数据存储 | 插件自带文件/SQLite（自身数据目录），主进程不托管 |
| 复杂数据需求 | 自定义 API 由主进程代理转发（插件 handler 内自行处理其数据源） |

---

## 12. 增强与替换（在原有功能上的拓展）

| 目标 | 机制 | 语义 |
|------|------|------|
| **新增功能** | 自定义 API + 前端槽位/独立页面 + `after_*` 钩子 | 完全新增，不影响原功能 |
| **增强行为** | `after_*` 钩子 + `notification.send` + `ai.after_generate` | 在原有流程后追加处理 |
| **拦截替换** | `before_*` / `api.middleware` 同步钩子返回 `OK=false` | 阻断原流程（反垃圾、内容策略、API 防护） |
| **改写替换** | `search.query` / `content.render` / `ai.before_generate` 返回 `Modify` | 改写原流程输入/输出（搜索词、正文、AI 输入） |
| **展示替换** | 前端槽位 `mode: "replace"` | 隐藏槽位默认内容，仅渲染插件内容 |

**边界说明**：不支持「整体替换核心页面/服务」；`comment.item` 槽位当前为 append 语义（逐条 replace 规划中，见第 19 章）。

---

## 13. 打包与发布

### 打包（.bpk）

```bash
# 1. 编译插件二进制（当前平台）
go build -o plugin.exe ./cmd/my-plugin

# 2. 打包（含校验和 + 可选公钥 + 前端资产）
go run ./cmd/bp pack \
  -plugin "cmd/my-plugin/yueyan-plugin.json" \
  -bin "plugin.exe" \
  -pubkey "keys/public.pem" \      # 付费插件许可证公钥（可选）
  -frontend "cmd/my-plugin/frontend" \  # 前端扩展目录（可选）
  -os "windows" -arch "amd64" \
  -version "1.0.0" \
  -out "dist"
```

### 发布渠道

1. **本地上传**：后台「我的插件 → 上传 .bpk」（≤50MB）
2. **市场（GitHub 源）**：仓库 `plugins.json` 声明 + Release 发布 `{id}-{version}-{os}-{arch}.bpk` 资产（主进程校验 SHA-256 + 签名）

### 付费（许可证 + 在线购买）

- **离线签发**：作者私钥签发 `license.jwt`（Ed25519，`cmd/license-issue` 工具）；用户后台输入激活 → `sdk.License(ctx).FeatureEnabled("demo_pro")` 判断付费功能；7 天宽限期降级
- **在线购买（M3.9 支付渠道）**：作者先在后台配置**签发私钥**（AES 加密存储）→ 用户购买付费插件：安装 → 创建订单 → 支付（**开发环境模拟直接成功**；真实微信/支付宝渠道接入点预留）→ **服务端自动签发许可证并激活 Pro**（详情见第 18 章）

---

## 14. 调试与日志

- 插件日志：`logs/plugins/{插件ID}.log`（stderr 重定向）
- 主进程插件生命周期日志：`logs/server.log` 中 `[plugin-mgr]` 前缀（启动/崩溃计数/退避重启/Call 拒绝）
- 手动验证：`GET /api/v1/plugins/{插件ID}/ping`（自定义 API 代理链路）
- 崩溃排查：插件进程被熔断后，后台「我的插件」显示 last_error；恢复需手动重新启用

---

## 15. 安全边界（插件侧清单）

| 项 | 机制 |
|----|------|
| 进程隔离 | 插件崩溃不影响主进程（熔断/超时/退避） |
| 契约校验 | 二进制 ID 与安装实例一致性校验 + 包内 checksums + zip-slip 防护 |
| 能力门控 | capabilities 安装校验 + data.read/admin.page/ai 运行时门控 |
| 数据脱敏 | 数据服务只读 + 白名单键 + 脱敏字段 |
| 前端沙箱 | iframe 沙箱 + 短期 token（1 小时）+ CSP 严格头（生产无 unsafe-eval）；槽位/页面 API 客户端带登录凭证 |
| 配置防注入 | 保存时按 schema 过滤未声明键 |
| 路径防护 | 静态资产 `/plugin-assets` 白名单 + 路径穿越拒绝 |
| 中间件拦截 | `api.middleware` 钩子可阻断写请求（403）；插件内部错误故障隔离不拖垮核心 |
| 签发安全 | 付费签发私钥 AES 加密存储（settings，不落明文）；订单支付幂等 |

---

## 16. 故障与恢复（插件作者须知）

- **崩溃自愈**：进程意外退出 → 退避重启（1s→60s）→ 连续 5 次熔断（需手动重新启用）
- **超时隔离**：同步钩子 2 秒超时放行；自定义 API 10 秒超时
- **进程重启**：配置/许可证自动重新下发（激活流程），插件应保证 OnActivate 幂等
- **流式通道**：异步钩子经 Stream 推送，断连自动回退 Execute（通道随进程重启重建）；插件侧无感知
- **升级**：一键升级会停用 → 替换二进制 → 重新激活；插件应兼容旧配置（缺失键用默认值）

---

## 17. 完整示例（综合演示）

见 `cmd/demo-plugin/main.go`（官方演示插件，覆盖全部能力）：
- **钩子**：11 个钩子点订阅 6 个（post.before/after_publish、comment.after_save、search.query、content.render、api.middleware、ai.after_generate）
- **自定义 API**：`/ping`、`/pro-status`（许可证）、`/settings`（配置）、`/data-demo`（数据服务）
- **设置项**：greeting/show_badge/theme（3 字段）
- **能力声明**：`data.read`
- **前端扩展**：`post.footer`（append）+ `comment.item`（评论卡片）+ 独立页面 `demo`（/admin/plugin-pages/demo-plugin/demo）

对照本手册逐项可运行验证（安装 → 钩子/API/设置/数据服务/页面 → 冒烟脚本 `scripts/smoke_plugin.py` 93 项）。

---

## 18. 已落地扩展（M3.9 后置七项全部完成）

| 规划项 | 落地情况 |
|--------|---------|
| `Stream` 流式钩子 | ✅ `HookService.Stream` client-streaming：异步事件经流推送（断连回退 Execute；进程重启重建） |
| `content.render` / `api.middleware` / `ai.*` | ✅ 内容渲染管道（改写正文）、API 中间件（写请求拦截）、ai.before/after_generate |
| `admin.page.*` 独立路由 | ✅ 壳路由 `/admin/plugin-pages/{pluginId}/{route}`（manifest `pages` 声明 + `registerPage(ctx)` 契约） |
| 前端槽位 `mode: replace` | ✅ `ExtensionPoint.mode: "append"|"replace"`（replace 隐藏槽位默认内容） |
| 每条评论独立 slot | ✅ `comment.item` 槽位（props 透传评论对象；extensions 缓存防重复请求） |
| `admin.page` / `ai` 能力开放 | ✅ 能力枚举扩展（声明 + 校验 + 门控） |
| 支付渠道 | ✅ 订单表 + dev 模拟支付 + 服务端签发许可证自动激活（真实渠道接入点预留） |

**新钩子**：`content.render`/`api.middleware`/`ai.before_generate`/`ai.after_generate` 已并入第 5 章钩子全表（11 个钩子）。

**插件独立页面**（manifest 声明）：

```json
// frontend/manifest.json
{
  "extensionPoints": [...],
  "pages": [{ "route": "dashboard", "entry": "page.js" }]
}
```

```js
// page.js：默认导出 registerPage(ctx)，返回清理函数
// ctx: { container, api, user, params: {pluginId, page} }
export default function registerPage(ctx) {
  const el = document.createElement("div");
  el.textContent = "我的插件页面";
  ctx.container.appendChild(el);
  return () => el.remove();
}
```

访问：`/admin/plugin-pages/{pluginId}/{route}`（后台权限守卫 + 插件 running 校验）。

**支付渠道**（付费插件在线购买）：

```bash
# 1. 后台配置服务端签发私钥（作者私钥，AES 加密存储）
PUT /api/v1/admin/plugins/issuer-key   # {private_key_pem}

# 2. 安装付费插件 → 创建订单 → 支付（dev 模拟直接成功）
POST /api/v1/admin/plugins/{实例ID}/orders   # {price}
POST /api/v1/admin/plugin-orders/{orderId}/pay  # → 服务端签发 license.jwt + 自动激活
```

真实支付渠道（微信/支付宝）接入点：`PayOrder` 前增加渠道回调验签（订单金额以服务端定价为准）。

---

## 19. 剩余规划（未开放）

| 规划项 | 说明 |
|--------|------|
| 源码构建模式 | 插件以源码仓库形式安装（主进程编译） |
| devUrl 本地调试 | 插件开发热加载（指向本地 dev server） |
| license-service | 独立许可证签发服务（当前服务端签发已内置） |
| OAuth webhook | 插件连接 GitHub 后的事件回调 |
| 评论逐条 replace | comment.item 槽位的 replace 模式（当前 append） |
