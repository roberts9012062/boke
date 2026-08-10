# 月言插件开发手册

| 项目 | 内容 |
|---|---|
| 手册版本 | v1.0 |
| 配套 SDK | `github.com/yueyan/plugin-sdk` v1.x |
| 配套主站 | 月言博客平台（架构文档见 `architecture.md`） |
| 读者 | 第三方插件开发者 |

---

## 1. 概述

插件是**独立编译、独立运行的二进制进程**：作者用 Go + 官方 SDK 编写插件，编译成可执行文件，打包为 `.bpk` 安装包，上传到自己的 **GitHub 仓库**（源码 + Release 产物），站点管理员在后台「插件市场」连接 GitHub 后即可一键安装。

```
┌──────────────── 主进程 (blog-server) ────────────────┐
│  业务模块 ──调用钩子──► 插件管理器 ──gRPC(加密)──► 插件进程  │
│  (文章/评论/AI…)        生命周期·崩溃重启           │
└─────────────────────────────────────────────────────┘
```

插件能做什么：

| 能力 | 说明 | 章节 |
|---|---|---|
| 响应钩子 | 在文章发布、内容渲染、评论保存等业务节点插入逻辑 | 第 5 章 |
| 自定义 API | 注册自己的 gRPC 方法，经主站代理为 HTTP 接口 | 第 6 章 |
| 前端扩展 | 在主题槽位（页头、文章页脚、后台菜单）注入 UI | 第 8 章 |
| 消费 AI | 通过 `ai.chat` / `ai.embedding` 钩子复用主站的 AI 供应商与密钥 | 第 5、9 章 |
| 插件配置 | 声明配置项，后台自动生成设置表单 | 第 7 章 |

**完整开发流程**：实现接口（约 40 行）→ 本地调试 → 打包 `.bpk` → 推 GitHub + 发 Release → 提交官方索引仓库（可选）→ 用户安装。

---

## 2. 环境准备

| 依赖 | 要求 | 说明 |
|---|---|---|
| Go | 1.22+（建议 1.24） | 插件与主站 SDK 需同大版本 |
| 操作系统 | Windows / Linux / macOS 均可 | 插件按平台分别构建分发 |
| 打包工具 | `yueyan-bp`（随 SDK 提供）或 Makefile | 第 10 章 |
| GitHub 账号 | 必须 | 托管源码与 Release |

初始化插件项目：

```bash
mkdir seo-helper && cd seo-helper
go mod init github.com/you/seo-helper
go get github.com/yueyan/plugin-sdk@latest
```

> SDK 是插件**唯一的第三方依赖**。不要引入与主站冲突的全局状态；gRPC 通信细节由 SDK 封装，插件作者不需要接触 go-plugin。

---

## 3. 快速开始：5 分钟 Hello 插件

### 3.1 目录结构

```
hello-plugin/
├── go.mod
├── main.go                 # 插件入口
└── yueyan-plugin.json      # 市场清单（发布时放仓库根目录）
```

### 3.2 完整代码

```go
package main

import (
    "context"
    "fmt"

    "github.com/yueyan/plugin-sdk"
)

// 1. 实现 sdk.Plugin 接口
type HelloPlugin struct{}

// 插件元信息（市场展示用）
func (p *HelloPlugin) Info() sdk.Info {
    return sdk.Info{
        ID:          "hello",
        Name:        "示例插件",
        Version:     "1.0.0",
        Description: "文章发布后向作者发送一条私信",
        Author:      sdk.Author{Name: "你", GitHub: "yourname"},
        License:     "MIT",
    }
}

// 启用时回调：可做资源初始化
func (p *HelloPlugin) OnActivate(ctx context.Context) error { return nil }

// 停用时回调：必须清理资源（进程随后被终止）
func (p *HelloPlugin) OnDeactivate(ctx context.Context) error { return nil }

// 声明订阅的钩子
func (p *HelloPlugin) Hooks() []sdk.Hook {
    return []sdk.Hook{{
        Name:     "post.after_publish", // 文章发布后（异步）
        Sync:     false,                // 异步：不阻塞发布流程
        Priority: 100,                  // 数值小先执行
        Handler:  p.onPostPublished,
    }}
}

func (p *HelloPlugin) onPostPublished(ctx context.Context, e sdk.Event) (sdk.Result, error) {
    // Event 携带业务数据（见第 5 章钩子明细）
    var ev struct {
        PostID int64  `json:"post_id"`
        Title  string `json:"title"`
        AuthorID int64 `json:"author_id"`
    }
    if err := e.Decode(&ev); err != nil {
        return sdk.Result{}, err
    }
    fmt.Printf("文章发布: #%d %s\n", ev.PostID, ev.Title)
    // 这里可以调用主站 API（见 6.2）给作者发私信
    return sdk.Result{OK: true}, nil
}

// 2. 启动：SDK 负责握手、注册、优雅退出
func main() {
    sdk.Serve(&HelloPlugin{})
}
```

