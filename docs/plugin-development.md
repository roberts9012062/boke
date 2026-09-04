# 月言博客 · 插件开发手册

> 面向第三方插件作者：如何开发、打包、发布插件到月言博客平台。
> 适用版本：核心 ≥0.1.0（M3 插件系统全量：进程外化 + 能力授权 + 数据服务 + 设置 + 许可证 + 流式钩子 + 独立页面 + 支付渠道；B 路线：分发模式 + waterfall + seam + 配置分层 + 内容块）
> 配套：**[插件系统参考手册](plugin-reference/index.md)**（按主题分页的架构/目录/契约参考：核心概念、钩子目录、能力服务接缝、前端扩展、能力与安全、打包分发）；`docs/architecture.md` 第 6 章（架构设计）；`discuss/插件能力授权-验收报告.md`（M3.8）；`discuss/插件后置七项-验收报告.md`（M3.9）

---

## 1. 架构与设计原则

插件是**独立进程**（自研进程桥：标准库 net/rpc + gob + 回环 TCP），与博客主进程隔离运行：

```
┌───────────────────────── 博客主进程 ─────────────────────────┐
│  HTTP API / 业务服务 / 数据库 / 钩子调度器                     │
│         ▲  Core net/rpc（契约 contract）   ▲ 只读数据（Data 回连）  │
└─────────┼──────────────────────────────────┼──────────────────┘
    插件子进程（plugin.exe）◄─────────────────┘
    插件作者代码：Info/Hooks/RegisterAPI/OnActivate/...
```

**设计原则（解耦边界）**：

| 原则 | 说明 |
|------|------|
| **进程隔离** | 插件崩溃/卡死不影响主进程（熔断 + 超时 + 退避重启） |
| **契约通信** | 插件只能经契约（gob 序列化 net/rpc）与主进程交互，不共享内存 |
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

**市场清单**（插件源 GitHub 仓库，**文件夹结构**：每个插件一个文件夹，文件夹名 = 插件 ID，内含 `plugin.json` + `README.md`）：

**源码归属（2026-08 重组约定）**：全部插件源码（Go 后端 + frontend/ 前端资产 + yueyan-plugin.json）
存放在插件库仓库的 `{插件ID}/` 文件夹内（本地工作副本为主仓 `marketplace-repo/{插件ID}/`，独立 go module：
`github.com/roberts9012062/yueyan-plugins`，经 `replace` 引用主仓 plugin-sdk）——**主程序仓库不再存放插件源码**。
构建/打包/发布统一走 `scripts/` 脚本（源码路径已指向插件库）。

```
yueyan-plugins/                  # 插件源仓库（默认 roberts9012062/yueyan-plugins）
├── market.json                  # 可选：商城名称/描述（{name, description}）
├── my-plugin/                   # 插件文件夹（文件夹名 = 插件 ID）
│   ├── plugin.json              # 市场元数据（下表字段）
│   └── README.md                # 插件介绍（商城「详情」弹窗渲染 Markdown）
└── ...
```

`plugin.json` 字段（除 `.bpk` 包内 manifest.json 的基础字段外，市场展示补充）：

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 插件 ID（唯一，小写字母数字连字符） |
| `name` / `version` / `description` | string | 基础信息 |
| `category` | string | 类别：seo/security/performance/analytics/writing/ops/enhancement |
| `price` | int | 价格（0=免费） |
| `installs` / `official` | int / bool | 安装量 / 官方标签 |
| `capabilities` | string[] | **能力声明**（见第 9 章；未知能力拒绝安装） |
| `core_version` | string | 兼容核心版本（如 `>=0.1.0`） |
| `requires` / `conflicts` | string[] | 依赖 / 冲突插件 ID |
| `platforms` | string[] | 支持平台（linux/darwin/windows） |
| `repo_url` | string | 源码仓库（Release 资产发布源） |
| `nav` | object | 后台侧栏入口（`{href, label, icon}`） |
| `settings_schema` | object[] | 设置项声明（`{key,label,type,default,options}`；type: text/switch/select） |
| `open_endpoints` | object[] | **声明式开放端点**（宿主 v1.4.1+，见 6.4 节）：声明插件对外暴露的开放接口，安装/升级后自动进后台「接口开放」目录，经泛化网关 `/api/v1/open/plugins/{id}/…` 转发到插件进程——插件上新开放接口无需主程序发版 |

