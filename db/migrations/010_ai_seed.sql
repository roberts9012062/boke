-- ============================================================
-- 迁移 010：M4-AI 种子数据 + reports.source 来源列
-- 说明：
--   1. reports 增加 source 列（user=人工举报 / ai=AI 审核标记），
--      供审核队列「高风险」统计（设计稿统计条：待处理/高风险/今日已审/平均耗时）。
--   2. ai_providers 写入 5 个默认 OpenAI 兼容供应商（api_key 为空，用户后台填写）。
--   3. ai_tasks 写入 3 条内置任务（post.summary / post.tags / comment.review）。
-- 幂等：可重复执行。
-- ============================================================

-- 1. 举报工单来源标记（AI 审核标记的高风险工单 source='ai'）
ALTER TABLE reports ADD COLUMN IF NOT EXISTS source VARCHAR(20) NOT NULL DEFAULT 'user';
COMMENT ON COLUMN reports.source IS '工单来源：user=人工举报 / ai=AI 审核标记';

-- 2. 默认 AI 供应商（OpenAI 兼容接口；api_key 加密存储，空串=未配置）
INSERT INTO ai_providers (name, base_url, api_key_encrypted, models, enabled, priority)
SELECT v.name, v.base_url, '', v.models::jsonb, TRUE, v.priority
FROM (VALUES
    ('deepseek', 'https://api.deepseek.com/v1',          '["deepseek-chat","deepseek-reasoner"]', 1),
    ('qwen',     'https://dashscope.aliyuncs.com/compatible-mode/v1', '["qwen-plus","qwen-turbo"]', 2),
    ('kimi',     'https://api.moonshot.cn/v1',           '["moonshot-v1-8k","moonshot-v1-32k"]', 3),
    ('glm',      'https://open.bigmodel.cn/api/paas/v4', '["glm-4-flash","glm-4-plus"]',          4),
    ('openai',   'https://api.openai.com/v1',            '["gpt-4o-mini","gpt-4o"]',              5)
) AS v(name, base_url, models, priority)
WHERE NOT EXISTS (SELECT 1 FROM ai_providers);

-- 3. 内置 AI 任务（provider_id NULL = 按 priority 自动路由到已启用供应商）
INSERT INTO ai_tasks (task_name, provider_id, model, prompt_template, max_tokens, enabled)
SELECT v.task_name, NULL, '', v.prompt, v.max_tokens, TRUE
FROM (VALUES
    ('post.summary',
     '你是一位中文博客摘要助手。请阅读下面帖子的标题与正文，用 2-3 句话（不超过 120 字）概括核心内容，语言自然、突出要点，不要使用「本文」「该帖」等套话，直接输出摘要。\n\n标题：{title}\n\n正文：{content}',
     800),
    ('post.tags',
     '你是一位中文内容标签助手。请阅读下面帖子的标题与正文，提炼 3-5 个标签，每个标签 1-6 个汉字或英文单词，小写，不要重复，直接输出 JSON 数组，例如：["技术","随笔"]。\n\n标题：{title}\n\n正文：{content}',
     500),
    ('comment.review',
     '你是一位社区评论审核员。请判断下面这条评论是否存在违规风险（垃圾广告、骚扰辱骂、色情低俗、违法违规、侵犯隐私等）。只输出 JSON：{"risk":"high"|"low","reason":"简短原因"}。risk=high 表示需要人工复核，reason 用中文简要说明。\n\n评论内容：{content}',
     300)
) AS v(task_name, prompt, max_tokens)
ON CONFLICT (task_name) DO NOTHING;
