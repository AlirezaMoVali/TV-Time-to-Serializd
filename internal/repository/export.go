package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExportStatus string

const (
	ExportPending   ExportStatus = "pending"
	ExportRunning   ExportStatus = "running"
	ExportCompleted ExportStatus = "completed"
	ExportFailed    ExportStatus = "failed"
)

type UserExport struct {
	ID              uuid.UUID
	TokenID         uuid.UUID
	Status          ExportStatus
	OutputFormat    tvtime.OutputFormat
	ShowCount       int
	WatchedEpisodes int
	DurationMs      *int64
	ErrorMessage    *string
	StartedAt       time.Time
	CompletedAt     *time.Time
}

type ExportRepository struct {
	pool *pgxpool.Pool
}

func NewExportRepository(pool *pgxpool.Pool) *ExportRepository {
	return &ExportRepository{pool: pool}
}

func (r *ExportRepository) Create(ctx context.Context, tokenID uuid.UUID, format tvtime.OutputFormat) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO user_exports (token_id, status, output_format)
		VALUES ($1, 'pending', $2)
		RETURNING id
	`, tokenID, format).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create export: %w", err)
	}
	return id, nil
}

func (r *ExportRepository) MarkRunning(ctx context.Context, exportID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_exports SET status = 'running', started_at = NOW() WHERE id = $1
	`, exportID)
	return err
}

func (r *ExportRepository) MarkCompleted(ctx context.Context, exportID uuid.UUID, showCount, watchedEpisodes int, durationMs int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_exports
		SET status = 'completed',
		    show_count = $2,
		    watched_episodes = $3,
		    duration_ms = $4,
		    completed_at = NOW()
		WHERE id = $1
	`, exportID, showCount, watchedEpisodes, durationMs)
	return err
}

func (r *ExportRepository) MarkFailed(ctx context.Context, exportID uuid.UUID, message string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_exports
		SET status = 'failed', error_message = $2, completed_at = NOW()
		WHERE id = $1
	`, exportID, message)
	return err
}

func (r *ExportRepository) GetByID(ctx context.Context, exportID uuid.UUID) (*UserExport, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, token_id, status, output_format, show_count, watched_episodes, duration_ms,
		       error_message, started_at, completed_at
		FROM user_exports WHERE id = $1
	`, exportID)

	var exp UserExport
	var status, outputFormat string
	err := row.Scan(
		&exp.ID, &exp.TokenID, &status, &outputFormat, &exp.ShowCount, &exp.WatchedEpisodes,
		&exp.DurationMs, &exp.ErrorMessage, &exp.StartedAt, &exp.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get export: %w", err)
	}
	exp.Status = ExportStatus(status)
	exp.OutputFormat = tvtime.OutputFormat(outputFormat)
	return &exp, nil
}

func (r *ExportRepository) SaveShows(ctx context.Context, exportID uuid.UUID, shows []tvtime.ExportShow, ensurer ShowEnsurer, seriesIDs map[string]int64, onProgress func(done, total int, showName string)) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	total := len(shows)
	for i, show := range shows {
		seriesID := int64(0)
		if show.UUID != nil {
			seriesID = seriesIDs[*show.UUID]
		}
		if seriesID == 0 && show.ID.TVDB != nil {
			seriesID = *show.ID.TVDB
		}

		showID, _, err := ensurer.EnsureShow(ctx, show, seriesID)
		if err != nil {
			return err
		}

		var tvtimeUUID *uuid.UUID
		if show.UUID != nil {
			if parsed, err := uuid.Parse(*show.UUID); err == nil {
				tvtimeUUID = &parsed
			}
		}

		var userShowID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO user_export_shows (
				export_id, show_id, tvtime_uuid, status, is_favorite,
				show_created_at, no_episode_data
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (export_id, show_id) DO UPDATE SET
				tvtime_uuid = EXCLUDED.tvtime_uuid,
				status = EXCLUDED.status,
				is_favorite = EXCLUDED.is_favorite,
				show_created_at = EXCLUDED.show_created_at,
				no_episode_data = EXCLUDED.no_episode_data
			RETURNING id
		`, exportID, showID, tvtimeUUID, show.Status, show.IsFavorite, parseTimePtr(show.CreatedAt), show.NoEpisodeData).Scan(&userShowID)
		if err != nil {
			return fmt.Errorf("insert user_export_show: %w", err)
		}

		if err := saveSeasons(ctx, tx, userShowID, show.Seasons); err != nil {
			return err
		}

		done := i + 1
		if onProgress != nil && (done%5 == 0 || done == total) {
			name := ""
			if show.Title != nil {
				name = *show.Title
			}
			onProgress(done, total, name)
		}
	}

	return tx.Commit(ctx)
}

