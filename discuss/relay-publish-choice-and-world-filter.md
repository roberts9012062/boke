# 中继站发布选择 + 大世界分类/标签筛选（方案）

> 需求来源：用户提出——发帖/说说时选择是否公开发送到中继站；选择"是"时再选分类与标签；
> 中继站标签按分类规范化（每个分类下有哪些标签需预填好）；大世界页面支持按分类、按标签筛选。

## 一、现状与差距

| 能力 | 现状 | 差距 |
| --- | --- | --- |
| 发帖时选择是否推送中继站 | 无选择项：只要帖子公开已发布、中继站启用且配了默认分类即自动推送 | 需用户显式选择 |
| 发布分类/标签 | 分类固定用后台默认分类；标签直接复用帖子自身标签 | 需发帖时选择 |
| 中继站标签规范 | 无：标签自由传入 | 需按分类的规范标签白名单 |
| 大世界分类筛选 | 后端 ListCache 已支持 category 参数，前台无 UI | 需补前端筛选 |
| 大世界标签筛选 | 后端不支持 | 需后端 + 前端 |
| 草稿→发布推送 | Publish() 不触发中继站钩子（仅直接发帖触发），属既有缺口 | 需修复 |

## 二、设计要点

### 1. 帖子级发布选择（posts 表 3 列，迁移 027）

- `relay_publish boolean NOT NULL DEFAULT false`：用户是否选择公开发送到中继站；
- `relay_category text NOT NULL DEFAULT ''`：用户选择的中继站分类（勾选后必填）；
- `relay_tags jsonb NOT NULL DEFAULT '[]'`：用户选择的中继站标签（≤5 个）。

请求体 `CreatePostReq` / `UpdatePostReq` 增加嵌套输入 `relay: { publish, category, tags }`（避免数据泥团）。

推送判定变化：`relay_publish=true` 且帖子 `published + public` 且中继站启用才推送；
后台"默认分类"退化为申请时的意向分类，不再决定推送。

### 2. 分类→标签规范注册表（本地静态，model/relaytaxonomy.go）

中继站分类由远端 handshake 动态下发，本地无法预知；因此规范表按 12 个常见内容分类预填
约 100 个规范标签（技术/生活/随笔/摄影/音乐/影视/读书/游戏/美食/旅行/创作/科学）。

- 发帖分类候选：优先握手快照 categories（权威），快照为空时回退规范表分类键；
- 标签候选：所选分类在规范表中的标签；分类不在规范表内（远端自定义分类）时不做白名单校验，
  仅校验数量（≤5）与长度（≤20 字）。

### 3. 新 API

- `GET /api/v1/relay/publish-options`（登录）：`{ enabled, categories, taxonomy, default_category }`
  ——发帖表单渲染"是否发送中继站 + 分类 + 标签"用；taxonomy 即规范表。
- `GET /api/v1/relay/contents?category=&tag=&before=&limit=`：新增 tag 参数
  （SQL：`payload_json->'tags' @> to_jsonb($n::text)`）。

### 4. 校验解耦

PostService 不依赖 RelayRepo：装配期注入校验钩子 `SetRelayChoiceCheck(fn)`（与既有
SetRelayHook 同模式），由 RelayService 提供 `ValidatePublishChoice`。

### 5. 大世界筛选 UI

- 新组件 `world-filters.tsx`：分类 chips（全部 + 分类），选中分类后展示该分类标签 chips
  （全部 + 规范标签）；点击后带参重拉；
- 集成两处：`/world` 独立页（移动端入口）与首页时间线「大世界」Tab。

## 三、涉及文件

后端：`db/migrations/027_relay_publish_choice.sql`、`internal/model/{post,relaytaxonomy}.go`、
`internal/repository/{post,relay}.go`、`internal/service/{post,relay}.go`、
`internal/handler/relay.go`、`internal/router/router.go`、`internal/server/server.go`。

前端：`src/types/api.ts`、`src/lib/api-relay.ts`、`src/components/compose/relay-publish-select.tsx`（新）、
`src/components/world-filters.tsx`（新）、`src/app/compose/page.tsx`、`src/app/world/page.tsx`、`src/app/page.tsx`。

## 四、兼容性

- 既有帖子 `relay_publish` 默认 false：升级后不再自动推送（正是需求所要的行为变化）；
- `default_category` 字段保留（申请流仍用），推送不再依赖；
- 草稿→发布（`POST /posts/:id/publish`）补触发中继站钩子，勾选过发送的草稿发布后正常推送。

## 五、验证记录（2026-09-18）

| 项 | 结果 |
| --- | --- |
| `go build ./...` / `go vet` | 通过 |
| `go test ./...`（含新增 relaytaxonomy_test） | 全绿 |
| `scripts/build-frontend.sh`（next build 生产构建） | 成功 |
| `scripts/migrate.sh`（迁移 027 应用） | 成功 |
| `GET /relay/taxonomy`（公开规范表） | 返回 12 分类 + 全部规范标签 |
| `GET /relay/contents?category=&tag=`（JSONB `@>` 过滤） | 200 |
| 发帖勾选发送 + 分类不在握手范围 | 拒绝（code 2003） |
| 发帖勾选发送 + 标签不在分类规范表 | 拒绝（code 2003） |
| 发帖勾选发送（技术 + 前端 + 人工智能） | 成功，mock 中继站收到推送且 category/tags 与所选一致 |
| 发帖不勾选 | 成功，无推送 |
| 草稿（勾选）→ 发布草稿 | mock 收到推送（既有缺口已修复） |
| 编辑关闭发送 → 详情回填 `relay` | 生效 |

冒烟环境已还原：relay_config 恢复原值、冒烟帖/用户已清理、临时工具已删除、服务已停止。
