package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/alireza/tvtime2serializd/internal/account"
	"github.com/alireza/tvtime2serializd/internal/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SerializdToken struct {
	ID         uuid.UUID
	EmailHash  string
	JWTToken   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	LastUsedAt *time.Time
}

type SerializdTokenRepository struct {
	pool   *pgxpool.Pool
	cipher *crypto.Cipher
}

func NewSerializdTokenRepository(pool *pgxpool.Pool, cipher *crypto.Cipher) *SerializdTokenRepository {
	return &SerializdTokenRepository{pool: pool, cipher: cipher}
}

func (r *SerializdTokenRepository) Upsert(ctx context.Context, email, jwtToken string) (uuid.UUID, error) {
	emailHash := account.Hash(email)

	jwtEnc, err := r.cipher.Encrypt(jwtToken)
	if err != nil {
		return uuid.Nil, fmt.Errorf("encrypt jwt_token: %w", err)
	}

	var id uuid.UUID
	err = r.pool.QueryRow(ctx, `
		INSERT INTO serializd_tokens (email, jwt_token_enc)
		VALUES ($1, $2)
		ON CONFLICT (email) DO UPDATE SET
			jwt_token_enc = EXCLUDED.jwt_token_enc,
			updated_at = NOW()
		RETURNING id
	`, emailHash, jwtEnc).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert serializd token: %w", err)
	}

	return id, nil
}

func (r *SerializdTokenRepository) GetByID(ctx context.Context, id uuid.UUID) (*SerializdToken, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, email, jwt_token_enc, created_at, updated_at, last_used_at
		FROM serializd_tokens
		WHERE id = $1
	`, id)

	return r.scanToken(row)
}

func (r *SerializdTokenRepository) TouchLastUsed(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE serializd_tokens SET last_used_at = NOW() WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("touch serializd last_used_at: %w", err)
	}
	return nil
}

func (r *SerializdTokenRepository) scanToken(row pgx.Row) (*SerializdToken, error) {
	var (
		token  SerializdToken
		jwtEnc []byte
	)

	err := row.Scan(
		&token.ID,
		&token.EmailHash,
		&jwtEnc,
		&token.CreatedAt,
		&token.UpdatedAt,
		&token.LastUsedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan serializd token: %w", err)
	}

	token.JWTToken, err = r.cipher.Decrypt(jwtEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt jwt_token: %w", err)
	}

	return &token, nil
}
