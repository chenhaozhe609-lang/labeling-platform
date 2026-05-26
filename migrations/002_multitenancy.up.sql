-- ============================================================================
-- 002_multitenancy: 组织多租户 + 登录硬化（meta-db）
-- 对齐 planning/AUTH_MULTI_TENANCY_PLAN.md §3
--   · organizations 多租户，数据按 org_id 隔离
--   · users 加 org_id / email（登录标识，全局唯一）/ token_version（吊销）/ is_superadmin（平台超管）
--   · invites 邀请加入既有组织
-- ============================================================================

CREATE TABLE organizations (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    owner_id   BIGINT,                 -- 指向 owner 用户；建组织时先 NULL，事务内回填
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users
    ADD COLUMN org_id        BIGINT REFERENCES organizations(id),   -- 业务用户必填；超管为 NULL
    ADD COLUMN email         TEXT,                                  -- 登录标识，全局唯一（见下方唯一索引）
    ADD COLUMN token_version INT  NOT NULL DEFAULT 0,               -- 吊销：+1 使旧 token 失效
    ADD COLUMN is_superadmin BOOL NOT NULL DEFAULT false;           -- 平台超管（跨组织）

ALTER TABLE datasets ADD COLUMN org_id BIGINT REFERENCES organizations(id);

CREATE TABLE invites (
    id          BIGSERIAL PRIMARY KEY,
    org_id      BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    role        TEXT   NOT NULL CHECK (role IN ('annotator','reviewer','admin')),
    token       TEXT   NOT NULL UNIQUE,          -- 随机不可猜，进邀请链接
    email       TEXT,                            -- 可选：限定受邀邮箱
    created_by  BIGINT REFERENCES users(id),
    expires_at  TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    accepted_by BIGINT REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_invites_org ON invites(org_id, created_at DESC);

-- 回填（保证现有 dev/测试账号继续可登录）：默认组织 + 现有用户/数据集归属其下，占位邮箱。
INSERT INTO organizations (id, name) VALUES (1, '默认组织');
SELECT setval(pg_get_serial_sequence('organizations', 'id'), 1, true);   -- 防止下一个 org 复用 id=1
UPDATE users    SET org_id = 1, email = lower(username) || '@local' WHERE org_id IS NULL;
UPDATE datasets SET org_id = 1 WHERE org_id IS NULL;
UPDATE organizations SET owner_id = (SELECT id FROM users WHERE role = 'admin' ORDER BY id LIMIT 1) WHERE id = 1;

-- 业务用户必须有组织，超管 org_id 可空。回填后再加约束，避免既有行违反。
ALTER TABLE users ADD CONSTRAINT users_org_or_super CHECK (is_superadmin OR org_id IS NOT NULL);

-- email 全局唯一（登录标识，大小写不敏感）；username 不再全局唯一（降为显示名）。
ALTER TABLE users DROP CONSTRAINT users_username_key;
CREATE UNIQUE INDEX users_email_key ON users (lower(email)) WHERE email IS NOT NULL;
CREATE INDEX idx_users_org ON users(org_id);
CREATE INDEX idx_datasets_org ON datasets(org_id, created_at DESC);
