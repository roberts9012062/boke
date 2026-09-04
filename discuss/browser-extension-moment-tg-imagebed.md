# 插件「写说说」插入图片支持 TG 图床通道（方案）

> 状态：已实现（2026-09-04）
> 关联：`discuss/tg-image-bed-plugin.md`（TG图床插件）、`browser-extension-guide.md`（插件手册）

## 1. 需求

写说说的「插入图片」按钮：若站点已装并启用「TG图床」插件（且当前 API Key 有其接口权限），点击时弹出「上传服务器 / 上传 TG 图床」双选项由用户选择；不可用时保持现状（直接弹文件选择器走服务器上传）。

## 2. 接口事实（已核对源码）

| 项 | 内容 |
|---|---|
| 上传端点 | `POST /api/v1/open/plugins/tg-image-bed/upload`（插件清单 `open_endpoints` 声明，开放网关泛化转发） |
| 请求体 | `{filename, mime, content_b64}`（单图 ≤ 20MB，jpg/jpeg/png/gif/webp，**原图保真**） |
| 响应 | 网关信封 `{code,message,data}`，data = `{type, storage_key, url, mime, size, markdown, mode}` |
| 不可用形态 | 404=站点未装/后端过旧；503=插件未启用或不可达；403=Key 未勾选该端点；400=插件业务错误（含参数校验失败） |

**可用性探测**：对 upload 端点发空 `{}` body——ApiKeyAuth 中间件先于插件执行，故 HTTP 400（插件参数校验拦下）即证明「插件在 + Key 已勾选」→ 可用；403/404/503/断网 → 不可用。零副作用（未到上传逻辑），相比 `tg-image-bed.list` 探测（60 条响应、且 list/upload 双端点授权可能不一致）更精确。

**关键约束**：浏览器要求 `fileInput.click()` 必须在用户手势的同步调用栈内，故**探测不能放在点击后再 await**——改为组件挂载时探测一次缓存进 state，点击时同步决策（连接断开后重开面板即刷新；断网误判时安全降级为服务器通道）。

## 3. 实现

1. **类型**（`shared/types/index.ts`）：`MomentAttach` 图片分支加 `source: 'server' | 'tg'`；`mediaId` 放宽为 `number | null`（TG 图无媒体库 ID，仅正文引用）。
2. **API**（新文件 `shared/api/image-bed.ts`，endpoints.ts 已 377 行超行数上限不再增长）：`uploadTgImageBed()`（FileReader 转 base64 直传，超时 120s）与 `checkTgImageBedAvailable()`。
3. **弹层**（`InsertSheets.tsx`）：新增 `ImageSheet`（复用 SheetShell）——「上传到服务器（大图自动压缩）」/「上传到 TG 图床（原图保真）」两选项。
4. **发布器**（`MomentComposer.tsx`）：
   - 挂载时探测 TG 可用性 → state；
   - 点图片按钮：可用 → 弹 `ImageSheet`；不可用 → 直弹文件选择器（现状不变）；
   - 双隐藏 `<input>` 分别对应两通道；**粘贴**保持服务器通道（惯性操作不打断）；
   - 上传循环抽公共高阶函数（注入单图上传），服务器通道压缩、TG 通道直传原图；
   - 发布时 `media_ids` 仅收集 `source === 'server'` 的图，TG 图走正文 `<img src>`（前台 DOMPurify 默认放行外链图，已核对 `frontend/src/lib/sanitize.ts`）。

## 4. 边界

- TG 通道不压缩（保真，与主站发帖 `storage_raw_upload` 语义一致）；超 20MB 由插件端报错透传。
- mime 白名单（jpg/jpeg/png/gif/webp）由插件端校验，前端不预检。
- Key 未勾选 TG 接口（403）归入不可用，静默走服务器通道；TG 上传失败的 403 错误信息已有「检查 Key 勾选」指引。
- 发版：manifest `0.28.0 → 0.29.0`，CHANGELOG 登记。
