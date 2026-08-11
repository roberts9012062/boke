# M4-AI 验收报告（AI SDK：OpenAI 兼容多供应商 + 内置三场景）

| 项目 | 内容 |
|---|---|
| 验收日期 | 2026-08-11 |
| 验收范围 | M4-AI：AI 内核（OpenAI 兼容客户端/任务路由/用量落库）+ 三个内置场景（摘要/自动标签/评论审核）+ 后台 AI 设置页（供应商/任务/用量）+ 审核队列「高风险」联动 |
| 设计稿依据 | 无 AI 画板；AI 设置页参照《SEO设置》画板后台模式自行设计；审核队列「高风险 3」统计项为设计稿原有（此前用「工单总数」替代，本次激活） |
| 验收方式 | ai 包单测 9 项 + 后端冒烟 22 项 + verdict 闭环冒烟 6 项 + 回归冒烟 8 项 + GUI（Playwright）18 项 + go build/vet/test.sh + 前端生产构建 |
| 结论 | ✅ 完成；侧栏新增「AI 设置」项；审核队列「高风险」卡激活（设计稿对齐） |

---

## 一、AI 内核（internal/ai/，零第三方依赖）

- **client.go**：OpenAI 兼容 Chat Completions 客户端（net/http 直连，Bearer 认证；解析 content + usage token；HTTP 非 2xx 透出上游错误信息便于排查）
- **router.go**：`RouteProvider` 纯函数路由（enabled 且 priority 最小）；无可用供应商 → 明确错误提示
- **crypto.go**：AES-256-GCM API Key 加解密（密钥 = sha256(config.AIKeySecret) 派生；config 新增 `AI_KEY_SECRET`，未配置回退 JWT_SECRET，注释说明生产建议独立配置）
- 单测 9 项全过：路由选择/跳过禁用/空路由、加密往返/错钥/非法密文、HTTP mock（成功/401 透出/空 choices）

## 二、数据层与迁移 010

- `reports` 增 `source` 列（user 人工举报 / ai AI 审核标记）——审核队列「高风险」统计与「放行/删除」复核的判别依据
- 种子 5 个默认供应商（deepseek/qwen/kimi/glm/openai，含默认 base_url 与模型，priority 1-5，api_key 为空待填）
- 种子 3 条内置任务（post.summary/post.tags/comment.review，中文提示词 + max_tokens，provider NULL = 自动路由）
- 迁移幂等（WHERE NOT EXISTS / ON CONFLICT DO NOTHING），已验证库内数据

## 三、三个内置场景（统一 runTask 流程：查任务 → 校验启用 → 路由 → 解密 Key → 调用 → 用量落库）

| 场景 | 触发点 | 行为 |
|---|---|---|
| **摘要** | 后台编辑页 SEO 面板「AI 生成摘要」按钮 | 标题+正文前 4000 字 → `seo_meta.summary` 落库（新增 SeoRepo.UpdateSummary，不动其他字段）→ 回填展示 |
| **自动标签** | 后台编辑页标签区「AI 生成标签」按钮 | AI 输出 JSON 数组（容错解析）→ 建议 chips → 用户点击合并进输入框（≤5 个，不自动写入） |
| **评论审核** | 新评论异步自动预审 + 评论管理「AI 审核」手动批量 | AI 判定 `{risk, reason}` → high → 评论隐藏 + 写 AI 来源工单（审核队列待处理+1）；低风险/解析失败放行（不误伤） |

- **异步预审解耦**：CommentService 注入 `CommentReviewer` 接口（AiService 实现，业务零感知）；Create/Reply 后 fire-and-forget goroutine（recover 兜底，失败静默不阻塞评论）
- **审核队列联动**：统计条「工单总数」→「高风险」（设计稿 4 卡片对齐：待处理/高风险/今日已审/平均耗时）；AI 工单行显示「放行/删除」按钮（`POST /admin/reports/:id/verdict`：allow=恢复可见+resolved，delete=删除+resolved）；人工工单沿用解决/驳回，两套互不干扰；处理操作后统计条即时刷新（顺带修复原有 handleStatus 后不刷新问题）

## 四、后台 AI 设置页（/admin/ai，侧栏 13→14 项）

