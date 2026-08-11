-- ============================================================
-- 迁移 012：浏览埋点去重（P1 真实日浏览统计完善）
-- 说明：
--   1. post_views 增加 view_date 列（按自然日去重维度）。
--   2. 清理既有重复记录（同帖同访客同日只保留最早一条）。
--   3. 建唯一索引 (post_id, viewer_hash, view_date) —— RecordView 走
--      ON CONFLICT DO NOTHING，同人同日同帖只计一次（防刷新刷量）。
-- 幂等：可重复执行（ADD COLUMN IF NOT EXISTS / 唯一索引先删重复再建）。
-- ============================================================

-- 1. 浏览日期列（默认当前日期，历史行自动回填）
ALTER TABLE post_views ADD COLUMN IF NOT EXISTS view_date DATE NOT NULL DEFAULT CURRENT_DATE;

-- 2. 清理重复（同帖同访客同日保留最早一条；唯一索引创建前置条件）
DELETE FROM post_views a
USING post_views b
WHERE a.id > b.id
  AND a.post_id = b.post_id
  AND a.viewer_hash = b.viewer_hash
  AND a.view_date = b.view_date;

-- 3. 每日去重唯一索引（幂等）
CREATE UNIQUE INDEX IF NOT EXISTS uq_post_views_daily
    ON post_views (post_id, viewer_hash, view_date);
