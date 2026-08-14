-- 供应商角色：账号归属字段
-- owner_id 指向 users.id，NULL = 平台托管账号（兼容存量数据）
-- 供应商登录后只能看到/操作 owner_id = 自己的账号
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS owner_id BIGINT NULL;

-- 供应商按归属过滤的索引（部分索引，排除软删除行）
CREATE INDEX IF NOT EXISTS idx_accounts_owner_id ON accounts (owner_id) WHERE deleted_at IS NULL;
