-- ============================================================
-- 迁移 017：插件包签名体系引导（P1 供应链加固部署配置）
-- 内容：1) 市场根公钥写入 settings.plugin_pkg_pubkeys（写入即开启强制验签）；
--       2) 存量插件实例刷新 capabilities 登记（仅覆盖空登记，不改动已配置值）。
-- 幂等：可重复执行；公钥更新时以本迁移最新值为准（轮换 = 追加新公钥后重跑或改值）。
-- ============================================================

INSERT INTO settings (key, value, description) VALUES
  ('plugin_pkg_pubkeys', '"-----BEGIN PUBLIC KEY-----\nMCowBQYDK2VwAyEA98EvIdIb/yjdYyiBLVjx5QoO21d0ZRq60vvXjcG0Nl8=\n-----END PUBLIC KEY-----"'::jsonb, '插件包签名信任公钥（市场根公钥，多个 PEM 块拼接；配置后安装强制验签）')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

-- 存量插件能力登记刷新（与各插件市场清单 capabilities 一致）
UPDATE plugin_instances SET capabilities = '["hooks", "api", "settings", "data.read"]'::jsonb, updated_at = now()
WHERE plugin_id = 'demo-plugin' AND capabilities = '[]'::jsonb;
UPDATE plugin_instances SET capabilities = '["api", "frontend", "admin.page", "settings"]'::jsonb, updated_at = now()
WHERE plugin_id = 'qq-music' AND capabilities = '[]'::jsonb;
UPDATE plugin_instances SET capabilities = '["api", "frontend", "admin.page", "settings"]'::jsonb, updated_at = now()
WHERE plugin_id = 'netease-music' AND capabilities = '[]'::jsonb;
UPDATE plugin_instances SET capabilities = '["hooks", "frontend", "settings"]'::jsonb, updated_at = now()
WHERE plugin_id = 'seo-optimizer' AND capabilities = '[]'::jsonb;
