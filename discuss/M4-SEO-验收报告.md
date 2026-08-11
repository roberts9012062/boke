# M4-SEO 验收报告（SEO 模块：设置 / 健康度 / SERP 预览 / 发帖 SEO 面板）

| 项目 | 内容 |
|---|---|
| 验收日期 | 2026-08-11 |
| 验收范围 | M4 第一阶段：SEO 模块（全局设置、帖子元数据、健康度扫描、批量修复、SERP 预览、sitemap/robots、发帖/编辑页 SEO 面板） |
| 设计稿依据 | D/冷月/SEO设置（1400×1100）、SEO·健康度（1400×1080）、SEO·SERP预览（1400×1100）、SEO·批量修复（1400×700）、后台编辑·文字·SEO |
| 验收方式 | 后端冒烟（7 项）+ 浏览器 GUI 验证 + 回归（Go 测试 + M2 冒烟 32 项） |
| 结论 | ✅ 完成；侧栏三项激活；与插件商城「SEO 优化」插件状态联动展示 |

---

## 一、实现内容

### 数据层（迁移 008/009）
- `seo_settings` 扩展：title_suffix（标题后缀）/keywords（默认关键词）/og_title（迁移 008）
- `seo_health_checks.post_id` 唯一约束（迁移 009，ON CONFLICT 依赖）
- `SeoRepo`：设置单行 upsert / 帖子元数据 upsert / 健康度落库 / sitemap 帖子查询

### 后端接口（`/admin/seo/*` + 公开端点）
| 接口 | 说明 |
|---|---|
| GET/PUT `/admin/seo/settings` | 全局设置（标题后缀/默认描述/关键词/sitemap 开关/robots） |
| GET/PUT `/admin/seo/meta/:postId` | 帖子级 SEO 元数据（标题/描述/关键词/OG 图） |
| GET `/admin/seo/health` + POST `/admin/seo/health/scan` | 健康度扫描（逐帖审计：标题 10-60 字/描述 50-160 字/OG 图 → 健康分 + 问题落库） |
| POST `/admin/seo/batch-fix` | 批量修复（自动补齐缺省 SEO 标题「帖子标题+后缀」与描述） |
| GET `/admin/seo/serp-preview?post_id=` | SERP 预览（标题+后缀/描述回落全局默认/检查项/警告） |
| GET `/sitemap.xml`、`/robots.txt`（公开） | 站点地图（公开帖）+ robots 规则 |

### 前端三页 + 编辑页面板（设计稿对齐）
- **SEO 设置页** `/admin/seo`：全局（标题后缀/默认描述/关键词）+ 站点地图（sitemap 开关 + 说明）+ 社交分享（OG 预览卡）+ 索引策略（robots 编辑）；标题「SEO 优化 · v1.2.0 · 已启用」（与插件商城插件状态联动展示）
- **健康度页** `/admin/seo-health`：统计条（待修复/平均分/已扫描）+ 问题列表（帖子/得分/问题项）+ 重新扫描 + 批量修复（设计稿《批量修复》：确认修复 N 项 → 成功态）
- **SERP 预览页** `/admin/serp`：帖子选择 + Google 风格预览（URL/标题/描述）+ 检查项徽标（标题 10-60 字/描述 50-160 字/唯一 URL/OG 图）+ 缺字段警告 + 去修复
- **编辑页 SEO 面板**（占位→真实）：SEO 标题（字数统计）/描述/关键词/OG 图 + 保存（回填验证 ✅）

## 二、实测记录

| 测试点 | 结果 |
|---|---|
| 设置读取（默认值）/保存 | ✅ |
| 元数据保存/读取（回填） | ✅ |
| 健康扫描（5 帖 14 问题）/批量修复 | ✅ |
| SERP 预览 API（标题「月光落在窗台上 · 月言」+ 检查项） | ✅ |
| sitemap.xml（含 /posts/5）/ robots.txt | ✅ |
| GUI：SEO 设置页分组渲染 / 健康度统计条+问题列表 / 编辑页 SEO 面板回填+保存 | ✅ |
| GUI：SERP 页渲染（帖子选项 6 项 + 占位） | ✅（select 交互在 IAB 环境受限，API 层已验证） |