> **约定**：`README.md` 为商城「详情」弹窗展示内容（渲染 Markdown，支持表格/列表/代码块），应包含功能特性、安装方式、配置说明与 FAQ；缺少该文件时详情页提示「暂无介绍」。

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

钩子是插件影响博客行为的主要通道。每个钩子有固定的**分发模式**（对齐事件目录约定）：

- **`serial`（串行拦截）**：多个插件按优先级顺序执行，任一返回 `OK=false` 即短路阻断核心流程；
- **`waterfall`（链式改写）**：改写型钩子——**下游插件收到上游插件改写后的载荷**，各插件的 `Modify` 沿管道链式组合（如两个插件都改写正文，后者基于前者的结果继续改，不再互相覆盖）；任一拒绝同样短路；
- **`emit`（异步通知）**：事后通知，不阻塞调用方，经流式通道推送。

| 钩子 | 模式 | 触发时机 | Payload | 用途 |
|------|------|---------|---------|------|
| `post.before_publish` | serial | 发布前 | 帖子对象 | **拦截**（OK=false 阻断发布） |
| `post.after_publish` | emit | 发布后 | 帖子对象 | 通知/统计 |
| `comment.before_save` | serial | 评论保存前 | 评论内容字符串 | **拦截**（反垃圾） |
| `comment.after_save` | emit | 评论保存后 | 评论对象 | 通知/统计 |
| `search.query` | waterfall | 搜索时 | 关键词字符串 | **链式改写**（Modify 回写关键词） |
| `notification.send` | emit | 通知发送后 | 通知对象 | 通知增强（如推送到外部） |
| `admin.page` | serial | 后台仪表盘 | 仪表盘数据 | 后台增强 |
| `content.render` | waterfall | 帖子详情返回 | `{post_id, content}` | **链式改写正文**（Modify 回写 content） |
| `api.middleware` | serial | 写请求（POST/PUT/DELETE） | `{method, path, user_id}` | **拦截**（OK=false → 403） |
| `ai.before_generate` | waterfall | AI 生成前 | `{task, input, model}` | **链式改写输入**（Modify 回写 input） |
| `ai.after_generate` | emit | AI 生成后 | `{task, result}` | 通知/统计 |

> **waterfall 多插件改写**：插件 A 在正文头部插入目录（`Modify` 回写），插件 B 收到的 Payload 已含目录，继续美化代码块——两者叠加生效；仅一个插件订阅时行为与旧版完全一致。
>
> **异步钩子传输**：emit 钩子经 **流式通道**（`HookService.Stream`）推送——主进程建立长期连接持续发送；断连自动回退 Execute（进程重启后重建通道）。插件侧无需感知通道差异（`Hooks()` 声明即可）。

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

### 6.4 声明式开放端点（对外部应用开放，宿主 v1.4.1+）

自定义 API 默认需登录才能调用；想让**外部应用**（如月言浏览器插件）凭 API Key 调用，
在 `plugin.json` 声明 `open_endpoints` 即可——安装/升级后自动出现在后台「接口开放」目录，
站长给 Key 勾选授权后，经泛化网关 `/api/v1/open/plugins/{插件ID}/…` 以 System 身份转发到
插件进程（**插件上新开放接口无需主程序发版**）：

```json
"open_endpoints": [
  {
    "endpoint": "my-plugin.stats",              // 必须以 {插件ID}. 前缀
    "method": "GET",                             // 对外方法（GET/POST）
    "plugin_method": "POST",                     // 可选：插件端方法（缺省=method；对外 GET 调插件 POST）
    "path": "/api/v1/open/plugins/my-plugin/stats", // 必须位于该命名空间下；去前缀即插件端路径
    "name": "统计数据",                           // 必填：目录展示名
    "description": "返回站点统计摘要",
    "params": [{"name":"days","type":"integer","location":"query","required":false,"description":"统计天数"}],
    "trusted_body": { "admin": true }            // 可选：受信 body 注入（覆盖外部同名键，防伪造身份字段）
  }
]
```

