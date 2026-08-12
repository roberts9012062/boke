-- ============================================================
-- 迁移 014：插件购买订单（M3.9 支付渠道——可插拔：dev 模拟支付 + 服务端许可证签发）
-- 说明：付费插件购买 → 创建订单（pending）→ 支付（真实渠道预留/开发环境模拟）
--       → 服务端持私钥签发 license.jwt → 自动激活。
-- ============================================================

CREATE TABLE IF NOT EXISTS plugin_orders (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    plugin_id   VARCHAR(100) NOT NULL,                -- 插件 ID
    instance_id BIGINT NOT NULL,                      -- 插件实例 ID
    price       INT NOT NULL DEFAULT 0,               -- 订单金额（¥；dev 模拟场景服务端不核价，真实渠道需服务端定价）
    state       VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending（待支付）/ paid（已支付已签发）/ failed（失败）
    license_jwt TEXT NOT NULL DEFAULT '',             -- 服务端签发的许可证（支付成功后写入）
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_plugin_orders_plugin ON plugin_orders (plugin_id);
