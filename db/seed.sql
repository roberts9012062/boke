-- ============================================================
-- 月言博客平台 - 基础种子数据（seed.sql）
-- 说明：仅插入站点运行必需的基础配置，可重复执行（UPSERT 语义）
-- ============================================================

-- ---------- 站点基础设置 ----------
INSERT INTO settings (key, value, description) VALUES
    ('site_name',               '"月言"',                    '站点名称'),
    ('site_description',        '"月言 - 多功能博客平台"',    '站点描述'),
    ('theme',                   '"cool-moon"',               '当前主题：cool-moon / mist'),
    ('allow_register',          'true',                      '是否开放注册'),
    ('comment_open',            'true',                      '是否开放评论'),
    ('post_audit_enabled',      'false',                     '文章发布是否需审核'),
    ('plugin_sync_interval',    '6',                         '插件市场同步间隔（小时）'),
    ('plugin_github_connected', 'false',                     '是否已连接 GitHub 账号'),
    ('sensitive_word_enabled',  'true',                      '是否启用敏感词过滤')
ON CONFLICT (key) DO NOTHING;

-- ---------- SEO 站点设置（单行配置） ----------
INSERT INTO seo_settings (id, site_name, site_description, robots_txt, sitemap_enabled) VALUES
    (1, '月言', '月言 - 多功能博客平台', 'User-agent: *\nAllow: /\n', TRUE)
ON CONFLICT (id) DO NOTHING;
