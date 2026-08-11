-- ============================================================
-- 迁移 011：M5 权限体系 · 角色枚举迁移
-- 说明：users.role 由两级（admin/user）迁移为五级 RBAC 角色
--       （superadmin / editor / author / visitor / restricted）。
--       存量映射：admin → superadmin、user → visitor（默认注册角色）。
-- 幂等：CASE 表达式对已迁移值不生效（superadmin/visitor 不匹配 WHEN 保持原值）。
-- ============================================================

UPDATE users
SET role = CASE role
    WHEN 'admin' THEN 'superadmin'
    WHEN 'user'  THEN 'visitor'
    ELSE role
END;
