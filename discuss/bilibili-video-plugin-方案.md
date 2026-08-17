# B站视频插件（bilibili-video）实施方案

> 状态：已批准并实施（2026-08-17）
> 需求来源：站长提出——发帖插入 B 站视频并选择 360P~1080P 清晰度；1080P 需登录 B 站（后台登录页，扫码/手机号）；游客可观看；游客登录自己的 B 站账号后用自己的身份看高清。

## 一、需求 → 架构映射

| 需求 | 落地机制 |
|------|---------|
| 后台安装后出现登录页 | 插件后台页面 `/admin/plugin-pages/bilibili-video/login`（扫码 + 手机号 + Cookie 导入）+ 侧栏 nav 入口（市场清单） |
| 登录回调保存 key/token | 扫码 poll 成功捕获 Set-Cookie（SESSDATA/bili_jct/DedeUserID），AES-256-GCM 加密落盘 `data/plugins/bilibili-video/state.json` |
| 插入地址 → 获取分辨率 | 宿主编辑器视频弹窗增强：B 站地址 + 插件 running → 调插件 `POST /resolve` → 清晰度单选（16/32/64/80，720P 起标注需登录） |
| 帖子播放所选分辨率 | 插件调 B 站 playurl（WBI 签名 + html5 mp4 durl）→ 内容块 `data-plugin-block="bilibili"` → 插件 player.js 播放（video no-referrer 绕防盗链，多段顺序播放） |
| 游客可观看 | 宿主公开桥接 `/api/v1/video/bilibili/*`（System 身份直达插件；匿名可访问）；设置项 `allow_guest_hd` 默认开 |
| 游客自己的 B 站账号 | 前台播放器扫码弹层 → 插件签发 guest_token（AES 封装该游客 cookie，浏览器 localStorage 持有，服务端不落盘）→ 播放解析时优先使用 |

## 二、关键设计决策

1. **自研播放器而非官方 iframe**：iframe 播放器清晰度由 B 站控制、无法用站长身份解析高清，不满足需求。解析真实流地址与音乐插件同模式。
2. **显式 cookie 管理（无 cookie jar）**：站长会话与游客 token 并存，共享 jar 会互相污染；所有请求显式 Cookie 头。
3. **guest_token 而非服务端 per-user 存储**：访客凭证不落盘（隐私与存储最小化），token 仅经同源请求往返；统一覆盖「宿主登录访客」与「纯匿名访客」。
4. **手机号登录双通道**：B 站短信接口有极验风控，发送被拦时返回 `need_captcha=true` 提示改用扫码（用户已确认该策略）。
5. **游客高清默认允许**（用户已确认）：`allow_guest_hd` 设置项可随时关闭，关闭后匿名最高 480P（B 站匿名可达上限）。
6. **WBI 签名**：playurl 接口必需；mixinKey 混淆表算法纯函数实现于 `bilibili/wbi.go`，密钥 12h 缓存（nav 接口派生）。
7. **buvid3 风控基础**：Client 初始化经 `finger/spi` 接口拉取匿名基础 cookie。

## 三、文件清单

### 插件（cmd/bilibili-video-plugin/，Go ≤400 行/文件）

| 文件 | 职责 |
|------|------|
| `main.go` | Info（capabilities: api/frontend/settings/admin.page + 设置项）/ 生命周期 / 入口 |
| `api_login.go` | 站长登录类端点（qr-init/qr-check/sms-send/sms-login/cookie-login/logout/status） |
| `api_video.go` | 视频端点（resolve/video/url/guest-qr-check/guest-status）+ RegisterAPI 汇总 + cookie 三级降级 |
| `bilibili/client.go` | HTTP 客户端（显式 cookie）、AES-GCM 持久化、buvid3、cookie 合并 |
| `bilibili/wbi.go` | WBI 签名纯函数（混淆表 + md5） |
| `bilibili/nav.go` | nav 接口（登录态校验/资料/WBI 密钥缓存） |
| `bilibili/qr.go` | 扫码 generate/poll/cookie 捕获 |
| `bilibili/sms.go` | 短信发送/登录（极验风控识别） |
| `bilibili/video.go` | view 详情 / playurl 解析（durl mp4）/ 清晰度辅助 / 短链展开 |
| `bilibili/token.go` | guest_token 签发/解封（90 天） |
| `frontend/manifest.json` | pages: login + blocks: bilibili |
| `frontend/login-page.js` | 后台登录页（B 站粉主题双 Tab + Cookie 导入） |
| `frontend/player.js` | 播放器块（封面卡片/清晰度菜单/游客扫码弹层/多段播放） |

