# 方案：书签「同步到站点」（本地 → 精品导航，双模式 + 进度条）

> 讨论稿 · 对应开发：插件 v0.27.0 批次（反向同步，与「同步站点导航」站点→本地互补）。

## 1. 需求（用户口径）

导入本地书签后可**反向同步到站点**：弹出全部书签与分类文件夹**多选**要同步的内容；点同步时二选一——
**直接同步**（不改变现有分类，按本地文件夹结构原样上传）或 **AI 自动整理同步**（按每个站点的内容补全说明、标签与分类）；AI 模式必须有**进度条**显示百分比。

## 2. 前提调研（结论）

| 结论 | 出处 |
|---|---|
| nav-links 插件（v1.3.7）API 面：`POST /links`（创建）、`/links/update`、`/links/delete`、`/links/reorder`、`/links/public`（公开读）、`/fetch-icon`、`/ai/suggest` | 插件 bpk 解包（plugin.bin 路由字符串） |
| Link 结构字段：`name/url/category/tags/description/icon/sort`（+id/created_at） | 同上（json tag 提取） |
| 宿主已有桥接模式：`PluginService.CallAPI` 以 `CallerIdentity{System:true}` 调插件端点（B站/TTS/统计/导航读同款） | `internal/handler/nav_bridge.go`、`video.go` |
| 插件引入 SDK `CallerSystem` 身份判定——宿主 System 桥接为 SDK 设计内路径 | plugin.bin 符号表 |

**开放网关现状无写入端点** → 需后端配套新增（与 posts.create / media.upload 同模式）。

## 3. 后端配套（navlinks.save，需重启后端生效）

| # | 改动 | 文件 |
|---|---|---|
| B1 | `OpenSave`（POST /api/v1/open/nav/links）：body `{links:[…]}`；先拉 `/links/public` 取站点已有 URL 集合 → 逐条转调插件 `POST /links` 创建（URL 已存在跳过、单条失败计数不中断）→ 返回 `{created, skipped, failed}` | `internal/handler/nav_bridge.go` |
| B2 | 开放组路由注册 | `internal/router/router.go` |
| B3 | 目录登记 `navlinks.save`（导航同步写入） | `internal/model/openapi.go` |
| B4 | Key 需勾选 `navlinks.save`；未勾选 403（插件端给出生成 Key 提示） | 鉴权机制既有 |

> 风险备忘：写入放行依赖插件端对 System caller 的判定（SDK 设计内）；若插件拒绝，需 nav-links 升级版放行或提供专用桥接端点——插件端收到 403 时明确提示。

## 4. 插件端设计

### 4.1 入口与流程（两步弹层）

书签「＋」菜单新增「⬆️ 同步到站点」→ `SyncToSiteSheet`：

```
步骤一（多选）：
  ☑ 全选（n 个文件夹 / m 条书签）
  ☑ 📚 站点导航 (3)      ← 文件夹为选择粒度，显示内含链接数
  ☑ ✨ AI 收藏 (5)
  ☑ 开发工具 (12)
  ☐ 未分类 (2)           ← 根级散链归入「未分类」
  [下一步]

步骤二（模式 + 执行）：
  ○ 直接同步 —— 保持现有分类，按本地文件夹名原样上传（快）
  ○ AI 自动整理 —— 按每个站点内容补全说明/标签/分类（逐条调 AI，慢）
  [开始同步]

  AI 模式执行中：进度条 ▓▓▓▓░░░░ 8/23（35%）+ 当前正在识别的站点名
```

### 4.2 两条模式的行为

| 项 | 直接同步 | AI 自动整理 |
|---|---|---|
| 分类 | 本地文件夹名（根级散链=「未分类」） | AI 识别（参考站点现有分类 + 本地分类名），空则回退本地文件夹名 |
| 名称/说明/标签 | 书签标题，说明/标签留空 | AI 识别补全（复用 ai-recognize.ts 三级容错解析；流式聚合）；名称识别为空回退书签标题 |
| 图标 | 带书签已有自定义 icon（无则不传，避免逐条转存拖慢批量） | 同左 |
| 上传 | 统一走 `navlinks.save`（批量体；每 20 条一批），站点已存在 URL 自动跳过 |

进度条覆盖「AI 整理 n/total」+「上传 n/total」两个阶段，同一进度条接力。

### 4.3 文件规划

```
bookmarks/sync-to-site.ts      # 执行引擎（纯编排）：选中链接 + 模式 + 进度回调 → AI 整理 + 分批上传
bookmarks/SyncToSiteSheet.tsx  # 两步弹层（多选 / 模式与进度）
shared/api/endpoints.ts        # saveNavLinks 封装
```

## 5. 验证

- [ ] 多选：文件夹勾选含子链接计数；全选/清空；下一步前校验非空
- [ ] 直接同步：本地分类原样上传、名称取书签标题；站点已有 URL 跳过
- [ ] AI 同步：逐条识别（说明/标签/分类补全），进度条按百分比推进；识别失败条目回退本地信息继续
- [ ] 上传分批推进、完成提示 created/skipped/failed 计数
- [ ] Key 未勾选 navlinks.save 的 403 提示；插件未启用 503 提示
- [ ] 双浏览器走手册 §12 清单
