-- ============================================================
-- 月言博客平台 - 初始数据库结构（schema.sql）
-- 依据：《架构设计文档》第 8 章「数据库设计概要」
-- 说明：
--   1. 全部表使用 IF NOT EXISTS，脚本可重复执行（幂等）
--   2. 时间戳统一使用 timestamptz（带时区）
--   3. 主键统一使用 bigint 自增（GENERATED ALWAYS AS IDENTITY）
--   4. 状态类字段使用 VARCHAR + 注释说明取值，避免过度约束
-- ============================================================

-- ---------- 扩展（可用则启用，无权限时跳过，不影响建表） ----------
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ============================================================
-- 1. 用户表
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email         VARCHAR(255) NOT NULL,                          -- 登录邮箱（唯一）
    username      VARCHAR(50)  NOT NULL,                          -- 用户名（唯一，可展示）
    password_hash VARCHAR(255) NOT NULL,                          -- 密码哈希（bcrypt）
    nickname      VARCHAR(50)  NOT NULL DEFAULT '',               -- 昵称
    avatar_url    VARCHAR(500) NOT NULL DEFAULT '',               -- 头像地址
    bio           VARCHAR(500) NOT NULL DEFAULT '',               -- 个人简介
    status        VARCHAR(20)  NOT NULL DEFAULT 'active',         -- 状态：active=正常 / banned=封禁 / disabled=停用
    last_login_at TIMESTAMPTZ,                                    -- 最后登录时间
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),             -- 注册时间
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()              -- 更新时间
);

-- 唯一约束：邮箱与用户名
CREATE UNIQUE INDEX IF NOT EXISTS uq_users_email    ON users (email);
CREATE UNIQUE INDEX IF NOT EXISTS uq_users_username ON users (username);

