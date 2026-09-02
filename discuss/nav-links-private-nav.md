# 方案：精品导航「私有导航」功能（开放/私有可见性 + 访问门禁）

> 讨论稿 · 对应插件版本：nav-links v1.3.13 → v1.3.14（宿主侧配套 3 条公开桥接路由）。

## 1. 需求（用户口径）

精品导航增加**私有导航**功能：

1. 私有导航栏可在**后台插件**里选择访问方式：**自己可访问**（仅站长）或**密码访问**；
2. 密码访问需要站长自己输入**访问密码**（访客凭密码解锁浏览）；
3. 添加地址（收藏站点）时默认是**开放**；
4. 选择**私有**的收藏导航进入**私有导航展示页面**（与公开导航页分离）。

## 2. 前提调研（结论）

| 结论 | 出处 |
|---|---|
| Link 模型字段 `name/url/category/tags/description/icon/sort`，无可见性字段 | `marketplace-repo/nav-links/store.go` |
| 前台公开页数据链路：页面 fetch `GET /api/v1/nav/links`（匿名）→ 宿主 System 身份调插件 `POST /links/public`（返回**全量**链接） | `internal/handler/nav_bridge.go`、`nav-page.js` |
| 插件代理 API 需登录；宿主桥接是访客访问插件的唯一公开通道 | `internal/router/router.go:181` |
| 管理员判定：宿主 `OptionalAuth` + `GetRole(c)`（admin/superadmin）；前台 token 在 `localStorage["yueyan-tokens"].access_token`（Bearer 头） | `internal/middleware/auth.go`、`frontend/src/lib/api-report.ts` |
| 插件数据目录 `data/plugins/nav-links/links.json`（JSON 原子写）；插件设置明文存宿主库（不适合放密码） | `store.go`、`PluginConfig` 机制 |
| 插件前端页面经 manifest.json `pages`（scope:site）注册，`siteNav` 挂前台导航入口 | `frontend/manifest.json` |

## 3. 总体设计

### 3.1 数据模型（向后兼容）

- `NavLink`/`LinkInput` 新增 `Visibility string`：空或 `"open"`=开放（旧数据默认），`"private"`=私有；
- 公开数据端点 `/links/public` **只返回开放条目**（分类/标签聚合同步只算开放条目）——防泄露的根本保障；
- 浏览器插件同步（`/links`、`/links/import`）不传 visibility 时默认开放；import upsert 时**空值不覆盖**已有可见性（避免同步把私有条目刷回开放）。

### 3.2 私有访问配置（插件数据目录 private.json）

```
{ mode, password_hash, salt, secret, title, subtitle }
```

- `mode`：`self`（默认，仅自己可见）| `password`（密码访问）；
- `password` 仅存 **SHA-256(salt + password)** 哈希，绝不明文（故不放宿主插件设置——那是明文库）；
- `secret`：32 字节随机数（首次生成），用于解锁 token 签名；**修改密码即轮换**，旧 token 全部失效。

### 3.3 解锁 token（密码模式）

- 签发：`token = "{expUnix}." + hex(HMAC-SHA256(secret, "unlock:" + expUnix))`，有效期 7 天；
- 验证：exp 未过期 + HMAC 匹配；访客存 `localStorage["nav-links-private-token"]`；
- 宿主透传：访客请求带 `X-Nav-Token` 头 → 宿主组装进桥接 body。

### 3.4 鉴权矩阵（私有数据端点）

| 请求者 | self 模式 | password 模式 |
|---|---|---|
| 管理员登录态（Bearer → OptionalAuth → admin/superadmin） | ✅ 放行 | ✅ 放行 |
| 有效解锁 token | ❌ 403 self_only | ✅ 放行 |
| 其他访客 | ❌ 403 self_only | ❌ 401 need_password |

self 模式下 unlock 端点直接 403（不给密码解锁的机会）。

## 4. 插件端改动（marketplace-repo/nav-links/）

| # | 文件 | 改动 |
|---|---|---|
| P1 | `store.go` | Visibility 字段 + 归一校验；`Add/Update/ImportLinks` 写入（import 空值保留）；新增 `ListPublic()` 只出开放条目 |
| P2 | `private.go`（新，~290 行） | `PrivateStore`（private.json 存取、哈希/盐/secret）；token 签发与验证；API：`GET/POST /private/config`（TrustedCaller，管理页配置）、`POST /private/meta`（公开元数据：模式/是否已设密码/私有页标题/私有条数）、`POST /private/unlock`（密码→token）、`POST /private/links`（鉴权矩阵→私有数据，聚合分类标签） |
| P3 | `main.go` | `/links/public` 过滤开放条目 + 聚合；注册 `registerPrivateAPI`；版本 1.3.14、描述与 Settings 补充 |
| P4 | `frontend/board.js`（新） | 从 nav-page.js 抽取的通用看板（搜索/分类/标签云/双视图），数据注入式渲染——公开页与私有页共用（DRY） |
| P5 | `frontend/nav-page.js` | 改薄入口：拉公开数据 → board |
| P6 | `frontend/private-page.js`（新） | 门禁流程：拉 meta → 尝试取数（Bearer/X-Nav-Token）→ self 403 显示「仅站长可见」/ password 401 显示密码解锁卡 → 解锁成功进看板 |
| P7 | `frontend/link-form.js` | 「可见性」单选：开放（默认）/ 私有 |
| P8 | `frontend/private-settings.js`（新） | 管理页「私有设置」卡片弹层：模式单选 + 访问密码（留空不改）+ 私有页标题 |
| P9 | `frontend/admin-page.js` | 列表行「私有」徽标 + 工具栏「私有设置」按钮 |
| P10 | `frontend/manifest.json` | pages 注册 `private`（site）；siteNav 挂「私有导航」入口 |
| P11 | `yueyan-plugin.json` / `plugin.json` / `README.md` | 版本 1.3.14、描述、构建后回填 assets 哈希、README 功能说明 |