func saveSeasons(ctx context.Context, tx pgx.Tx, userShowID uuid.UUID, seasons []tvtime.ExportSeason) error {
	for _, season := range seasons {
		var userSeasonID uuid.UUID
		err := tx.QueryRow(ctx, `
			INSERT INTO user_export_seasons (user_show_id, season_number, is_specials)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_show_id, season_number) DO UPDATE SET is_specials = EXCLUDED.is_specials
			RETURNING id
		`, userShowID, season.Number, season.IsSpecials).Scan(&userSeasonID)
		if err != nil {
			return fmt.Errorf("insert season: %w", err)
		}

		for _, ep := range season.Episodes {
			var tvdbEpisodeID *int64
			if ep.ID.TVDB != nil {
				tvdbEpisodeID = ep.ID.TVDB
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO user_export_episodes (
					user_season_id, tvdb_episode_id, episode_number, name,
					is_special, is_watched, watched_at, rewatch_count, watched_count
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
				ON CONFLICT (user_season_id, episode_number) DO UPDATE SET
					tvdb_episode_id = EXCLUDED.tvdb_episode_id,
					name = EXCLUDED.name,
					is_special = EXCLUDED.is_special,
					is_watched = EXCLUDED.is_watched,
					watched_at = EXCLUDED.watched_at,
					rewatch_count = EXCLUDED.rewatch_count,
					watched_count = EXCLUDED.watched_count
			`, userSeasonID, tvdbEpisodeID, ep.Number, ep.Name, ep.Special, ep.IsWatched,
				parseTimePtr(ep.WatchedAt), ep.RewatchCount, ep.WatchedCount)
			if err != nil {
				return fmt.Errorf("insert episode: %w", err)
			}
		}
	}
	return nil
}

func (r *ExportRepository) LoadShows(ctx context.Context, exportID uuid.UUID) ([]tvtime.ExportShow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			ues.tvtime_uuid, s.tvdb_id, s.imdb_id, ues.show_created_at, s.title,
			ues.status, ues.is_favorite, ues.no_episode_data, ues.id
		FROM user_export_shows ues
		JOIN shows s ON s.id = ues.show_id
		WHERE ues.export_id = $1
		ORDER BY s.title
	`, exportID)
	if err != nil {
		return nil, fmt.Errorf("load export shows: %w", err)
	}
	defer rows.Close()

	var shows []tvtime.ExportShow
	userShowIDs := make([]uuid.UUID, 0)

	for rows.Next() {
		var (
			show         tvtime.ExportShow
			tvtimeUUID   *uuid.UUID
			tvdbID       *int64
			imdbID       *string
			createdAt    *time.Time
			title        string
			userShowID   uuid.UUID
		)
		if err := rows.Scan(
			&tvtimeUUID, &tvdbID, &imdbID, &createdAt, &title,
			&show.Status, &show.IsFavorite, &show.NoEpisodeData, &userShowID,
		); err != nil {
			return nil, err
		}

		if tvtimeUUID != nil {
			s := tvtimeUUID.String()
			show.UUID = &s
		}
		show.ID = tvtime.ExternalIDs{TVDB: tvdbID, IMDB: imdbID}
		show.Title = &title
		show.CreatedAt = tvtime.PtrTimeRFC3339(createdAt)
		show.Seasons = []tvtime.ExportSeason{}

		shows = append(shows, show)
		userShowIDs = append(userShowIDs, userShowID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i, userShowID := range userShowIDs {
		seasons, err := r.loadSeasons(ctx, userShowID)
		if err != nil {
			return nil, err
		}
		shows[i].Seasons = seasons
	}

	return shows, nil
}

func (r *ExportRepository) loadSeasons(ctx context.Context, userShowID uuid.UUID) ([]tvtime.ExportSeason, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, season_number, is_specials
		FROM user_export_seasons
		WHERE user_show_id = $1
		ORDER BY season_number
	`, userShowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type seasonRow struct {
		id     uuid.UUID
		season tvtime.ExportSeason
	}
	var seasonRows []seasonRow

	for rows.Next() {
		var row seasonRow
		if err := rows.Scan(&row.id, &row.season.Number, &row.season.IsSpecials); err != nil {
			return nil, err
		}
		row.season.Episodes = []tvtime.ExportEpisode{}
		seasonRows = append(seasonRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i, row := range seasonRows {
		episodes, err := r.loadEpisodes(ctx, row.id)
		if err != nil {
			return nil, err
		}
		seasonRows[i].season.Episodes = episodes
	}

	seasons := make([]tvtime.ExportSeason, len(seasonRows))
	for i, row := range seasonRows {
		seasons[i] = row.season
	}
	return seasons, nil
}

func (r *ExportRepository) loadEpisodes(ctx context.Context, userSeasonID uuid.UUID) ([]tvtime.ExportEpisode, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tvdb_episode_id, episode_number, name, is_special,
		       is_watched, watched_at, rewatch_count, watched_count
		FROM user_export_episodes
		WHERE user_season_id = $1
		ORDER BY episode_number
	`, userSeasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []tvtime.ExportEpisode
	for rows.Next() {
		var ep tvtime.ExportEpisode
		var watchedAt *time.Time
		if err := rows.Scan(
			&ep.ID.TVDB, &ep.Number, &ep.Name, &ep.Special,
			&ep.IsWatched, &watchedAt, &ep.RewatchCount, &ep.WatchedCount,
		); err != nil {
			return nil, err
		}
		ep.WatchedAt = tvtime.FormatLiberatorDateTime(watchedAt)
		episodes = append(episodes, ep)
	}
	return episodes, rows.Err()
}

func parseTimePtr(raw *string) *time.Time {
	if raw == nil || *raw == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, *raw); err == nil {
			return &t
		}
	}
	return nil
}