-- ============================================================
-- 2. 用户关系表（关注 / 收藏 / 黑名单）
-- ============================================================
CREATE TABLE IF NOT EXISTS user_relations (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- 关系发起者
    target_id  BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- 关系对象
    type       VARCHAR(20) NOT NULL,                                    -- follow=关注 / favorite=收藏 / blacklist=黑名单
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 同一对用户同类型关系唯一
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_relations ON user_relations (user_id, target_id, type);

-- ============================================================
-- 3. 文章表
-- ============================================================
CREATE TABLE IF NOT EXISTS posts (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    author_id      BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- 作者
    title          VARCHAR(200) NOT NULL,                                   -- 标题
    summary        VARCHAR(500) NOT NULL DEFAULT '',                        -- 摘要
    content        TEXT NOT NULL DEFAULT '',                                -- 正文（Markdown）
    content_format VARCHAR(20)  NOT NULL DEFAULT 'markdown',                -- 正文格式：markdown / html
    content_type   VARCHAR(20)  NOT NULL DEFAULT 'text',                    -- 媒体形态：text/image/audio/video（迁移 002）
    post_kind      VARCHAR(20)  NOT NULL DEFAULT 'moment',                  -- 帖子形态：moment=说说 / article=文章（迁移 019）
    media_ids      JSONB NOT NULL DEFAULT '[]'::jsonb,                      -- 关联媒体 ID 有序数组（迁移 002）
    status         VARCHAR(20)  NOT NULL DEFAULT 'draft',                   -- 状态：draft=草稿 / pending=待审核 / published=已发布 / deleted=已删除
    visibility     VARCHAR(20)  NOT NULL DEFAULT 'public',                  -- 可见性：public=公开 / private=私密 / password=密码访问
    cover_url      VARCHAR(500) NOT NULL DEFAULT '',                        -- 封面图
    gallery_style  VARCHAR(20)  NOT NULL DEFAULT '',                        -- 图片展示风格：grid/carousel/flip/filmstrip/masonry/polaroid（空=网格）
    view_count     BIGINT NOT NULL DEFAULT 0,                               -- 浏览量
    like_count     BIGINT NOT NULL DEFAULT 0,                               -- 点赞数
    comment_count  BIGINT NOT NULL DEFAULT 0,                               -- 评论数
    published_at   TIMESTAMPTZ,                                             -- 发布时间
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 查询索引：按状态与发布时间（列表页）、按作者、按媒体形态、按帖子形态
CREATE INDEX IF NOT EXISTS idx_posts_status_published ON posts (status, published_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_author ON posts (author_id);
CREATE INDEX IF NOT EXISTS idx_posts_type_status ON posts (content_type, status, published_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_kind_status ON posts (post_kind, status, published_at DESC);

-- ============================================================
-- 4. 文章版本表（编辑历史）
-- ============================================================
CREATE TABLE IF NOT EXISTS post_versions (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    post_id    BIGINT NOT NULL REFERENCES posts (id) ON DELETE CASCADE,  -- 所属文章
    version    INTEGER NOT NULL,                                         -- 版本号（从 1 递增）
    content    TEXT NOT NULL,                                            -- 该版本的正文快照
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 同一文章的版本号唯一
CREATE UNIQUE INDEX IF NOT EXISTS uq_post_versions ON post_versions (post_id, version);

-- ============================================================
-- 5. 标签表
-- ============================================================
CREATE TABLE IF NOT EXISTS tags (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        VARCHAR(50)  NOT NULL,                    -- 标签名（唯一）
    slug        VARCHAR(50)  NOT NULL,                    -- 标签别名（URL 用，唯一）
    description VARCHAR(200) NOT NULL DEFAULT '',         -- 描述
    post_count  BIGINT NOT NULL DEFAULT 0,                -- 引用计数（冗余，便于排行）
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_tags_name ON tags (name);
CREATE UNIQUE INDEX IF NOT EXISTS uq_tags_slug ON tags (slug);

-- ============================================================
-- 6. 文章-标签关联表
-- ============================================================
CREATE TABLE IF NOT EXISTS post_tags (
    post_id BIGINT NOT NULL REFERENCES posts (id) ON DELETE CASCADE,  -- 文章
    tag_id  BIGINT NOT NULL REFERENCES tags (id) ON DELETE CASCADE,   -- 标签
    PRIMARY KEY (post_id, tag_id)
);

-- ============================================================
-- 7. 媒体资源表（图片 / 视频 / 音频 / 文件）
-- ============================================================
CREATE TABLE IF NOT EXISTS media_assets (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- 上传者
    type        VARCHAR(20)  NOT NULL,                                   -- 类型：image / video / audio / file
    storage_key VARCHAR(500) NOT NULL,                                   -- 存储键（对象存储路径）
    url         VARCHAR(500) NOT NULL DEFAULT '',                        -- 访问地址
    mime_type   VARCHAR(100) NOT NULL DEFAULT '',                        -- MIME 类型
    size_bytes  BIGINT NOT NULL DEFAULT 0,                               -- 文件大小
    width       INTEGER NOT NULL DEFAULT 0,                              -- 宽度（图片/视频）
    height      INTEGER NOT NULL DEFAULT 0,                              -- 高度（图片/视频）
    status      VARCHAR(20)  NOT NULL DEFAULT 'ready',                   -- 状态：ready=可用 / processing=处理中 / failed=失败
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 按上传者查询
CREATE INDEX IF NOT EXISTS idx_media_owner ON media_assets (owner_id);

-- ============================================================
-- 8. 评论表（支持楼中楼嵌套）
-- ============================================================
CREATE TABLE IF NOT EXISTS comments (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    post_id    BIGINT NOT NULL REFERENCES posts (id) ON DELETE CASCADE,   -- 所属文章
    author_id  BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,   -- 评论者
    parent_id  BIGINT REFERENCES comments (id) ON DELETE CASCADE,         -- 父评论（楼中楼；NULL = 楼顶层）
    content    TEXT NOT NULL,                                             -- 评论内容
    floor      INTEGER NOT NULL DEFAULT 0,                                -- 楼层号（按文章内递增）
    status     VARCHAR(20) NOT NULL DEFAULT 'visible',                    -- 状态：visible=可见 / hidden=隐藏 / deleted=已删除
    like_count BIGINT NOT NULL DEFAULT 0,                                 -- 点赞数
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 查询索引：文章评论列表（按楼中楼展开）
CREATE INDEX IF NOT EXISTS idx_comments_post ON comments (post_id, parent_id, floor);
CREATE INDEX IF NOT EXISTS idx_comments_author ON comments (author_id);

-- ============================================================
-- 9. 私信会话表
-- ============================================================
CREATE TABLE IF NOT EXISTS conversations (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_a         BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- 会话方 A（约定 user_a < user_b）
    user_b         BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- 会话方 B
    last_message_id BIGINT NOT NULL DEFAULT 0,                               -- 最后一条消息 ID（冗余，列表排序用）
    unread_a       INTEGER NOT NULL DEFAULT 0,                               -- A 的未读数
    unread_b       INTEGER NOT NULL DEFAULT 0,                               -- B 的未读数
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 两人之间只允许一个会话
CREATE UNIQUE INDEX IF NOT EXISTS uq_conversations ON conversations (user_a, user_b);

-- ============================================================
-- 10. 私信消息表
-- ============================================================
CREATE TABLE IF NOT EXISTS messages (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,  -- 所属会话
    sender_id       BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,          -- 发送者
    content         TEXT NOT NULL,                                                    -- 消息内容
    read_at         TIMESTAMPTZ,                                                      -- 已读时间（NULL = 未读）
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 按会话与时间分页查询
CREATE INDEX IF NOT EXISTS idx_messages_conv ON messages (conversation_id, created_at);

-- ============================================================
-- 11. 举报表（帖子 / 评论 / 用户）
-- ============================================================
CREATE TABLE IF NOT EXISTS reports (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reporter_id BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- 举报人
    target_type VARCHAR(20)  NOT NULL,                                    -- 对象类型：post / comment / user
    target_id   BIGINT NOT NULL,                                          -- 对象 ID
    reason      VARCHAR(50)  NOT NULL,                                    -- 举报原因（预置选项）
    detail      TEXT NOT NULL DEFAULT '',                                 -- 补充说明
    status      VARCHAR(20)  NOT NULL DEFAULT 'pending',                  -- 状态：pending=待处理 / processing=处理中 / resolved=已解决 / rejected=驳回
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 按状态处理队列
CREATE INDEX IF NOT EXISTS idx_reports_status ON reports (status, created_at);

-- ============================================================
-- 12. 举报处理单表
-- ============================================================
CREATE TABLE IF NOT EXISTS report_tickets (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    report_id  BIGINT NOT NULL REFERENCES reports (id) ON DELETE CASCADE,  -- 关联举报
    handler_id BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,    -- 处理人
    action     VARCHAR(50) NOT NULL,                                       -- 处理动作：ban=封禁 / delete=删除 / warn=警告 / none=不处理
    note       TEXT NOT NULL DEFAULT '',                                   -- 处理说明
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- 13. 审计日志表（管理操作留痕，按月分区策略见架构文档）
-- ============================================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_id      BIGINT NOT NULL DEFAULT 0,             -- 操作者（0 = 系统）
    action        VARCHAR(50)  NOT NULL,                 -- 动作：login / delete_post / ban_user ...
    resource_type VARCHAR(50)  NOT NULL,                 -- 资源类型：post / comment / user / plugin ...
    resource_id   BIGINT NOT NULL DEFAULT 0,             -- 资源 ID
    before_data   JSONB,                                 -- 变更前数据快照
    after_data    JSONB,                                 -- 变更后数据快照
    ip            VARCHAR(45)  NOT NULL DEFAULT '',      -- 操作者 IP（兼容 IPv6）
    user_agent    VARCHAR(300) NOT NULL DEFAULT '',      -- 浏览器 UA
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 审计查询索引
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_logs (actor_id, created_at);
CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_logs (resource_type, resource_id);

-- ============================================================
-- 14. 敏感词表
-- ============================================================
CREATE TABLE IF NOT EXISTS sensitive_words (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    word       VARCHAR(100) NOT NULL,        -- 敏感词内容（唯一）
    level      VARCHAR(20)  NOT NULL DEFAULT 'forbidden',  -- 级别：forbidden=直接拦截 / review=进入审核
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_sensitive_words ON sensitive_words (word);

-- ============================================================
-- 15. 封禁记录表
-- ============================================================
CREATE TABLE IF NOT EXISTS ban_records (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- 被封禁用户
    reason     VARCHAR(200) NOT NULL,                                    -- 封禁原因
    until      TIMESTAMPTZ,                                              -- 解封时间（NULL = 永久）
    created_by BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- 封禁操作者
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ban_user ON ban_records (user_id);

-- ============================================================
-- 16. 站点设置表（key-value，值存 JSONB）
-- ============================================================
CREATE TABLE IF NOT EXISTS settings (
    key         VARCHAR(100) PRIMARY KEY,               -- 配置键
    value       JSONB NOT NULL,                         -- 配置值（JSON）
    description VARCHAR(200) NOT NULL DEFAULT '',       -- 配置说明
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- 17. 插件实例表
-- ============================================================
CREATE TABLE IF NOT EXISTS plugin_instances (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    plugin_id  VARCHAR(100) NOT NULL,                   -- 插件 ID（市场清单中的 id）
    name       VARCHAR(100) NOT NULL DEFAULT '',        -- 插件名称
    version    VARCHAR(20)  NOT NULL DEFAULT '',        -- 当前版本
    repo_url   VARCHAR(500) NOT NULL DEFAULT '',        -- 来源仓库（GitHub）
    state      VARCHAR(20)  NOT NULL DEFAULT 'installed',  -- 状态：installed / verified / running / disabled / crashed / degraded / uninstalled
    config     JSONB,                                   -- 插件配置（后台表单生成）
    pubkey_pem TEXT,                                    -- 许可证公钥（付费插件）
    last_error TEXT NOT NULL DEFAULT '',                -- 最近一次错误信息
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_plugin_instances ON plugin_instances (plugin_id);

-- ============================================================
-- 18. 插件许可证表
-- ============================================================
CREATE TABLE IF NOT EXISTS plugin_licenses (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    plugin_id  VARCHAR(100) NOT NULL,                    -- 插件 ID
    licensee   VARCHAR(100) NOT NULL DEFAULT '',         -- 被许可方（站点 ID / 用户）
    edition    VARCHAR(20)  NOT NULL DEFAULT '',         -- 版本：free / pro
    features   JSONB,                                    -- 授权功能列表
    license_jwt TEXT NOT NULL,                           -- 许可证原文（Ed25519 签名 JWT）
    expires_at TIMESTAMPTZ,                              -- 到期时间
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_plugin_licenses ON plugin_licenses (plugin_id);

-- ============================================================
-- 19. AI 供应商表
-- ============================================================
CREATE TABLE IF NOT EXISTS ai_providers (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name              VARCHAR(50)  NOT NULL,             -- 供应商名称：deepseek / qwen / kimi / glm / openai
    base_url          VARCHAR(300) NOT NULL,             -- OpenAI 兼容接口地址
    api_key_encrypted TEXT NOT NULL,                     -- API 密钥（AES 加密后存储）
    models            JSONB NOT NULL DEFAULT '[]',       -- 可用模型列表
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,     -- 是否启用
    priority          INTEGER NOT NULL DEFAULT 100,      -- 路由优先级（小先选）
    price_input       NUMERIC(12, 6) NOT NULL DEFAULT 0, -- 输入单价（元/百万 token）
    price_output      NUMERIC(12, 6) NOT NULL DEFAULT 0, -- 输出单价（元/百万 token）
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- 20. AI 任务路由表（任务 → 供应商 / 模型 / 提示词）
-- ============================================================
CREATE TABLE IF NOT EXISTS ai_tasks (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    task_name       VARCHAR(50) NOT NULL,                -- 任务名：post.summary / post.tags / comment.review / seo.advice
    provider_id     BIGINT REFERENCES ai_providers (id) ON DELETE SET NULL,  -- 使用的供应商
    model           VARCHAR(100) NOT NULL DEFAULT '',    -- 模型名
    prompt_template TEXT NOT NULL DEFAULT '',            -- 提示词模板（可后台编辑）
    max_tokens      INTEGER NOT NULL DEFAULT 2048,       -- 最大输出 token
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,       -- 是否启用
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_tasks ON ai_tasks (task_name);

-- ============================================================
-- 21. AI 用量统计表
-- ============================================================
CREATE TABLE IF NOT EXISTS ai_usage (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    task_name    VARCHAR(50) NOT NULL,                   -- 任务名
    provider_id  BIGINT NOT NULL DEFAULT 0,              -- 供应商 ID
    tokens_in    BIGINT NOT NULL DEFAULT 0,              -- 输入 token 数
    tokens_out   BIGINT NOT NULL DEFAULT 0,              -- 输出 token 数
    cost         NUMERIC(12, 6) NOT NULL DEFAULT 0,      -- 费用（按供应商单价折算）
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 按日聚合统计
CREATE INDEX IF NOT EXISTS idx_ai_usage_day ON ai_usage (created_at);

-- ============================================================
-- 22. 文章 SEO 元信息表
-- ============================================================
CREATE TABLE IF NOT EXISTS seo_meta (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    post_id        BIGINT NOT NULL UNIQUE REFERENCES posts (id) ON DELETE CASCADE,  -- 关联文章
    title          VARCHAR(300) NOT NULL DEFAULT '',      -- SEO 标题
    description    VARCHAR(500) NOT NULL DEFAULT '',      -- SEO 描述
    keywords       TEXT NOT NULL DEFAULT '',              -- 关键词
    canonical_url  VARCHAR(500) NOT NULL DEFAULT '',      -- 规范链接
    og_image       VARCHAR(500) NOT NULL DEFAULT '',      -- 分享图
    summary        TEXT NOT NULL DEFAULT '',              -- AI 生成摘要
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- 23. SEO 站点设置表（单行配置）
-- ============================================================
CREATE TABLE IF NOT EXISTS seo_settings (
    id               INTEGER PRIMARY KEY CHECK (id = 1),  -- 强制单行
    site_name        VARCHAR(100) NOT NULL DEFAULT '',    -- 站点名称
    site_description VARCHAR(300) NOT NULL DEFAULT '',    -- 站点描述
    robots_txt       TEXT NOT NULL DEFAULT '',            -- robots.txt 内容
    sitemap_enabled  BOOLEAN NOT NULL DEFAULT TRUE,       -- 是否生成站点地图
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- 24. SEO 健康度检查记录表
-- ============================================================
CREATE TABLE IF NOT EXISTS seo_health_checks (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    post_id    BIGINT NOT NULL REFERENCES posts (id) ON DELETE CASCADE,  -- 检查的文章
    score      INTEGER NOT NULL DEFAULT 0,                               -- 健康度评分（0-100）
    issues     JSONB NOT NULL DEFAULT '[]',                              -- 问题清单（[{code,message}]）
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_seo_health_post ON seo_health_checks (post_id, checked_at);

-- ============================================================
-- 25. 备份记录表
-- ============================================================
CREATE TABLE IF NOT EXISTS backup_records (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    type           VARCHAR(20)  NOT NULL,                -- 类型：manual=手动 / scheduled=定时
    status         VARCHAR(20)  NOT NULL,                -- 状态：running=进行中 / success=成功 / failed=失败
    file_path      VARCHAR(500) NOT NULL DEFAULT '',     -- 打包文件路径
    file_size      BIGINT NOT NULL DEFAULT 0,            -- 文件大小
    db_dump        VARCHAR(500) NOT NULL DEFAULT '',     -- 数据库 dump 路径
    media_snapshot VARCHAR(500) NOT NULL DEFAULT '',     -- 媒体快照路径
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- 26. 站内通知表
-- ============================================================
CREATE TABLE IF NOT EXISTS notifications (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- 接收者
    type       VARCHAR(30)  NOT NULL,                    -- 类型：system / message / reply / follow / report_result
    title      VARCHAR(100) NOT NULL DEFAULT '',         -- 标题
    content    TEXT NOT NULL DEFAULT '',                 -- 内容
    link       VARCHAR(300) NOT NULL DEFAULT '',         -- 跳转链接
    read_at    TIMESTAMPTZ,                              -- 已读时间（NULL = 未读）
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications (user_id, read_at);

-- ============================================================
-- 27. 自定义页面表（后台创建独立页面，前台经 /pages/{slug} 访问；迁移 018）
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
