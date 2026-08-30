# 方案：boke 浏览器插件 v1（侧边栏形态）

> 讨论稿 · 对应开发：月言浏览器插件第一版。
> 开发完成后本方案的定论应回填至 `docs/browser-extension-guide.md`。

## 1. 需求还原（用户口径）

1. 点击工具栏图标 → 打开**侧边栏**面板，布局参照 provided 截图（顶栏头像行 + Tab 栏 + 内容区问候/功能卡片 + 底部输入区）。
2. 初始为**未登录**状态；登录对接 boke：填写「站点 URL + API Key」完成连接。
3. 有**接口 Key 配置页**概念：被勾选的开放接口即可远程授权调用的接口（主站后台已有生成入口，见下文）；插件内展示这些可调用接口的目录。
4. 左上角头像 = 对应站点登录身份；点击头像出现 URL / Key 配置，保存后拉取用户信息（昵称、头像、计数）。

## 2. 现状结论（调研）

| 结论 | 出处 |
|---|---|
| `/api/v1/open/*` 已有 12 个接口，统一 `X-Api-Key` 头鉴权 | `internal/router/router.go:244-257` |
| Key 绑定接口范围（endpoints），但**不绑定用户**，匿名视角 | `internal/middleware/apikey.go` |
| 没有「凭 key 查用户」的端点 | — |
| CORS 未放行 `X-Api-Key` 头与扩展来源，浏览器直连会被预检拦截 | `internal/middleware/cors.go:24` |
| 主站 admin 已有完整「开放接口目录 + 生成 Key」页面（用户第二张截图） | `frontend/src/components/admin/open-api/*` |
| Edge **不支持** `chrome.sidePanel` API；Chrome 114+ 支持 | 双浏览器硬性约束决定必须降级方案 |

## 3. 后端改造（小步扩展）

| # | 改动 | 文件 |
|---|---|---|
| B1 | 迁移 021：`open_api_keys` 增加 `user_id BIGINT NULL REFERENCES users(id)`（旧 Key 为空=未绑定） | `db/migrations/021_open_api_key_user.sql` |
| B2 | 目录新增条目 `me.profile`：GET `/api/v1/open/me`（凭 Key 返回绑定用户的公开资料） | `internal/model/openapi.go` |
| B3 | `ApiKeyAuth` 放行时把 `record.UserID` 写入 gin context；新增 `GetAPIKeyUserID()` 读取函数 | `internal/middleware/apikey.go` |
| B4 | 新增 handler `Me`：无绑定返回 403 明确文案；有绑定走 `AuthService.GetProfile(uid, self=false)` | `internal/handler/openapi_me.go` |
| B5 | 生成 Key 自动绑定当前管理员（admin JWT 身份），无需前端改表单 | `service.CreateKey` 加 ownerUserID 参数 |
| B6 | CORS：Allow-Headers 追加 `X-Api-Key`；`/api/v1/open/` 前缀路径回显任意 Origin（Key 本身即凭证，面向外部应用开放是产品语义） | `internal/middleware/cors.go` |

依赖注入链更新：`server.go:307` 构造 OpenAPIHandler 追加 `authSvc`（167 行已先于构造点创建，顺序安全）。

## 4. 插件端设计

### 4.1 打开方式（双浏览器）

- manifest 不设 default_popup；注册 `action.onClicked`：
  - Chrome ≥114：顶层 `chrome.sidePanel.setPanelBehavior({openPanelOnActionClick:true})`，点击原生打开侧边栏；
  - Edge / 旧版：事件回调中 `window.open(chrome.runtime.getURL('sidepanel.html'), '_blank', 'width=420,height=780')` 打开同尺寸独立小窗。
- 同一份 `sidepanel.html` 服务两种宿主形态。

### 4.2 连接流程（未登录 → 登录）

```
输入站点 URL + API Key（默认预置本站地址）
  → 解析 origin
  → GET {origin}/api/v1/open/meta      （校验连通性，拿站点名/描述）
  → GET {origin}/api/v1/open/me        （校验 Key 与用户绑定，拿昵称/头像/计数）
     403 且文案含"未授权此接口" → 提示重新生成 Key 并勾选"我的资料"
     401 → Key 无效或过期
  → 成功：写 chrome.storage.local，进入已登录视图
```

### 4.3 视图结构（对照参考图一）

```
┌──────────────────────────────────┐
│ (头像) 昵称    [刷新][主题][设置] │ ← HeaderBar；头像/设置点击弹出连接管理层
├──────────────────────────────────┤
│ [首页] [AI 助手] [接口中心]      │ ← Tabs
├──────────────────────────────────┤
│ 👋 你好，{昵称}                  │
│ 我是月言站点助手                  │   首页：问候 + 功能卡片网格 + 最新帖子预览
│ [帖子流][搜索][话题]…卡片        │
│ 最新帖子列表…                    │
├──────────────────────────────────┤
│ 输入框（回车切 AI 助手发送）     │
└──────────────────────────────────┘
AI 助手：模型选择(/open/ai/models) + 消息流(/open/ai/chat，非流式 JSON)
接口中心：站点信息卡 + 目录清单(12 条只读；说明授权语义) 
```

主题：直接复用主站 `tokens.css` 的 `--yy-*` 令牌与 `data-theme="cool-moon"|"mist"` 切换，观感一致；未连接时整屏替换为欢迎+连接表单。

### 4.4 技术落点

- Vite 多入口手动构建（不用 crxjs：维护风险大；manifest.json 由脚本从源码拷贝校验）。
- React 19 + Tailwind v4（`@tailwindcss/vite`），TypeScript strict，全 ESM。
- 网络请求由 sidepanel/options 直连（扩展页面自身 origin 安全，fetch 走服务端 CORS），**零 host_permissions**——权限最小化（存储权限仅 `storage`）。这与手册原第 7.2 条不同，理由见 §5，将同步修订手册。
- background 仅承担“点图标开面板”一个职责。

## 5. 对手册的修订（开发完成后执行）

1. 差异备忘表补充一行：Edge 无 `chrome.sidePanel` → 降级独立窗口打开同一页面。
2. 第 7.2 网络请求细则改为：「内容脚本禁止直连，须经 background；扩展页面（sidepanel/options）经服务端 CORS 授权后可直连 boke API」。
3. 依赖管理 npm 对齐 frontend（package-lock.json），原 pnpm 表述修正。
4. options 页要求放宽：v1 以侧边栏内置「连接管理弹层」替代独立 options 页（保留 options_page 字段指向 sidepanel 同构页面亦可——v1 直接复用 sidepanel 页面）。

## 6. 交付物清单

- 后端：迁移 + 6 文件改动，`go build` 通过。
- 插件工程：`browser-extension/` 完整源码 + 三脚本（setup/dev/build），产物 `dist/browser-extension/`。
- 文档：手册修订 + CHANGELOG v0.1.0。

## 7. 风险与边界（v1 不做）

- 不做：内容脚本注入、右键菜单、OAuth 登录、发帖写入类接口（开放目录均为只读 + AI）、多站点账号并存（单站点配置）。
- 风险：自建站 http 内网源 fetch 混合内容限制——扩展页面不受页面 CSP 影响，仅受 manifest 主机权限影响；CORS 回显任意 Origin 的开放网关安全性依赖 Key 本身的机密性（与既有的外部脚本调用形态一致，不引入新暴露面）。
