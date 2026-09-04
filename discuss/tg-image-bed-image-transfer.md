# TG图床插件 · 图片体检与转存方案（v0.4.0）

> 日期：2026-09-04　状态：已拍板（沿用 A2 零宿主改动的架构取向）

## 1. 需求

检测说说和文章正文中的**外部图片**（外链 URL，易失效）与**本地图片**（`/media/`，占服务器空间），
支持一键**转存到 TG 图床**（sendDocument 进频道 → 反代 Worker URL），并更新帖子正文引用。

## 2. 方案选型

| 路线 | 结论 |
|---|---|
| A. 管理页检测 + 转存 + 宿主 REST 改正文（**选定**） | 零宿主改动、零 SDK 新特权、插件保持无写库能力 |
| B. `content.render` 钩子渲染时动态替换 | 治标不治本（帖内数据仍是旧 URL），每帖渲染都过插件，弃 |
| C. 宿主内置迁移功能 | 超出插件范畴，弃 |

### 关键决策依据

- 插件 `data.read` 的 GetPost **不含正文全文**（脱敏），离线扫描若走 SDK 必须扩宿主 → 违反最小改动；
  插件页面与站点**同源**，前端直接 `fetch` 宿主 REST（带登录 Cookie）即可拿正文。
- 正文替换走 `PUT /api/v1/admin/posts/:id`（管理员或作者本人，服务端校验兜底），插件后端不获得任何写帖子特权。
- 外链图下载必须放**插件后端**（浏览器 fetch 外链受 CORS 限制）；本地图 `/media/` 同源无 CORS，前端读流转 base64。

## 3. 交互与数据流

新增后台页面 `detect`（`/admin/plugin-pages/tg-image-bed/detect`，从图库页顶部入口跳转）：

```
[扫描帖子] → 分页 GET /api/v1/posts（摘要）→ 逐条 GET /api/v1/posts/{id}（正文）
          → 正则提取图片（html <img src> + markdown ![]()）→ 分类渲染：
             外部图片（http/https 外链）｜本地图片（/media/ 相对路径）｜已TG（proxy_base 前缀，跳过）
[勾选图片/整帖] → [转存到TG图床]
   外部图：POST 插件 /manage/transfer {url}      （插件后端下载，30s 超时，≤20MB，白名单校验）
   本地图：前端 fetch(src) → base64 → POST 插件 /manage/upload（现有端点，复用）
   每张成功后按帖聚合替换正文：
          GET /api/v1/admin/posts/{id} → content 全局替换 oldSrc → tgUrl → PUT 全量回写
```

- 替换匹配**原文形态**（含 `&amp;` 转义变体两种都替换），避免「提取时反转义、替换时对不上」。
- PUT 回写全量字段（title/tags/media_ids/visibility/status 原样带回；tags 按 AdminUpdatePostReq 约定去 `#` 前缀）。
- 私密帖扫描得到的是 403/隐藏——登录管理员走前台详情接口可见，无需特判。
- 不删除本地媒体源文件（危险操作，超出本期；替换成功仅正文引用改变）。

## 4. 插件端改动

- `main.go`：`handleUpload` 的「校验后发送 + 落历史 + 组响应」段抽为 `sendImage()` 供两处复用；注册 `POST /manage/transfer`（登录用户）；版本 0.4.0。
- `transfer.go`（新，~150 行）：外链下载（Content-Type 白名单 + URL 扩展名兜底、文件名取 URL 尾段清理、20MB 前置校验）→ 复用 `sendImage`。
- `frontend/detect-page.js`（新，≤300 行）：扫描/分类/勾选/转存/替换进度 UI。
- `frontend/manifest.json`：pages 注册 detect。
- 双清单 `plugin.json` / `yueyan-plugin.json` 版本同步 0.4.0（双清单易踩坑，见插件手册 §6.4）。
- `README.md` 补使用说明。

## 5. 边界与安全

- 转存端点登录用户可用（与上传同权限级）；换正文仅管理员/作者（宿主校验）。
- SSRF 考量：`/manage/transfer` 让服务端请求任意外链——与「媒体转存开放接口 /open/media/transfer」同风险级（站点已有先例），仅限 http/https、≤20MB、白名单 MIME；不做内网地址黑名单（与既有先例保持一致，后续如需再统一加固）。
- 重复转存：TG 前缀跳过；转存成功前端即时更新分类标记。

## 6. 验证

- 后端单测：文件名/扩展名推导、MIME 白名单判定、URL 校验（纯函数）。
- 本地 `scripts/build-tg-image-bed.sh` 编译通过 + `scripts/test.sh` 全量绿。
- 生产实测（143.47.108.63 已有真实 TG 配对与帖子）：扫描 9 帖 → 转存一条外部/本地图 → 正文替换生效、前台图片仍显示。
