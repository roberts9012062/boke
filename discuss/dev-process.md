# 月言博客 · 开发流程文档

| 项目 | 内容 |
|---|---|
| 文档版本 | v1.0（评审稿） |
| 编写日期 | 2026-08-10 |
| 关联文档 | 需求文档（discuss/requirements.md）、MVP 规划（discuss/mvp-plan.md）、架构设计（docs/architecture.md）、项目规则（AGENTS.md） |
| 状态 | 待用户审查 → 审查通过后按 M1.1 起实施 |

---

## 1. 技术栈与环境（已确认版）

| 层 | 选型 | 版本/说明 |
|---|---|---|
| 后端 | Go + Gin | Go 1.26（本机已装）；Gin 最新稳定 |
| 数据库 | PostgreSQL | 18.4（192.168.6.58:5438，库 Blog 已建 26 表） |
| ORM | pgx v5 | 已引入；查询走原生 SQL + 结构化扫描（轻量、可控） |
| 鉴权 | golang-jwt/jwt v5 | access 15min + refresh 7d |
| 权限 | Casbin | MVP 两级角色（admin / user），gorm adapter 后置（M2 完整 RBAC） |
| 缓存 | Redis | 192.168.6.33:6379（.env 已配置；MVP 用于限流/角标，P1 再上页面缓存） |
| **前端** | **Next.js 15.4 + React 19 + Tailwind CSS v4 + TypeScript** | 用户确认；模块系统 ESM，禁 CommonJS |
| 媒体 | 本地磁盘 + 外部 URL 双支持 | `data/media/` 本地存储；URL 直存；均写 media_assets |
| 主题 | CSS 变量设计令牌 | 冷月（dark）/ 薄雾（light） |

## 2. 仓库与目录结构（monorepo）

```
boke/
├── cmd/
│   ├── server/               # 主服务入口（Gin 装配）
│   ├── dbcheck/              # 连接检查（已就绪）
│   ├── dbinit/               # 数据库初始化（已就绪）
│   └── peninspect/           # 设计稿文本提取（已就绪）
├── internal/
│   ├── config/               # 配置加载（.env / 环境变量）
│   ├── server/               # Gin 引擎、中间件链、优雅退出
│   ├── router/               # 路由注册（前台 /admin）
│   ├── middleware/           # 请求ID / 恢复 / CORS / 限流 / JWT / 角色
│   ├── handler/              # 控制器层（参数绑定、响应组装）
│   ├── service/              # 业务逻辑层（事务边界）
│   ├── repository/           # 数据访问层（pgx 封装）
│   ├── model/                # 结构化类型 + DTO
│   ├── auth/                 # JWT 签发/刷新
│   ├── media/                # 上传/存储/校验
│   ├── audit/                # 审计日志写入
│   └── bootstrap/            # 启动装配
├── pkg/
│   ├── dbcfg/                # 数据库配置（已就绪）
│   ├── errs/                 # 错误码
│   └── resp/                 # 统一响应
├── db/
│   ├── schema.sql            # 初始 26 表（已就绪）
│   ├── seed.sql              # 基础种子（已就绪）
│   └── migrations/           # 增量迁移（按序编号 001_*.sql）
├── frontend/                 # Next.js 15.4 应用
│   ├── src/
│   │   ├── app/              # App Router 页面
│   │   ├── components/       # 组件（按 2.3 规范分层）
│   │   ├── themes/           # 冷月/薄雾设计令牌（tokens.css）
│   │   ├── lib/              # api client / auth / utils
│   │   └── types/            # 强类型定义（与后端 DTO 对应）
├── scripts/                  # 全部启停脚本（规则强制）
├── data/
│   └── media/                # 本地媒体存储（gitignore）
├── logs/                     # 运行日志（gitignore）
├── docs/                     # 正式文档
├── discuss/                  # 讨论评审文档（当前三份）
├── .env                      # 环境变量（gitignore，勿提交）
└── AGENTS.md                 # 项目规则
```

## 3. 开发规范（执行 AGENTS.md 全部条款）

### 3.1 Go 侧

- 中文注释、注释详细；函数式优先（纯函数不修改入参/全局状态）；OOP 仅用于连接器/外部接口。
- 严格类型；无默认参数（全部显式）；复杂结构先定义类型；import 置顶。
- 单函数单一职责；无标记参数；文件 ≤400 行；每层目录文件 ≤8 个（超出拆子目录）。
- 分层单向依赖：handler → service → repository；handler 无业务判断。
- 新代码前先查已有逻辑复用（如 pkg/dbcfg）。

### 3.2 前端（Next.js 15.4 / React 19 / Tailwind v4 / TS）

