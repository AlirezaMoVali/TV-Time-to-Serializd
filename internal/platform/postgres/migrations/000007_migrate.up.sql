CREATE TYPE migrate_status AS ENUM ('pending', 'running', 'completed', 'failed');

CREATE TABLE migrate_jobs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status        migrate_status NOT NULL DEFAULT 'pending',
    progress      JSONB NOT NULL DEFAULT '{}',
    dump_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
    dump_format   export_output_format,
    export_id     UUID REFERENCES user_exports (id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ
);

CREATE INDEX idx_migrate_jobs_status ON migrate_jobs (status);

-- Global unresolved show catalog (no user-specific data).
CREATE TABLE unresolved_shows (
    id         BIGSERIAL PRIMARY KEY,
    tvdb_id    BIGINT,
    imdb_id    TEXT,
    title      TEXT NOT NULL DEFAULT '',
    year       INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_unresolved_shows_tvdb_id ON unresolved_shows (tvdb_id) WHERE tvdb_id IS NOT NULL;
