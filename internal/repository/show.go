package repository

import (
	"context"
	"fmt"

	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TMDBLookupInput contains show identifiers used to resolve a TMDB ID.
type TMDBLookupInput struct {
	TVDBID *int64
	IMDBID *string
	Title  string
	Year   *int
}

// TMDBResolver resolves TMDB TV series IDs from show metadata.
type TMDBResolver interface {
	ResolveTMDBID(ctx context.Context, input TMDBLookupInput) (*int, error)
}

// ShowEnsurer stores show metadata and resolves TMDB IDs via Redis, Postgres, and the resolver chain.
type ShowEnsurer interface {
	EnsureShow(ctx context.Context, show tvtime.ExportShow, tvtimeSeriesID int64) (showID int64, tmdbID *int, err error)
}

type ShowCatalog struct {
	pool *pgxpool.Pool
}

func NewShowCatalog(pool *pgxpool.Pool) *ShowCatalog {
	return &ShowCatalog{pool: pool}
}

type ShowRecord struct {
	ID             int64
	TVDBID         *int64
	TVTimeSeriesID *int64
	TMDBID         *int
	Year           *int
	IMDBID         *string
	Title          string
}

// TVDBTMDBMapping is a resolved TVDB to TMDB ID pair from the show catalog.
type TVDBTMDBMapping struct {
	TVDBID int64
	TMDBID int
}

func (r *ShowCatalog) Upsert(ctx context.Context, show tvtime.ExportShow, tvtimeSeriesID int64) (int64, error) {
	return r.UpsertWithoutResolver(ctx, show, tvtimeSeriesID)
}

func (r *ShowCatalog) UpsertTx(ctx context.Context, tx pgx.Tx, show tvtime.ExportShow, tvtimeSeriesID int64) (int64, error) {
	params, err := r.prepareUpsert(show, tvtimeSeriesID)
	if err != nil {
		return 0, err
	}
	return r.upsert(ctx, tx, params)
}

// UpsertWithoutResolver stores show metadata without running TMDB resolution.
func (r *ShowCatalog) UpsertWithoutResolver(ctx context.Context, show tvtime.ExportShow, tvtimeSeriesID int64) (int64, error) {
	params, err := r.prepareUpsert(show, tvtimeSeriesID)
	if err != nil {
		return 0, err
	}
	return r.upsert(ctx, r.pool, params)
}

type showUpsertParams struct {
	tvdbID         *int64
	tvtimeSeriesID int64
	imdbID         *string
	title          string
	year           *int
	tmdbID         *int
}

func (r *ShowCatalog) prepareUpsert(show tvtime.ExportShow, tvtimeSeriesID int64) (showUpsertParams, error) {
	params := showUpsertParams{
		tvtimeSeriesID: tvtimeSeriesID,
		imdbID:         show.ID.IMDB,
	}
	if show.ID.TVDB != nil {
		params.tvdbID = show.ID.TVDB
	}
	if show.Title != nil {
		params.title = *show.Title
	}
	params.year = show.Year
	return params, nil
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *ShowCatalog) upsert(ctx context.Context, db queryRower, params showUpsertParams) (int64, error) {
	var id int64
	if params.tvdbID != nil {
		err := db.QueryRow(ctx, `
			INSERT INTO shows (tvdb_id, tvtime_series_id, imdb_id, title, year, tmdb_id)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (tvdb_id) DO UPDATE SET
				tvtime_series_id = COALESCE(EXCLUDED.tvtime_series_id, shows.tvtime_series_id),
				imdb_id = COALESCE(EXCLUDED.imdb_id, shows.imdb_id),
				title = EXCLUDED.title,
				year = COALESCE(EXCLUDED.year, shows.year),
				tmdb_id = COALESCE(shows.tmdb_id, EXCLUDED.tmdb_id),
				updated_at = NOW()
			RETURNING id
		`, params.tvdbID, params.tvtimeSeriesID, params.imdbID, params.title, params.year, params.tmdbID).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("upsert show: %w", err)
		}
		return id, nil
	}

	err := db.QueryRow(ctx, `
		INSERT INTO shows (tvdb_id, tvtime_series_id, imdb_id, title, year, tmdb_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tvtime_series_id) DO UPDATE SET
			tvdb_id = COALESCE(EXCLUDED.tvdb_id, shows.tvdb_id),
			imdb_id = COALESCE(EXCLUDED.imdb_id, shows.imdb_id),
			title = EXCLUDED.title,
			year = COALESCE(EXCLUDED.year, shows.year),
			tmdb_id = COALESCE(shows.tmdb_id, EXCLUDED.tmdb_id),
			updated_at = NOW()
		RETURNING id
	`, params.tvdbID, params.tvtimeSeriesID, params.imdbID, params.title, params.year, params.tmdbID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert show: %w", err)
	}
	return id, nil
}

func (r *ShowCatalog) SetTMDBID(ctx context.Context, showID int64, tmdbID int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE shows SET tmdb_id = $2, updated_at = NOW() WHERE id = $1
	`, showID, tmdbID)
	if err != nil {
		return fmt.Errorf("set tmdb id: %w", err)
	}
	return nil
}

func (r *ShowCatalog) ListTVDBTMDBMappings(ctx context.Context) ([]TVDBTMDBMapping, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tvdb_id, tmdb_id
		FROM shows
		WHERE tvdb_id IS NOT NULL AND tmdb_id IS NOT NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("list tvdb tmdb mappings: %w", err)
	}
	defer rows.Close()

	var mappings []TVDBTMDBMapping
	for rows.Next() {
		var m TVDBTMDBMapping
		if err := rows.Scan(&m.TVDBID, &m.TMDBID); err != nil {
			return nil, fmt.Errorf("scan tvdb tmdb mapping: %w", err)
		}
		mappings = append(mappings, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tvdb tmdb mappings: %w", err)
	}
	return mappings, nil
}

func (r *ShowCatalog) GetByTVDBID(ctx context.Context, tvdbID int64) (*ShowRecord, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tvdb_id, tvtime_series_id, tmdb_id, year, imdb_id, title
		FROM shows WHERE tvdb_id = $1
	`, tvdbID)
	return scanShow(row)
}

func scanShow(row pgx.Row) (*ShowRecord, error) {
	var rec ShowRecord
	err := row.Scan(&rec.ID, &rec.TVDBID, &rec.TVTimeSeriesID, &rec.TMDBID, &rec.Year, &rec.IMDBID, &rec.Title)
	if err != nil {
		return nil, fmt.Errorf("scan show: %w", err)
	}
	return &rec, nil
}
