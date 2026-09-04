# 方案讨论：浏览器右键菜单 + 悬浮球执行框（桌宠）

| 项目 | 内容 |
|---|---|
| 日期 | 2026-09-04 |
| 状态 | 已评审（随 v0.31.0 落地） |
| 目标版本 | browser-extension v0.31.0 |
| 关联手册 | docs/browser-extension-guide.md v1.3 新增 §14 |

## 1. 需求原始描述

1. 浏览器右键功能：总结网页内容并选择发布到个人博客。
2. 收藏夹功能：AI 自动收藏，或主动收藏到某个文件夹。
3. 选择一段话 + 图片，发送说说。
4. 执行时悬浮球周围出现执行框：展示执行过程、执行中的交互、执行后的完成提示——浏览器桌宠体验。
5. 悬浮球未启动（隐藏/不可注入）时，自动弹出右侧插件面板。

## 2. 总体设计

三层结构：**右键菜单（background）→ 任务暂存（chrome.storage）→ 执行器 UI（悬浮球旁 iframe / 面板内叠加层）**。

```
用户右键 → chrome.contextMenus.onClicked（background）
  ├─ 构建 ExecTask 写入 storage（exec_task_v1，含 nonce 与 target）
  ├─ ① tabs.sendMessage(tabId, yy-exec-offer) → 悬浮球可见则应答 ok:true
  │     → 悬浮球在球旁展开执行框 iframe（index.html?mode=exec）
  └─ ② 球不可用（隐藏/特权页/无内容脚本）→ target 改写为 panel
        → openPanel 三级降级（原生侧栏 → 页内停靠 → 悬浮窗）
        → runtime 广播 yy-exec-run → 面板任意形态叠加 ExecutorCard 执行
```

关键取舍：

- **执行器是扩展页面（iframe / 面板页），不是内容脚本 UI**。网络请求（AI 总结、发布、图转存）全部发生在扩展页 origin，符合手册 §7.2「内容脚本禁止直接 fetch boke API」；且无需 content script 新增任何 fetch。
- **任务单消费者**：`exec_task_v1.target` 标记 `ball | panel`，只有被指定的消费者执行，天然防重复（embed 面板与执行框 iframe 并存时不会双跑）。
- **消息通道数超过 10 条**（§8.1 阈值）：按手册要求落 `src/shared/messages/types.ts` 集中定义判别联合；background / content script 侧因自包含约束继续本地声明 + 注释同步。

## 3. 右键菜单清单（chrome.contextMenus）

| 菜单 id | 上下文 | 文案 | 动作 |
|---|---|---|---|
| `yy-root` | page / selection / image | 月言助手 | 根菜单 |
| `yy-summary` | page | 📝 总结本页，发布到博客 | ExecTask: summary |
| `yy-fav-ai` | page | ⭐ 收藏本页（AI 自动分类） | ExecTask: bookmark(mode=ai) |
| `yy-fav-pick` | page | 📁 收藏本页到指定文件夹… | ExecTask: bookmark(mode=pick) |
| `yy-moment-text` | selection | 💬 发说说：加入选中文字 | ExecTask: moment(addText) |
| `yy-moment-image` | image | 💬 发说说：加入此图片 | ExecTask: moment(addImage) |

- 全部限定 `documentUrlPatterns: http/https/file`（与内容脚本注入范围一致）。
- 菜单常驻注册（onInstalled 全量重建），不随书签树变化（文件夹选择在执行器交互层完成，避免动态菜单的维护成本——僵化风险）。

## 4. 三类任务执行流程

### 4.1 summary（总结本页并发布）

1. 【过程】`yy-page-text` → dock 内容脚本返回可见文本（≤12K，已有通道零新代码）。
2. 【过程】AI 流式总结（`sendAiChatStream`），执行框内实时滚动生成文本（桌宠"正在思考"观感）。
3. 【交互】可编辑标题 / 摘要（markdown）/ 标签 + 可见性；按钮「存草稿」「发布到博客」。
4. 【完成】`renderMarkdown` 转 HTML → `createPost(article)`；提示含「查看文章」链接（`{站点}/posts/{id}`，新标签打开）。

### 4.2 bookmark（AI 自动收藏 / 指定文件夹收藏）

1. 【过程】读取书签树（`readBookmarkStore`，IndexedDB 主存）抽取文件夹路径清单。
2. AI 模式：【过程】`sendAiChat`（JSON 输出）推荐目标文件夹与书签标题 →【交互】下拉可改文件夹（可新建）、标题可编辑 →【完成】写入书签树（`saveBookmarkStore` 双写，与球收藏同一调和机制）。
3. 指定模式：跳过 AI 直接进入【交互】。
4. 未连接站点时 AI 模式降级提示（书签本身是纯本地能力，指定模式不受影响）。

### 4.3 moment（选中文字 + 图片发说说）

