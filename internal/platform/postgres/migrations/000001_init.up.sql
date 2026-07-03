CREATE TABLE tvtime_accounts (
    id                  BIGSERIAL PRIMARY KEY,
    tvtime_user_id      BIGINT NOT NULL UNIQUE,
    email               TEXT NOT NULL,
    jwt_token           TEXT NOT NULL,
    jwt_refresh_token   TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tvtime_accounts_email ON tvtime_accounts (email);