**校验链**：打包期（`bp pack` 拒绝违规声明）→ 安装期（解析 manifest 校验）→ 运行期
（聚合目录防御性复校 + 与宿主内置目录标识冲突跳过）。**响应语义**：插件 200 数据被网关
包为 `{code:0,data}`；插件 `200+{"error"}` 转网关 400；插件 401/403 语义透传。
白名单精确匹配（未声明子路径/方法不匹配一律 404）。参考实现：nav-links v1.3.16。

> **声明位置（易踩坑）**：`open_endpoints` 须同时写在 **`yueyan-plugin.json`（包内清单源，
> `bp pack -plugin` 从此文件写入包内 manifest——漏写则包内缺失、安装后目录不出现）**
> 与市场清单 `plugin.json`（双清单同步维护，参考 nav-links）。另外发版时若先 `publish`
> 后同步清单，Release tag 会指向旧 commit——发布完成后应把 `v{version}` tag 移至最新
> main HEAD（升级链路按 tag 钉扎拉取）。

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

**插件页面同源调用宿主 REST**（先例：tg-image-bed v0.4.0 图片体检页）——非沙箱 ESM 页面与主站
同源，可直接 `fetch('/api/v1/...')`（`credentials: "same-origin"`）调用宿主 REST 接口：
登录态自动携带、权限由宿主服务端校验兜底（插件不获得任何越权能力）。适合「插件页面需要读
站点数据（如帖子正文）又不必扩 SDK `data.read`」的场景；端到端参考 `tg-image-bed` 源码
（检测正文图片 → 转存 TG → `PUT /admin/posts/:id` 回写正文）。注意：沙箱（`sandbox: true`）
页面在 iframe 内不同源，不适用此方式。

**前台公开页面**（`pages[].scope: "site"`，site.page 能力）——插件声明自己的前台页面，访客可访问：
```json
{ "pages": [
    { "route": "dashboard", "entry": "page.js" },
    { "route": "radio", "entry": "radio-page.html", "sandbox": true, "scope": "site" }
] }
```
- 访问路径 `/plugins/{插件ID}/{route}`（前台布局，无需登录；`scope` 缺省为 `admin`，存量插件不受影响）；
- 页面契约与后台页面一致（`registerPage(ctx)`；`sandbox: true` 走 iframe 强隔离，第三方页面推荐）；
- **数据边界**：访客（未登录）调用受限插件 API（`/api/plugins/{id}/**`）会得到 401——公开页面所需
  数据请走宿主公开端点（参考音乐插件的播放地址公开代理），或在页面内引导登录后使用；
- 声明 `scope: "site"` 页面的插件需在 `yueyan-plugin.json` 的 `capabilities` 中声明 `site.page`；
- 直链防护：宿主经公开接口校验插件 running 且已声明该 site 路由，未启用显示占位提示。

**前台导航项**（`manifest.siteNav`，与 site.page 配套）——插件向前台头部导航注册入口：
```json
{ "siteNav": [ { "label": "电台", "path": "/plugins/netease-music/radio" } ] }
```
- 导航项自动追加在前台桌面端头部导航（管理员在后台「头部导航」页配置的项）之后；
- 约束：`path` 仅允许站内路径（`/` 开头，`//` 与外链协议拒绝）、`label` ≤30 字符、每插件 ≤5 项；
- 生命周期随插件：停用/卸载后导航项自动消失（前台 30 秒缓存窗口内）；后台「头部导航」页
  以只读形式展示插件注册的导航项（不可在此编辑）。

