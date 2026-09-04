# 方案：AI 助手「发布图床」设置（TG 图床 / CF 图床）

> 状态：已评审待实施（随插件 v0.30.0 落地）
> 关联：`discuss/browser-extension-moment-tg-imagebed.md`（写说说 TG 通道，先例）、`marketplace-repo/image-cdn`（CF 图床插件）、`marketplace-repo/tg-image-bed`（TG 图床插件）

## 1. 需求

AI 助手（浏览器插件）设置中新增「发布图床」设置：

| 选择 | 发布文章时正文图片的存储 |
|---|---|
| 不选（默认） | 直接保存在站点服务器（现状行为不变） |
| TG图床 | 发布时图片上传保存在 TG 图床 |
| CF图床 | 发布时图片上传保存在 CF 图床（Cloudflare R2） |

范围边界：仅作用于 **AI 面板「生成文章」的发布链路**（`ArticlePanel`）；写说说已有自己的图片通道选择 UI（0.29.0），不联动、保持现状。

## 2. 现状与关键事实

### 2.1 发布文章的图片现状（ArticlePanel）

1. **AI 配图**：`aiAssist('image')` 服务端生图直接落站点媒体库，返回 `/media/...` 地址，`media_id` 收集后随发布 `media_ids` 关联。
2. **外链图**（网页总结携带的原文图片）：润色后以原始 URL 混入正文，发布前 `media.transfer` 逐张转存站点媒体库（src 替换为本站地址、media_id 并入；单张失败保留原址计数提示，不阻断发布）。

### 2.2 两类图床的插件端可达性（决定了实现方式不对称）

| 图床 | 站点端插件 | 插件端上传路径 | 额外配置 |
|---|---|---|---|
| TG图床 | `tg-image-bed`，声明 `open_endpoints: upload` | 开放网关泛化转发 `/api/v1/open/plugins/tg-image-bed/upload`（站点 Key 鉴权，站点对插件源回显 CORS） | 无（复用站点连接） |
| CF图床 | `image-cdn`，**未声明 open_endpoints**（`grep` 证实），开放网关无法转发 | 直连用户自部署的 CF Worker：`POST {workers_url}/upload`，`Authorization: Bearer API_KEY`，multipart `file` → `{url,key,size,mime}`；白名单 jpg/jpeg/png/gif/webp、≤10MB | 需填 Workers URL + API Key（站点管理员在 CF图床插件设置里可查到同一对值） |

CF Worker 参考实现（`marketplace-repo/image-cdn/worker/index.js`）**不回 CORS 头、不处理 OPTIONS**；扩展页面（`chrome-extension://` origin）直连会被 CORS 拦截，必须持有目标主机的主机权限（MV3 中 `host_permissions` 覆盖的 origin 不受 CORS 约束）。

另注：站点端 `image-cdn` 启用后经 `media.storage` seam **全站接管**媒体上传（`internal/service/plugin_seam.go`，站点管理员全局决策）。此时「不选图床」的服务器通道实际可能仍被站点重定向进 R2——这是站点行为，插件不干预也不规避；插件侧设置只决定**发布链路把图片发往哪里**。

## 3. 设计

### 3.1 设置模型（`plugin_settings_v1` 原地扩展，键不变）

```typescript
/** 发布图床（文章发布时正文图片的存储通道） */
export type PublishImageBed = 'none' | 'tg' | 'cf';

export interface PluginSettings {
  // …既有字段…
  /** 文章发布图床：none=站点服务器 / tg=TG图床 / cf=CF图床R2 */
  publishImageBed: PublishImageBed;
  /** CF 图床 Workers 地址（如 https://imgs.example.com，publishImageBed==='cf' 时使用） */
  cfBedUrl: string;
  /** CF 图床 API Key（Worker 部署时 wrangler secret put API_KEY 的值） */
  cfBedKey: string;
}
```

`readSettings` 读取时白名单归一化（非法值回退 `none`）、`cfBedUrl` 复用 `normalizeBaseUrl`。

### 3.2 API 层（`shared/api/image-bed.ts` 扩展）

- `uploadCfImageBed(workerUrl, apiKey, file)`：multipart 直传 Worker `/upload`，60s 超时，返回 `{url,key,size,mime}`；非 2xx 时透传 Worker `error` 字段抛 `ApiError`。
- `checkCfImageBedAvailable(workerUrl, apiKey)`：`GET /health`，`{ok:true}` 即可用（零副作用配对测试）。
- `client.ts` 的 `fetchWithTimeout` 由模块私有改为导出（CF 请求是 Worker 原生 JSON 协议、非开放网关信封，复用超时与网络错误文案，避免重复造轮子）。