### 3.3 本地运行与调试

```bash
go build -o hello-plugin.exe .        # Windows 示例

# 将产物放入主站插件目录后，在后台「插件管理」执行
# 添加本机插件 → 启用。日志在 plugins/hello/logs/hello.log
```

主站在**开发模式**下支持直接添加本地二进制插件（跳过签名与市场校验），生产模式只能安装 `.bpk`。

### 3.4 打包

```bash
yueyan-bp pack --os windows --arch amd64 --out dist/
# 生成 dist/hello-1.0.0-windows-amd64.bpk（结构见第 10 章）
```

---

## 4. 插件核心接口与生命周期

### 4.1 Plugin 接口

```go
type Plugin interface {
    Info() Info                                    // 元信息
    OnActivate(ctx context.Context) error          // 启用
    OnDeactivate(ctx context.Context) error        // 停用
    Hooks() []Hook                                 // 订阅的钩子
}
```

`Info` 字段（`yueyan-bp pack` 时自动从 `yueyan-plugin.json` 读取，两处必须一致）：

| 字段 | 必填 | 说明 |
|---|---|---|
| ID | ✅ | 全局唯一，小写字母 + 连字符，如 `seo-helper` |
| Name / Version | ✅ | 显示名；语义化版本 `1.4.2` |
| Description | ✅ | 市场展示 |
| Author | ✅ | 姓名 + GitHub 账号 |
| License | ✅ | 免费插件推荐 MIT/Apache-2.0；付费插件用 BSL-1.1 / Commons Clause（见第 9 章） |
| SDK 兼容范围 | ✅ | 在 `yueyan-plugin.json` 的 `sdk` 字段声明 |

### 4.2 生命周期

```
主站启动时:   已启用插件按依赖顺序拉起进程 → OnActivate
启用插件:     拉起进程 → 握手 → 校验 → OnActivate → 状态 running
停用插件:     通知 → OnDeactivate → 优雅退出 → 强杀兜底(10s)
崩溃:         主站自动重启（1s→2s→…→60s 退避），连续 5 次失败置 crashed
卸载:         停用 → 删除目录与数据
```

- `OnActivate` 失败 → 插件置为 `error` 状态，后台可见错误原因。
- `OnDeactivate` 里**必须**停止自己启动的 goroutine / 后台任务，进程随后被回收。
- 插件内不要调用 `os.Exit`——用返回错误表达失败。

### 4.3 Hook 声明规则

```go
type Hook struct {
    Name     string                    // 钩子名（必须属于第 5 章全集）
    Sync     bool                      // true=同步(阻塞业务请求链) / false=异步
    Priority int                       // 越小越先执行（默认 100）
    Handler  func(ctx, e Event) (Result, error)
}
```

- **同步钩子**：在业务请求链内执行，**默认超时 2 秒**（主站可配）。超时 → 主站返回 `4004 钩子超时` 并熔断该插件（临时摘除）。
- **异步钩子**：主站投递到 Redis 队列后立即返回；失败自动重试（指数退避，最多 3 次）。
- 同一钩子多个插件订阅时按 `Priority` 升序执行；同步钩子的 `Result` 可**改写**业务结果（如拦截发布）。

---

## 5. 钩子全解（v1.0 全集）

