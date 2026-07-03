DROP TABLE IF EXISTS tvtime_accounts;

CREATE TABLE tokens (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tvtime_user_id        BIGINT NOT NULL UNIQUE,
    email                 TEXT NOT NULL,
    jwt_token_enc         BYTEA NOT NULL,
    jwt_refresh_token_enc BYTEA NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at          TIMESTAMPTZ
);

CREATE INDEX idx_tokens_email ON tokens (email);
CREATE INDEX idx_tokens_tvtime_user_id ON tokens (tvtime_user_id);
