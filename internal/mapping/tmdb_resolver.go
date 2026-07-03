package mapping

import (
	"context"

	"github.com/alireza/tvtime2serializd/internal/lookupdebug"
	"github.com/alireza/tvtime2serializd/internal/repository"
	"github.com/alireza/tvtime2serializd/internal/tmdb"
	"github.com/alireza/tvtime2serializd/internal/wikidata"
)

var _ repository.TMDBResolver = (*TMDBResolver)(nil)

type wikidataLookup interface {
	TMDBIDByTVDB(ctx context.Context, tvdbID int64) (*int, error)
	TMDBIDByIMDB(ctx context.Context, imdbID string) (*int, error)
}

type tmdbAPILookup interface {
	TMDBIDByTVDB(ctx context.Context, tvdbID int64) (*int, error)
}

// TMDBResolverDeps groups dependencies for TMDBResolver construction.
type TMDBResolverDeps struct {
	Wikidata *wikidata.Client
	TMDB     *tmdb.Client
}

// TMDBResolver resolves TMDB IDs using TVDB Wikidata, TMDB API, then IMDB Wikidata.
type TMDBResolver struct {
	wikidata wikidataLookup
	tmdb     tmdbAPILookup
}

func NewTMDBResolver(deps TMDBResolverDeps) *TMDBResolver {
	return &TMDBResolver{wikidata: deps.Wikidata, tmdb: deps.TMDB}
}

func (r *TMDBResolver) ResolveTMDBID(ctx context.Context, input repository.TMDBLookupInput) (*int, error) {
	if input.TVDBID != nil {
		tmdbID, err := r.wikidata.TMDBIDByTVDB(ctx, *input.TVDBID)
		if err != nil {
			lookupdebug.Printf("wikidata tvdb lookup failed tvdb=%d: %v", *input.TVDBID, err)
		} else if tmdbID != nil {
			lookupdebug.Printf("tmdb resolved via wikidata tvdb=%d -> %d", *input.TVDBID, *tmdbID)
			return tmdbID, nil
		}

		if r.tmdb != nil {
			tmdbID, err := r.tmdb.TMDBIDByTVDB(ctx, *input.TVDBID)
			if err != nil {
				lookupdebug.Printf("tmdb api tvdb lookup failed tvdb=%d: %v", *input.TVDBID, err)
			} else if tmdbID != nil {
				lookupdebug.Printf("tmdb resolved via tmdb api tvdb=%d -> %d", *input.TVDBID, *tmdbID)
				return tmdbID, nil
			}
		} else {
			lookupdebug.Printf("tmdb api skipped tvdb=%d: client not configured", *input.TVDBID)
		}
	}

	if input.IMDBID != nil && *input.IMDBID != "" {
		tmdbID, err := r.wikidata.TMDBIDByIMDB(ctx, *input.IMDBID)
		if err != nil {
			lookupdebug.Printf("wikidata imdb lookup failed imdb=%s: %v", *input.IMDBID, err)
		} else if tmdbID != nil {
			lookupdebug.Printf("tmdb resolved via wikidata imdb=%s -> %d", *input.IMDBID, *tmdbID)
			return tmdbID, nil
		}
	}

	return nil, nil
}