1. 说说草稿篮 `exec_moment_draft_v1`：`addText` 追加文字、`addImage` 追加图片 URL（去重），跨次右键累积——实现「选一段话 + 图片」的组合流；发送成功或点「清空」后移除。
2. 【交互】文字可编辑 + 图片缩略图可删；校验 ≤2000 字（`countChars`）。
3. 【过程】发布：http(s) 图走 `transferImage`（服务端转存，返回 media_id）；data:/blob: 图经 dock `yy-image-data`（页面上下文 fetch → dataURL）→ `uploadMedia`；转存失败的图按原链接内嵌正文并提示。
4. 【完成】`createMomentPost`；提示含「查看说说」链接。

## 5. 悬浮球执行框（桌宠）

- ball.ts 监听 `yy-exec-offer`：球隐藏（showBall=false）时应答 `{ok:false}` 由 background 走面板兜底；可见则应答 `{ok:true}` 并在球旁展开 320×430 执行框（与 panel-wrap 同款贴边翻转定位，尺寸更小）。
- 执行期间球体加 `busy` 类：月晕呼吸加速（工作观感），完成或关闭执行框后恢复。
- 执行框为 iframe `index.html?mode=exec&nonce=…`：复用整个 sidepanel React 工程与 Tailwind 主题，不引入第二套 UI 栈。
- 执行器完成/用户关闭 → `yy-exec-close` → 球收起执行框。
- 点击面板外自动收起的既有逻辑不作用于执行框（执行中可能需要用户在页面上操作，误关破坏体验）；仅由完成按钮 / 关闭按钮收起。

## 6. 面板兜底（悬浮球未启动）

- background 复用三级降级打开面板（原生 `sidePanel.open` → `yy-dock-open` 只开不关 → 悬浮窗），随后广播 `yy-exec-run`。
- 面板页（dock / float / embed 任一形态）挂 ExecutorHost：挂载时检查 + 运行时消息双通道领取 `target==='panel'` 的任务，以覆盖层卡片执行（与球旁执行框同一组件）。
- 任务超过 2 分钟未领取视为过期（浏览器重启残留），静默清除。

## 7. 权限与兼容

- 新增唯一权限 `contextMenus`（Chrome/Edge 均自远古版本支持，≥110 无虞）：解决右键菜单注册，无主机语义，权限三问通过（无替代 API；无法延后申请——菜单须在安装时注册）。
- 不新增任何主机权限：正文抓取走已注入的 dock 消息；图片转存走服务端 `transferImage`；仅兜底路径（外链图转存失败时）在发布按钮（用户手势）内 `ensureWideHostPermission` 后由扩展页 fetch。
- 双浏览器：Edge 无原生侧栏场景由既有页内停靠兜底，执行框 iframe 在 Edge 中同样工作（同为 Chromium iframe + web_accessible_resources 已放行面板页）。

## 8. 存储与消息增量

- 存储键：`exec_task_v1`（待执行任务，含 nonce/target）、`exec_moment_draft_v1`（说说草稿篮：text + 图片 URL 列表）。
- 消息通道：`yy-exec-offer`（background→球，探测+展开）、`yy-exec-run`（background→扩展页广播，面板领取）、`yy-exec-close`（执行器→球，收起执行框）、`yy-image-data`（执行器→dock，页面上下文取图 dataURL）。

## 9. 文件清单

| 文件 | 动作 |
|---|---|
| public/manifest.json | +contextMenus；版本 0.31.0 |
| src/shared/messages/types.ts | 新增：跨上下文消息与 ExecTask 判别联合（集中定义） |
| src/shared/storage/settings.ts | STORAGE_KEYS +execTask/+momentDraft |
| src/shared/storage/exec-task.ts | 新增：任务与说说草稿读写封装 |
| src/background/main.ts | 右键菜单注册 + 任务构建投递 + openPanel 辅助收敛 |
| src/content/ball.ts | yy-exec-offer 应答、执行框、busy 态、yy-exec-close |
| src/content/dock.ts | yy-image-data 应答 |
| src/sidepanel/components/exec/* | 新增：ExecutorCard（外壳）/ ExecutorHost（领取+宿主）/ SummaryExec / BookmarkExec / MomentExec |
| src/sidepanel/App.tsx | mode=exec 渲染执行器页；其余形态挂 ExecutorHost |
| CHANGELOG.md / 手册 | v0.31.0 记录；手册 v1.3 新增 §14 |

## 10. 验证要点（双浏览器）

- Chrome：右键四类菜单均可触发；球可见时执行框出现在球旁；球隐藏时自动开原生侧栏并叠加执行卡。
- Edge：同上（预期走页内停靠兜底）。
- 未连接站点时：执行框给出「先连接站点」引导（打开完整面板），不白屏。
- 说说组合流：选中文字右键加入 → 图片右键加入 → 发送成功后草稿篮清空。
- 收藏写入后打开书签页可见（savedAt 调和收敛进 IndexedDB 主存）。