### 宿主（最小改动，均有先例）

| 文件 | 改动 |
|------|------|
| `frontend/src/components/compose/bilibili-embed.tsx` | 新增 Tiptap 节点（序列化为块协议 data-props） |
| `frontend/src/components/compose/bilibili-picker.tsx` | 新增解析弹窗（fetch 插件 /resolve + authHeaders） |
| `frontend/src/components/compose/rich-text-editor.tsx` | 注册节点 + 插件 running 检测 + 弹窗分支（~40 行） |
| `internal/handler/video.go` | 新增公开桥接（System 身份 CallAPI 直达） |
| `internal/router/router.go` | 公开组 +5 路由 + Handlers 字段 |
| `internal/server/server.go` | 构造 VideoHandler（1 行） |
| `frontend/src/components/admin/nav-icons.tsx` | 新增 video 图标键 |
| `marketplace-repo/bilibili-video/` | 市场清单 plugin.json + README |
| `scripts/build-bilibili-plugin.sh` | 构建脚本 |

## 四、API 契约

### 插件端点（宿主代理，需登录）

- `POST /qr-init`（管理员）→ `{qrcode_key, qr_png, session_token}`
- `POST /qr-check`（管理员）`{qrcode_key, session_token}` → `{code}`（86101/86090/86038/0）
- `POST /sms-send` / `POST /sms-login`（管理员）→ `{ok, need_captcha, message}`
- `POST /cookie-login`（管理员）`{cookie}` → `{ok, nickname}`
- `POST /logout`（管理员）、`GET /status` → `{logged_in, profile{mid,nickname,avatar,vip,level}}`
- `POST /resolve`（登录）`{url}` → `{video{bvid,cid,title,cover,duration,author}, qualities[{qn,desc,need_login}], admin_logged_in}`

### 公开桥接（宿主 `/api/v1/video/bilibili/*`，匿名可访问，System 身份透传）

- `POST /resolve` / `POST /url` / `POST /qr-init` / `POST /guest-qr-check` / `POST /guest-status`

### 播放地址 cookie 三级降级

```
guest_token 有效 → 游客自己的 B 站账号（source=guest）
allow_guest_hd=on 且站长已登录 → 站长账号（source=admin）
匿名（B 站自动降级到可达清晰度，一般 480P）（source=anonymous）
```

## 五、风险与对策

- **B 站风控/接口变更**（WBI/buvid3/极验）：全部隔离在 `bilibili/` 包；扫码登录不受极验影响
- **流地址约 2h 时效**：播放时实时解析，不做长期缓存
- **多段 durl**：video ended 事件顺序切源
- **大会员专属清晰度（1080P+/4K）**：不涉及，仅 16/32/64/80 四档

## 六、实施中沉淀的关键经验（2026-08-17 端到端验证）

1. **B 站 WAF 412 规律**（实测）：非浏览器 TLS 指纹（Go/curl）+ 浏览器 UA + Referer 三者叠加必返 412；仅 UA 或仅 Referer 放行 → 客户端**不携带 Referer**。
2. **playurl 双路径**（实测）：
   - html5 平台端点（`platform=html5`，免 WBI 签名）匿名可用（实测匿名可达 720P）；
   - 带 WBI 签名的 web 端点匿名报 -101（走登录鉴权路径）；
   - 故 PlayURL 按身份分流：匿名走 html5，带 SESSDATA 走 web+WBI（登录可达 1080P，WBI 密钥获取失败自动降级 html5）。