- 强制 ESM（禁 CommonJS）；尽量 TypeScript，禁止 any（必须时先征求用户同意）。
- 数据结构强类型：`src/types/` 与后端 DTO 一一对应。
- 文件 ≤300 行；组件单文件单职责；hooks 抽公共逻辑。
- Tailwind v4 用 CSS-first 配置（`@import "tailwindcss"` + `@theme` 定义设计令牌）。

### 3.3 通用

- 坏味道自查（僵化/冗余/循环依赖/脆弱/晦涩/数据泥团/过度设计），发现即提出优化建议。
- 文档全部中文；正式文档 → docs/；讨论评审 → discuss/。

## 4. 启停与脚本流程（规则强制）

| 脚本 | 用途 | 状态 |
|---|---|---|
| `scripts/check-db.sh` | 数据库 + GitHub 连接检查（退出码 0/2 通过） | ✅ 已就绪 |
| `scripts/init-db.sh` | 初始化数据库（先检查后建库建表种子） | ✅ 已就绪 |
| `scripts/pen-inspect.sh` | 查看 boke.pen 画板/文案 | ✅ 已就绪 |
| `scripts/dev-server.sh` | 启动后端（go run cmd/server，含日志落盘） | M1.1 新增 |
| `scripts/dev-frontend.sh` | 启动前端（frontend/ 内 next dev，含日志落盘） | M1.1 新增 |
| `scripts/migrate.sh` | 执行 db/migrations/ 增量迁移 | 需要时新增 |
| `scripts/seed-admin.sh` | 写入管理员账号（初始化后） | M1.2 新增 |
| `scripts/stop-all.sh` | 停止全部开发进程 | M1.1 新增 |

约定：
- **一切启停必须走 scripts/ 脚本**，禁止直接 `go run` / `npm` / `next` 裸命令。
- 脚本失败先修复脚本/代码，修复后仍用脚本。
- 所有服务日志统一输出 `logs/`（后端 zap file output；前端 next dev stdout 重定向）。
- 脚本内不回显 `.env` 敏感值。

## 5. 数据库开发流程

1. **初始结构**：`db/schema.sql` + `db/seed.sql`（已执行，Blog 库 26 表就绪）。
2. **增量变更**：一律新增 `db/migrations/00N_描述.sql`（可重复执行），`scripts/migrate.sh` 统一执行；**禁止直接改 schema.sql 已上线部分**（schema.sql 仅作为全新环境基线）。
3. **已确认的首个迁移**（开发前执行）：匿名评论字段（requirements.md 5.2）——`001_guest_comments.sql`。
4. 变更评审：任何 DDL 需在文档（discuss 或 docs）记录变更说明。

## 6. 设计稿对照流程（强制要求：以 boke.pen 设计稿为准开发）

**所有页面必须按照 `boke.pen` 设计稿（及 `UI设计/` 导出 PNG）开发**：布局、配色、字号、间距、文案、交互状态均与设计稿保持一致；仅当设计稿未覆盖的细节（如扩展字段、后台数据表格列）才允许自行补充，并记录说明。

- 每个页面开发前**必须先跑 `scripts/pen-inspect.sh 关键词`** 提取画板文本（布局/文案/交互要点），再对照 `UI设计/` 对应主题（冷月/薄雾）的导出 PNG。
- 双主题令牌从 `月言 · Style Guide` / `Style Guide Light` 画板提取，做成 `themes/tokens.css`。
- 画板命名规律：`D/主题/页面`（桌面 1400）/ `M/主题/页面`（移动 390）——按此查找。
- 页面完成后对照设计稿自检：布局一致性 → 文案一致性 → 交互一致性，并在阶段验收报告中说明与设计稿的差异点（如有）。

## 7. 迭代计划（M1 内 7 阶段）

### M1.1 双端骨架 + 双主题
- 后端：Gin 装配（config/router/middleware/统一响应/错误码/健康检查）、zap 日志（logs/）、`scripts/dev-server.sh`、`scripts/stop-all.sh`。
- 前端：Next.js 15.4 脚手架（App Router + TS + Tailwind v4）、`scripts/dev-frontend.sh`、全局布局（桌面三栏/移动底部导航骨架）、双主题令牌 tokens.css + `data-theme` 切换、请求封装（api client + 统一响应解析）、类型定义骨架。
- 数据库：执行迁移 `001_guest_comments.sql`。
- **验收**：`scripts/dev-server.sh` 起服务 → `/healthz` 200；`scripts/dev-frontend.sh` 打开首页骨架，冷月/薄雾即时切换；双端布局壳符合设计稿。

### M1.2 认证 + 用户模块
- 后端：注册/登录/登出/刷新（JWT）、bcrypt、登录限流、Casbin 两级角色（admin/user）、`/me` 与资料接口、审计写入（登录/注册）。
- 前端：登录/注册页（双端）、用户状态管理（token 持久化/静默刷新）、头像菜单、后台入口。
- 种子：管理员账号（`scripts/seed-admin.sh`，密码初始化并提示首登修改）。
- **验收**：需求 3.1 全部验收点；admin 可登录 `/admin`。

