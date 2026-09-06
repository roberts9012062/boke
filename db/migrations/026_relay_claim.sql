-- 026 · 申请审核制（协议 v1.4）：博客侧保存申请凭据，轮询中继站审批结果
ALTER TABLE relay_config ADD COLUMN IF NOT EXISTS claim_token text NOT NULL DEFAULT '';
