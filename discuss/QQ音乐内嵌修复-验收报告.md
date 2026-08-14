# QQ 音乐内嵌播放器修复 · 验收报告

> 日期：2026-08-13
> 类型：Bug 修复（M7 增量）
> 结论：✅ 修复完成，端到端验证通过

---

## 一、问题现象

在「写一帖」（/compose）用「♪ 音乐」粘贴 QQ 音乐链接插入后，内嵌播放器无法播放：
播放器 iframe 正常加载，但始终提示 **「请粘贴正确链接」**，歌曲名/播放按钮不出现。

复现链接形态（均复现）：
- `https://y.qq.com/n/ryqq/songDetail/{songmid}`（新版网页版分享）
- `https://i.y.qq.com/v8/playsong.html?songmid={songmid}`（旧版分享）

## 二、根因（已定位并实测验证）

1. 前端 `frontend/src/lib/music-embed.ts` 的 `qqEmbed()` 生成：
   `https://i.y.qq.com/n2/m/outchain/player/index.html?songmid={songmid}`
2. 逆向 QQ 音乐外链播放器 JS（index.2ee216446.js）并浏览器实测对比：
   - `?songid={数字ID}` → ✅ 正常渲染「晴天 - 周杰伦 00:00 04:29」
   - `?songmid={字符串MID}` → ❌「请粘贴正确链接」（参数已废弃）
   - `?shorttag={短链码}` → ❌ 实测未触发解析
3. 即 **QQ 音乐外链播放器参数格式已更新，只认 songid，不再认 songmid**；
   而用户粘贴的分享链接只含 songmid，两者之间腾讯无可靠公开直转接口
   （musicu.fcg 返回 40000/103901、老接口 404 下线、页面数据接口带签名）。

## 三、修复方案（用户选定：方案 B 后端代理转换）

**转换链路（两步，均为腾讯公开接口，实测稳定）**：
1. 歌词接口 `c.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg?songmid=xxx`
   → LRC 头解析 `[ti:歌名] [ar:歌手]`
2. 搜索接口 `c.y.qq.com/soso/fcgi-bin/client_search_cp?w={歌名 歌手}&n=5`
   → 结果中按 songmid 精确匹配 → songid

## 四、改动清单

### 后端（新增 2 文件 + 2 处注册）
| 文件 | 说明 |
|---|---|
| `internal/service/music.go`（新增） | QQMusicService：歌词→搜索两步解析 + 内存缓存（24h TTL，实测命中 32ms）；歌曲不存在 404、外部接口故障 6002 |
| `internal/handler/music.go`（新增） | MusicHandler.ResolveQQ：GET /api/v1/music/qq-resolve?songmid=xxx（需登录） |
| `internal/router/router.go` | Handlers 加 Music 字段 + authed 组注册路由 |
| `internal/server/server.go` | 装配 musicSvc → NewMusicHandler |

### 前端（3 文件）
| 文件 | 说明 |
|---|---|
| `frontend/src/lib/api.ts` | 新增 `apiResolveQQMusic(songmid)` |
| `frontend/src/lib/music-embed.ts` | MusicEmbed 加 `songmid` 字段；qqEmbed 只提取 songmid（embedUrl 留空）；新增纯函数 `qqPlayerURL(songid)` |
| `frontend/src/components/compose/rich-text-editor.tsx` | confirmMusic 改异步：QQ 音乐先调后端解析（按钮「解析中…」），成功生成 songid 播放器 URL 后插入；失败展示后端提示 |

## 五、验证结果（Playwright + 系统 Chrome，真实浏览器）

### 1. 后端接口
| 场景 | 结果 |
|---|---|
| 正常解析 `0039MnYb0qxYhV` | 200 → `{"songid":97773,"title":"晴天","artist":"周杰伦"}` |
| 缓存命中（第二次调用） | 32ms 秒回 |
| 无效 songmid | 404（CodeNotFound） |
| 缺参数 | 400（CodeBadRequest） |

### 2. UI 端到端（/compose 插入）
| 链接形态 | 解析 | iframe src | 播放器 |
|---|---|---|---|
| songDetail 新版 | ✅ | `?songid=97773` | ✅「晴天 - 周杰伦 00:00 04:29」 |
| playsong 旧版 | ✅ | `?songid=97773` | ✅ 同上 |
| c.y.qq.com 短链 | ❌ 提示「无法识别该音乐链接」（原有行为，未变更） | — | — |

### 3. 发布链路
写一帖（正文 + QQ 音乐）→ 发布 → 详情页 `/posts/71`：
- 详情页 iframe src = `?songid=97773`（sanitize 域名白名单 `i.y.qq.com` 放行，无需改动）
- 播放器渲染「晴天 - 周杰伦 00:00 04:29」✅

### 4. 回归
网易云音乐 outchain 播放器接口层面正常（未受影响）；发帖/插入流程无改动回归。

### 5. 顺带修复：时间线摘要泄漏 HTML 源码（用户反馈「首页看到一堆代码」）
| 项 | 内容 |
|---|---|
| 现象 | 首页时间线卡片显示 `<p>分享一首歌</p><div data-music-embed=...` 等 HTML 源码文本 |
| 根因 | `internal/service/post_assembly.go` 的 `summaryPreviewText()` 只调 `stripMarkdown`，漏调 `stripHTML`；富文本帖子的 `<div>/<iframe>/<h2>` 等标签（含按 60 字符截断产生的未闭合标签）原样进入 summary → 前端卡片文本节点转义显示 |
| 修复 | 改用 `plainText()`（先剥 HTML 再剥 Markdown，与 `buildSummary` 语义一致） |
| 验证 | 时间线 API 与浏览器首页卡片均为纯文本（如「QQ音乐内嵌播放器修复验证（songid 链路）」「分享一首歌」） |

### 6. 功能增强：首页卡片直接渲染音乐迷你播放器（用户反馈「首页看不到播放插件」）
| 项 | 内容 |
|---|---|
| 需求 | 首页时间线卡片直接显示可播放的音乐插件（原设计仅详情页渲染播放器，卡片只显示文字摘要） |
| 实现 | 后端 `PostSummary` 新增 `music` 字段（`MusicEmbedDTO`：platform/kind/url），`assembleSummaries` 用 `extractMusicEmbed()` 正则提取正文首个 `div[data-music-embed]` 节点并 `html.UnescapeString` 还原 URL 实体；前端 `post-card.tsx` 在摘要下方渲染同款迷你播放器（QQ 110px / 网易云单曲 66px / 歌单专辑 430px） |
| 验证 | 首页 20 卡片中 3 个音乐播放器正常渲染：QQ「晴天 - 周杰伦」×2、网易云「分享一首歌」×1（&amp; 实体正确解码） |

## 六、说明与后置

- **历史已发布帖子**中的 QQ 音乐（旧 songmid src）仍无法播放，需重新编辑插入或后续做内容迁移（本报告不处理）。
- 转换结果内存缓存 24h，多实例部署时可换 Redis（当前单体够用，KISS）。
- 腾讯接口若后续再变更，错误会以 404/6002 形式暴露在插入弹窗提示中，便于发现。
- 测试脚本保留：`scripts/ui-music-test.mjs`（链接解析+播放器检查）、`scripts/ui-qq-publish.mjs`（发布链路）。
