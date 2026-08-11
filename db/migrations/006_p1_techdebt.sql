-- 006_p1_techdebt.sql
-- P1 技术债数据层（2026-08-11）：
--   1) 浏览埋点表（真实日浏览统计，替代「区间发布帖累计 view_count」近似值）
--   2) 密码版本号（找回密码后全局会话退出：旧 refresh token 失效）
--   3) 审核耗时（工单处理时间落库，统计平均耗时）
--   4) 敏感词命中统计
-- 全部幂等，可重复执行。

-- 1) 浏览埋点表
CREATE TABLE IF NOT EXISTS post_views (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    post_id     BIGINT NOT NULL REFERENCES posts (id) ON DELETE CASCADE, -- 被浏览帖子
    viewer_hash VARCHAR(64) NOT NULL DEFAULT '',  -- 访客标识（登录用户 ID 的 SHA256 / 匿名空），用于日 UV 近似
    viewed_at   TIMESTAMPTZ NOT NULL DEFAULT now() -- 浏览时间（日浏览统计维度）
);
CREATE INDEX IF NOT EXISTS idx_post_views_post_time ON post_views (post_id, viewed_at);
CREATE INDEX IF NOT EXISTS idx_post_views_time ON post_views (viewed_at);

-- 2) 密码版本号（JWT claims 携带，refresh 时校验；重置密码自增使旧会话失效）
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_version INT NOT NULL DEFAULT 1;

-- 3) 审核耗时（resolved/rejected 时写入，统计平均处理时长）
ALTER TABLE reports ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ;

-- 4) 敏感词命中统计（拦截命中 +1）
ALTER TABLE sensitive_words ADD COLUMN IF NOT EXISTS hit_count INT NOT NULL DEFAULT 0;
