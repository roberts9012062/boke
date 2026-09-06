-- 025 · 中继站对接（大世界）：单行配置 + 本地内容缓存
-- 契约：Relay Station docs/02-协议规范.md v1.2（B-1/B-2）

-- 中继站配置（单行，id 恒为 1；未启用时 url/key 为空）
CREATE TABLE IF NOT EXISTS relay_config (
    id                   smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled              boolean NOT NULL DEFAULT false,      -- 「大世界」开关
    url                  text NOT NULL DEFAULT '',            -- 中继站基础 URL
    site_key             text NOT NULL DEFAULT '',            -- 站点 key（明文本地保存，仅站长可见）
    mode                 text NOT NULL DEFAULT 'public'       -- 站点模式：public / bridged
        CHECK (mode IN ('public','bridged')),
    default_category     text NOT NULL DEFAULT '',            -- 发布到大世界的默认分类
    local_retention_days int NOT NULL DEFAULT 7,              -- 本地缓存保存天数（1~30）
    relay_meta_json      jsonb,                               -- 握手元信息快照（名称/规则/配额/分类）
    last_seq             bigint NOT NULL DEFAULT 0,           -- 订阅游标（补洞依据）
    updated_at           timestamptz NOT NULL DEFAULT now()
);
INSERT INTO relay_config (id) VALUES (1) ON CONFLICT (id) DO NOTHING;

-- 大世界内容本地缓存（content_id 全局唯一，重复投递忽略——at-least-once 幂等）
CREATE TABLE IF NOT EXISTS relay_content_cache (
    content_id   text PRIMARY KEY,
    payload_json jsonb NOT NULL,                -- 信封 ContentPayload 原样
    published_at timestamptz NOT NULL,
    fetched_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL           -- fetched_at + local_retention_days
);
CREATE INDEX IF NOT EXISTS idx_relay_cache_pub ON relay_content_cache (published_at DESC);
CREATE INDEX IF NOT EXISTS idx_relay_cache_expire ON relay_content_cache (expires_at);
