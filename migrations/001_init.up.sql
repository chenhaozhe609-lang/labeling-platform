-- ============================================================================
-- 001_init: 用户 / 数据集 / 任务 / 标注 / 导入批次（meta-db）
-- 对齐 PRD §8
-- ============================================================================

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'annotator'
                  CHECK (role IN ('annotator', 'reviewer', 'admin')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE datasets (
    id                  BIGSERIAL PRIMARY KEY,
    name                TEXT NOT NULL,
    source_schema       TEXT NOT NULL,
    source_table        TEXT NOT NULL,
    source_pk_column    TEXT NOT NULL,
    hash_columns        TEXT[] NOT NULL DEFAULT '{}',
    form_schema         JSONB NOT NULL,
    form_schema_version INT  NOT NULL DEFAULT 1,
    last_imported_pk    TEXT,
    status              TEXT NOT NULL DEFAULT 'IMPORTING'
                        CHECK (status IN ('IMPORTING', 'READY', 'PAUSED', 'DONE', 'FAILED')),
    total_rows          INT  NOT NULL DEFAULT 0,
    created_by          BIGINT REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_datasets_status ON datasets(status, created_at DESC);

CREATE TABLE import_batches (
    id                 BIGSERIAL PRIMARY KEY,
    dataset_id         BIGINT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    file_name          TEXT,
    file_size_bytes    BIGINT,
    new_task_count     INT NOT NULL DEFAULT 0,
    updated_task_count INT NOT NULL DEFAULT 0,
    imported_by        BIGINT REFERENCES users(id),
    error              TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_import_batches_dataset ON import_batches(dataset_id, created_at DESC);

CREATE TABLE tasks (
    id               BIGSERIAL PRIMARY KEY,
    dataset_id       BIGINT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    source_row_pk    TEXT   NOT NULL,
    content_hash     TEXT,
    status           TEXT   NOT NULL DEFAULT 'PENDING'
                     CHECK (status IN ('PENDING', 'CLAIMED', 'COMPLETED', 'NEEDS_REDO')),
    assigned_to      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    claimed_at       TIMESTAMPTZ,
    lease_expires_at TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ,
    import_batch_id  BIGINT REFERENCES import_batches(id),
    round            INT    NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (dataset_id, source_row_pk)
);
CREATE INDEX idx_tasks_pending   ON tasks(dataset_id, id) WHERE status = 'PENDING';
CREATE INDEX idx_tasks_lease     ON tasks(lease_expires_at) WHERE status = 'CLAIMED';
CREATE INDEX idx_tasks_assigned  ON tasks(assigned_to, status);
CREATE INDEX idx_tasks_completed ON tasks(dataset_id, status, completed_at DESC) WHERE status = 'COMPLETED';

CREATE TABLE annotations (
    id                  BIGSERIAL PRIMARY KEY,
    task_id             BIGINT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    dataset_id          BIGINT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    user_id             BIGINT NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    data                JSONB  NOT NULL,
    form_schema_version INT    NOT NULL,
    round               INT    NOT NULL DEFAULT 1,
    superseded_at       TIMESTAMPTZ,
    reviewed_at         TIMESTAMPTZ,
    reviewed_by         BIGINT REFERENCES users(id) ON DELETE SET NULL,
    review_status       TEXT CHECK (review_status IN ('approved', 'needs_redo')),
    review_note         TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_annotations_task   ON annotations(task_id, round DESC);
CREATE INDEX idx_annotations_review ON annotations(dataset_id, reviewed_at) WHERE review_status IS NULL;
CREATE INDEX idx_annotations_active ON annotations(task_id) WHERE superseded_at IS NULL;