| 钩子 | 触发时机 | 同步/异步 | 典型用途 |
|---|---|---|---|
| `post.before_publish` | 文章发布前（可拦截/改写） | 同步 | 合规检查、敏感词、AI 标题润色 |
| `post.after_publish` | 文章发布后 | 异步 | 索引、SEO 推送、AI 摘要生成 |
| `content.render` | 文章/评论渲染前 | 同步 | 内容改写、外链处理、代码高亮 |
| `comment.before_save` | 评论保存前（可拒绝） | 同步 | AI 审核、垃圾识别 |
| `search.query` | 搜索执行时 | 同步 | 扩展搜索范围 |
| `api.middleware` | 插件自注册路由的中间件 | 同步 | 统计、限流 |
| `ai.chat` / `ai.embedding` | AI 推理调用时 | 同步/流式 | 模型路由、提示词注入 |
| `notification.send` | 通知发送前 | 异步 | 邮件 / Webhook 转发 |
| `admin.page` | 后台扩展点渲染 | 同步 | 提供后台新页面数据 |

### 5.1 Event 与 Result

```go
type Event struct {
    TraceID  string          // 贯穿日志的追踪 ID，务必随日志输出
    ActorID  int64           // 触发者用户 ID（系统事件为 0）
    Payload  json.RawMessage // 业务数据，用 e.Decode(&struct) 解析
}

type Result struct {
    OK      bool            // false 表示业务应被拦截/回滚
    Reason  string          // 拦截原因（OK=false 时展示给用户）
    Modify  json.RawMessage // 可选：改写后的 payload（回传给主站）
}
```

同步钩子返回 `OK:false` 会**阻断业务**（如 `comment.before_save` 返回 false → 评论被拒，用户看到 `Reason`）。

### 5.2 各钩子 payload 明细

**post.before_publish（同步，可改写）**

```go
type ev struct {
    PostID  int64    `json:"post_id"`
    Title   string   `json:"title"`
    Content string   `json:"content"`   // 原始 Markdown
    Tags    []string `json:"tags"`
    AuthorID int64   `json:"author_id"`
}
// 改写：返回 Result{OK:true, Modify: 新的 payload 字节}
// 拦截：返回 Result{OK:false, Reason:"内容含违规词"}
```

**content.render（同步）**

```go
type ev struct {
    Kind    string `json:"kind"`     // post | comment
    HTML    string `json:"html"`     // 已渲染 HTML，可改写
    Context string `json:"context"`  // 页面上下文
}
```

**ai.chat（同步/流式）**——复用主站 AI 能力：

```go
type ev struct {
    Model    string    `json:"model"`
    Messages []aiMsg   `json:"messages"`
    MaxTokens int      `json:"max_tokens"`
}
// 插件可：改写 messages（提示词注入）、替换 model（路由）、
// 或直接返回自己的 AI 结果（Result.Modify 携带响应）
```

> 插件通过 `ai.chat` 使用主站 AI 时**不接触任何 API 密钥**（密钥在主进程内），用量计入主站配额统计。

### 5.3 钩子兼容性承诺

- payload 字段**只增不删**，新增字段用可选语义；插件解析时用 `json.Unmarshal` 并容忍缺省。
- 钩子名全集冻结后追加新钩子用 `v2` 后缀（如 `post.before_publish_v2`），不修改旧钩子语义。

---

## 6. 插件自定义 API

### 6.1 注册 gRPC 方法

```go
type HelloPlugin struct{ /* ... */ }

func (p *HelloPlugin) Methods() []sdk.Method {
    return []sdk.Method{{
        Name:   "notify_author",        // 方法名
        Sync:   true,
        Handler: func(ctx context.Context, req json.RawMessage) (json.RawMessage, error) {
            return json.Marshal(map[string]any{"ok": true})
        },
    }}
}
```

主站自动代理为 HTTP 接口：

```
POST /api/plugins/hello/notify_author
Authorization: Bearer <用户token>
Body: 任意 JSON（原样透传）
```

- 代理层注入插件身份，**插件不解析也不信任外部 JWT**——用户身份由主站解析后放入上下文（`sdk.UserFromContext(ctx)`，可能为空 = 匿名）。
- 方法超时默认 5s；长任务注册 `Stream:true` 走流式。

### 6.2 调用主站 API

SDK 提供受限的内部客户端（防止插件绕过权限）：

```go
client := sdk.CoreClient(ctx)          // 从钩子上下文获取
resp, err := client.Post("/api/v1/internal/notify", map[string]any{
    "user_id": ev.AuthorID,
    "message": "你的文章已发布 🎉",
})
```

