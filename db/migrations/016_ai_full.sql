-- ============================================================
-- 迁移 011：AI 完整蓝图扩展（统一接口 / 流式 / 嵌入 / 费用折算 / 2 新场景）
-- 说明：
--   1. ai_providers 增加单价列（输入/输出，元/百万 token），供用量费用折算。
--   2. ai_tasks 新增 2 条内置任务：reply.assistant（智能回复助手）、seo.advice（SEO 建议）。
-- 幂等：可重复执行。
-- ============================================================

-- 1. 供应商单价（费用折算用；默认 0 = 未配置单价，费用记 0）
ALTER TABLE ai_providers ADD COLUMN IF NOT EXISTS price_input NUMERIC(12, 6) NOT NULL DEFAULT 0;
ALTER TABLE ai_providers ADD COLUMN IF NOT EXISTS price_output NUMERIC(12, 6) NOT NULL DEFAULT 0;
COMMENT ON COLUMN ai_providers.price_input IS '输入单价（元/百万 token）';
COMMENT ON COLUMN ai_providers.price_output IS '输出单价（元/百万 token）';

-- 2. 智能回复助手 + SEO 建议任务（provider_id NULL = 按 priority 自动路由）
INSERT INTO ai_tasks (task_name, provider_id, model, prompt_template, max_tokens, enabled)
SELECT v.task_name, NULL, '', v.prompt, v.max_tokens, TRUE
FROM (VALUES
    ('reply.assistant',
     '你是月言博客的写作助手。请根据「操作类型」对下面的内容执行相应处理，直接输出处理后的文本，不要解释、不要加前缀。\n\n操作类型：{action}\n\n{content}',
     1024),
    ('seo.advice',
     '你是一位中文 SEO 优化专家。请阅读下面帖子的标题与正文，输出 JSON：{"title":"SEO 标题（≤60 字）","description":"SEO 描述（≤160 字）","keywords":["关键词1","关键词2","关键词3"]}。只输出 JSON，不要代码块。\n\n标题：{title}\n\n正文：{content}',
     800)
) AS v(task_name, prompt, max_tokens)
ON CONFLICT (task_name) DO NOTHING;
