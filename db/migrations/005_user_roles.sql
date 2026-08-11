-- 005_user_roles.sql
-- 用户角色持久化（M2 角色调整 UI）：
--   users.role 列（admin / user）作为角色唯一数据源，casbin 启动时全量加载到内存策略。
--   schema.sql 已上线部分不改动，角色列通过增量迁移补充（幂等，可重复执行）。
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';

-- 既有管理员账号标记为 admin（幂等：仅升级未标记的 admin 账号）
UPDATE users SET role = 'admin' WHERE username = 'admin' AND role = 'user';
