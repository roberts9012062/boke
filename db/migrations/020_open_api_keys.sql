-- ============================================================
-- 迁移 020：接口开放 API Key（后台「接口开放」页面）
-- 说明：
--   open_api_keys 存放对外开放接口的调用凭证：
--   - key 明文存库（oa_ 前缀 + 64 位随机 hex），后台列表可见便于复制使用；
--   - endpoints 绑定该 key 可调用的开放接口标识（TEXT[]，与目录 catalog 对应）；
--   - expires_at 为空表示永不过期；last_used_at 记录最近一次网关调用时间。
-- 幂等：可重复执行。
-- ============================================================

CREATE TABLE IF NOT EXISTS open_api_keys (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name         VARCHAR(100) NOT NULL DEFAULT '',              -- 备注名（可选）
    key          VARCHAR(100) NOT NULL,                         -- API Key（oa_ 前缀，明文）
    endpoints    TEXT[]        NOT NULL DEFAULT '{}',           -- 已授权接口标识数组
    expires_at   TIMESTAMPTZ,                                   -- 过期时间（NULL=永久有效）
    last_used_at TIMESTAMPTZ,                                   -- 最近调用时间
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now()
);

-- Key 唯一索引（查凭证用）
CREATE UNIQUE INDEX IF NOT EXISTS uq_open_api_keys_key ON open_api_keys (key);

COMMENT ON TABLE open_api_keys IS '接口开放 API Key（外部应用凭 X-Api-Key 调用 /api/v1/open/*）';