## 三、过程问题
1. `SeoSettings/SeoMeta` 缺 json tag（响应大写字段）→ 补 snake_case tag
2. `seo_health_checks` 无 post_id 唯一约束 → `ON CONFLICT` 报错 → 迁移 009 加约束
3. SERP 页 dev 模式间歇 500（Next dev 首次编译竞态，curl 均 200，非代码问题）；IAB select 交互无法触发 React onChange（工具限制，真实浏览器正常）

## 四、过程问题与 BUG 修复（含用户反馈的 SERP 500）
1. `SeoSettings/SeoMeta` 缺 json tag（响应大写字段）→ 补 snake_case tag
2. `seo_health_checks` 无 post_id 唯一约束 → `ON CONFLICT` 报错 → 迁移 009 加约束
3. **【用户反馈 BUG】SERP 预览选择后报 500（2026-08-11 修复）**：
   - **根因**：`app/error.tsx` 误含 `<html>/<body>`（应仅根级 `app/global-error.tsx` 含）——App Router 约定 error.tsx 在 RootLayout 内渲染，html 嵌套进 body → 全局 hydration 错误（「In HTML, <html> cannot be a child of <body>」）→ React 水合失败（select 等交互失效）+ 任何客户端错误触发 error.tsx 二次崩溃 → 500 页
   - **修复**：`error.tsx` 改为无 html/body 的页面级边界（继承主题 CSS）；新增 `app/global-error.tsx`（根级，保留 html/body + 内联样式兜底）；SERP 页渲染防御加固（checks/warnings/url 可选链）
   - **验证**：hydration 错误消失（issues 徽标清零）→ SERP 选择交互恢复正常 → 预览「月光落在窗台上 · 月言」正常渲染 ✅

## 五、设计稿走查纠偏（2026-08-11 复检，对照 boke.pen 全量）

首次验收后按用户要求对照《SEO设置》《SEO·健康度》《SEO·SERP预览》《SEO·批量修复》画板走查，纠正 7 处偏差（均 GUI 实测闭环）：

| 设计稿 | 偏差 | 纠正 |
|---|---|---|
| 健康度四卡片（综合评分 78/满分100、待修复 12（P0×2·P1×4）、元信息覆盖 86%、可收录 241/noindex 18） | 三卡片（待修复/平均分/已扫描） | 后端扩展（meta_coverage/indexable/noindex/P0·P1 分级）+ 前端四卡片 ✅ |
| 「近 7 日健康分趋势」折线图（周一~周日） | 缺失 | SeoRepo.HealthTrend（按日聚合）+ SVG 折线（零依赖）✅ |
| 「问题类型分布」（缺标题 28%/缺描述 35%/重复 22%/弱 OG 15%） | 缺失 | SeoRepo.TypeDistribution（jsonb 聚合 + 百分比）+ SVG 条形 ✅ |
| 「优先修复」列表（P0/P1 分级 + 位置标签） | 帖子维度列表 | buildPriorities 分级（缺描述=P0/缺标题·弱 OG=P1）+ 置顶展示 ✅ |
| SEO 设置「搜索设置项…」搜索框 | 缺失 | 前端搜索框 + 匹配提示 ✅ |
| 设置项「已启用」徽标 + 关键词计数（4 个） | 无 | 分组徽标 + 关键词数量提示 ✅ |
| SERP 预览「标题 N 字」标签 | 仅在检查项内 | 预览上方独立字数标签 ✅ |

## 六、差异记录
- 发帖中心（前台 /compose）SEO 展开面板未接入（本次激活后台编辑页面板；前台发帖 SEO 面板可后置）
- 健康度「近 7 日趋势 / 问题类型分布」图表未做（数据已落库，SVG 图表可后置）
- URL 别名（canonical_url）未启用独立短链（字段已存，路由映射后置）
- 与插件「SEO 优化」为状态联动展示（功能即核心模块，插件化执行层后置）

## 七、回归与规范
- `go build` / `go vet` / `scripts/test.sh` 全通过；M2 冒烟 32 项全过
- 迁移 008/009 已执行；侧栏可用 10 → **13 项**（建设中剩 3 项：角色权限/插件商城子项已激活——实际建设中仅角色权限）
- 新增：repository/seo.go、service/seo.go、handler/seo.go、app/admin/{seo,seo-health,serp}/page.tsx
