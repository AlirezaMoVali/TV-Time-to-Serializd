package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UnresolvedShow struct {
	ID     int64
	TVDBID *int64
	IMDBID *string
	Title  string
	Year   *int
}

type UnresolvedShowRepository struct {
	pool *pgxpool.Pool
}

func NewUnresolvedShowRepository(pool *pgxpool.Pool) *UnresolvedShowRepository {
	return &UnresolvedShowRepository{pool: pool}
}

func (r *UnresolvedShowRepository) ListUnresolvedTVDBIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.tvdb_id
		FROM unresolved_shows u
		WHERE u.tvdb_id IS NOT NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM shows s
		    WHERE s.tvdb_id = u.tvdb_id AND s.tmdb_id IS NOT NULL
		  )
	`)
	if err != nil {
		return nil, fmt.Errorf("list unresolved tvdb ids: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan unresolved tvdb id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list unresolved tvdb ids: %w", err)
	}
	return ids, nil
}

func (r *UnresolvedShowRepository) Record(ctx context.Context, tvdbID *int64, imdbID *string, title string, year *int) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO unresolved_shows (tvdb_id, imdb_id, title, year)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tvdb_id) WHERE tvdb_id IS NOT NULL DO UPDATE SET
			imdb_id = COALESCE(EXCLUDED.imdb_id, unresolved_shows.imdb_id),
			title = EXCLUDED.title,
			year = COALESCE(EXCLUDED.year, unresolved_shows.year),
			updated_at = NOW()
	`, tvdbID, imdbID, title, year)
	if err != nil {
		return fmt.Errorf("record unresolved show: %w", err)
	}
	return nil
}