**内容块**（`manifest.blocks`，B4 keyed renderer）——插件向正文注册自定义嵌入块：
```json
{ "blocks": [ { "type": "vote", "entry": "vote-block.js" } ] }
```
- 协议：帖子正文（html 格式）中的 `<div data-plugin-block="vote" data-props='{"id": 123}'></div>`
  节点由宿主按 `type` 查注册表分发到插件 `entry` 渲染（`register(ctx)` 契约与槽位一致，
  `ctx.slot` 为 `block:{type}`、`ctx.props` 为 `data-props` 解析结果）；
- 块标记可由插件自身经 `content.render` 钩子（waterfall 链式改写）注入正文，形成
  「后端改写注入 → 前端注册表渲染」的完整闭环；也可由作者在富文本中手写；
- 提供方插件未启用/未安装时，块渲染为占位提示（不影响正文其余部分）；
- `data-*` 属性经 DOMPurify 默认放行，块内交互走受限插件 API 客户端。

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

- **生效配置 = 默认值层 ⊕ 实例配置层**（B3 分层叠加）：未保存的设置项自动回退
  `Default`——`sdk.Config(ctx)` 读到的即生效值，插件**无需自行处理默认值**；
  显式保存空串视为用户清空（不回退默认）
- 保存后**即时生效**（主进程推送合并后的生效配置）；重启/升级后保持（激活时自动下发）
- 插件侧读取：`cfg := sdk.Config(ctx); cfg["greeting"]`（schema 声明的 key 恒存在，
  无默认未设置时为空串）
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
| `site.page` | 扩展 | 前台公开页面（manifest `pages` 声明 `scope: "site"` + 壳路由 `/plugins/{id}/{route}`，访客可访问；配套 `siteNav` 可注册前台导航项） |
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
openKeys, err := svc.GetOpenAPIKeys(ctx)  // 开放接口 API Key 清单（含明文 Key，见下文联动场景）
```

**安全边界**：
- **只读**：无任何写接口（数据变更走钩子/主进程 API）
- **脱敏**：不含邮箱/手机/密码/正文全文/私信
- **白名单**：设置仅返回 `site_name`/`site_description`/`site_keywords`/`site_logo`/`site_icp`/`site_announcement`（密钥类永不暴露；`GetOpenAPIKeys` 是唯一的显式例外，见下文）
- 查询不存在/失败返回空占位（不暴露内部细节）

### 10.1 与浏览器插件联动：读取 API Key 远传验证

**场景**：站点插件作为桥梁，与配套的**浏览器插件**（Chrome Extension 等）联动——浏览器插件需要调用本站开放接口（`/api/v1/open/*`，凭 `X-Api-Key` 请求头鉴权，后台「接口开放」页面管理 Key 与授权范围）。站点插件经数据服务读取 Key，远传给浏览器插件，即可在浏览器侧验证和使用重要接口。

`GetOpenAPIKeys` 返回 `[]DataOpenAPIKey`：

| 字段 | 说明 |
|------|------|
| `ID` / `Name` | 凭证 ID 与备注名 |
| `Key` | API Key 明文（`oa_` 前缀，67 字符） |
| `Endpoints` | 已授权接口标识（如 `posts.list`，对应后台接口目录） |
| `ExpiresAt` | 过期时间（RFC3339；**空串 = 永久有效**） |
| `LastUsedAt` | 最近调用时间（空串 = 从未使用） |
| `CreatedAt` | 创建时间 |

完整示例——站点插件自定义 API 把「可用的 Key + 开放接口基础路径」下发给浏览器插件：

```go
// 浏览器插件经站点前台的插件 API 通道拉取配置（宿主代理 /api/v1/plugins/{id}/api/*）
api.Handle("GET", "/browser-extension/config", func(ctx context.Context, method string, path string, body []byte) (int, []byte, error) {
    svc := sdk.Data(ctx)
    if svc == nil {
        return 200, []byte(`{"error":"未授权数据服务（需声明 data.read 能力）"}`), nil
    }
    keys, err := svc.GetOpenAPIKeys(ctx)
    if err != nil {
        return 500, []byte(fmt.Sprintf(`{"error":%q}`, err.Error())), nil
    }
    // 过滤出未过期的 Key（ExpiresAt 空串 = 永久；浏览器插件据此判断可用性）
    type keyPayload struct {
        Key       string   `json:"key"`
        Endpoints []string `json:"endpoints"`
        ExpiresAt string   `json:"expires_at"`
    }
    usable := make([]keyPayload, 0, len(keys))
    for _, k := range keys {
        if k.ExpiresAt != "" {
            if t, err := time.Parse(time.RFC3339, k.ExpiresAt); err == nil && t.Before(time.Now()) {
                continue // 已过期：跳过
            }
        }
        usable = append(usable, keyPayload{Key: k.Key, Endpoints: k.Endpoints, ExpiresAt: k.ExpiresAt})
    }
    raw, _ := json.Marshal(map[string]any{"open_api_base": "/api/v1/open", "keys": usable})
    return 200, raw, nil
})
```

浏览器插件拿到 `Key` 后按 AI 开发手册的方式调用：

```js
// 浏览器插件（MV3，host_permissions 已声明站点地址）
const res = await fetch(`${siteOrigin}/api/v1/open/posts?page=1`, {
  headers: { "X-Api-Key": key },
});
const body = await res.json(); // {code, message, data, request_id}，code=0 成功
```

**安全注意事项**：
- **明文 Key 例外说明**：数据服务整体脱敏，但 `GetOpenAPIKeys` 按联动需求显式返回明文 Key——信任依据是插件由管理员手动安装且必须声明 `data.read` 能力（清单与二进制一致才下发数据通道），与后台「接口开放」页面显示明文 Key 同级信任
- **按需下发**：只把必要 Key 传给浏览器插件（按 `Endpoints` 过滤），不要全量转发
- **泄露处置**：Key 疑似泄露时，在后台删除该 Key 并重新生成即可（旧 Key 立即失效）
- **过期语义**：`ExpiresAt` 空串 = 永久；非空时浏览器插件侧应校验剩余有效期

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
2. **市场（GitHub 源）**：插件源仓库按**文件夹结构**声明（`{id}/plugin.json` + `{id}/README.md`，见第 3 章）+ 源码仓库 Release 发布 `{id}-{version}-{os}-{arch}.bpk` 资产（主进程校验 SHA-256 + 签名）

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

见 `marketplace-repo/demo-plugin/main.go`（官方演示插件，覆盖全部能力）：
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

## 19. 音乐源扩展（E7 可 pluggable 契约 + B2 capability seam）

宿主提供通用音乐桥接端点，音乐源完全由插件实现——新增音乐源**无需改宿主代码**。
B2 起音乐源经 **music capability seam**（服务接缝）解析：宿主消费方依赖
`plugin.MusicSource` 接口（`internal/plugin/seam_music.go`）而非具体插件——查找经
服务注册表（`ServiceRegistry`），未命中时按清单发现并懒注册，插件停用时自动清理
（注册可逆）。对插件作者完全透明（仍只需实现契约端点）。

### 19.0 capability seam 目录（三角色）

seam 是「可替换能力」的正式抽象，每个 seam 按三角色设计与审查（对齐 dsh
capability-seams 思想）：

| seam 键 | 服务定义 | 提供方 | 消费方 | 状态 |
|---------|---------|--------|--------|------|
| `music.{provider}` | `plugin.MusicSource`（`seam_music.go`） | 音乐插件适配器（`NewMusicSourceAdapter`，懒注册） | `PluginService.MusicSource` → 音乐桥接 handler | **已落地** |
| `ai.*` | 预留（AI 生成供应商接缝） | — | — | 预留（出现第二个 Provider 时落地） |
| `search.*` | 预留（搜索后端接缝） | — | — | 预留（同上） |

**新增 seam 检查单**（三件套一起设计才算完整接缝）：

1. **服务定义**：`internal/plugin/seam_{name}.go` 声明接口 + 键构造函数；
2. **提供方**：内置实现或插件适配器注册进 `ServiceRegistry`（`Register`，注册即副作用可逆）；
3. **消费方**：`internal/service/plugin_seam.go` 加查找门面方法，业务 handler 只依赖接口。

> 注意：seam 是「宿主消费的可替换能力」，不是插件间通信通道——插件间协作仍走钩子
> （waterfall 链式改写天然支持多插件组合）。

### 19.1 声明与发现

- 插件市场清单（`marketplace-repo/{插件}/plugin.json`）声明 `music_provider` 字段
  （provider 名，全局唯一小写，如 `"qq"`、`"netease"`）；
- 宿主按「清单 `music_provider` 声明 + 插件已安装且 running」动态发现
  provider → 插件 ID 映射，构造适配器注册进服务注册表；清单不可用时回退宿主静态
  兜底表（官方源兜底）。首次桥接请求后注册表直达，插件停用时统一清理。

### 19.2 插件契约端点（必选/可选）

| 端点 | 方法 | 入参 | 返回 | 说明 |
|------|------|------|------|------|
| `/music/url` | POST | `{"src": "<源特定标识>"}` | `{"url": "<播放地址>"}` 或 `{"error": "..."}` | 必选。src 语义由源定义（qq=songmid、netease=歌曲 id） |
| `/music/bgm` | GET | — | `{"enabled": bool, "playlist_tid": "...", "songs": [...]}` | 可选。首页背景音乐聚合（配置 + 歌曲列表一次返回） |

说明：桥接调用携带**系统调用者身份**（`sdk.CallerIsSystem(ctx) == true`）；
播放地址端点对访客公开（匿名可播），管理类端点仍应校验 `sdk.TrustedCaller(ctx)`。

### 19.3 宿主公开端点

- `GET /api/v1/music/:provider/url?src=xxx` —— 播放地址（公开）；
- `GET /api/v1/music/:provider/bgm` —— 背景音乐（公开；插件未实现契约时返回空配置）。

### 19.4 参考实现

`marketplace-repo/qq-music/main.go`（`/music/url` + `/music/bgm`）与
`marketplace-repo/netease-music/main.go`（`/music/url`）。前端帖内嵌入解析
（`music-embed.ts`）属宿主产品形态，按源定制，不随插件分发。

## 20. 前端沙箱模式（E1 强隔离页面）

第三方插件后台页面可声明 iframe 沙箱（与宿主页面不同文档，无同源权限），
适合不完全信任的插件作者接入。

### 20.1 声明

插件前端 `manifest.json` 的 `pages[].sandbox: true`（缺省 false，走同源 ESM 模式）：

```json
{
  "pages": [
    { "route": "panel", "entry": "panel.html", "sandbox": true }
  ]
}
```

`sandbox: true` 时 `entry` 为 **HTML 页面**（相对 frontend/ 的路径），经
`/plugin-assets/{id}/frontend/panel.html` 由 iframe 加载。

### 20.2 宿主 ↔ 沙箱通信（postMessage）

iframe 页面引入宿主共享 SDK（`<script type="module" src="/plugin-sdk/shared.js">`），
用 `createSandboxApi()` 获得受限 API 客户端：

```html
<script type="module">
  import { createSandboxApi } from "/plugin-sdk/shared.js";
  const api = await createSandboxApi(); // 就绪后与宿主握手完成
  const status = await api.get("/status"); // 插件自身代理 API（自动带短期 token）
</script>
```

- 协议：`init`（宿主 → 页面：用户信息 + 1 小时短期 token）、`api` / `api-result`
  （页面 ↔ 宿主：请求往返）；
- 安全：宿主校验 `event.origin` 同源 + pluginId 匹配；短期 token 过期后页面需提示
  管理员刷新（宿主页面刷新即重发）。

## 21. 剩余规划（未开放）

| 规划项 | 说明 |
|--------|------|
| 源码构建模式 | 插件以源码仓库形式安装（主进程编译） |
| devUrl 本地调试 | 插件开发热加载（指向本地 dev server） |
| license-service | 独立许可证签发服务（当前服务端签发已内置） |
| OAuth webhook | 插件连接 GitHub 后的事件回调 |
| 评论逐条 replace | comment.item 槽位的 replace 模式（当前 append） |
