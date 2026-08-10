-- ============================================================
-- 迁移 003：帖子互动与评论点赞表
-- 依据：《需求文档》3.10 互动落点 + 3.5 评论点赞
-- 说明：
--   1. post_reactions：帖子点赞/收藏（type=like|favorite），
--      统一表避免重复结构（user_relations.target_id 引用 users，
--      仅适用于用户间关系，不承载帖子互动）
--   2. comment_likes：评论点赞（登录用户）
--   3. 计数冗余：posts.like_count / posts.comment_count 在事务内同步增减
--   4. 本脚本幂等（IF NOT EXISTS，可重复执行）
-- ============================================================

-- 1. 帖子点赞/收藏（联合唯一：同一用户对同一帖子同类型仅一次）
CREATE TABLE IF NOT EXISTS post_reactions (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- 操作者
    post_id    BIGINT NOT NULL REFERENCES posts (id) ON DELETE CASCADE,  -- 帖子
    type       VARCHAR(20) NOT NULL,                                     -- like=点赞 / favorite=收藏
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_post_reactions ON post_reactions (user_id, post_id, type);
CREATE INDEX IF NOT EXISTS idx_post_reactions_post ON post_reactions (post_id, type);

-- 2. 评论点赞（登录用户对评论）
CREATE TABLE IF NOT EXISTS comment_likes (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- 操作者
    comment_id BIGINT NOT NULL REFERENCES comments (id) ON DELETE CASCADE, -- 评论
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_comment_likes ON comment_likes (user_id, comment_id);
