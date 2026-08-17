# 方案评审:插件注册前台页面 + 头部导航项(站点扩展点)

> 状态:方案设计(待评审)
> 背景:宿主已有「自定义页面」(/pages/{slug})与「头部导航自定义」(settings.nav_links)。
> 需求:让插件也能声明自己的**前台公开页面**与**头部导航项**,新插件可一键生成前端页面。

---

## 一、现状结论

**目前插件不能注册前台页面,也不能注册头部导航项。** 现有插件前端扩展能力与目标对比:

| 能力 | 现状 | 差距 |
|---|---|---|
| 后台独立页面 | ✅ `admin.page` 能力 + manifest `pages[]` + 壳路由 `/admin/plugin-pages/{id}/{route}`(管理员登录可见) | 无公开版本 |
| 后台侧栏导航 | ✅ 市场清单 `nav {href,label,icon}` → admin 侧栏「插件」子菜单 | 仅后台 |
| 前台槽位注入 | ✅ `theme.header` 等 6 个槽位(PluginSlot;theme.header 在顶栏**右侧**图标区,不在导航链接区) | 非声明式导航项,不参与排序/二级菜单 |
| 帖子内容块 | ✅ `blocks[]` 注册表(data-plugin-block 分发) | — |
| **前台公开页面** | ❌ 无任何前台壳路由;`/pages/{slug}` 纯管理员手工(CRUD 接口在 admin 域) | 本方案新增 |
| **前台头部导航项** | ❌ 导航数据源唯一(settings.nav_links),无插件合并通道 | 本方案新增 |

复用度高:资产服务 `/plugin-assets/{id}/*`(running 门控)、`fetchManifest`/`loadModule`/`mountSandbox`(iframe 沙箱 + postMessage API 代理)、公开接口 `GET /api/v1/plugin-extensions`(running + 有前端资产筛选,前端 30 秒缓存)均已存在,均可在前台复用。

## 二、设计方案

### 1. 插件前台页面(site pages)

- **声明**:前端资产清单 `frontend/manifest.json` 现有 `pages: [{route, entry, sandbox?}]` 增加可选字段
  `scope: "admin" | "site"`(默认 `admin`,存量插件零影响):
  ```json
  { "pages": [
      { "route": "login", "entry": "login-page.js" },
      { "route": "radio", "entry": "radio-page.html", "sandbox": true, "scope": "site" }
  ] }
  ```
- **路由**:新增公开壳路由 `/plugins/{pluginId}/{page}`(client 组件)。
  - 渲染机制照抄 admin 壳:scope=site 页面 → sandbox 页用 `mountSandbox`(无登录态时 init 不发 token),ESM 页用 `loadModule` + 受限 `makePluginApi`
  - running 校验走公开接口(见 §3),非 running 显示「插件未启用」占位
- **能力**:新增 `site.page`(`plugin_capability.go` 常量 + knownSet + 安装双侧校验 + 文档第 9 章);
  运行时不做硬门控(与 frontend/admin.page 同为声明性能力,资产服务已按 running 门控)。
- **API 通道**:公开页面内宿主 API 走插件已有公开端点模式(如音乐插件的 `/music/{provider}/url`),
  登录用户经沙箱代理 `/api/plugins/{id}/**` 不变;不新开匿名插件代理(安全边界不扩大)。
- **冲突规避**:`/plugins/...` 前缀与 `/pages/{slug}`(自定义页面)、`/p/:alias`(URL 别名)、
  `/plugin-pages/...`(后台)均不冲突;Next 静态路由优先。

### 2. 插件头部导航项(site nav)

- **声明**:同一份 `frontend/manifest.json` 顶层新增:
  ```json
  { "siteNav": [ { "label": "电台", "path": "/plugins/netease-music/radio", "icon": "music" } ] }
  ```
  校验:`path` 必须以 `/` 开头(复用 nav_links 的协议白名单思路,拒绝 `javascript:` 等);
  `label` ≤30 字符;每插件 ≤5 项。
- **数据流**:扩展公开接口 `GET /api/v1/plugin-extensions` 响应,由
  `[{plugin_id, name}]` → `[{plugin_id, name, site_nav: [...], site_pages: [{route}]}]`
  (plugin_runner.FrontendExtensions 读各 running 插件 manifest 补充;manifest 由前端逐插件拉,
  也可由后端解包目录直接读 JSON —— 建议后端读,一次返回,前端免多次往返)。
- **渲染合并**:`desktop-nav.tsx` 在 nav_links(管理员配置)之后追加插件导航项
  (icon 可选渲染;不支持二级,保持简单);插件停用后 ≤30 秒自动消失(现有缓存节奏)。
- **管理端可见性**:后台「头部导航」页底部只读展示「插件注册的导航项」列表(灰字,
  提示由插件声明、不可在此编辑),避免管理员困惑。

### 3. 前台壳路由的直链防护

admin 壳用 `apiInstalledPlugins`(管理员接口)防直链;前台壳改用公开的
`plugin-extensions`(含 site_pages 路由表)校验 `route` 声明存在且插件 running,
未声明/未启用的 route 显示占位页(不暴露资产)。

## 三、改动清单(预估)

| 层 | 文件 | 改动 |
|---|---|---|
| 后端 | internal/service/plugin_capability.go | `SitePage` 能力常量 + knownSet |
| 后端 | internal/service/plugin_runner.go | FrontendExtensions 响应扩展(site_nav/site_pages,后端读解包 manifest) |
| 前端 | frontend/src/app/plugins/[pluginId]/[page]/page.tsx | 新增公开壳路由(照抄 admin 壳,鉴权换公开接口) |
| 前端 | frontend/src/components/desktop-nav.tsx | 合并插件导航项(useSiteMeta 之外并行拉 plugin-extensions) |
| 前端 | frontend/src/app/admin/nav/page.tsx(nav-menu-manager) | 底部「插件注册导航项」只读展示 |
| 前端 | frontend/src/plugins/loader.ts | PluginManifest 类型加 scope/siteNav |
| 文档 | docs/plugin-development.md | 第 7/9 章补 site 页面与 siteNav 声明、示例 |

工作量预估:一天以内(机制全部复用,无新协议)。

## 四、备选与否决项

- ❌ **让插件写 custom_pages 表**(把生成页面塞进 /pages/{slug}):否决 —— 管理员内容与插件代码
  混存一表,插件卸载残留/内容归属/更新覆盖都会打架;插件页面应由插件资产承载,生命周期随插件。
- ❌ **新开 `theme.nav` 槽位**(DOM 注入式):改动最小,但非声明式、不参与导航统一样式/无障碍,
  与 nav_links 管理割裂;仅当追求最小实现时备选。
- ⚠️ 公开匿名插件 API 代理(让访客直接调插件后端):安全边界扩大,暂不开放,
  公开页所需数据一律走宿主公开端点或登录后的沙箱代理。

## 五、安全要点

- 插件页面资产仅 running 状态可访问(既有 PublicAssetDir 门控),scope=site 不改变这一点
- 导航 path 白名单(仅 `/` 开头站内路径),防协议注入头部
- 沙箱页面维持 iframe 隔离 + origin 校验 + 短期令牌(登录态),公开访客无令牌(插件公开数据走宿主端点)
