package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/alireza/tvtime2serializd/internal/account"
	"github.com/alireza/tvtime2serializd/internal/crypto"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Token struct {
	ID              uuid.UUID
	TVTimeUserID    int64
	EmailHash       string
	JWTToken        string
	JWTRefreshToken string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastUsedAt      *time.Time
}

type TokenRepository struct {
	pool   *pgxpool.Pool
	cipher *crypto.Cipher
}

func NewTokenRepository(pool *pgxpool.Pool, cipher *crypto.Cipher) *TokenRepository {
	return &TokenRepository{pool: pool, cipher: cipher}
}

func (r *TokenRepository) Upsert(ctx context.Context, email string, tokens *tvtime.Tokens) (uuid.UUID, error) {
	emailHash := account.Hash(email)

	jwtEnc, err := r.cipher.Encrypt(tokens.JWTToken)
	if err != nil {
		return uuid.Nil, fmt.Errorf("encrypt jwt_token: %w", err)
	}

	refreshEnc, err := r.cipher.Encrypt(tokens.JWTRefreshToken)
	if err != nil {
		return uuid.Nil, fmt.Errorf("encrypt jwt_refresh_token: %w", err)
	}

	var id uuid.UUID
	err = r.pool.QueryRow(ctx, `
		INSERT INTO tokens (tvtime_user_id, email, jwt_token_enc, jwt_refresh_token_enc)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tvtime_user_id) DO UPDATE SET
			email = EXCLUDED.email,
			jwt_token_enc = EXCLUDED.jwt_token_enc,
			jwt_refresh_token_enc = EXCLUDED.jwt_refresh_token_enc,
			updated_at = NOW()
		RETURNING id
	`, tokens.UserID, emailHash, jwtEnc, refreshEnc).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert token: %w", err)
	}

	return id, nil
}

func (r *TokenRepository) GetByID(ctx context.Context, id uuid.UUID) (*Token, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tvtime_user_id, email, jwt_token_enc, jwt_refresh_token_enc,
		       created_at, updated_at, last_used_at
		FROM tokens
		WHERE id = $1
	`, id)

	return r.scanToken(row)
}

func (r *TokenRepository) GetByTVTimeUserID(ctx context.Context, tvtimeUserID int64) (*Token, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tvtime_user_id, email, jwt_token_enc, jwt_refresh_token_enc,
		       created_at, updated_at, last_used_at
		FROM tokens
		WHERE tvtime_user_id = $1
	`, tvtimeUserID)

	return r.scanToken(row)
}

func (r *TokenRepository) TouchLastUsed(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tokens SET last_used_at = NOW() WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("touch last_used_at: %w", err)
	}
	return nil
}

func (r *TokenRepository) scanToken(row pgx.Row) (*Token, error) {
	var (
		token      Token
		jwtEnc     []byte
		refreshEnc []byte
	)

	err := row.Scan(
		&token.ID,
		&token.TVTimeUserID,
		&token.EmailHash,
		&jwtEnc,
		&refreshEnc,
		&token.CreatedAt,
		&token.UpdatedAt,
		&token.LastUsedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan token: %w", err)
	}

	token.JWTToken, err = r.cipher.Decrypt(jwtEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt jwt_token: %w", err)
	}

	token.JWTRefreshToken, err = r.cipher.Decrypt(refreshEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt jwt_refresh_token: %w", err)
	}

	return &token, nil
}

func (t *Token) TVTimeTokens() *tvtime.Tokens {
	return &tvtime.Tokens{
		UserID:          t.TVTimeUserID,
		JWTToken:        t.JWTToken,
		JWTRefreshToken: t.JWTRefreshToken,
	}
}
