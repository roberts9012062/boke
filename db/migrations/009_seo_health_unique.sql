-- 009_seo_health_unique.sql
-- seo_health_checks.post_id 唯一约束（M4：SaveHealthCheck 的 ON CONFLICT (post_id) 依赖）。
-- 幂等（约束已存在时静默跳过）。
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'uq_seo_health_post'
    ) THEN
        ALTER TABLE seo_health_checks ADD CONSTRAINT uq_seo_health_post UNIQUE (post_id);
    END IF;
END $$;