- **供应商 Tab**：表格（名称/接口/模型/优先级/Key 状态/启用）+ 新增/编辑弹层（复用 components/ui/modal + switch）+ 测试连接（max_tokens=1 连通性）+ 删除（任务引用自动置空）；API Key 掩码回显、编辑留空不改
- **任务配置 Tab**：三张任务卡（绑定供应商下拉/模型/最大 token/提示词 textarea/启用开关/保存配置；草稿态，保存后生效）
- **用量统计 Tab**：汇总卡（今日调用/今日 token/累计调用/累计 token）+ 近 7 日柱状图（SVG 零依赖，复用 M1.7 图表模式；后端 StatsByDay 按日补零）

## 五、实测记录

| 测试点 | 结果 |
|---|---|
| ai 包单测（路由/加密/HTTP mock）9 项 | ✅ |
| 后端冒烟 22 项（5 供应商种子/3 任务/用量结构/摘要标签未配 Key 6002 拦截/任务停用 3002/供应商 CRUD+加密留空不改/测试连接错误透出/任务配置更新/high_risk 字段/verdict 2002/批量审核空列表 2001） | ✅ |
| verdict 闭环 6 项（高风险统计=2 → allow 放行 → 重复处理 3002 → delete 删除 → 不存在工单 2002 → 处理后统计回落 0） | ✅ |
| 回归 8 项（帖子详情/时间线/评论发布（AI 预审注入后正常）/评论列表/评论删除/审核队列含 source/评论管理） | ✅ |
| GUI（Playwright）18 项：/admin/ai 三 Tab 渲染与切换、供应商表格、任务三卡+3 开关、用量卡+趋势图、审核队列高风险卡、编辑页 AI 摘要/AI 生成标签按钮、评论管理标题 | ✅ |
| go build / go vet / scripts/test.sh / 前端生产构建 | ✅ |

## 六、过程问题

1. **seed 验证环境变量问题**：git bash 直接 `source .env` 不导出变量给 Windows 子进程（go.exe），须 `set -a`——与 scripts/*.sh 一致（非代码问题）
2. **8080 端口残留进程**：旧 server.exe 未清干净导致 healthz 返回错误页——stop-all.sh 反查端口清理后正常
3. **GUI IAB 受限**：ZCode IAB webview 无法挂载（guest not attached），改用项目自带 Playwright（channel: chrome）做 DOM 断言 + 截图（gui-test-screenshots/ 留档）
4. **冒烟脚本断言失误 1 处**：TestProvider 无 body 参数（空 body 也执行连通性测试）——修正脚本预期，非代码缺陷

## 七、差异记录

- 设计稿无 AI 画板：/admin/ai 页面自行设计（参照《SEO设置》模式），后续设计稿补充时再走查纠偏
- AI 用量「费用折算」未启用（ai_usage.cost 记 0；供应商单价字段后置）
- 评论预审为「高风险自动隐藏+人工复核」模型；自动删除/自动驳回（高风险直接删）未启用（误伤风险，人工兜底更稳妥）
- 前台发帖中心（/compose）AI 摘要/标签按钮未接入（与 M4-SEO「前台 SEO 面板后置」一致，本次聚焦后台）
- 敏感词「review 级别进入审核队列」未联动（现有 forbidden 直接拦截已满足 MVP；review 联动可后置）

## 八、回归与规范

- `go build` / `go vet` / `scripts/test.sh` 全通过（新增 internal/ai 包 9 单测）
- 冒烟/回归脚本固化：`scripts/smoke_ai.py`（22 项）、`scripts/smoke_ai_regression.py`（8 项）
- 新增文件：internal/ai/{crypto,client,router,ai_test}.go、repository/ai.go、service/{ai,ai_scenes}.go、handler/ai.go、lib/api-ai.ts、components/admin/ai/{providers-tab,tasks-tab,usage-tab,ai-summary}.tsx、app/admin/ai/page.tsx、db/migrations/010_ai_seed.sql
- 修改文件：config.go、router.go、server.go、repository/{moderation,seo}.go、service/{moderation,comment}.go、handler/moderation.go、api.ts、layout.tsx、nav-icons.tsx、audit/page.tsx、comments/page.tsx、post-edit-form.tsx、post-edit-panel.tsx
- 行数规范：全部新文件 Go ≤400 行 / 前端 ≤300 行 ✓