- 仅开放 `internal` 白名单接口（通知、查询等），主站侧按插件 ID 记账与限流。
- 插件**不得**直接连接主站数据库或 Redis——数据访问统一走内部 API。

---

## 7. 插件配置

### 7.1 声明配置项

在 `yueyan-plugin.json` 中声明（JSON Schema 子集）：

```json
{
  "config": {
    "type": "object",
    "properties": {
      "notify_author": { "type": "boolean", "default": true, "title": "通知作者" },
      "webhook_url":   { "type": "string",  "format": "uri",  "title": "Webhook 地址" },
      "level":         { "type": "string",  "enum": ["info", "warn"], "default": "info" }
    }
  }
}
```

后台「插件 → 设置」自动按 schema 生成表单；`format` 支持 `uri` / `email` / `password`（加密存储）。

### 7.2 运行时读取与变更通知

```go
cfg := sdk.Config(ctx)                    // map[string]any，含默认值
webhook := cfg["webhook_url"].(string)

// 配置被修改时（插件常驻状态下）：
func (p *HelloPlugin) OnConfigChange(ctx context.Context, cfg map[string]any) error {
    return nil // 重新加载自身状态
}
```

---

## 8. 前端插件开发

### 8.1 轻量扩展（推荐）

`.bpk` 内 `frontend/` 目录：

```
frontend/
├── manifest.json     # 扩展点声明
├── index.js          # ESM 模块（打包后产物）
└── style.css
```

`manifest.json`：

```json
{
  "extensionPoints": [
    { "slot": "post.footer",  "entry": "index.js" },
    { "slot": "admin.menu",   "entry": "index.js", "props": { "label": "SEO 助手" } }
  ]
}
```

可用槽位（v1.0）：

| 槽位 | 位置 |
|---|---|
| `theme.header` | 主题页头右侧 |
| `post.footer` | 文章页脚 |
| `comment.footer` | 单条评论下方 |
| `admin.menu` | 后台左侧菜单 |
| `admin.page.*` | 后台独立页面（`admin.page.seo` → 路由 `/admin/plugins/seo-helper/page`） |

入口模块约定：

```js
// index.js —— 必须默认导出一个注册函数
export default function register(ctx) {
  // ctx: { slot, el(挂载点), api(受限API客户端), user }
  ctx.el.innerHTML = '<div>来自插件的扩展</div>'
  // 卸载时必须清理
  return () => { ctx.el.innerHTML = '' }
}
```

主站按 `.bpk` 内 `checksums.json` 校验前端资源后注入页面；插件停用/卸载时同步移除。

### 8.2 复杂扩展（iframe 沙箱）

独立管理页 / 交互复杂的插件用 iframe：

- 插件前端自己打包为静态站点，随 `.bpk` 提供，挂载到 `/api/plugins/{id}/assets/...`。
- 主站通过 `postMessage` 下发：短期 token（1 小时）、用户基础信息（不含密钥）。
- 插件前端凭短期 token 调用 `POST /api/plugins/{id}/...` 代理 API。

### 8.3 本地调试

```bash
# frontend/ 下起 Vite dev server，manifest.json 里配置 devUrl
# 主站开发模式直接加载 devUrl，HMR 生效
```

---

## 9. 许可证（付费插件）

### 9.1 模式选择

| 模式 | 许可 | 说明 |
|---|---|---|
| 免费 | MIT / Apache-2.0 | 无需任何许可证代码 |
| 付费 | **BSL 1.1 或 Commons Clause**（fair-code） | 源码公开可读、个人自用免费；商业部署需购买激活 |

付费插件流程：

```
用户安装 → demo 模式（基础功能）→ 购买 → 作者签发 license.jwt
→ 后台输入/自动激活 → 验签通过 → 全功能 → 到期未续 → 自动降级 demo（7 天宽限期）
```

### 9.2 SDK 集成

```go
// 付费功能开关：每次调用前检查
lic := sdk.License(ctx)               // 含 edition / features / 到期时间
if !lic.FeatureEnabled("batch_fix") {
    return sdk.Result{OK: false, Reason: "批量修复为 Pro 功能，请购买激活"}
}
```