## 5. 宿主端改动（internal/）

| # | 文件 | 改动 |
|---|---|---|
| H1 | `internal/handler/nav_bridge.go` | 新增 `PrivateMeta`（GET，直通插件 JSON）、`PrivateUnlock`（POST {password} 透传，401 透传）、`PrivateLinks`（GET，OptionalAuth 判定 admin → body {admin:true}；否则 X-Nav-Token → body {token}；**响应 no-store**）；`navSaveLink` 增加 visibility 透传（浏览器插件同步预留） |
| H2 | `internal/router/router.go` | 公开组注册：`GET /nav/private/meta`、`POST /nav/private/unlock`、`GET /nav/private/links`（挂 OptionalAuth） |

## 6. 交互流（前台私有页）

```
访客打开 /plugins/nav-links/private
  → GET /api/v1/nav/private/meta           {mode, has_password, count, title}
  → GET /api/v1/nav/private/links          （带 Bearer（如有）+ X-Nav-Token（如有））
      ├─ 200 → 渲染私有看板（与公开页同款交互）
      ├─ 403 self_only → 「🔒 此导航仅站长可见」占位（附登录提示）
      └─ 401 need_password / token 失效 → 密码解锁卡
            → POST /api/v1/nav/private/unlock {password}
                 ├─ 200 {token} → 存 localStorage → 重取数据
                 └─ 401 → 「访问密码不正确」
```

## 7. 验证

- [ ] `go build` 宿主与插件双平台零错误（build-nav-links-bpk.sh）
- [ ] node --check 全部插件前端 JS
- [ ] 旧数据（无 visibility 字段）加载后全部按开放展示，公开页内容不回归
- [ ] 添加地址默认开放；选私有后：公开页消失、私有页出现
- [ ] self 模式：未登录/非管理员 403；管理员登录直出数据
- [ ] password 模式：错误密码 401；正确密码解锁 7 天 token；改密码后旧 token 失效
- [ ] 密码不以明文出现在 private.json / 任何响应
- [ ] 浏览器插件同步（navlinks.save/list）行为不回归（同步默认开放）

## 8. 补记：私有能力对外开放网关（浏览器插件对接）

> 发布后追加：用户要求浏览器插件凭 X-Api-Key 对接私有导航读/写与访问设置。

### 8.1 设计

- **插件端零改动**：`/private/links`（body `{admin:true}` 直通）与 `/private/config`（TrustedCaller 认 System 桥接）已满足网关调用条件，nav-links 无需发版。
- **信任链**：开放 Key 由站长生成并逐条目勾选授权（ApiKeyAuth 按「Method+路由模板」反查目录），命中即视为站长授权——宿主以 System 身份、管理员语义调插件（与 `navlinks.save` 同模型）。
- **业务错误转译**：插件惯例 `200 + {"error":...}` 在宿主 `callPluginJSON` 统一转网关 400（`code≠0`），浏览器插件按 code 判定不误判成功。

### 8.2 新增端点与目录（internal/handler/nav_bridge.go + internal/model/openapi.go + router.go）

| 接口标识 | 方法 | 路径 | 行为 |
|---|---|---|---|
| `navlinks.private.list` | GET | `/api/v1/open/nav/private/links` | 私有条目数据（响应与 `navlinks.list` 同构） |
| `navlinks.private.config` | GET | `/api/v1/open/nav/private/config` | 读 `{mode, has_password, title, subtitle, count}`（无密码材料） |
| `navlinks.private.save` | POST | `/api/v1/open/nav/private/config` | 改 `{mode, password?, title?, subtitle?}`（password 留空=不改；改后前台旧解锁失效） |
| （已有）`navlinks.save` | POST | `/api/v1/open/nav/links` | 链接级 `visibility: "private"` 透传已可用（v1.3.14 预留字段） |

### 8.3 验证（本机全链路，临时 Key 已清理）

- [x] 无 Key 401；Key 未勾选对应条目 403（目录反查生效）
- [x] 读设置 → 切 password 模式 + 设密码 → `has_password:true`；短密码 → 400 中文错误
- [x] `navlinks.save` 写 `visibility:"private"` 一条 + 开放一条 → 私有列表恰含私有条目、公开列表恰含开放条目（UTF-8 无损）
