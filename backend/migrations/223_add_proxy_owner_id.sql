-- 223_add_proxy_owner_id.sql
-- 代理新增 owner_id 字段，用于供应商管理自己的代理。
-- NULL = 平台托管代理（历史数据保持 NULL）。

ALTER TABLE proxies ADD COLUMN IF NOT EXISTS owner_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_proxies_owner_id
    ON proxies (owner_id)
    WHERE owner_id IS NOT NULL;
