-- ============================================================
-- 迁移 024：media_assets.owner_id 允许空（系统生成媒体）
-- 说明：
--   AI 辅助生成物（配图/配乐）经 SaveBytes 落盘后登记 media_assets；
--   此前 owner_id NOT NULL + 外键约束导致系统归属（无上传者）无法登记。
--   放开 NOT NULL：NULL = 系统生成（AI 产物）；外键保留（NULL 不参与校验）。
-- 幂等：可重复执行。
-- ============================================================

ALTER TABLE media_assets ALTER COLUMN owner_id DROP NOT NULL;
COMMENT ON COLUMN media_assets.owner_id IS '上传者（NULL=系统生成，AI 辅助产物）';
