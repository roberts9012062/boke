# AI 统一接口 + 设置页补全 - 验收报告

> 批次：AI 统一接口（完整蓝图）+ 设置页功能补全
> 完成日期：2026-08-13
> 依据：docs/architecture.md 第 7 章「AI SDK 模块设计」

---

## 一、目标与结论

**目标**（已与用户确认）：
1. **AI 统一接口**按架构蓝图完整实现：Provider 多态接口 + Manager 路由 + SSE 流式 + Embedding + HTTP 通用接口。
2. **设置页补全**：用量费用折算、智能回复助手场景、SEO 建议场景。

**结论：验收通过。** 后端编译 + 13 项内核单测通过、前端生产构建通过、数据库迁移幂等执行、接口端到端冒烟全部验证通过。

---

## 二、后端改造

### 1. AI 内核（`internal/ai`，纯内核无 DB 依赖）

| 文件 | 说明 |
|---|---|
| `types.go`（新） | `Message` / `ChatRequest` / `ChatStream` / `StreamChunk` / `EmbeddingRequest` / `EmbeddingResult` / `Provider` 接口 |
| `client.go`（扩展） | 新增 `ChatMessages`（统一契约）、`Embedding`（向量嵌入）；抽 `post` 通用方法 |
| `stream.go`（新） | SSE 流式解析（`ChatStream` 逐 chunk 返回增量，遇 `[DONE]`/EOF 结束） |
| `provider.go`（新） | `OpenAICompatProvider` 实现 `Provider` 接口（Name/Chat/ChatStream/Embedding） |
| `manager.go`（新） | `Manager` 供应商注册表 + 按名路由的统一 Chat/ChatStream/Embedding 入口 |
| `router.go` | 路由输入结构体改名 `ProviderCandidate`（避免与 `Provider` 接口重名） |

### 2. service 统一推理 + 费用 + 两场景（`internal/service`）

- **`ai_invoke.go`（新）**：统一推理核心——`resolveProviderByModel` / `buildProvider` / `chatProvider` / `chatStreamProvider` / `embedProvider` / `calcCost`，消除「runTask 与 Generate 两条重复路径」。
- **`ai.go`**：`Generate` 改走统一 `chatProvider`；新增 `GenerateStream`、`Embedding`；`AiProviderDTO/Input` 增 `price_input/price_output` 单价字段（校验非负）。
- **`ai_scenes.go`**：`runTask` 改走统一 `chatProvider`（保留任务路由与 ai.before/after_generate 钩子）；新增 `TaskReplyAssistant`（reply.assistant）、`TaskSeoAdvice`（seo.advice）两任务；新增 `GenReplyAssistant`（续写/润色/翻译，action 校验）、`GenSeoAdvice`（返回标题/描述/关键词结构化建议）。
- **费用折算**：`calcCost = tokens_in/1e6*price_input + tokens_out/1e6*price_output`，落库时写入 `ai_usage.cost`。

### 3. repository 扩展（`internal/repository/ai.go`）

- `AiProvider` 增 `PriceInput/PriceOutput`；List/Find/Create/Update SQL 增两列。
- `AiUsageSummary` 增 `TodayCost/TotalCost`；`AiDayStat` 增 `Cost`；`Summary`/`StatsByDay` SQL 增费用聚合。

### 4. handler + 路由（`internal/handler/ai.go` + `router.go`）

新增接口（均挂 `/api/v1/admin/ai`，鉴权 + Casbin `ai` 域）：
- `POST /generate` — 统一非流式生成
- `POST /generate/stream` — 统一流式生成（SSE，逐 chunk JSON 编码，规避换行歧义）
- `POST /embedding` — 统一向量嵌入
- `POST /gen/reply?post_id=&action=` — 智能回复助手
- `POST /gen/seo-advice?post_id=` — SEO 建议

### 5. 数据库迁移（`db/migrations/016_ai_full.sql`）

- `ai_providers` 增 `price_input`/`price_output`（NUMERIC(12,6)，默认 0）。
- `ai_tasks` 种子增 2 任务：`reply.assistant`、`seo.advice`（`ON CONFLICT DO NOTHING` 幂等）。

> 注：文件名曾临时用 `011_ai_full.sql`，与既有 `011_roles_rbac.sql` 编号冲突，已改名为 `016_ai_full.sql` 并清理残留迁移记录。

---

## 三、前端改造（`frontend/src`）

