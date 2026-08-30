# 方案：浏览器插件「写说说」（首页底部快捷输入改造）

> 讨论稿 · 对应开发：插件首页底部「向 AI 提问」快捷输入 → 改造为「写说说」发布器。
> 状态：已实现（插件 v0.27.0 + 后端 media.upload 开放端点）。

## 1. 需求还原（用户口径）

1. 插件首页底部快捷输入框改为**写说说**；
2. 参考 web 端「写一帖」但做减法：**不要 AI 辅助**、不要标签行、不要形态/类型 Tab——只要 **公开/私有 + 发送按钮**；
3. 支持**复制（粘贴）图片**，图文一起发送；
4. 支持四类可发送内容：**图、视频、音乐、链接**。

## 2. 调研结论（现状）

| 结论 | 出处 |
|---|---|
| 说说 = `post_kind: "moment"`（≤2000 字，标题可空），`posts.create` 通道已支持显式传参 | `internal/handler/openapi_post.go`（缺省归一，插件可覆盖） |
| 说说正文格式 `content_format: "html"`，富文本混排形态 `content_type: "text"`；图片上传后内嵌 `<img>` 且 media_id 同步进 `media_ids` | `frontend/src/app/compose/page.tsx` 提交逻辑 |
| 前台渲染：html 正文经 DOMPurify 消毒后直接渲染；`div[data-music-embed=netease][data-music-id]` 拆为自研播放器；iframe 域名白名单：player.bilibili.com / www.youtube.com / v.qq.com / player.vimeo.com / music.163.com / i.y.qq.com | `frontend/src/components/post-content.tsx`、`frontend/src/lib/sanitize.ts` |
| 视频内嵌协议：`<div data-video-embed="{platform}"><iframe src="{embed地址}"></iframe></div>` | `frontend/src/components/compose/video-embed.tsx` |
| 本地上传走 `POST /api/v1/media`（JWT 鉴权），**开放网关无上传端点**——插件凭 API Key 无法上传本地图 | `internal/router/router.go:158`、`internal/handler/media.go` |

## 3. 后端配套（media.upload，小改动）

| # | 改动 | 文件 |
|---|---|---|
| B1 | 新增 `MediaUpload` handler：`POST /api/v1/open/media`（multipart `file`），凭 Key 绑定用户身份复用 `PostService.UploadMedia`（类型/大小校验沿用存储层） | `internal/handler/openapi_media.go` |
| B2 | 开放组注册路由 | `internal/router/router.go`（`/open/media/transfer` 旁） |
| B3 | 目录登记 `media.upload`（媒体上传） | `internal/model/openapi.go` |
| B4 | 授权语义：Key 需勾选 `media.upload`；**旧 Key 未勾选时 403**——插件端识别后提示「后台重新生成 Key 并勾选媒体上传」 | 鉴权机制 `internal/middleware/apikey.go`（FullPath+Method 反查目录） |

需重启后端生效。

## 4. 插件端设计

### 4.1 UI（392–430px 窄面板，自上而下）

```
┌──────────────────────────────────┐
│ [附件条] 图缩略图 / 视频·音乐·链接条目（可删）│ ← 有附件才显示
│ ┌──────────────────────────────┐ │
│ │ 记一点…（textarea 自增高 ≤6 行）│ │
│ │                    1234/2000 │ │ ← 超限红字禁发
│ └──────────────────────────────┘ │
│ [🖼图][▶视频][♪音乐][🔗链接]      │
│            （🌐公开 ⇄ 🔒私有）（发布）│
└──────────────────────────────────┘
```

- 粘贴图片（`paste` 事件 clipboardData）与本地多选（file input）同通道上传；
- 成功：清空输入 + 顶部「已发布 ✓」提示（3s 自散）；失败：错误条携带后端 message；
- 可见性：公开/私有胶囊一键互切（`visibility: public/private`）；
- 恒定直接发布（`status: "published"`，不做草稿——用户口径只要发送）。

### 4.2 四类内容的落地形态（对齐 web 端富文本混排模型）

| 类型 | 输入方式 | 落地 | 前台渲染 |
|---|---|---|---|
| 图 | 本地多选 / 粘贴 | `open media.upload` 上传 → `<img src="站点地址">` 按序内嵌，media_id 收进 `media_ids` | 正文直渲染（DOMPurify 放行 img） |
| 视频 | 贴链接弹层 | 解析 B站（BV 号/网页/b23.tv 短链）与 YouTube（watch/youtu.be/shorts）→ `div[data-video-embed]>iframe`（player.bilibili.com / youtube.com/embed） | 消毒白名单放行，iframe 直渲染 |
| 音乐 | 贴链接弹层 | 解析网易云歌曲链接（music.163.com/song?id=）→ `div[data-music-embed="netease"][data-music-id]`（title/artist 留空） | 拆为自研播放器 MusicRefPlayer |
| 链接 | 贴 URL + 文字弹层 | `<a href="…" target="_blank" rel="noopener">文字</a>` | 正文直渲染 |

- QQ 音乐/腾讯视频/Vimeo：v1 不做（输入弹层明示支持范围），需时再扩；
- 提交：`post_kind="moment"`、`content_format="html"`、`content_type="text"`（富文本混排形态，对齐 web 端文字 Tab）、`media_ids`=上传图片集合、纯文本 ≤2000 字校验。

### 4.3 文件规划（行数/目录红线内）

```
browser-extension/src/sidepanel/components/moment/
├── MomentComposer.tsx   # 主组件：状态、粘贴、提交、反馈（≤300 行）
├── AttachBar.tsx        # 附件条（缩略图/条目/删除）
├── InsertSheets.tsx     # 视频/音乐/链接三个输入弹层
└── compose.ts           # 纯函数：链接解析（视频/音乐）、正文 HTML 组装、转义
```

`shared/types/index.ts` 增 `UploadResult`、`MomentAttach`；`shared/api/endpoints.ts` 增 `uploadMedia`、`createPost` 类型放宽支持 moment。`App.tsx` 底部快捷输入替换为 `<MomentComposer>`（AI 快捷入口随之移除——AI 页入口保留在功能卡与 Tab）。

## 5. 验证清单（双浏览器）

- [ ] `scripts/build-browser-extension.sh` 通过（tsc 严格 + vite）；后端 `go build` 通过
- [ ] Chrome / Edge：写说说纯文字发布成功，时间线可见
- [ ] 粘贴截图 / 本地多选图 → 上传 → 发布 → 前台图文展示、不裂图
- [ ] B站视频链接（BV 号 / 网页 / b23.tv）→ 前台可播放；YouTube 链接 → 前台可播放
- [ ] 网易云歌曲链接 → 前台出现自研播放器并可播放
- [ ] 链接插入 → 前台可点击跳转
- [ ] 私有发布 → 匿名时间线不可见、本人可见
- [ ] 未勾选 media.upload 的 Key：上传报 403 时提示重新生成 Key
- [ ] 断网 / 后端 5xx / 超字数（>2000）三类异常提示友好

## 6. 版本与登记

- 插件 `0.26.2 → 0.27.0`（新功能提次版本），CHANGELOG 登记（含后端配套说明）；
- 手册 §5.1 模板 version 示例同步；开放接口目录变更由后端目录自动呈现，手册无需改权限章节。
