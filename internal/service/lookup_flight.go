package service

import (
	"context"
	"fmt"

	"github.com/alireza/tvtime2serializd/internal/repository"
)

type lookupFlightResult struct {
	tmdbID *int
}

func lookupFlightKey(input repository.TMDBLookupInput) string {
	if input.TVDBID != nil {
		return fmt.Sprintf("tvdb:%d", *input.TVDBID)
	}
	imdb := ""
	if input.IMDBID != nil {
		imdb = *input.IMDBID
	}
	return fmt.Sprintf("imdb:%s:title:%s", imdb, input.Title)
}

func (s *ShowLookupService) resolveTMDBIDFlight(ctx context.Context, input repository.TMDBLookupInput) (*int, error) {
	key := lookupFlightKey(input)
	v, err, _ := s.flight.Do(key, func() (any, error) {
		if input.TVDBID != nil {
			tvdbID := *input.TVDBID
			if tmdbID, found, err := s.cache.Get(ctx, tvdbID); err == nil && found && tmdbID != nil {
				return lookupFlightResult{tmdbID: tmdbID}, nil
			}
			existing, err := s.shows.GetByTVDBID(ctx, tvdbID)
			if err == nil && existing.TMDBID != nil {
				s.bestEffortCacheSet(ctx, tvdbID, existing.TMDBID)
				return lookupFlightResult{tmdbID: existing.TMDBID}, nil
			}
		}
		tmdbID, err := s.fetchTMDBID(ctx, input)
		return lookupFlightResult{tmdbID: tmdbID}, err
	})
	if err != nil {
		return nil, err
	}
	result, ok := v.(lookupFlightResult)
	if !ok {
		return nil, fmt.Errorf("lookup flight: unexpected result type %T", v)
	}
	return result.tmdbID, nil
}
