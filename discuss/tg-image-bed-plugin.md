# TG 图床插件方案（tg-image-bed）

> 状态：已拍板（2026-09-03）——形态 A1 独立图库（零宿主改动，实现 /storage/* 契约端点备用）；默认发送模式 document 原图（设置项可切 photo）；api_proxy 可选代理设置项
> 追加（2026-09-03 晚）：**A2 落地**——宿主媒体存储 seam 提供方从硬编码 image-cdn 改为「设置项 `media_storage_plugin` 显式指定 → 市场清单 `storage_provider: true` 声明发现（running 校验，字典序稳定）→ 静态兜底 image-cdn」三级解析；每次上传即时解析（不走注册表缓存，切换设置即时生效；上传低频成本可控）；`InstalledPluginDTO` 透出 `storage_provider` 供后台设置页下拉候选。发帖插图（POST /api/v1/media → UploadMedia → seam）直达 TG，前端上传链路零改动。
> 日期：2026-09-03
> 参考：[x-dr/telegraph-Image](https://github.com/x-dr/telegraph-Image)（TG 渠道机制研究）、`marketplace-repo/image-cdn`（同架构先例：R2 图床 + Worker 反代）
> 目标：把 telegraph-Image 的 **TG Bot API 渠道**（`TG_BOT_TOKEN` + `TG_CHAT_ID` 模式）做成月言博客插件；不做 telegra.ph 渠道、不做 58img/tencent/r2 等其他渠道。

---

## 1. telegraph-Image 的 TG 渠道机制（源码研究结论）

telegraph-Image 部署在 Cloudflare Pages（Edge Functions），TG 渠道由两个路由构成：

### 1.1 上传（`src/app/api/enableauthapi/tgchannel/route.js`）

1. 前端 multipart POST（字段名 `file`）到服务端路由；
2. 服务端按 MIME 选 Telegram 端点与字段名：

   | 文件类型 | TG 端点 | multipart 字段 | 响应取值 |
   |---|---|---|---|
   | `image/*` | `sendPhoto` | `photo` | `result.photo[]` 取**最大尺寸**的 `file_id` |
   | `video/*` | `sendVideo` | `video` | `result.video.file_id` |
   | `audio/*` | `sendAudio` | `audio` | `result.audio.file_id` |
   | 其他（含 PDF） | `sendDocument` | `document` | `result.document.file_id` |

3. POST `https://api.telegram.org/bot<TOKEN>/<端点>`，multipart 携带 `chat_id` + 文件字段；
4. 返回给前端的 URL：`{站点域名}/api/cfile/{file_id}`。

### 1.2 访问（`src/app/api/cfile/[name]/route.js`）

- URL 键是 **file_id**（公开标识，泄露无风险；不是 file_path）；
- 流程：`GET /api/cfile/{file_id}` → 服务端 `getFile?file_id=` 解析 `file_path` → 服务端 fetch `https://api.telegram.org/file/bot<TOKEN>/{file_path}` → 命中 Cloudflare Cache 直接回字节，否则回源后写缓存；
- **Bot Token 只存在于服务端环境变量，浏览器永远拿不到**；`Content-Type` 按 `file_path` 扩展名推断。

### 1.3 限制与特性（插件方案需消化）

- Bot `getFile` 下载上限 **20MB**（上传文件本身可以更大，但经 file_id 下载受限）；
- `sendPhoto` 会被 Telegram 服务端**压缩重编码**（多尺寸），`sendDocument` 保原文件；
- `file_path` 是**临时链接**（有效期不保证），所以每次访问都要重新 `getFile`——缓存（CDN/Cache API）是性能关键；
- `api.telegram.org` 在中国大陆不可直连——服务器侧与访客侧的可达性要分开考虑。

---

## 2. 月言架构下的映射

结论：**月言插件进程承担 telegraph-Image「服务端」的角色，访客访问链路由站长自备的反代 Worker 承担**——与官方 `image-cdn` 插件（R2 + Worker）完全同构的架构。

```
发帖人（登录用户）                月言宿主                    插件进程(tg-image-bed)           Telegram
──────────────                 ─────────                   ────────────────────           ────────
后台图库页(壳路由)
  │ POST /api/v1/plugins/tg-image-bed/manage/upload
  ├──────────────────────────▶ 宿主代理(登录校验) ──────────▶ sendPhoto/sendDocument
  │                                                    ───────────────────────────────▶ (bot token,
  │                                                    ◀── result.photo[].file_id        chat_id)
  │ ◀── {url: "{worker}/f/{file_id}", markdown:...}
  │
  │ 复制 markdown → 粘贴进帖子正文（URL 为 Worker 公开地址）

访客浏览器                        站长部署的反代 Worker(Cloudflare)
──────────────                   ──────────────────────────────
<img src="{worker}/f/{file_id}">
  ├────────────────────────────▶ getFile?file_id= → file_path
  │                              fetch api.telegram.org/file/bot<TOKEN>/{file_path}
  ◀───── 图片字节（CF Cache 缓存；token 不出 Worker）──────────┘
```

**安全模型**（对齐 image-cdn / telegraph-Image）：

- Bot Token / Chat ID 存插件设置（宿主 settings 落库，经 `sdk.Config` 下发），只在插件进程内存中使用；
- 访客永远只见 `{worker}/f/{file_id}`，token 不出现在任何 HTML / 前端响应里；
- 上传/列表需登录（`CallerID > 0`），删除仅管理员（`TrustedCaller`），与 image-cdn 权限模型一致。

**为什么图片访问不走路由插件 API**：`/api/v1/plugins/{id}/**` 宿主代理强制登录，访客 `<img>` 匿名请求会 401（手册第 7 章「数据边界」）；宿主也没有面向插件的公开文件代理端点（音乐播放地址是宿主产品决策的 seam，图片无对应物）。自备反代是零宿主改动的唯一解，且有 image-cdn 先例背书。

---

## 3. 关键决策点（待拍板）

### 决策 A：插件形态——独立图库，还是接管发帖上传？

宿主的**媒体存储 seam**（`internal/plugin/seam_media.go`：`GET /storage/health` + `POST /storage/upload` → 返回公开 URL 落库）可以让图床插件接管发帖插图链路，**但其提供方 ID 在 `internal/service/plugin_seam.go:75` 硬编码为 `image-cdn`**：

```go
const mediaStorageProviderID = "image-cdn"
```

| 选项 | 说明 | 代价 |
|---|---|---|
| **A1 独立图库（推荐先做）** | 后台图库页上传 → 复制 Markdown → 粘贴正文；实现 `/storage/*` 契约端点备用但暂不接管 | 零宿主改动；发帖插图多一步「去图库复制」 |
| **A2 接管发帖上传** | 宿主把 seam 提供方从硬编码改为可配置（宿主设置选当前图床插件，或按市场清单声明发现；与 image-cdn 互斥二选一） | 需改宿主 `plugin_seam.go` + 设置项/后台 UI + 发版；体验最好（发帖上传直达 TG） |

建议：先按 A1 交付插件（含 `/storage/upload` 契约实现，方便 A2 落地时直接接管），A2 作为后续宿主增强单独立项。

### 决策 B：默认发送模式（sendDocument vs sendPhoto）

- `document`（原图保真，推荐默认）：`sendDocument` 存原文件，无 TG 压缩；符合「图床保真」预期；
- `photo`（TG 压缩）：`sendPhoto` 服务端重编码生成多尺寸，流量更省、加载更快，与 telegraph-Image 默认一致。

无论默认哪个，均做成插件设置项 `send_mode`（select），上传时按设置选端点。

### 决策 C：服务器到 api.telegram.org 的可达性

博客服务器若在中国大陆，插件进程无法直连 `api.telegram.org`。方案：设置项 `api_proxy`（可选 HTTP 代理地址，如 `http://127.0.0.1:7890`；留空直连）。访客侧访问的是 Cloudflare Worker 域名（海外），一般不受影响。

---

## 4. 设计稿

### 4.1 能力与清单声明

`yueyan-plugin.json`（包内 manifest，与 `Info()` 一致）：

```json
{
  "id": "tg-image-bed",
  "name": "TG图床",
  "version": "0.1.0",
  "author": { "name": "月言官方" },
  "description": "Telegram 频道图床：上传直达 TG（Bot API），自备反代 Worker 访问，后台图库管理与 Markdown 插图。",
  "sdk": ">=1.0.0",
  "capabilities": ["settings", "api", "admin.page"]
}
```

`plugin.json`（市场清单，节选关键字段）：

```json
{
  "id": "tg-image-bed",
  "name": "TG图床",
  "version": "0.1.0",
  "category": "enhancement",
  "price": 0,
  "official": true,
  "capabilities": ["settings", "api", "admin.page"],
  "repo_url": "https://github.com/roberts9012062/yueyan-plugins",
  "core_version": ">=0.1.0",
  "nav": { "href": "/admin/plugin-pages/tg-image-bed/library", "label": "TG图床", "icon": "media" },
  "settings_schema": [
    { "key": "tg_bot_token", "label": "Bot Token（@BotFather 创建，如 123456:AAxxx）", "type": "text", "default": "" },
    { "key": "tg_chat_id", "label": "频道/群 Chat ID（如 -1001234567890 或 @channel）", "type": "text", "default": "" },
    { "key": "proxy_base", "label": "反代 Worker 地址（如 https://img.example.com）", "type": "text", "default": "" },
    { "key": "send_mode", "label": "发送模式", "type": "select", "default": "document", "options": ["document", "photo"] },
    { "key": "api_proxy", "label": "TG API 代理（服务器在大陆时填，如 http://127.0.0.1:7890；留空直连）", "type": "text", "default": "" }
  ],
  "assets": { "pattern": "{id}-{version}-{os}-{arch}.bpk" }
}
```

能力只用三枚举：`settings`（配对信息）+ `api`（上传/列表/删除）+ `admin.page`（后台图库页）。不需要 hooks / frontend 槽位 / data.read / site.page / ai。

`frontend/manifest.json`：

```json
{
  "extensionPoints": [],
  "pages": [ { "route": "library", "entry": "library-page.js" } ]
}
```

### 4.2 插件 API 契约（全部 POST + JSON body，规避代理不含 query 的限制）

| 端点 | 权限 | 请求 | 响应 |
|---|---|---|---|
| `GET /storage/health` | 登录 | — | `{ok, error?}`——`getMe` 验 Token + `getChat` 验 Chat ID + proxy_base 非空 |
| `POST /manage/upload` | 登录 | `{filename, mime, content_b64}` | `{url, markdown, file_id, file_name, size, mime}` |
| `POST /manage/list` | 登录 | `{cursor}` | `{objects: [{file_id, file_name, url, markdown, size, mime, uploaded_at}], cursor}` |
| `POST /manage/delete` | 管理员 | `{file_ids: []}` | `{deleted}`——尽力调 `deleteMessage`（需历史记录存 message_id）+ 移除本地记录 |

- 上传历史持久化到插件数据目录 `data/plugins/tg-image-bed/history.json`（量级小，KISS，不引 SQLite）；
- `url = {proxy_base}/f/{file_id}`；`markdown = ![file_name](url)`。

### 4.3 目录结构（每文件 ≤300 行、每层 ≤8 项）

```
marketplace-repo/tg-image-bed/
├── main.go              # Info / 生命周期 / API 路由注册
├── telegram.go          # TG Bot API 客户端（sendDocument/sendPhoto/getFile/getMe/getChat/deleteMessage，支持代理）
├── history.go           # 上传历史 JSON 读写（纯函数 + 原子写）
├── yueyan-plugin.json
├── plugin.json
├── README.md
├── frontend/
│   ├── manifest.json
│   └── library-page.js  # 图库页（拖拽上传 / 网格预览 / 复制 Markdown / 删除；参考 image-cdn library-page）
└── worker/
    ├── index.js         # 反代 Worker 参考实现（~80 行）
    ├── wrangler.example.toml
    └── README.md
```

### 4.4 反代 Worker 设计（worker/index.js）

职责即 telegraph-Image 的 `cfile` 路由精简版：

```
GET /f/{file_id}   → getFile?file_id= → fetch https://api.telegram.org/file/bot<TOKEN>/{file_path}
                    → 流式返回（Content-Type 按 file_path 扩展名；Cache-Control: public, max-age=604800）
GET /health        → {"ok":true}（免鉴权，供插件配对探测与存活检查）
```

- 环境变量：`TG_BOT_TOKEN`（secret）；
- 缓存：`caches.default` 缓存字节响应（命中则跳过 getFile，对齐 telegraph-Image 做法）；
- 不做鉴黄 / D1 日志 / 黑白名单（YAGNI，需要时再加）；
- 部署：`wrangler deploy`（example.toml 改账号即可），无需 R2 绑定——比 image-cdn 的 Worker 更简单。

---

## 5. 风险与限制（README 需向用户说明）

| 项 | 说明 |
|---|---|
| 单文件下载 ≤20MB | Bot API `getFile` 限制；上传时插件侧前置校验并给出明确报错 |
| file_path 时效 | 每次访问重新 `getFile`（Worker 侧实现），CF 缓存兜底性能 |
| 服务器可达性 | 大陆服务器需配 `api_proxy`；访客访问 Worker（CF 海外节点）一般无碍 |
| Token 安全 | Token 仅存宿主 settings + 插件进程内存 + Worker secret，三处均不外泄；泄露处置 = BotFather revoke + 更新配置 |
| 与 image-cdn 关系 | 互不冲突可共存；均不接管宿主上传（A1 形态）；A2 落地时二选一 |

---

## 6. 实施步骤（拍板后执行）

1. Go 后端三文件（main/telegram/history）+ 三份清单/README；
2. 前端图库页（原生 ESM，参考 image-cdn library-page.js 与宿主 `/plugin-sdk/shared.js`）；
3. 反代 Worker 参考实现 + 部署 README；
4. `scripts/` 打包脚本对齐现有插件（build 脚本模式），`cmd/bp` 打 `.bpk`；
5. 本地 mock 验证：`http://127.0.0.1:7890` 代理可选；`GET /api/v1/plugins/tg-image-bed/storage/health` 冒烟。
