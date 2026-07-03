package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SerializdImportedRepository struct {
	pool *pgxpool.Pool
}

func NewSerializdImportedRepository(pool *pgxpool.Pool) *SerializdImportedRepository {
	return &SerializdImportedRepository{pool: pool}
}

func (r *SerializdImportedRepository) Add(ctx context.Context, accountHash string, tmdbID int) error {
	if accountHash == "" || tmdbID <= 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO serializd_imported_shows (account_hash, tmdb_id)
		VALUES ($1, $2)
		ON CONFLICT (account_hash, tmdb_id) DO NOTHING
	`, accountHash, tmdbID)
	if err != nil {
		return fmt.Errorf("record serializd imported show: %w", err)
	}
	return nil
}

func (r *SerializdImportedRepository) All(ctx context.Context, accountHash string) (map[int]struct{}, error) {
	if accountHash == "" {
		return map[int]struct{}{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT tmdb_id FROM serializd_imported_shows WHERE account_hash = $1
	`, accountHash)
	if err != nil {
		return nil, fmt.Errorf("list serializd imported shows: %w", err)
	}
	defer rows.Close()

	out := make(map[int]struct{})
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan serializd imported show: %w", err)
		}
		if id > 0 {
			out[id] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list serializd imported shows: %w", err)
	}
	return out, nil
}