### M1.3 帖子 + 时间线 + 媒体
- 后端：帖子 CRUD（draft/published/taken_down/deleted）、时间线（全部/图/音/影过滤 + 分页）、详情、可见性、媒体上传（本地磁盘 + URL 双支持、类型/大小校验、图片压缩接口）。
- 前端：发帖中心（文字/图片/音频 + 字数统计 + 标签 + 可见性 + 草稿）、帖子卡片组件（四形态）、时间线页、详情页（灯箱/音频播放器）、发布成功页。
- **验收**：需求 3.2 / 3.3 / 3.4 验收点；上传→展示闭环。

### M1.4 评论（楼中楼 + 匿名）
- 后端：评论 CRUD（含匿名身份：guest_name + 匿名 token 签发/校验/限频）、回复（2 级）、点赞、楼数统计。
- 前端：评论区（楼中楼展开/收起、匿名昵称弹层、评论点赞、删除自己的评论）。
- **验收**：需求 3.5 全部验收点；匿名防刷生效。

### M1.5 话题 / 搜索 / 通知 / 关注流
- 后端：话题聚合（tag → 帖子）、话题列表（热门/最新/关注）、全文检索（pg_trgm + FTS）、通知（赞/评论/回复/关注/系统、未读/全部已读）、关注流 feed、user_relations 全操作。
- 前端：话题页/话题详情、搜索页、通知页（Tab + 时间分组 + 角标轮询 30s）、关注流页、个人主页（帖子/媒体/赞过 Tab）、粉丝/关注/收藏/黑名单列表、编辑资料。
- **验收**：需求 3.6 / 3.7 / 3.8 / 3.9 / 3.10 验收点。

### M1.6 后台管理
- 后端：后台鉴权（admin 角色）、仪表盘聚合接口、内容管理 CRUD（筛选/搜索/上下架）、评论管理、用户管理（封禁/角色）、站点设置读写、审计日志写入。
- 前端：后台布局（侧栏 + 建设中占位页）、后台登录、仪表盘（卡片/图表/内容分布/最近动态）、内容/评论/用户管理表格、站点设置表单。
- **验收**：需求 4.x 全部验收点；5 项可用模块闭环，其余侧栏为建设中占位。

### M1.7 移动端打磨 + 收尾
- 移动端 390px 全面适配（底部导航、发帖流程、详情、后台不要求移动端——设计稿后台仅桌面）。
- 空态/骨架/404/500/维护/无网络/引导 全部就位；主题切换完善；分享复制链接。
- 性能自查（列表接口缓存 P1）、安全自查（上传/注入/XSS/限流）、日志检查。
- 对照需求文档做全量自测清单 → 输出验收报告（discuss/）。

**每阶段完成 → 更新讨论文档 → 用户验收 → 进入下一阶段。**

## 8. 接口联调约定

- 后端先行定义 `pkg/errs` 错误码 + 统一响应 `{code,message,data,request_id}`（架构文档 11.2/11.3）。
- 前端 api client 统一封装：自动带 token、401 静默刷新一次、错误码 → Toast 文案映射。
- 联调环境：后端 `:8080`，前端 `:3000` 代理 `/api`（next.config rewrites）。
- DTO 类型：`frontend/src/types/` 与后端 `model/dto` 手工同步，字段以中文注释说明（DRY 受限场景，注释标注"与后端 dto 同步"）。

## 9. 测试与验收流程

| 层级 | 方式 | 要求 |
|---|---|---|
| 单元测试 | Go：service/repository 核心逻辑（分页、状态机、限频）；前端：纯函数/工具 | 关键逻辑必须覆盖 |
| 接口测试 | 后端 handler 层（httptest）关键接口冒烟 | 每阶段新增接口全冒烟 |
| 手工验收 | 按需求文档「验收」逐条过（双端双主题） | 每阶段末输出验收清单 |
| 端到端 | MVP 末期：注册→发帖→评论→后台 全流程人工走查 | 验收报告 |

- 脚本：测试也走 scripts/（如 `scripts/test.sh` 包装 `go test ./...` + 前端 test）。

## 10. 文档维护约定

| 文档 | 位置 | 更新时机 |
|---|---|---|
| 架构设计 | docs/architecture.md | 架构级决策变更时 |
| MVP 规划 | discuss/mvp-plan.md | 评审后冻结；重大调整时修订 |
| 需求文档 | discuss/requirements.md | 评审通过后冻结 v1.0；后续变更走变更记录表 |
| 开发流程 | discuss/dev-process.md | 流程变更时 |
| 阶段验收报告 | discuss/ | 每阶段末新增 |

- 评审通过后：三份 discuss 文档 → 用户确认后正式化（可移入 docs/ 或保留 discuss/ 供追溯，按用户要求执行）。
