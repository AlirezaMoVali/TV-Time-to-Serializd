-- Tracks shows imported to Serializd per account (account_hash = SHA-256 of normalized email).
CREATE TABLE serializd_imported_shows (
    account_hash TEXT NOT NULL,
    tmdb_id      INT NOT NULL,
    imported_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_hash, tmdb_id)
);

CREATE INDEX idx_serializd_imported_shows_account_hash ON serializd_imported_shows (account_hash);