3. **流地址 Referer 风控**（实测）：B 站 CDN（\*.bilivideo.com）要求「浏览器 UA + 空 Referer」；部分 webview 给 media 请求注入页面 Referer 即 403 → 新增**宿主流代理** `GET /api/v1/video/bilibili/stream?src=`（域名白名单 \*.bilivideo.com + 浏览器 UA + 无 Referer + Range 透传 206），前端统一同源加载。
4. **宿主 settings jsonb 解码 bug 修复**：`repository/setting.go` 的 `unquoteJSONB` 原来只去首尾引号、不解内部 `\n` 转义，导致多行设置值（如 PEM 公钥）传给 `pem.Decode` 失败（「信任公钥配置无效」）——改用 `json.Unmarshal` 完整解码，非 JSON 原样返回。
5. **匿名可达清晰度好于预期**：html5 端点匿名实测可达 720P（原按 web 端预期 480P），播放提示按实际 quality 动态报告。

## 六点五、Bug 修复批次（2026-08-17 用户实测反馈）

1. **登录态未保存**：扫码成功后原实现先经 nav 校验资料、失败即不保存——带登录 cookie 的 nav 请求易被 B 站风控临时拦截，导致登录永远不落地（前端还静默回扫码界面）。修复：扫码/短信成功后**无条件保存** cookie（资料置空），/status 懒刷新经 nav 补全（EnsureProfile）；前端 qr-check 的 error 如实展示。
2. **1080P 降级 720P**：原「登录走 web+WBI」路径的 mp4 格式在 web 端点上限 720P。修复：统一走 html5 端点（登录 SESSDATA 可达 1080P mp4）；WBI 路径与 wbi.go 删除（死代码清理）。
3. **封面/头像防盗链**：B 站图床（*.hdslb.com）Referer 风控 + 接口返回 http 地址。修复：宿主新增图床代理 GET /api/v1/video/bilibili/image（白名单 hdslb.com、http 自动升级 https、1h 缓存头），前端封面/头像统一走代理（实测 naturalWidth=1920 加载成功）。
4. **菜单高亮语义**：清晰度高亮跟随用户选择（selectedQn），实际播放档位（自动降级）由提示条文案说明，二者分离。

## 六点六、时间线播放器通道（2026-08-17 第二批反馈）

- **问题**：首页时间线不渲染帖子 HTML（只显示文本摘要 + 结构化 media/music 字段），B站视频块在列表不可见。
- **方案**（对齐音乐嵌入 M7 模式）：
  - 后端 `post_assembly.go` 新增 `extractBilibiliEmbed`：正则提取正文首个 `div[data-plugin-block=bilibili]` 的 `data-props`（&quot; 反转义 + JSON 校验）→ `PostSummary.Bilibili`（json.RawMessage）；
  - 前端 `post-card.tsx`：`post.bilibili` 存在时经宿主 `PluginBlock`（type=bilibili）分发到插件 player.js——与详情页同一渲染通道（清晰度菜单/游客扫码/流代理全套复用）。
- **验证**：时间线 API 返回 bvid/quality；首页渲染完整播放器块并稳定播放（匿名 720P）。

## 六点七、升级数据迁移与崩溃误报（2026-08-17 第三批反馈）

- **「进程崩溃，第 2 次退避重启中」误报**：开发期部署用「先杀插件进程、后杀宿主」的错误顺序，旧宿主的 watchExit 把强杀记为崩溃（DB last_error 残留；state 实际 running）。已清库；此后部署遵循「先停宿主再停插件」（stop-all 语义）。
- **登录态丢失的真实根因**：升级安装走「整目录原子替换」（RemoveAll + Rename），插件自建的用户数据文件（state.json 登录态 / settings.json 自有配置）不在包内、随替换被清空——用户扫码成功后一次升级即丢。属宿主升级流程缺陷（影响全部插件）。
  修复（internal/service/plugin_bpk.go）：升级前 backupPluginData 备份旧目录中「包内不提供且非二进制」的文件（临时文件放插件目录同级同盘——恢复用 os.Rename，跨盘必失败），替换后恢复；RemoveAll 加 5×150ms 重试消解 Windows 上刚 Kill 进程的 exe 句柄释放窗口。已实测：升级后 state.json 完整保留。