### 3.3 主机权限（`shared/permissions.ts` 新公共模块）

- `ensureWideHostPermission(): Promise<boolean>`：`{origins:['http://*/*','https://*/*']}` contains→request，复用网页总结/截图的既有授权语义（同一授权三处共用：网页总结、截图、图床直连）。
- `AiChatTab` 私有实现迁移到该模块（DRY）。

### 3.4 设置 UI（新分区，新子目录 `components/settings/`）

`settings/ImageBedSection.tsx`（components 根级已满 8 文件，新分区落子目录）：

- 三张单选卡：**站点服务器**（默认，说明「图片保存站点服务器」）/ **TG图床**（说明「经站点开放网关直传，需站点启用 TG图床插件」）/ **CF图床**（说明「直连 Cloudflare R2 Worker」）。
- 点选即保存（与面板内其他开关「即改即存」一致）；选 TG 时探测 `tg-image-bed` 可用性展示状态行（复用 `checkTgImageBedAvailable`）。
- 选 CF 展开 Workers URL / API Key 输入框 + 「保存并连接」按钮：click 手势内先 `ensureWideHostPermission()`（Worker 无 CORS，必须持主机权限）→ 保存 → `/health` 探测回显结果。
- `SettingsPanel` 新增「发布图床」分区挂载；`App` 增加具名回调 `onSaveImageBed(config: ImageBedConfig)` 持久化。

### 3.5 发布路由（新模块 `ai/publish-image-router.ts`）

ArticlePanel 已 528 行（超 300 行硬指标），新增逻辑一律外置：

```typescript
export interface ArticleImageRouteResult {
  html: string;        // 路由后正文（src 已替换）
  mediaIds: number[];  // 需随发布关联的站点媒体库 ID（图床模式恒为空）
  failed: number;      // 转存失败张数
  failMsg: string;     // 首条失败原因（聚合计数提示用）
}

export async function routeArticleImages(
  sourceHtml: string,
  settings: PluginSettings,
  onProgress: (text: string) => void,
): Promise<ArticleImageRouteResult>
```

- `none`：迁入现有 `transferExternalImages` 逻辑（外链 `media.transfer` 转存，本站地址跳过，失败保留原址计数）。
- `tg` / `cf`：正文 `<img>` 逐张处理——
  - `http(s)` 图（含本站 AI 配图）：fetch 为 Blob → 构造带白名单扩展名的 File（mime 映射扩展名，URL 文件名优先；`image/jpeg→.jpg` 等）→ 上传所选图床 → `src` 替换为图床公开 URL；
  - `data:image/...` 图（粘贴图）：同路径统一处理（`fetch` dataURL 无 CORS 问题）；
  - 其余（相对路径）跳过保留；
  - 存在 http 图时先 `ensureWideHostPermission()`（拒绝则全部按失败计数保留原址，走统一失败提示，不阻断发布）；
  - `mediaIds` 恒空（图床文件不属站点媒体库，与说说 TG 通道 `mediaId=null` 同语义）；AI 配图的 `media_id` 在图床模式下不再收集（ArticlePanel 判 `publishImageBed==='none'` 才收集）。
- 失败策略与现状一致：单张失败保留原地址、计数提示、不阻断发布。

## 4. 双浏览器与发版

- Manifest `0.29.0 → 0.30.0`（新功能提次版本）；CHANGELOG 登记。
- 权限无新增 manifest 声明（仍走 `optional_host_permissions` 运行时申请），CHANGELOG 说明主机授权新增使用场景。
- 手册 §8.2 存储键登记 `plugin_settings_v1` 行补充发布图床字段；手册变更记录升 v1.2。
- 构建验证：`scripts/build-browser-extension.sh`；双浏览器按手册第 12 章清单走查（设置分区渲染/保存、TG 探测、CF 配对探测、三通道发布）。

## 5. 风险与备忘

1. **CF 配置不对称**：TG 零配置、CF 需手填 Worker 对——由 CF 图床「用户自部署 Worker」的架构决定，不做站点侧代理（站点未开放该插件 open endpoint，改动站点超出插件仓库范围）。
2. **AI 配图双落盘**：图床模式下服务端生图必然先落站点（服务端固有行为），发布时再转存图床——正文最终引用图床 URL，站点仅多一份源图，可接受。
3. **站点 seam 接管与「不选」语义**：见 §2.2 末注，插件不强改站点行为，文档向用户说明即可。
4. **`ai/` 目录文件数超标**（历史遗留 `_patch*.py` 临时文件）：本次新增 `publish-image-router.ts` 加剧该问题，后续应清理临时文件并按域拆分子目录（另行处理，不在本方案扩散）。