- **许可文件只读不写**：验签、状态、宽限期全部由主站处理，插件只查询。
- demo 模式 = 代码内用 `FeatureEnabled` 收敛到基础功能，**不要**把付费逻辑放进前端。
- 密钥（作者 Ed25519 私钥）由作者自持或委托官方 `license-service`；主站只存公钥。

### 9.3 许可证数据结构

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

`license.jwt` 随激活接口下发，主站验签后缓存；公钥由作者在插件包 `pubkey.pem` 中提供（安装时登记，换公钥需随新版本发布）。

---

## 10. 打包与发布（GitHub 流程）

### 10.1 .bpk 结构

```
seo-helper-1.4.2-linux-amd64.bpk      # zip 封装
├── manifest.json        # id/名称/版本/作者/描述/sdk兼容范围（与 yueyan-plugin.json 一致）
├── plugin.bin           # 插件二进制
├── pubkey.pem           # 许可证公钥（仅付费插件）
├── frontend/            # 前端扩展资产（可选）
│   ├── manifest.json
│   ├── index.js
│   └── style.css
└── checksums.json       # 以上文件的 SHA-256
```

### 10.2 仓库要求（市场可发现）

插件仓库根目录必须放 `yueyan-plugin.json`（市场数据源，字段见 `architecture.md` 6.5.3）：

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

> 主站按 `assets.pattern` 在仓库 **Release 资产**中按平台匹配下载；`{version}` 取 GitHub tag。

### 10.3 多平台构建（Makefile 示例）

```makefile
VERSION ?= 1.4.2
PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

build:
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		GOOS=$$os GOARCH=$$arch go build -o dist/plugin.bin . && \
		yueyan-bp pack --os $$os --arch $$arch --version $(VERSION) --out dist/; \
	done
```

或直接用 `yueyan-bp release --version 1.4.2`（自动构建全部平台并生成 .bpk）。

### 10.4 发布 Release

```bash
git add . && git commit -m "feat: v1.4.2" 
git tag v1.4.2
git push origin v1.4.2

# 创建 Release 并上传全部平台的 .bpk 资产
gh release create v1.4.2 dist/*.bpk \
  --title "SEO 助手 v1.4.2" \
  --notes "新增批量修复功能"
```

**命名必须匹配** `yueyan-plugin.json` 的 `assets.pattern`：`{id}-{version}-{os}-{arch}.bpk`。

### 10.5 进入官方索引仓库（可选，推荐）

1. 完成上述发布。
2. 向 `yueyan/plugin-registry` 提交 PR：在 `plugins.json` 追加一行：

   ```json
   { "id": "seo-helper", "repo": "yourname/seo-helper", "category": "seo" }
   ```

3. 审核通过后，所有连接了 GitHub 的市场都能在「官方索引」Tab 看到你的插件。

### 10.6 发布检查清单

- [ ] `yueyan-plugin.json` 与 `Info()`、打包时 manifest 三者 id/version 一致
- [ ] `sdk` 兼容范围正确（如 `>=1.0.0`）
- [ ] 全部目标平台 `.bpk` 已上传为 Release 资产
- [ ] 资产命名匹配 `assets.pattern`
- [ ] 免费插件：LICENSE 文件 + 代码可构建
- [ ] 付费插件：`pubkey.pem` 已在包内、demo 降级逻辑用 `FeatureEnabled` 实现
- [ ] 前端资源 `checksums.json` 已生成
- [ ] 仓库 README 含：功能、截图、安装说明、联系方式

---

## 11. 调试与排障

### 11.1 日志

- 插件 stdout 由主站重定向到 `plugins/{id}/logs/{date}.log`，`sdk.Logger(ctx)` 输出时自动附带 `trace_id`。
- 后台「插件 → 日志」实时查看；崩溃时自动附带最后 100 行上下文。

### 11.2 钩子不触发？

| 现象 | 排查 |
|---|---|
| 钩子完全没反应 | ① 插件状态是否为 running（后台查看）② 钩子名是否拼写正确（大小写敏感）③ `yueyan-plugin.json` 的 `hooks` 是否声明 |
| 偶发不触发 | 同步钩子超时被熔断 → 看日志 `hook_timeout`，优化耗时或改为异步 |
| 只执行了一部分 | 前序插件 `Priority` 更低且返回了 `OK:false` 拦截——检查多插件优先级 |

