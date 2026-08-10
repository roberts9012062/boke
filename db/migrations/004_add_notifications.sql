-- ============================================================
-- 迁移 004：通知扩展（触发者/相关帖子）+ 话题关注表
-- 依据：《需求文档》3.8 通知（触发者头像昵称/动作文案/跳转帖子）
--       + 3.6 话题（关注话题）
-- 说明：
--   1. notifications 增加 actor_id（触发者，0=系统）与 post_id（相关帖子，跳转用）
--   2. topic_follows：用户关注话题（user_relations.target_id 引用 users，
--      不承载话题关系，故独立建表）
--   3. 本脚本幂等（IF NOT EXISTS，可重复执行）
-- ============================================================

-- 1. 通知触发者（0 = 系统通知）
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS actor_id BIGINT NOT NULL DEFAULT 0;

-- 2. 相关帖子 ID（跳转用：/posts/{id}）
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS post_id BIGINT NOT NULL DEFAULT 0;

-- 查询索引：按接收者 + 类型（通知列表）
CREATE INDEX IF NOT EXISTS idx_notifications_user_type ON notifications (user_id, type, created_at DESC);

-- 3. 话题关注（用户 × 话题唯一）
CREATE TABLE IF NOT EXISTS topic_follows (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- 关注者
    tag_id     BIGINT NOT NULL REFERENCES tags (id) ON DELETE CASCADE,   -- 话题（标签）
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_topic_follows ON topic_follows (user_id, tag_id);
