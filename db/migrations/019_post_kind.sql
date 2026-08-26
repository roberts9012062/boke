-- ============================================================
-- 迁移 019：帖子形态 post_kind（说说 / 文章）
-- 说明：
--   post_kind 与 content_type（媒体形态 text/image/audio/video）正交：
--   - moment 说说：短内容（≤2000 字），标题可空，时间轴直接读全文摘要；
--   - article 文章：长内容（≤20000 字），标题必填，时间轴展示
--     标题 + 摘要（正文前 200 字）+ 标签，点击标题进详情页阅读。
--   文章的图片走 media_ids 图集与正文内嵌图，content_type 固定 text。
-- 幂等：可重复执行。
-- ============================================================

ALTER TABLE posts ADD COLUMN IF NOT EXISTS post_kind VARCHAR(20) NOT NULL DEFAULT 'moment';
COMMENT ON COLUMN posts.post_kind IS '帖子形态：moment=说说 / article=文章';

-- 文章过滤查询索引（时间轴「文」Tab）
CREATE INDEX IF NOT EXISTS idx_posts_kind_status ON posts (post_kind, status, published_at DESC);
