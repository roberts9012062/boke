-- ============================================================
-- 迁移 022：AI 供应商预置渠道追加 opencode
-- 说明：
--   opencode（opencode.ai）可作为模型聚合服务对外提供对话能力；
--   此前为旧库手动添加的渠道，未进种子导致重装后丢失——本迁移固化。
--   base_url 取 opencode serve 默认地址（http://127.0.0.1:4096），
--   接入协议遵循站内统一 OpenAI 兼容适配器（base_url + /chat/completions），
--   api_key 留空由站长后台填写；models 为常见占位，可在后台编辑或拉取。
-- 幂等：可重复执行（已存在同名渠道则跳过）。
-- ============================================================

INSERT INTO ai_providers (name, base_url, api_key_encrypted, models, enabled, priority)
SELECT 'opencode', 'http://127.0.0.1:4096/v1', '', '["claude-sonnet-4-5","gpt-5-mini"]'::jsonb, TRUE, 6
WHERE NOT EXISTS (SELECT 1 FROM ai_providers WHERE name = 'opencode');
