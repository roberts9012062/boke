-- ============================================================
-- 迁移 022：AI 供应商预置渠道追加 opencode Go
-- 说明：
--   opencode Go 为 opencode.ai 官方云端订阅服务（OpenCode Zen，月订阅制），
--   对 OpenAI 兼容模型（GLM / Kimi / DeepSeek / MiMo / LongCat / Hy3 等）
--   提供 chat/completions 端点，基础地址 https://opencode.ai/zen/go/v1，
--   完整模型列表见 https://opencode.ai/zen/go/v1/models。
--   此前为旧库手动添加的渠道，未进种子导致重装后丢失——本迁移固化。
--   api_key 留空由站长后台填写（订阅后于 opencode.ai/auth 复制）。
-- 幂等：可重复执行（已存在同名渠道则跳过）。
-- ============================================================

INSERT INTO ai_providers (name, base_url, api_key_encrypted, models, enabled, priority)
SELECT 'opencode', 'https://opencode.ai/zen/go/v1', '', '["glm-5.3","kimi-k3","deepseek-v4-pro"]'::jsonb, TRUE, 6
WHERE NOT EXISTS (SELECT 1 FROM ai_providers WHERE name = 'opencode');
