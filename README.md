# 月言（Yueyan）· 月色微博客

> 写短句，收声音，偶尔录一点夜色。

「月言」是一个**短文 + 多媒体的轻博客社区**：以短句（≤2000 字）与图片、音频、视频为载体的安静社区。氛围关键词：安静、夜色、月光、慢节奏。设计依据 `boke.pen` 设计稿（294 画板，双主题 × 双端）。

## ✨ 功能全景

### 前台（访客 / 注册用户）
- **认证**：注册（自动用户名）/ 登录（邮箱或用户名）/ 静默刷新（JWT access 15min + refresh 7d）/ 登录限流 / **找回密码**（邮件，30 分钟令牌 + 60s 重发限制）
- **时间线**：全部/图/音/影过滤 + 滚动分页 + 推荐/关注流
- **创作**：文字 / 图片（≤9 张压缩）/ **音频（录音）** / **视频** 四形态发帖；标签（≤5）、可见性（公开/仅关注者/仅自己）、草稿箱
- **帖子详情**：图片灯箱、音频/视频播放器、点赞/收藏（真实落库）、**举报**（6 预置原因）
- **评论**：楼中楼 2 级 + **匿名评论**（昵称 + token 防刷）、评论点赞/删除
- **互动**：话题、搜索、通知（30s 轮询角标）、**私信**（会话/未读/已读/在线状态）、关注/粉丝/收藏、个人主页
- **体验**：冷月/薄雾**双主题**即时切换（跟随系统）、阅读字号/内容密度/减少动效/高对比/自动播放媒体外观设置、移动端 390px 底部导航、404/500/维护/无网络/引导状态页

### 后台（管理员）
- 仪表盘（7 日指标 + 环比 + 互动趋势图 + 内容分布环形图）
- 内容管理（上下架/删除）、评论管理、用户管理（**封禁弹层**：原因 + 永久/限时，封禁记录台账）
- **审核队列**（举报工单处理）、**敏感词管理**（拦截/审核两级，命中即拦截发帖评论）
- 站点设置（meta 实时生效）、封禁管理

## 🛠 技术栈

| 层 | 选型 |
|---|---|
| 后端 | Go 1.26 + Gin + pgx v5（连接池）+ golang-jwt v5 + Casbin + zap |
| 数据库 | PostgreSQL（26 表 + 增量迁移体系） |
| 前端 | Next.js 15.4 + React 19 + Tailwind CSS v4 + TypeScript（ESM） |
| 缓存 | Redis（限流/令牌黑名单，不可用自动降级） |
| 图表 | SVG 自绘（零依赖，双主题适配） |

## 🚀 快速开始

```bash
# 0. 前置：PostgreSQL（.env 配置连接）、Go 1.26+、Node.js 20+
cp .env.example .env   # 填写数据库连接与 JWT_SECRET

# 1. 初始化数据库（26 表 + 种子 + 管理员）
./scripts/init-db.sh
./scripts/seed-admin.sh

# 2. 启动双端（日志统一输出 logs/）
./scripts/dev-server.sh --daemon    # 后端 :8080
./scripts/dev-frontend.sh --daemon  # 前端 :3000

# 3. 停止
./scripts/stop-all.sh

# 其他脚本
./scripts/migrate.sh      # 增量迁移
./scripts/test.sh         # 后端测试
./scripts/build-frontend.sh  # 前端构建
./scripts/pen-inspect.sh  # 设计稿文案提取
./scripts/screenshot.sh   # Playwright 截图（视觉比对）
```

> 管理员初始账号：`admin@yueyan.site` / `Yueyan2026`（首次登录后请修改）。
> 邮件发送：`.env` 配置 `SMTP_*` 后自动启用真实发送；未配置时重置链接写入 `logs/`（开发模式）。

## 📁 目录结构

```
boke/
├── cmd/            # 服务入口 + 工具（dbcheck/dbinit/dbmigrate/peninspect/seedadmin）
├── internal/       # config/server/router/middleware/handler/service/repository/model
│                   # + auth(JWT/匿名/重置) media(存储) mail(邮件) redis casbin
├── pkg/            # dbcfg / errs(错误码) / resp(统一响应)
├── db/             # schema.sql(基线) + seed.sql + migrations/00N_*.sql(增量)
├── frontend/       # Next.js 15.4（src/app 页面 + components + lib + themes + types）
├── scripts/        # 全部启停/构建/测试脚本（Windows 兼容）
├── discuss/        # 讨论与验收文档（需求/MVP/开发流程/阶段验收报告/状态清单）
│                   # 注：docs/（架构设计/插件开发手册）为内部文档，不随开源仓库分发
└── data/media/     # 本地媒体存储（gitignore）
```

## 📈 开发进度

**M1（基础闭环）✅ 全部完成**：双端骨架 + 双主题 → 认证 → 帖子/时间线/媒体 → 评论（楼中楼+匿名）→ 话题/搜索/通知/关注流 → 后台管理 → 移动端打磨与收尾。验收报告见 `discuss/M1.1~M1.7-验收报告.md`。

**M2（进阶功能）5/8 完成**：
- ✅ 视频发帖（mp4/mov/webm ≤200MB）
- ✅ 私信/消息（会话/未读/已读 + 通知）
- ✅ 举报/审核/敏感词/封禁（拦截 + 后台三页）
- ✅ 找回密码（邮件 + 30 分钟令牌）
- ✅ 可见性「仅关注者」（互相关注可见）
- ⏳ 后台编辑表单 / 评论隐藏恢复 / 角色调整 UI
- ⏳ 匿名身份 Redis 化（跨实例）
- ⏳ 全站维护开关 / 引导页拦截

**M3（插件系统）**：go-plugin SDK + GitHub 插件市场（.bpk 打包）——规划中
**M4（SEO + AI + 报表）**：SEO 健康度 / OpenAI 兼容多供应商 AI / 数据报表备份——规划中

> 详细进度与交接信息见 `discuss/开发状态清单.md`（每次开发会话结束点更新）。

## 📄 文档

| 文档 | 位置 |
|---|---|
| 需求文档 / MVP 规划 / 开发流程 | `discuss/`（评审稿） |
| 阶段验收报告 | `discuss/M1.1~M1.7-验收报告.md` |
| 开发状态清单（交接） | `discuss/开发状态清单.md` |
| 架构设计 / 插件开发手册 | 内部文档（`docs/`，不随开源仓库分发） |

## 📝 说明

- **设计稿**（`boke.pen` / `UI设计/` 约 90MB）为项目设计依据，体积过大不随仓库分发；需要时可联系作者获取，或使用 `./scripts/pen-inspect.sh` 提取画板文案辅助开发。
- 所有页面均对照设计稿开发（布局/配色/文案/交互），设计稿未覆盖处记录于验收报告。
- 代码注释为中文，遵循函数式优先、强类型、DRY/KISS/YAGNI 原则。

## 📄 License

[MIT](LICENSE) — 自由使用与修改，保留署名。
