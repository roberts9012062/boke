-- ============================================================
-- 迁移 018：自定义页面（后台创建独立页面，前台经 /pages/{slug} 访问）
-- 说明：
--   slug 路由标识（小写字母/数字/连字符，全局唯一）；
--   content_format 取值：html（富文本编辑器产物）/ markdown（预留）；
--   status 取值：draft 草稿 / published 已发布（前台仅可见 published）；
--   description 为 SEO 描述（前台页面 meta 用）。
-- 幂等：可重复执行。
-- ============================================================

CREATE TABLE IF NOT EXISTS custom_pages (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug           VARCHAR(100) NOT NULL,                -- 路由标识（前台访问 /pages/{slug}）
    title          VARCHAR(200) NOT NULL,                -- 页面标题
    content        TEXT NOT NULL DEFAULT '',             -- 正文（富文本 HTML）
    content_format VARCHAR(20)  NOT NULL DEFAULT 'html', -- 正文格式：html / markdown
    description    VARCHAR(500) NOT NULL DEFAULT '',     -- SEO 描述
    status         VARCHAR(20)  NOT NULL DEFAULT 'draft',-- 状态：draft / published
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_custom_pages_slug ON custom_pages (slug);

COMMENT ON TABLE custom_pages IS '自定义页面：后台创建、前台 /pages/{slug} 访问的独立页面';
COMMENT ON COLUMN custom_pages.slug IS '路由标识：小写字母/数字/连字符，全局唯一';
COMMENT ON COLUMN custom_pages.content_format IS '正文格式：html / markdown';
COMMENT ON COLUMN custom_pages.status IS '状态：draft 草稿 / published 已发布';