### 11.3 崩溃与重启

- 崩溃原因在 `plugins/{id}/logs/` 与主站 `plugin_crash` 事件中记录。
- 连续崩溃 5 次 → 插件置 `crashed`，停止自动重启，后台告警——先修 bug 再手动启用。
- 开发期可在 IDE 里断点调试：主站开发模式对本地插件支持 `reattach`（复用现有进程，不重复拉起）。

### 11.4 常见错误码

| 码 | 含义 | 处理 |
|---|---|---|
| 4001 | 插件不存在 | 检查 id |
| 4002 | 插件崩溃 | 看 11.3 |
| 4003 | 许可证无效/过期 | 重新激活或续费 |
| 4004 | 钩子超时 | 拆异步 / 优化耗时 |
| 5001/5002/5003 | AI 供应商不可用/超时/配额不足 | 检查主站 AI 配置 |

### 11.5 常见坑

- **不要在同步钩子里调 AI 或网络**：2 秒超时很容易被打爆，AI 放异步钩子。
- **Event payload 用 `e.Decode`**：直接 `json.Unmarshal(e.Payload)` 会漏掉主站追加字段的兼容处理。
- **配置读不到**：检查 `yueyan-plugin.json` 的 `config` schema 是否有 `default`。
- **Windows 上进程杀不掉**：主站会先发停用信号再强杀，开发时确保 `OnDeactivate` 不阻塞。

---

## 12. 最佳实践

| 原则 | 说明 |
|---|---|
| 钩子幂等 | 同一事件可能重试多次（异步钩子最多 3 次），处理前先查重（如按 `post_id`） |
| 同步钩子快速返回 | 预算 < 200ms；重活一律异步钩子 + 内部限并发 |
| 错误即返回 | 不要 `panic`（会触发崩溃重启）；返回 `(Result{}, err)` 即可 |
| 资源清理 | `OnDeactivate` 停止所有 goroutine/定时器；长驻任务需可取消（ctx） |
| 版本兼容 | `sdk` 范围写准；字段只增不删；破坏性变更必须升主版本 |
| 安全 | 密钥、token 一律不存插件内——用 `sdk.Config` 的 `password` 类型或主站 secrets；输出内容做 HTML 转义 |
| 成本 | AI 调用默认计入主站配额，插件内做自己的频率控制 |
| 可观测 | 每个入口输出 `trace_id` + 耗时；关键路径打点（`sdk.Metric`） |
| 文档 | README 写清功能、配置项、钩子行为、FAQ——市场页会展示 |

---

## 13. 示例项目

| 项目 | 用途 | 亮点 |
|---|---|---|
| `hello-plugin` | 官方入门模板 | 最小完整插件（第 3 章） |
| `plugins/seo`（官方仓库） | SEO 增强插件参考 | 同步+异步钩子、付费功能降级、前端扩展点组合 |
| `plugins/ai-assistant`（官方仓库） | AI 插件参考 | `ai.chat` 钩子消费、流式输出、配置 schema 实例 |

**新插件速查**：复制 `hello-plugin` 模板 → 改 `Info()` → 写钩子 → `make build` → 发 Release → 提交索引仓库。

---

## 附录 A：术语表

| 术语 | 含义 |
|---|---|
| 钩子（Hook） | 主站业务节点向插件开放的事件点 |
| 扩展点（Extension Point） | 前端主题槽位中可由插件填充的位置 |
| .bpk | 插件安装包（zip：manifest + 二进制 + 前端资产 + 校验和） |
| yueyan-plugin.json | 仓库根目录市场清单 |
| fair-code | 代码可读、个人自用免费，商业部署需许可（BSL 1.1 / Commons Clause） |
| demo 模式 | 付费插件未激活时的降级运行状态 |

## 附录 B：许可证/授权相关

- 插件分发使用 fair-code 许可时，**务必**在 README 显著位置说明商业使用条款。
- 主站对插件产生的数据与内容归属不作声明——在插件 README 中写明数据处理说明（涉及隐私的需合规）。
- 提交官方索引仓库即视为同意收录规则；违规插件（恶意、盗版）将被下架并可能停止授权。

---

*手册与 `architecture.md` 保持同步；SDK 发布时若接口有演进，以 changelog 为准。*