| 文件 | 说明 |
|---|---|
| `lib/api-ai.ts` | 增 `apiAiGenerate` / `apiAiGenerateStream`（fetch+ReadableStream 消费 SSE）/ `apiAiEmbedding` / `apiAiGenReply` / `apiAiGenSeoAdvice`；`AiProviderDTO/Input` 增单价；`AiUsageSummary/AiDayStat` 增费用字段 |
| `lib/api.ts` | 导出 `authHeaders()`（供流式 SSE 复用鉴权头） |
| `components/admin/ai/providers-tab.tsx` | 表单增「输入/输出单价」两输入框；表格增单价列 |
| `components/admin/ai/tasks-tab.tsx` | `TASK_META` 增「智能回复助手」「SEO 建议」两任务说明 |
| `components/admin/ai/usage-tab.tsx` | 启用费用折算——汇总卡增「今日/累计费用」；移除「暂未启用」说明 |
| `components/admin/post-edit-panel.tsx` | SEO 面板标题旁增「AI 生成建议」按钮，回填标题/描述/关键词 |
| `components/admin/post-edit-form.tsx` | 正文区增「AI 续写/润色/翻译」三按钮，结果追加到正文末尾 |

---

## 四、插件复用确认

插件 `GenerateAI` 已走 `AiService.Generate`，重构后自动复用统一 `chatProvider` 路径（含费用落库），**无需改 SDK/插件协议**；`GetAIModels` 自动带新单价字段。

---

## 五、验证结果

1. **后端编译**：`go build ./...` 通过。
2. **内核单测**：13 项全过（路由/加解密/客户端 Chat/Embedding/Stream/Manager 注册与路由）。
3. **前端构建**：`scripts/build-frontend.sh` 通过。
4. **数据库迁移**：`scripts/migrate.sh` 幂等执行 `016_ai_full.sql`。
5. **接口冒烟**（`curl` + Playwright）：
   - 供应商列表含 `price_input/price_output` ✅
   - 任务列表含 `reply.assistant`/`seo.advice` ✅
   - 用量汇总含 `today_cost/total_cost` ✅
   - 统一 `generate`/`embedding`/`gen/reply`/`gen/seo-advice` 错误处理正确（未配 Key → code 6002 明确提示；非法 action → code 2001）✅
   - 流式接口未配 Key 时快速返回 JSON 错误（0.157s），不挂起 ✅
   - AI 设置页三 Tab（供应商单价列/两新任务/累计费用）Playwright 渲染验证通过，无 JS 错误 ✅

---

## 六、测试供应商实测记录（重要）

用户提供第三方中转 `https://ds.02b.top/v1` 进行真实调用验证，结果如下：

- **我方代码链路完全打通**：后端统一 `generate` 接口真实调用该供应商，返回 `code=0`（5/5 成功），响应解析正确。
- **该中转为非标准 OpenAI 服务**，存在两处兼容性问题：
  1. **不支持 `system` 角色消息**——带 `system` 角色即返回 `500 Internal Server Error` 或将内容吞掉（即使纯英文）。
  2. **不支持中文内容**——`user` 消息含中文即返回 `500`（纯英文 `user` 正常）。

**结论**：这是第三方中转服务的非标准行为，**非我方代码问题**。我方内核按标准 OpenAI 协议实现（httptest mock 单测已证明请求格式正确），且本博客业务场景（摘要/标签/评论审核/SEO 建议）均为中文，该中转无法承载。建议使用正规 OpenAI 兼容供应商（如 DeepSeek 官方 `api.deepseek.com`、通义千问 DashScope 等，均完整支持 `system` 角色与中文）。

---

## 七、遗留与后置

1. **流式用量落库**：SSE 长连接的 token 用量需上游 `usage` 流式块（多数供应商在最后一块返回），当前流式调用暂不落 `ai_usage`（观测不影响主流程），后续增强。
2. **Embedding 无业务场景消费**：作为蓝图要求的统一接口能力预留（YAGNI），未强行接入无需求场景。
3. **`/admin/ai` 页面无设计稿**（沿用自设计），后续补设计稿走查纠偏（与 M4-AI 验收遗留一致）。

---

## 八、改动文件清单

**后端新增**：`internal/ai/types.go`、`stream.go`、`provider.go`、`manager.go`、`internal/service/ai_invoke.go`、`db/migrations/016_ai_full.sql`

**后端修改**：`internal/ai/client.go`、`router.go`、`ai_test.go`、`internal/repository/ai.go`、`internal/service/ai.go`、`ai_scenes.go`、`internal/handler/ai.go`、`internal/router/router.go`、`db/schema.sql`

**前端修改**：`lib/api-ai.ts`、`lib/api.ts`、`components/admin/ai/providers-tab.tsx`、`tasks-tab.tsx`、`usage-tab.tsx`、`components/admin/post-edit-panel.tsx`、`post-edit-form.tsx`
