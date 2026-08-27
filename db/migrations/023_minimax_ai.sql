-- ============================================================
-- 迁移 023：MiniMax 渠道 + 发帖 AI 辅助任务种子
-- 说明：
--   1. ai_providers 追加 minimax 渠道（api.minimaxi.com，OpenAI 兼容 chat 端点；
--      MiniMax-M3 支持图片输入即图片识别；image-01 文生图 / music-3.0 音乐生成
--      走同域名的专用端点，模型列入清单供路由）。
--   2. ai_tasks 追加 5 条发帖辅助任务（后台「AI 设置-任务配置」可改提示词/启停）：
--      post.expand   内容扩写（chat，任意已配置供应商）
--      post.polish   内容润色（chat）
--      post.image    配图提示词模板（路由模型 image-01，文生图）
--      post.music    配乐提示词模板（路由模型 music-3.0，纯音乐生成）
--      image.recognize 图片识别提问模板（路由模型 MiniMax-M3，多模态）
-- 参考文档：https://platform.minimaxi.com/docs/api-reference/api-overview
-- 幂等：可重复执行。
-- ============================================================

-- 1. MiniMax 供应商（api_key 留空由站长后台填写）
INSERT INTO ai_providers (name, base_url, api_key_encrypted, models, enabled, priority)
SELECT 'minimax', 'https://api.minimaxi.com/v1', '',
       '["MiniMax-M3","image-01","music-3.0"]'::jsonb, TRUE, 7
WHERE NOT EXISTS (SELECT 1 FROM ai_providers WHERE name = 'minimax');

-- 2. 发帖 AI 辅助任务
INSERT INTO ai_tasks (task_name, provider_id, model, prompt_template, max_tokens, enabled)
SELECT v.task_name, NULL, v.model, v.prompt, v.max_tokens, TRUE
FROM (VALUES
    ('post.expand', '',
     '你是一位中文博客写作助手。请对下面这篇帖子内容进行扩写：保留原文风格与核心信息，补充细节、例子或过渡，使内容更充实完整。直接输出扩写后的完整正文，不要任何解释或前后缀。',
     4000),
    ('post.polish', '',
     '你是一位中文文字润色编辑。请润色下面这篇帖子：修正错别字与标点，优化不通顺的表达，使文字更流畅自然，但不改变原意与篇幅量级。直接输出润色后的完整正文，不要任何解释。',
     4000),
    ('post.image', 'image-01',
     '为下面这篇博客内容生成一张意境贴合的封面配图。内容：{content}',
     0),
    ('post.music', 'music-3.0',
     '生成一段安静的纯音乐背景曲，氛围贴合下面内容的意境。内容：{content}',
     0),
    ('image.recognize', 'MiniMax-M3',
     '请详细描述这张图片：主体内容、场景、色调氛围，以及图中出现的全部文字信息。',
     1000)
) AS v(task_name, model, prompt, max_tokens)
ON CONFLICT (task_name) DO NOTHING;
