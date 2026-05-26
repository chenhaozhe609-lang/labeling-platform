-- 回滚 002_multitenancy。先拆引用 organizations 的列/表，再删 organizations；恢复 username 全局唯一。
DROP TABLE IF EXISTS invites;

ALTER TABLE datasets DROP COLUMN IF EXISTS org_id;          -- 连带 idx_datasets_org

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_org_or_super;
DROP INDEX IF EXISTS users_email_key;
ALTER TABLE users
    DROP COLUMN IF EXISTS org_id,                           -- 连带 idx_users_org
    DROP COLUMN IF EXISTS email,
    DROP COLUMN IF EXISTS token_version,
    DROP COLUMN IF EXISTS is_superadmin;
ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username);

DROP TABLE IF EXISTS organizations;
