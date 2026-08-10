-- ============================================================
-- 迁移 002：帖子内容类型与媒体关联字段
-- 依据：《需求文档》3.2-3.4 四类型帖子（文字/图片/音频/视频）+ 设计稿灯箱多图有序
-- 说明：
--   1. posts.content_type：帖子内容类型（text/image/audio/video），
--      列表按类型过滤（首页 全部/图/音/影 Tab）与后台内容分布统计依赖
--   2. posts.media_ids：关联媒体 ID 有序数组（JSONB），
--      图片帖多图顺序（灯箱 2/5 左右切换）与音频/视频帖单媒体均适用；
--      不新建关联表（KISS，媒体生命周期独立于帖子）
--   3. 本脚本幂等（IF NOT EXISTS，可重复执行）
-- ============================================================

-- 1. 内容类型（草稿/已发布均可区分）
ALTER TABLE posts ADD COLUMN IF NOT EXISTS content_type VARCHAR(20) NOT NULL DEFAULT 'text';

-- 2. 媒体 ID 有序数组（JSONB，如 [3,5,7]）
ALTER TABLE posts ADD COLUMN IF NOT EXISTS media_ids JSONB NOT NULL DEFAULT '[]';

-- 查询索引：类型 + 发布状态（列表过滤）
CREATE INDEX IF NOT EXISTS idx_posts_type_status ON posts (content_type, status, published_at DESC);
