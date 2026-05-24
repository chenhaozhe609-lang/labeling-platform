-- source-db 初始化（首次创建卷时执行一次）。对齐 PRD §13.2 / §13.3。
-- 连接身份：postgres（POSTGRES_USER），当前库：sandbox_template（POSTGRES_DB）。

-- sandbox 恢复角色：可在 sandbox_template 内建 schema/恢复数据，
-- 但无 superuser、不能建库、不能建角色（防逃逸）。
CREATE ROLE sandbox_role LOGIN PASSWORD 'sandbox'
    NOSUPERUSER NOCREATEDB NOCREATEROLE;

-- backend 只读角色：恢复完成后由后端按 schema 授予 SELECT（此处仅给连接权）。
CREATE ROLE labeling_reader LOGIN PASSWORD 'reader'
    NOSUPERUSER NOCREATEDB NOCREATEROLE;

GRANT CONNECT ON DATABASE sandbox_template TO sandbox_role, labeling_reader;

-- 恢复时需要在 sandbox_template 里建独立 schema
GRANT CREATE ON DATABASE sandbox_template TO sandbox_role;
GRANT USAGE, CREATE ON SCHEMA public TO sandbox_role;
GRANT USAGE ON SCHEMA public TO labeling_reader;
