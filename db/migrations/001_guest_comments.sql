-- ============================================================
-- 迁移 001：开放评论无需登录（匿名评论）
-- 依据：《需求文档》5.2 匿名评论迁移 + 设计稿「写一条评论…（开放，无需登录）」
-- 说明：
--   1. comments.author_id 允许 NULL（匿名访客无账号）
--   2. 新增 guest_name：匿名昵称（默认「匿名访客」+ 随机后缀）
--   3. 新增 guest_token_hash：匿名 token 哈希（防刷，需求 3.5）
--   4. 本脚本幂等（IF NOT EXISTS / ALTER COLUMN DROP NOT NULL 可重复执行）
-- ============================================================

-- 1. 允许匿名评论：author_id 取消 NOT NULL
ALTER TABLE comments ALTER COLUMN author_id DROP NOT NULL;

-- 2. 匿名昵称（开放评论，无需登录时自填，默认匿名访客）
ALTER TABLE comments ADD COLUMN IF NOT EXISTS guest_name VARCHAR(50) NOT NULL DEFAULT '';

-- 3. 匿名 token 哈希（服务端签发短期 token 的哈希，用于限频防刷）
ALTER TABLE comments ADD COLUMN IF NOT EXISTS guest_token_hash VARCHAR(64) NOT NULL DEFAULT '';
