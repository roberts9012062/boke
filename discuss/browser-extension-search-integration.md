# 方案备忘：AI 联网搜索对接（SearXNG 自建）

> 状态：**等待后端接口**。后端正在集成自建 SearXNG，完成后会自动出现在开放接口目录（`/api/v1/open/*`），插件端届时按下表对接，本备忘即为对接清单。

## 约定（用户确认）

- 搜索源：站点自建 SearXNG（服务器自托管，无第三方 API 费用）；
- 对外接口由 boke 后端自动集成进开放网关，插件不做任何抓取/解析搜索引擎的实现（明确否决的方案④）。

## 后端接口预估形态（以最终目录为准，对接时校正）

按 `ai.assist` 的既有模式推测，大概率是以下二选一：

- A. `ai.assist` 新增 action：`POST /api/v1/open/ai/assist` + `action: "search"`；
- B. 独立端点：`POST /api/v1/open/ai/search`，参数 `q`（+ 可选 `page_size`）。

响应预估：`{ answer: 带引用的回答文本, sources: [{title, url, snippet}] }`（若后端直接返回检索结果列表，插件端拼提示词走现有 `ai.chat`）。

## 插件端对接清单（接口就绪后执行）

1. `shared/api/endpoints.ts`：新增搜索调用封装（超时按慢任务 90s 分级）；
2. `AiChatTab` 能力卡「联网搜索」`ready: true`，tapCapability 分支：
   - 输入框有内容 → 直接以输入为问题发起搜索；
   - 无内容 → 聚焦输入框提示输入；
3. 消息流：user 气泡显示「🔍 {问题}」；assistant 气泡走现有 Markdown 渲染，若响应含 `sources` 在气泡下方渲染来源列表（标题 + 外链，样式参照书签条目行）；
4. 历史会话与提示词系统无需改动（消息结构复用）；
5. 版本递增 + CHANGELOG 登记「联网搜索上线（站点自建 SearXNG）」。

## 验证要点

- 中文/英文关键词各一；空结果与超时文案；来源链接可点；
- 与提示词（角色设定）叠加发送时 system 注入不冲突；
- 双浏览器（Chrome/Edge）走一遍验证清单（手册 §12）。
