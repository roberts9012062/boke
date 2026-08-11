-- 008_seo_settings_ext.sql
-- SEO 设置扩展列（M4：设计稿《SEO设置》——站点标题后缀/默认关键词/OG 标题）。
-- seo_settings 表为单行（id=1），补设计稿字段。幂等，可重复执行。
ALTER TABLE seo_settings ADD COLUMN IF NOT EXISTS title_suffix VARCHAR(100) NOT NULL DEFAULT '· 月言';
ALTER TABLE seo_settings ADD COLUMN IF NOT EXISTS keywords TEXT NOT NULL DEFAULT '';
ALTER TABLE seo_settings ADD COLUMN IF NOT EXISTS og_title VARCHAR(300) NOT NULL DEFAULT '';
