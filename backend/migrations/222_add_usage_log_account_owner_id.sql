-- 222_add_usage_log_account_owner_id.sql
-- 使用日志新增 account_owner_id 冗余字段，用于供应商视角按 owner 直接过滤。
-- NULL = 平台托管账号或历史数据（回填后仍为 NULL 的表示账号无归属）。

-- 1. 新增列
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS account_owner_id BIGINT;

-- 2. 部分索引：只索引有归属的行（供应商查询只命中这些行）
CREATE INDEX IF NOT EXISTS idx_usage_logs_account_owner_id_created_at
    ON usage_logs (account_owner_id, created_at)
    WHERE account_owner_id IS NOT NULL;

-- 3. 历史数据回填：从 accounts 表关联取 owner_id
UPDATE usage_logs ul
    SET account_owner_id = a.owner_id
    FROM accounts a
    WHERE ul.account_id = a.id
      AND a.owner_id IS NOT NULL
      AND ul.account_owner_id IS NULL;
