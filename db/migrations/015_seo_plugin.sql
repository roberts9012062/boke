-- ============================================================
-- 迁移 015：SEO 插件化通道（发帖 SEO 面板 + URL 别名 + 收录策略）
-- 说明：seo_meta 增加 url_alias（短链 /p/{alias}）与 robots（每帖收录策略）；
--       通道由主进程提供，前台发帖 SEO 面板由插件（seo-optimizer）渲染。
-- ============================================================

ALTER TABLE seo_meta ADD COLUMN IF NOT EXISTS url_alias VARCHAR(200) NOT NULL DEFAULT '';
ALTER TABLE seo_meta ADD COLUMN IF NOT EXISTS robots VARCHAR(50) NOT NULL DEFAULT '';

-- 短链解析索引（/p/{alias} → post_id）
CREATE UNIQUE INDEX IF NOT EXISTS uq_seo_meta_url_alias ON seo_meta (url_alias) WHERE url_alias <> '';
