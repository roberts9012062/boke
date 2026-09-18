-- ============================================================
-- 迁移 027：中继站发布选择（发帖时用户决定是否公开发送到大世界）
-- 说明：
--   relay_publish  用户是否选择将该帖公开发送到中继站（默认 false，
--                  既有帖子不自动推送——推送由用户显式选择）；
--   relay_category 用户选择的中继站分类（勾选发送后必填，须为中继站
--                  handshake 下发分类或本地规范表分类）；
--   relay_tags     用户选择的中继站标签（≤5 个，按分类规范表校验）。
-- 幂等：可重复执行。
-- ============================================================

ALTER TABLE posts ADD COLUMN IF NOT EXISTS relay_publish boolean NOT NULL DEFAULT false;
ALTER TABLE posts ADD COLUMN IF NOT EXISTS relay_category text NOT NULL DEFAULT '';
ALTER TABLE posts ADD COLUMN IF NOT EXISTS relay_tags jsonb NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN posts.relay_publish IS '是否公开发送到中继站（大世界）：用户发帖时显式选择';
COMMENT ON COLUMN posts.relay_category IS '中继站发布分类（relay_publish=true 时必填）';
COMMENT ON COLUMN posts.relay_tags IS '中继站发布标签（≤5 个，按分类规范表）';
