-- ============================================================
-- 迁移 013：M3.5 插件许可证唯一约束
-- 说明：plugin_licenses 表由普通索引改为唯一索引（单站点单插件单许可证），
--       支撑仓库层 ON CONFLICT (plugin_id) 覆盖写入。
-- 幂等：IF NOT EXISTS 对已建索引不生效。
-- ============================================================

CREATE UNIQUE INDEX IF NOT EXISTS uq_plugin_licenses ON plugin_licenses (plugin_id);

-- 说明：旧普通索引 idx_plugin_licenses 保留（前缀一致可复用，无碍）。