- **「未登录 B 站」提示**：为上述登录态丢失的直接表现；扫码链路本身实测正常（qr-init 出码、poll 86101/86090/0 语义正确、成功后无条件保存）。

## 六点八、1080P DASH 播放链路（2026-08-17 第四批反馈「登录后仍 720P」）

- **根因（实测结论）**：B 站 mp4/durl 路径上限 720P（登录与否都一样），**1080P 仅有 DASH 形态**（音视频分离 m4s，fnval=16；登录 cookie + html5 端点即可，无需 WBI）。
- **实现**：
  - 后端 PlayURL 改 fnval=16，解析 dash.video/audio 流组（avc1 优先，浏览器兼容最好）；
  - 新增 `frontend/dash-player.js`：MSE 双 SourceBuffer 装载（MSE 按时间戳自动对轨）；**Range 4MB 分块 + 失败重试**（CDN 对连续大流量全量拉取 503 限流，实测教训）；流地址经宿主代理；
  - 宿主流代理去掉整体超时（改 ResponseHeaderTimeout，长流不被掐断）。
- **两个隐蔽宿主级修复**：
  1. **CSP media-src 缺 blob:**——MSE 的 objectURL 被 CSP 拦截即 video error（探测页无 CSP 而成功、帖页失败的分水岭）；next.config.ts 已加。
  2. **Chromium 模块图进程级缓存**：固定 URL 的插件 ESM 在 webview 不重启时永远复用旧实例（文档刷新也不重取）——宿主 loader.ts 给模块 URL 加 30s 周期指纹；插件侧 manifest entry 与内部 import 同步加 ?v=2 强制换图。
- **验证**：帖页点 1080P →「正在播放 1080P」；升级安装后登录态保留（state.json 完好，status=logged_in）。

## 六点九、更新误报与崩溃脏记录（2026-08-17 第五批反馈）

- **「进程崩溃，第 N 次退避重启中」再现**：仍是部署顺序问题（先杀插件进程后停宿主，旧宿主 watchExit 记崩溃）。已清库；此后统一「先停宿主再停插件」（stop-all 语义）。
- **「可更新至 1.2.0」集体误报的根因**：发布脚本对主仓库统一发 `v{version}` tag（**tag 空间全插件共享**），CheckUpdates 拿 Release latest（= 全局最高版本 v1.2.0）一刀切对比所有插件 → 全站旧版本插件集体误报。
  修复（internal/service/plugin_bpk.go）：CheckUpdates / UpdatePlugin 的目标版本统一改为**市场清单 per-plugin version 优先**（5 分钟缓存），清单缺失该插件才回退 Release latest；UpdatePlugin 按目标版本钉扎拉取 v{target} Release。
- **验证**：可更新列表只剩演示插件（0.1.0 → 0.2.0，正是期望的演示形态）；一键升级实测成功（下载 Release 资产 → 校验替换 → running v0.2.0，全程无报错）。

## 七、验证结论

- ✅ 安装（.bpk 签名校验）→ running → 公开扩展清单出现
- ✅ 桥接 resolve：视频信息 + 四档清晰度（720P/1080P 正确标注「需登录」）
- ✅ 桥接 url：匿名请求 1080P 自动降 720P，返回 https mp4 直链
- ✅ 流代理：Range 请求 206 直通
- ✅ 编辑器：视频弹窗自动切换 B 站解析模式 → 清晰度选择 → 插入块 → getHTML 序列化 `data-plugin-block` ✓ → 发布
- ✅ 帖子渲染：插件 player.js 播放器块（封面卡片 + 清晰度菜单 + 扫码登录入口）→ 点击播放 →「正在播放 720P」稳定播放
- ✅ 后台登录页：扫码 Tab（二维码加载 + 轮询）+ 手机号 Tab + Cookie 导入折叠
- ⏳ 站长扫码登录后 1080P / 游客扫码解锁高清：需真实哔哩哔哩 App 扫码，待站长实操验证（链路已通：qr-init 返回二维码、poll 端点就绪、guest_token 签发逻辑就位）
