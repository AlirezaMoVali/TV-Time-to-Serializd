CREATE TYPE export_status AS ENUM ('pending', 'running', 'completed', 'failed');

-- Global show catalog (TVDB/TMDB mapping shared across users and exports).
CREATE TABLE shows (
    id                BIGSERIAL PRIMARY KEY,
    tvdb_id           BIGINT UNIQUE,
    tvtime_series_id  BIGINT UNIQUE,
    tmdb_id           INT,
    imdb_id           TEXT,
    title             TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shows_has_external_id CHECK (tvdb_id IS NOT NULL OR tvtime_series_id IS NOT NULL)
);

CREATE INDEX idx_shows_tvdb_id ON shows (tvdb_id) WHERE tvdb_id IS NOT NULL;
CREATE INDEX idx_shows_tvtime_series_id ON shows (tvtime_series_id) WHERE tvtime_series_id IS NOT NULL;
CREATE INDEX idx_shows_tmdb_id ON shows (tmdb_id) WHERE tmdb_id IS NOT NULL;

-- One row per export run for a linked token (user session).
CREATE TABLE user_exports (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_id         UUID NOT NULL REFERENCES tokens (id) ON DELETE CASCADE,
    status           export_status NOT NULL DEFAULT 'pending',
    show_count       INT NOT NULL DEFAULT 0,
    watched_episodes INT NOT NULL DEFAULT 0,
    duration_ms      BIGINT,
    error_message    TEXT,
    started_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMPTZ
);

CREATE INDEX idx_user_exports_token_id ON user_exports (token_id);
CREATE INDEX idx_user_exports_status ON user_exports (status);

-- Per-show snapshot for an export (normalized, no large JSON blobs).
CREATE TABLE user_export_shows (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    export_id       UUID NOT NULL REFERENCES user_exports (id) ON DELETE CASCADE,
    show_id         BIGINT NOT NULL REFERENCES shows (id),
    tvtime_uuid     UUID,
    status          TEXT NOT NULL DEFAULT 'unknown',
    is_favorite     BOOLEAN NOT NULL DEFAULT FALSE,
    show_created_at TIMESTAMPTZ,
    no_episode_data BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (export_id, show_id)
);

CREATE INDEX idx_user_export_shows_export_id ON user_export_shows (export_id);

CREATE TABLE user_export_seasons (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_show_id  UUID NOT NULL REFERENCES user_export_shows (id) ON DELETE CASCADE,
    season_number INT NOT NULL,
    is_specials   BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (user_show_id, season_number)
);

CREATE INDEX idx_user_export_seasons_user_show_id ON user_export_seasons (user_show_id);

CREATE TABLE user_export_episodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_season_id  UUID NOT NULL REFERENCES user_export_seasons (id) ON DELETE CASCADE,
    tvdb_episode_id BIGINT,
    episode_number  INT NOT NULL,
    name            TEXT,
    is_special      BOOLEAN NOT NULL DEFAULT FALSE,
    is_watched      BOOLEAN NOT NULL DEFAULT FALSE,
    watched_at      TIMESTAMPTZ,
    rewatch_count   INT NOT NULL DEFAULT 0,
    watched_count   INT NOT NULL DEFAULT 0,
    UNIQUE (user_season_id, episode_number)
);

CREATE INDEX idx_user_export_episodes_user_season_id ON user_export_episodes (user_season_id);
CREATE INDEX idx_user_export_episodes_tvdb_episode_id ON user_export_episodes (tvdb_episode_id) WHERE tvdb_episode_id IS NOT NULL;
