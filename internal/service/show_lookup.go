package service

import (
	"context"
	"errors"

	"github.com/alireza/tvtime2serializd/internal/applog"
	"github.com/alireza/tvtime2serializd/internal/cache"
	"github.com/alireza/tvtime2serializd/internal/lookupdebug"
	"github.com/alireza/tvtime2serializd/internal/repository"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/singleflight"
)

var _ repository.ShowEnsurer = (*ShowLookupService)(nil)

type tmdbCache interface {
	Get(ctx context.Context, tvdbID int64) (*int, bool, error)
	Set(ctx context.Context, tvdbID int64, tmdbID *int) error
}

type showStore interface {
	UpsertWithoutResolver(ctx context.Context, show tvtime.ExportShow, tvtimeSeriesID int64) (int64, error)
	GetByTVDBID(ctx context.Context, tvdbID int64) (*repository.ShowRecord, error)
	SetTMDBID(ctx context.Context, showID int64, tmdbID int) error
}

type unresolvedStore interface {
	Record(ctx context.Context, tvdbID *int64, imdbID *string, title string, year *int) error
}

// ShowLookupDeps groups dependencies for ShowLookupService construction.
type ShowLookupDeps struct {
	TMDBCache  *cache.TMDBCache
	Shows      *repository.ShowCatalog
	Unresolved *repository.UnresolvedShowRepository
	Resolver   repository.TMDBResolver
}

// ShowLookupService resolves TMDB IDs using Redis, Postgres, then the resolver chain.
type ShowLookupService struct {
	cache      tmdbCache
	shows      showStore
	unresolved unresolvedStore
	resolver   repository.TMDBResolver
	flight     singleflight.Group
}

func NewShowLookupService(deps ShowLookupDeps) *ShowLookupService {
	return &ShowLookupService{
		cache:      deps.TMDBCache,
		shows:      deps.Shows,
		unresolved: deps.Unresolved,
		resolver:   deps.Resolver,
	}
}

// ResolveTMDBID looks up a TMDB ID only: Redis, then Postgres, then external resolver.
// It does not create or update show records.
func (s *ShowLookupService) ResolveTMDBID(ctx context.Context, input repository.TMDBLookupInput) (*int, error) {
	return s.lookupTMDBID(ctx, input)
}

// EnsureShow stores show metadata and resolves a TMDB ID when possible.
func (s *ShowLookupService) EnsureShow(ctx context.Context, show tvtime.ExportShow, tvtimeSeriesID int64) (int64, *int, error) {
	input := lookupInputFromShow(show)

	tmdbID, err := s.lookupTMDBID(ctx, input)
	if err != nil {
		return 0, nil, err
	}

	if input.TVDBID != nil {
		showID, err := s.ensureShowRecord(ctx, show, tvtimeSeriesID, *input.TVDBID)
		if err != nil {
			return 0, nil, err
		}
		if tmdbID != nil {
			if err := s.persistTMDBOnShow(ctx, showID, *input.TVDBID, *tmdbID); err != nil {
				return showID, tmdbID, err
			}
		}
		return showID, tmdbID, nil
	}

	showID, err := s.shows.UpsertWithoutResolver(ctx, show, tvtimeSeriesID)
	if err != nil {
		return 0, nil, err
	}
	if tmdbID != nil {
		if err := s.shows.SetTMDBID(ctx, showID, *tmdbID); err != nil {
			return showID, tmdbID, err
		}
		return showID, tmdbID, nil
	}
	s.bestEffortRecordUnresolved(ctx, input)
	return showID, tmdbID, nil
}

func (s *ShowLookupService) bestEffortCacheSet(ctx context.Context, tvdbID int64, tmdbID *int) {
	applog.LogBestEffort(
		s.cache.Set(ctx, tvdbID, tmdbID),
		"tmdb cache set",
		"tvdb_id", tvdbID,
	)
}

func (s *ShowLookupService) bestEffortRecordUnresolved(ctx context.Context, input repository.TMDBLookupInput) {
	applog.LogBestEffort(
		s.unresolved.Record(ctx, input.TVDBID, input.IMDBID, input.Title, input.Year),
		"record unresolved show",
		"title", input.Title,
	)
}

func (s *ShowLookupService) lookupTMDBID(ctx context.Context, input repository.TMDBLookupInput) (*int, error) {
	if input.TVDBID != nil {
		tvdbID := *input.TVDBID

		tmdbID, found, err := s.cache.Get(ctx, tvdbID)
		if err != nil {
			lookupdebug.Printf("tmdb redis cache tvdb=%d: %v", tvdbID, err)
		}
		if err == nil && found && tmdbID != nil {
			lookupdebug.Printf("tmdb cache hit tvdb=%d -> %d", tvdbID, *tmdbID)
			return tmdbID, nil
		}

		existing, err := s.shows.GetByTVDBID(ctx, tvdbID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err == nil && existing.TMDBID != nil {
			s.bestEffortCacheSet(ctx, tvdbID, existing.TMDBID)
			lookupdebug.Printf("tmdb postgres hit tvdb=%d -> %d", tvdbID, *existing.TMDBID)
			return existing.TMDBID, nil
		}
	}

	return s.resolveTMDBIDFlight(ctx, input)
}

func (s *ShowLookupService) fetchTMDBID(ctx context.Context, input repository.TMDBLookupInput) (*int, error) {
	if s.resolver == nil {
		s.bestEffortRecordUnresolved(ctx, input)
		return nil, nil
	}

	tmdbID, err := s.resolver.ResolveTMDBID(ctx, input)
	if err != nil {
		return nil, err
	}
	if tmdbID != nil {
		if input.TVDBID != nil {
			s.bestEffortCacheSet(ctx, *input.TVDBID, tmdbID)
			lookupdebug.Printf("tmdb resolved tvdb=%d -> %d title=%q", *input.TVDBID, *tmdbID, input.Title)
			return tmdbID, nil
		}
		lookupdebug.Printf("tmdb resolved title=%q -> %d", input.Title, *tmdbID)
		return tmdbID, nil
	}

	s.bestEffortRecordUnresolved(ctx, input)
	if input.TVDBID != nil {
		lookupdebug.Printf("tmdb not resolved tvdb=%d title=%q", *input.TVDBID, input.Title)
		return nil, nil
	}
	lookupdebug.Printf("tmdb not resolved title=%q", input.Title)
	return nil, nil
}

func (s *ShowLookupService) ensureShowRecord(ctx context.Context, show tvtime.ExportShow, tvtimeSeriesID, tvdbID int64) (int64, error) {
	existing, err := s.shows.GetByTVDBID(ctx, tvdbID)
	if err == nil {
		return existing.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	return s.shows.UpsertWithoutResolver(ctx, show, tvtimeSeriesID)
}

func (s *ShowLookupService) persistTMDBOnShow(ctx context.Context, showID int64, tvdbID int64, tmdbID int) error {
	existing, err := s.shows.GetByTVDBID(ctx, tvdbID)
	if err == nil && existing.TMDBID != nil && *existing.TMDBID == tmdbID {
		return nil
	}
	return s.shows.SetTMDBID(ctx, showID, tmdbID)
}

func lookupInputFromShow(show tvtime.ExportShow) repository.TMDBLookupInput {
	title := ""
	if show.Title != nil {
		title = *show.Title
	}
	return repository.TMDBLookupInput{
		TVDBID: show.ID.TVDB,
		IMDBID: show.ID.IMDB,
		Title:  title,
		Year:   show.Year,
	}
}

func strPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
