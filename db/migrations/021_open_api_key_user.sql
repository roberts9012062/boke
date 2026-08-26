-- ============================================================
-- 迁移 021：开放 API Key 绑定用户（浏览器插件等外部应用身份）
-- 说明：
--   open_api_keys 增加 user_id，使 Key 携带站点用户归属：
--   - 生成 Key 时自动绑定当前操作管理员（后台 admin JWT 身份）；
--   - GET /api/v1/open/me 凭 Key 返回绑定用户的公开资料；
--   - 历史存量 Key 的 user_id 为 NULL = 未绑定（/open/me 返回 403 提示重新生成）。
-- 幂等：可重复执行。
-- ============================================================

ALTER TABLE open_api_keys
    ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);

COMMENT ON COLUMN open_api_keys.user_id IS '绑定用户（NULL=未绑定；/open/me 以此返回资料）';
