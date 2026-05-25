-- source-db 初始化（首次创建卷时执行一次）。对齐 PRD §13.2 / §13.3。
-- 连接身份：postgres（POSTGRES_USER），当前库：sandbox_template（POSTGRES_DB）。

-- sandbox 恢复角色：可在 sandbox_template 内建 schema/恢复数据，
-- 但无 superuser、不能建库、不能建角色（防逃逸）。
CREATE ROLE sandbox_role LOGIN PASSWORD 'sandbox'
    NOSUPERUSER NOCREATEDB NOCREATEROLE;

-- backend 只读角色：恢复完成后由后端按 schema 授予 SELECT（此处仅给连接权）。
CREATE ROLE labeling_reader LOGIN PASSWORD 'reader'
    NOSUPERUSER NOCREATEDB NOCREATEROLE;

-- ── 最小权限收紧（C6.1/C6.2）────────────────────────────────────────────
-- 跨库隔离：移除 PUBLIC 在维护库上的默认 CONNECT，沙箱/只读角色只能连 sandbox_template。
REVOKE CONNECT ON DATABASE postgres  FROM PUBLIC;
REVOKE CONNECT ON DATABASE template1 FROM PUBLIC;

-- sandbox_template 仅显式角色可连（去掉 PUBLIC 默认 CONNECT）。
REVOKE CONNECT ON DATABASE sandbox_template FROM PUBLIC;
GRANT  CONNECT ON DATABASE sandbox_template TO sandbox_role, labeling_reader;

-- 恢复时需要在 sandbox_template 里建独立 schema（仅 sandbox_role，且不出本库）。
GRANT CREATE ON DATABASE sandbox_template TO sandbox_role;

-- public schema：任何角色都不得在此建对象（每个数据集走独立隔离 schema）。
-- reader 不碰 public，仅在恢复完成后由后端按 schema 授予 SELECT（见 GrantReader）。
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL    ON SCHEMA public FROM labeling_reader;
